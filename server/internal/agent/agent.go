// 本文件实现 S12 任务 8：AI 分身对话服务（agent.Service）。
//
// 职责：
//   - 用 eino-ext/components/model/openai 的 ChatModel（APIKey/BaseURL/Model 取自
//     config.Config，默认 DeepSeek 兼容端点）与 flow/agent/react.NewAgent 组装
//     ReAct 智能体：挂载三只直接工具（tools.go）、MaxStep=8、流式输出；
//   - 对话入口第一步登记 AI 用量（store.IncrAIUsage），预算超限映射为 ErrBudget，
//     未配置 AI_API_KEY 的进程在构造期即返回 ErrNoAPIKey（服务不启动 AI 能力）；
//   - 系统提示 = 人设（settings persona，可写）+ 内置知识（embed knowledge.md）+
//     硬性规则，纯静态内容，绝不含用户当前问题（见 prompt.go）；
//   - 请求历史只取最近 4 条（maxHistoryMessages），避免上下文无限膨胀；
//   - 对外只暴露稳定的 agent.Service 接口与 request/reply/chunk 类型，
//     Eino/react/schema 全部封在包内，HTTP 层（chat.go）与 agenttest.Fake 都
//     只依赖本接口。
//
// 本文件不实现 HTTP 路由；流式输出通过 Stream(ctx, req, onChunk) 回调交付
// 文本增量与工具调用事件，由 HTTP 层（chat.go）适配为 SSE。
package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"

	"github.com/T-DWAG/blog_build/server/internal/config"
	"github.com/T-DWAG/blog_build/server/internal/store"
)

// 常量。
const (
	// maxStep 是 ReAct 智能体的最大执行步数（每步 = 一次图节点执行）。
	// 一次「模型→工具」往返占 2 步，8 步约等于 4 轮工具调用，随后必须直接作答。
	maxStep = 8
	// maxHistoryMessages 是发给模型的最近历史消息条数（不含系统提示与当前问题）。
	maxHistoryMessages = 4
	// modelTimeout 是单次模型请求的超时时间。
	modelTimeout = 60 * time.Second
)

// 错误。
var (
	// ErrNoAPIKey 表示未配置 AI_API_KEY：进程仍可启动，但 AI 分身不可用。
	ErrNoAPIKey = errors.New("agent: AI_API_KEY 未配置，AI 分身不可用")
	// ErrBudget 表示当月 AI 预算已用尽（映射自 store.ErrBudgetExceeded）。
	ErrBudget = errors.New("agent: 本月 AI 预算已用尽")
	// ErrEmptyQuestion 表示请求缺少有效问题内容。
	ErrEmptyQuestion = errors.New("agent: 问题内容为空")
)

// Role 是对话消息的角色，与 Eino schema.Role 解耦，供 API 层直接序列化。
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Message 是 API 层可见的单条对话消息。
type Message struct {
	Role       Role   `json:"role"`
	Content    string `json:"content"`
	ToolName   string `json:"tool_name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// Request 是单轮对话请求：当前问题 + 最近对话历史。
// 服务端只取历史最后 4 条；系统提示由服务端生成，历史里的 system 消息会被忽略。
type Request struct {
	Message Message   `json:"message"`
	History []Message `json:"history,omitempty"`
}

// Reply 是单轮对话的最终回复。
type Reply struct {
	Message Message        `json:"message"`
	Usage   *store.AIUsage `json:"usage,omitempty"` // 登记后的当月用量快照
}

// Chunk 是流式输出的一个分片：
//   - Content 是文本增量（非空时才携带）；
//   - ToolName 是工具调用事件（agent 调用 search_articles/get_article/
//     list_projects 时携带，HTTP 层转成 tool 帧，见 S12-AI 任务 9）；
//   - Done 标记最后一个分片。
//
// 工具调用事件先于它触发的最终文本分片被交付，顺序与图执行一致。
type Chunk struct {
	Content  string `json:"content"`
	ToolName string `json:"tool_name,omitempty"`
	Done     bool   `json:"done"`
}

// Service 是 AI 分身对外（API 层）的稳定接口：隔离 Eino/react 实现细节，
// 便于 HTTP handler 使用，也便于 agenttest.Fake 注入替身。
type Service interface {
	// Chat 走完一轮对话并返回最终回复。
	Chat(ctx context.Context, req Request) (*Reply, error)
	// Stream 走完一轮对话，把文本增量逐个交给 onChunk（可为 nil），返回最终回复。
	Stream(ctx context.Context, req Request, onChunk func(Chunk)) (*Reply, error)
}

// 编译期断言：*Agent 实现 Service。
var _ Service = (*Agent)(nil)

// BudgetChecker 是可选的预算预检接口。实现者在 SSE 头写出前报告额度错误，
// 让 HTTP 层可以返回真实的 429；真正的原子扣减仍由 Stream/Chat 完成。
type BudgetChecker interface {
	CheckBudget(ctx context.Context) error
}

// CheckBudget 检查当前月份是否还有额度，但不占用额度。
func (a *Agent) CheckBudget(ctx context.Context) error {
	_, err := a.store.CheckAIUsage(ctx)
	if errors.Is(err, store.ErrBudgetExceeded) {
		return ErrBudget
	}
	return err
}

// Agent 是 Service 对外的生产实现：内部持有 ReAct 智能体。
// 用量登记与人设读取通过可注入函数（usageFn / personaFn）解耦，测试可替换为
// 内存替身，避免依赖真实数据库；生产默认值分别指向 store.IncrAIUsage 与
// settings persona。
type Agent struct {
	react     *react.Agent
	store     *store.Store
	usageFn   func(ctx context.Context) (*store.AIUsage, error)
	personaFn func(ctx context.Context) string
}

// New 构造 AI 分身。
//   - cfg.AIAPIKey 为空 → 返回 ErrNoAPIKey（进程可启动，AI 能力不可用）；
//   - st 不能为 nil；
//   - 内部构建三只直接工具（NewTools）与 openai ChatModel（APIKey/BaseURL/Model
//     取自 cfg），再组装 ReAct 智能体（MaxStep=8、流式、工具绑定）。
func New(ctx context.Context, cfg config.Config, st *store.Store) (*Agent, error) {
	if cfg.AIAPIKey == "" {
		return nil, ErrNoAPIKey
	}
	if st == nil {
		return nil, errors.New("agent: store is nil")
	}
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  cfg.AIAPIKey,
		BaseURL: cfg.AIBaseURL,
		Model:   cfg.AIModel,
		Timeout: modelTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("agent: new openai chat model: %w", err)
	}
	ts, err := NewTools(st)
	if err != nil {
		return nil, err
	}
	return newWithModel(ctx, st, cm, toBaseTools(ts.All()))
}

// toBaseTools 把 InvokableTool 切片转成 BaseTool 切片（ToolsNodeConfig 需要）。
func toBaseTools(in []tool.InvokableTool) []tool.BaseTool {
	out := make([]tool.BaseTool, 0, len(in))
	for _, t := range in {
		out = append(out, t)
	}
	return out
}

// toolEventCtxKey 是工具事件缓冲区在单次调用 context 里的键：缓冲区按调用
// 隔离（并发对话互不串帧），中间件从 ctx 取当前调用的缓冲区。
type toolEventCtxKey struct{}

// newWithModel 用显式注入的模型与工具组装智能体（测试注入假模型/假工具）。
// 同时注入一个工具中间件：图执行期间每次工具调用都把名字记进本次调用的
// 缓冲区（从 ctx 读取，见 toolEventCtxKey），Stream 循环按序把事件转成
// Chunk{ToolName}（react.Stream 只输出最终文本消息，工具调用过程只能通过
// 中间件观察，见 S12-AI 任务 9）。
func newWithModel(ctx context.Context, st *store.Store, cm model.ToolCallingChatModel, tools []tool.BaseTool) (*Agent, error) {
	ra, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: cm,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: tools,
			ToolCallMiddlewares: []compose.ToolMiddleware{{Invokable: func(
				next compose.InvokableToolEndpoint,
			) compose.InvokableToolEndpoint {
				return func(ctx context.Context, in *compose.ToolInput) (*compose.ToolOutput, error) {
					if b, ok := ctx.Value(toolEventCtxKey{}).(*toolEventBuffer); ok && in != nil {
						b.add(in.Name)
					}
					return next(ctx, in)
				}
			}}},
		},
		MaxStep:   maxStep,
		GraphName: "BlogAIAgent",
	})
	if err != nil {
		return nil, fmt.Errorf("agent: new react agent: %w", err)
	}
	return &Agent{
		react: ra,
		store: st,
		usageFn: func(ctx context.Context) (*store.AIUsage, error) {
			return st.IncrAIUsage(ctx, 1)
		},
		personaFn: func(ctx context.Context) string {
			return personaFromSettings(ctx, st)
		},
	}, nil
}

// Chat 执行一轮对话：登记用量 → 组装输入（系统提示+最近4条历史+当前问题）→
// ReAct 生成 → 返回最终回复。
func (a *Agent) Chat(ctx context.Context, req Request) (*Reply, error) {
	usage, err := a.prepare(ctx)
	if err != nil {
		return nil, err
	}
	msgs, err := buildInput(req, a.personaFn(ctx))
	if err != nil {
		return nil, err
	}
	out, err := a.react.Generate(ctx, msgs)
	if err != nil {
		return nil, fmt.Errorf("agent: chat: %w", err)
	}
	if out == nil {
		return nil, errors.New("agent: chat: 模型返回空回复")
	}
	return &Reply{
		Message: Message{Role: RoleAssistant, Content: out.Content},
		Usage:   usage,
	}, nil
}

// Stream 执行一轮对话并流式交付文本增量（onChunk 可为 nil，此时只收集全文）。
func (a *Agent) Stream(ctx context.Context, req Request, onChunk func(Chunk)) (*Reply, error) {
	usage, err := a.prepare(ctx)
	if err != nil {
		return nil, err
	}
	msgs, err := buildInput(req, a.personaFn(ctx))
	if err != nil {
		return nil, err
	}
	// 本次调用的工具事件缓冲区：中间件从 ctx 写入，这里在收消息时排空。
	events := &toolEventBuffer{}
	ctx = context.WithValue(ctx, toolEventCtxKey{}, events)
	sr, err := a.react.Stream(ctx, msgs)
	if err != nil {
		return nil, fmt.Errorf("agent: stream: %w", err)
	}
	defer sr.Close()

	var sb strings.Builder
	for {
		m, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("agent: stream: %w", err)
		}
		// 图执行期间积累的工具调用事件：先于本轮最终文本交付（工具一定
		// 先于其后的模型输出执行，缓冲顺序即执行顺序）。
		for _, name := range events.drain() {
			if onChunk != nil {
				onChunk(Chunk{ToolName: name})
			}
		}
		if m == nil || m.Content == "" {
			continue // 跳过工具调用等无文本中间消息
		}
		sb.WriteString(m.Content)
		if onChunk != nil {
			onChunk(Chunk{Content: m.Content})
		}
	}
	// 收尾兜底：极端情况下流已结束但事件尚未被循环排空。
	for _, name := range events.drain() {
		if onChunk != nil {
			onChunk(Chunk{ToolName: name})
		}
	}
	if onChunk != nil {
		onChunk(Chunk{Done: true})
	}
	return &Reply{
		Message: Message{Role: RoleAssistant, Content: sb.String()},
		Usage:   usage,
	}, nil
}

// toolEventBuffer 是图执行 goroutine（add）与 Stream 循环 goroutine（drain）
// 之间线程安全的工具调用名缓冲。
type toolEventBuffer struct {
	mu    sync.Mutex
	names []string
}

func (b *toolEventBuffer) add(name string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.names = append(b.names, name)
}

func (b *toolEventBuffer) drain() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := b.names
	b.names = nil
	return out
}

// prepare 是对话入口的第一步：登记一次 AI 用量（原子预算硬上限）。
// store.ErrBudgetExceeded 映射为 ErrBudget；其它登记错误原样包装返回。
func (a *Agent) prepare(ctx context.Context) (*store.AIUsage, error) {
	usage, err := a.usageFn(ctx)
	if errors.Is(err, store.ErrBudgetExceeded) {
		if usage != nil {
			return nil, fmt.Errorf("%w（本月已用 %d/%d）", ErrBudget, usage.Count, usage.Budget)
		}
		return nil, ErrBudget
	}
	if err != nil {
		return nil, fmt.Errorf("agent: 登记 AI 用量: %w", err)
	}
	return usage, nil
}

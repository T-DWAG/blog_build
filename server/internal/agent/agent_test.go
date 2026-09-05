// 本文件覆盖 S12 任务 8 的对话服务：
//   - 构造契约：未配置 AI_API_KEY → ErrNoAPIKey；
//   - 系统提示：内置知识（embed）+ 人设 + 硬性规则，且不含用户问题；
//   - 输入组装：历史只取最后 4 条、忽略历史里的 system、空问题报错；
//   - 对话往返：no-tool（模型直接作答、工具不被调用）与 tool（模型先调工具再作答）；
//   - 步数上限：模型始终要调工具时，MaxStep=8 触发 compose.ErrExceedMaxSteps；
//   - 预算硬上限：入口第一步即 ErrBudget，模型不被调用。
//
// 全部离线：用假模型（fakeModel）与假工具（fakeTool）替代真实模型/数据库，
// 不依赖 AI_API_KEY 与网络。
package agent

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/T-DWAG/blog_build/server/internal/config"
	"github.com/T-DWAG/blog_build/server/internal/store"
)

// fakeModel 是可编程的 ToolCallingChatModel 替身（不联网、不调真实 API）。
// 行为：
//   - alwaysTool：每次调用都返回工具调用（配合 max-step 测试）；
//   - toolOnce：第一次调用返回工具调用，之后（输入里已有 tool 结果）返回最终答案；
//   - 否则：始终直接返回最终答案（no-tool）。
type fakeModel struct {
	alwaysTool bool
	toolOnce   bool
	toolName   string
	finalText  string
	calls      int
}

var _ model.ToolCallingChatModel = (*fakeModel)(nil)

func (m *fakeModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (m *fakeModel) Generate(ctx context.Context, in []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.calls++
	if m.wantTool(in) {
		return toolCallMessage(m.toolName), nil
	}
	return schema.AssistantMessage(m.finalText, nil), nil
}

func (m *fakeModel) Stream(ctx context.Context, in []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.calls++
	if m.wantTool(in) {
		return schema.StreamReaderFromArray([]*schema.Message{toolCallMessage(m.toolName)}), nil
	}
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage(m.finalText, nil)}), nil
}

// wantTool 决定本次调用是否返回工具调用：
// alwaysTool 恒真；toolOnce 且输入里还没有 tool 结果消息 → 真。
func (m *fakeModel) wantTool(in []*schema.Message) bool {
	if m.alwaysTool {
		return true
	}
	if !m.toolOnce {
		return false
	}
	for _, msg := range in {
		if msg.Role == schema.Tool {
			return false
		}
	}
	return true
}

// toolCallMessage 构造一条带工具调用的 assistant 消息（调用 search_articles）。
func toolCallMessage(toolName string) *schema.Message {
	return schema.AssistantMessage("", []schema.ToolCall{{
		ID:   "call_fake_1",
		Type: "function",
		Function: schema.FunctionCall{
			Name:      toolName,
			Arguments: `{"query":"测试"}`,
		},
	}})
}

// fakeTool 是 search_articles 的替身工具：记录被调用并返回固定卡片，不连库。
func fakeTool(t *testing.T, called *bool) tool.InvokableTool {
	t.Helper()
	tl, err := utils.InferTool(toolSearchArticles, "测试工具（不连库）",
		func(ctx context.Context, in SearchArticlesArgs) (*SearchArticlesResult, error) {
			*called = true
			return &SearchArticlesResult{
				Count:    1,
				Articles: []ArticleCard{{Slug: "fake-slug", Title: "测试文章", Summary: "测试摘要"}},
			}, nil
		})
	if err != nil {
		t.Fatalf("infer fake tool: %v", err)
	}
	return tl
}

// newFakeAgent 用假模型 + 假工具组装 Agent，并把用量/人设替换为内存实现。
func newFakeAgent(t *testing.T, cm model.ToolCallingChatModel, toolCalled *bool) *Agent {
	t.Helper()
	a, err := newWithModel(context.Background(), nil, cm, []tool.BaseTool{fakeTool(t, toolCalled)})
	if err != nil {
		t.Fatalf("newWithModel: %v", err)
	}
	a.usageFn = func(ctx context.Context) (*store.AIUsage, error) {
		return &store.AIUsage{Month: "2026-09", Count: 1, Budget: 10}, nil
	}
	a.personaFn = func(ctx context.Context) string { return "测试人设" }
	return a
}

// chatReq 构造一个最小合法请求。
func chatReq(content string) Request {
	return Request{Message: Message{Role: RoleUser, Content: content}}
}

func TestNew_NoAPIKey(t *testing.T) {
	// AIAPIKey 为空 → 构造期即 ErrNoAPIKey，不依赖 store/模型。
	_, err := New(context.Background(), config.Config{}, nil)
	if !errors.Is(err, ErrNoAPIKey) {
		t.Fatalf("New with empty AIAPIKey: want ErrNoAPIKey, got %v", err)
	}
}

func TestSystemPrompt_EmbedKnowledgePersonaRules_NoQuestion(t *testing.T) {
	prompt := buildSystemPrompt("测试人设")

	// 内置知识被 embed 进系统提示（取 knowledge.md 里的稳定句子做锚点）。
	if !strings.Contains(prompt, "从气象转向代码") {
		t.Fatalf("系统提示缺少内置知识（knowledge.md 未 embed）")
	}
	// 人设与硬性规则都在。
	if !strings.Contains(prompt, "测试人设") {
		t.Fatalf("系统提示缺少人设段")
	}
	if !strings.Contains(prompt, "硬性规则") || !strings.Contains(prompt, "禁止编造") {
		t.Fatalf("系统提示缺少硬性规则段")
	}
	// 用户问题绝不进系统提示。
	if strings.Contains(prompt, "你是谁") {
		t.Fatalf("系统提示不应包含用户问题")
	}

	// 组装后的首条消息是 system 且不含问题；问题只出现在最后一条 user 消息。
	msgs, err := buildInput(Request{
		Message: Message{Role: RoleUser, Content: "你是谁？"},
		History: []Message{{Role: RoleUser, Content: "你好"}, {Role: RoleAssistant, Content: "你好呀"}},
	}, "测试人设")
	if err != nil {
		t.Fatalf("buildInput: %v", err)
	}
	if msgs[0].Role != schema.System {
		t.Fatalf("首条消息角色 = %v, want system", msgs[0].Role)
	}
	if strings.Contains(msgs[0].Content, "你是谁") {
		t.Fatalf("system 消息包含用户问题")
	}
	last := msgs[len(msgs)-1]
	if last.Role != schema.User || last.Content != "你是谁？" {
		t.Fatalf("问题应放在最后一条 user 消息，got %v %q", last.Role, last.Content)
	}
}

func TestBuildInput_HistoryLastFour(t *testing.T) {
	history := make([]Message, 0, 6)
	for i := 0; i < 6; i++ {
		history = append(history, Message{Role: RoleUser, Content: "q" + strconv.Itoa(i)})
	}
	msgs, err := buildInput(Request{
		Message: Message{Role: RoleUser, Content: "现在的问题"},
		History: history,
	}, "")
	if err != nil {
		t.Fatalf("buildInput: %v", err)
	}
	// 1 条 system + 4 条历史 + 1 条当前问题。
	if len(msgs) != 1+maxHistoryMessages+1 {
		t.Fatalf("消息数 = %d, want %d", len(msgs), 1+maxHistoryMessages+1)
	}
	if msgs[1].Content != "q2" || msgs[4].Content != "q5" {
		t.Fatalf("历史应只保留最后 4 条（q2..q5），got %q..%q", msgs[1].Content, msgs[4].Content)
	}
	if msgs[5].Content != "现在的问题" {
		t.Fatalf("最后一条应是当前问题，got %q", msgs[5].Content)
	}
}

func TestBuildInput_SystemHistorySkippedAndEmptyQuestion(t *testing.T) {
	// 历史里的 system 消息被忽略。
	msgs, err := buildInput(Request{
		Message: Message{Role: RoleUser, Content: "在吗"},
		History: []Message{
			{Role: RoleSystem, Content: "不许外传的旧系统提示"},
			{Role: RoleUser, Content: "上一句"},
			{Role: RoleAssistant, Content: "上一答"},
		},
	}, "人设")
	if err != nil {
		t.Fatalf("buildInput: %v", err)
	}
	for _, m := range msgs {
		if m.Role == schema.System && m.Content == "不许外传的旧系统提示" {
			t.Fatalf("历史中的 system 消息不应被透传")
		}
	}
	if len(msgs) != 1+2+1 {
		t.Fatalf("消息数 = %d, want 4（system 被跳过）", len(msgs))
	}

	// 空问题 → ErrEmptyQuestion。
	if _, err := buildInput(Request{Message: Message{Role: RoleUser, Content: "   "}}, "人设"); !errors.Is(err, ErrEmptyQuestion) {
		t.Fatalf("空问题: want ErrEmptyQuestion, got %v", err)
	}
}

func TestChat_NoToolRoundtrip(t *testing.T) {
	cm := &fakeModel{finalText: "你好，我是站主的分身。"}
	toolCalled := false
	a := newFakeAgent(t, cm, &toolCalled)

	reply, err := a.Chat(context.Background(), chatReq("你好"))
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if reply.Message.Role != RoleAssistant || reply.Message.Content != cm.finalText {
		t.Fatalf("回复 = %+v, want %q", reply.Message, cm.finalText)
	}
	if reply.Usage == nil || reply.Usage.Count != 1 {
		t.Fatalf("回复缺少用量快照: %+v", reply.Usage)
	}
	if toolCalled {
		t.Fatalf("no-tool 场景工具不应被调用")
	}
}

func TestChat_ToolRoundtrip(t *testing.T) {
	cm := &fakeModel{toolOnce: true, toolName: toolSearchArticles, finalText: "根据搜索，找到一篇测试文章。"}
	toolCalled := false
	a := newFakeAgent(t, cm, &toolCalled)

	reply, err := a.Chat(context.Background(), chatReq("搜索一下测试"))
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !toolCalled {
		t.Fatalf("tool 场景工具应被调用：ReAct 工具挂载失败")
	}
	if reply.Message.Content != cm.finalText {
		t.Fatalf("回复 = %q, want %q", reply.Message.Content, cm.finalText)
	}
}

func TestChat_MaxStep(t *testing.T) {
	// 模型始终要调工具 → 8 步后触发步数上限。
	cm := &fakeModel{alwaysTool: true, toolName: toolSearchArticles}
	toolCalled := false
	a := newFakeAgent(t, cm, &toolCalled)

	_, err := a.Chat(context.Background(), chatReq("一直搜索"))
	if !errors.Is(err, compose.ErrExceedMaxSteps) {
		t.Fatalf("want compose.ErrExceedMaxSteps, got %v", err)
	}
	if !toolCalled {
		t.Fatalf("max-step 过程中工具应至少被调用一次")
	}
}

func TestStream_NoToolRoundtrip(t *testing.T) {
	cm := &fakeModel{finalText: "流式回复：" + strings.Repeat("好", 3)}
	toolCalled := false
	a := newFakeAgent(t, cm, &toolCalled)

	var chunks []Chunk
	reply, err := a.Stream(context.Background(), chatReq("你好"), func(c Chunk) { chunks = append(chunks, c) })
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var joined strings.Builder
	var sawDone bool
	for _, c := range chunks {
		joined.WriteString(c.Content)
		if c.Done {
			sawDone = true
		}
	}
	if joined.String() != cm.finalText {
		t.Fatalf("流式拼装 = %q, want %q", joined.String(), cm.finalText)
	}
	if !sawDone {
		t.Fatalf("缺少 Done 分片")
	}
	if reply.Message.Content != cm.finalText {
		t.Fatalf("最终回复 = %q, want %q", reply.Message.Content, cm.finalText)
	}
}

// TestStream_ToolEventEmitted 工具往返时，Stream 先交付工具调用事件
// （Chunk{ToolName}）再交付最终文本（S12-AI 任务 9 的工具调用帧数据来源）。
func TestStream_ToolEventEmitted(t *testing.T) {
	cm := &fakeModel{toolOnce: true, toolName: toolSearchArticles, finalText: "搜索完成。"}
	toolCalled := false
	a := newFakeAgent(t, cm, &toolCalled)

	var chunks []Chunk
	_, err := a.Stream(context.Background(), chatReq("搜索一下"), func(c Chunk) { chunks = append(chunks, c) })
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if !toolCalled {
		t.Fatalf("工具应被调用")
	}

	// 顺序：工具调用事件 → 文本 → Done。
	if len(chunks) < 3 {
		t.Fatalf("分片数 = %d, want ≥3（tool+text+done），got %+v", len(chunks), chunks)
	}
	if chunks[0].ToolName != toolSearchArticles || chunks[0].Content != "" {
		t.Fatalf("首片应为工具调用事件，got %+v", chunks[0])
	}
	if chunks[1].Content != cm.finalText || chunks[1].ToolName != "" {
		t.Fatalf("次片应为最终文本，got %+v", chunks[1])
	}
	if !chunks[len(chunks)-1].Done {
		t.Fatalf("末片应为 Done，got %+v", chunks[len(chunks)-1])
	}
	// 工具调用事件不得进入最终回复文本。
	var joined strings.Builder
	for _, c := range chunks {
		joined.WriteString(c.Content)
	}
	if joined.String() != cm.finalText {
		t.Fatalf("拼装文本 = %q, want %q", joined.String(), cm.finalText)
	}
}

func TestChat_BudgetExceeded(t *testing.T) {
	cm := &fakeModel{finalText: "不该出现"}
	toolCalled := false
	a := newFakeAgent(t, cm, &toolCalled)
	// 模拟预算用尽：入口第一步即 ErrBudget。
	a.usageFn = func(ctx context.Context) (*store.AIUsage, error) {
		return &store.AIUsage{Month: "2026-09", Count: 10, Budget: 10}, store.ErrBudgetExceeded
	}

	_, err := a.Chat(context.Background(), chatReq("你好"))
	if !errors.Is(err, ErrBudget) {
		t.Fatalf("want ErrBudget, got %v", err)
	}
	if cm.calls != 0 {
		t.Fatalf("预算超限不应调用模型，实际调用 %d 次", cm.calls)
	}
}

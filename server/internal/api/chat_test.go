// 本文件覆盖 S12 任务 9 的 POST /api/ai/chat（公开 SSE 流式对话）：
//   - 流式成功：200 + text/event-stream，message/done 帧，文本增量实时 Flush；
//   - 安全分帧：data 行是单行 JSON，正文换行被转义，不产生裸行（帧边界不破）；
//   - 预算映射：agent.ErrBudget → SSE error 帧 code 429；
//   - 未配置密钥（ag == nil）→ 503，不进入 SSE；
//   - 校验：空问题 / 非法 JSON / 超限请求体 → 400 / 400 / 413；
//   - 断开取消：客户端断开（ctx 取消）后 handler 应立即退出；
//   - 透传：请求与历史原样交给 agent（fake.Calls 断言）。
//
// 全部离线：用 agenttest.Fake 与自定义阻塞替身，不连数据库、不连模型。
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/T-DWAG/blog_build/server/internal/agent"
	"github.com/T-DWAG/blog_build/server/internal/agent/agenttest"
	"github.com/T-DWAG/blog_build/server/internal/config"
	"github.com/T-DWAG/blog_build/server/internal/ratelimit"
	"github.com/T-DWAG/blog_build/server/internal/store"
)

// sseFrame 是一条解析后的 SSE 帧。
type sseFrame struct {
	event string
	data  string
}

// parseSSE 把 SSE 响应体解析为帧序列；同时校验：除 event/data 行与空行外
// 不允许出现其它裸行（裸行即说明 data 里泄漏了未转义的换行，分帧不安全）。
func parseSSE(t *testing.T, body string) []sseFrame {
	t.Helper()
	var frames []sseFrame
	var event, data string
	for _, line := range strings.Split(body, "\n") {
		switch {
		case line == "":
			if event != "" || data != "" {
				frames = append(frames, sseFrame{event: event, data: data})
				event, data = "", ""
			}
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		default:
			t.Fatalf("SSE 分帧不安全：出现裸行 %q（data 中的换行未被转义）", line)
		}
	}
	if event != "" || data != "" {
		t.Fatalf("SSE 响应未以空行结束：event=%q data=%q", event, data)
	}
	return frames
}

// chatServer 用 fake 构造一个含 AI 分身的 Server（不连 DB：/api/ai/chat 只依赖 ag）。
func chatServer(ag agent.Service) *Server {
	return New(config.Config{Addr: ":0"}, &store.Store{}, ratelimit.NewWindow(time.Minute), ag)
}

func TestChat_SSEStreamingAndSafeFraming(t *testing.T) {
	content := "第一行\n第二行\n第三行"
	f := &agenttest.Fake{ReplyContent: content}
	srv := chatServer(f)

	rec := postJSON(t, srv, "/api/ai/chat", map[string]any{
		"message": map[string]any{"content": "你是谁？"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	if !rec.Flushed {
		t.Fatal("SSE 应调用 Flush 让增量实时到达客户端")
	}

	frames := parseSSE(t, rec.Body.String())
	if len(frames) != 2 {
		t.Fatalf("帧数 = %d, want 2（message + done）; body=%q", len(frames), rec.Body.String())
	}

	// message 帧：正文里的换行在 JSON data 行内被转义，解码后原样还原。
	if frames[0].event != "message" {
		t.Fatalf("首帧 event = %q, want message", frames[0].event)
	}
	var chunk agent.Chunk
	if err := json.Unmarshal([]byte(frames[0].data), &chunk); err != nil {
		t.Fatalf("解析 message data: %v; data=%q", err, frames[0].data)
	}
	if chunk.Content != content {
		t.Fatalf("chunk.Content = %q, want %q（换行应被转义后还原）", chunk.Content, content)
	}

	// done 帧：带最终回复全文。
	if frames[1].event != "done" {
		t.Fatalf("末帧 event = %q, want done", frames[1].event)
	}
	var done struct {
		Reply agent.Reply `json:"reply"`
	}
	if err := json.Unmarshal([]byte(frames[1].data), &done); err != nil {
		t.Fatalf("解析 done data: %v; data=%q", err, frames[1].data)
	}
	if done.Reply.Message.Role != agent.RoleAssistant || done.Reply.Message.Content != content {
		t.Fatalf("done.reply = %+v, want assistant %q", done.Reply.Message, content)
	}

	// 请求原样透传给 agent。
	if len(f.Calls) != 1 || f.Calls[0].Message.Content != "你是谁？" {
		t.Fatalf("fake 收到的请求 = %+v, want 你是谁？", f.Calls)
	}
}

func TestChat_HistoryForwarded(t *testing.T) {
	f := &agenttest.Fake{ReplyContent: "收到"}
	srv := chatServer(f)

	postJSON(t, srv, "/api/ai/chat", map[string]any{
		"message": map[string]any{"role": "user", "content": "现在的问题"},
		"history": []any{
			map[string]any{"role": "user", "content": "上一句"},
			map[string]any{"role": "assistant", "content": "上一答"},
		},
	})
	if len(f.Calls) != 1 {
		t.Fatalf("fake 未被调用")
	}
	if f.Calls[0].Message.Content != "现在的问题" {
		t.Fatalf("问题 = %q", f.Calls[0].Message.Content)
	}
	if len(f.Calls[0].History) != 2 {
		t.Fatalf("history 应原样透传，got %d", len(f.Calls[0].History))
	}
}

func TestChat_BudgetMappedToSSEError(t *testing.T) {
	f := &agenttest.Fake{Err: agent.ErrBudget}
	srv := chatServer(f)

	rec := postJSON(t, srv, "/api/ai/chat", map[string]any{
		"message": map[string]any{"content": "你好"},
	})
	// 预算错误发生在流开始后（agent.Stream 入口第一步），HTTP 状态已锁定为 200，
	// 通过 SSE error 帧交付，业务码 429。
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200（SSE 已开始，错误走事件帧）", rec.Code)
	}
	frames := parseSSE(t, rec.Body.String())
	if len(frames) != 1 || frames[0].event != "error" {
		t.Fatalf("帧 = %+v, want 单条 error 帧", frames)
	}
	var ev struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(frames[0].data), &ev); err != nil {
		t.Fatalf("解析 error data: %v", err)
	}
	if ev.Code != http.StatusTooManyRequests {
		t.Fatalf("error.code = %d, want 429", ev.Code)
	}
	if !strings.Contains(ev.Msg, "预算") {
		t.Fatalf("error.msg = %q, want 含预算提示", ev.Msg)
	}
}

func TestChat_GenericErrorMappedTo500(t *testing.T) {
	f := &agenttest.Fake{Err: context.DeadlineExceeded}
	srv := chatServer(f)

	rec := postJSON(t, srv, "/api/ai/chat", map[string]any{
		"message": map[string]any{"content": "你好"},
	})
	frames := parseSSE(t, rec.Body.String())
	var ev struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal([]byte(frames[0].data), &ev); err != nil {
		t.Fatalf("解析 error data: %v", err)
	}
	if ev.Code != http.StatusInternalServerError {
		t.Fatalf("error.code = %d, want 500", ev.Code)
	}
}

// TestChat_ToolFrameEmitted 工具调用事件转成 SSE tool 帧
// （S12-AI 任务 9：{"type":"tool","name":...}），且先于文本增量。
func TestChat_ToolFrameEmitted(t *testing.T) {
	f := &agenttest.Fake{ReplyContent: "查到的答案", ToolChunks: []string{"search_articles", "get_article"}}
	srv := chatServer(f)

	rec := postJSON(t, srv, "/api/ai/chat", map[string]any{
		"message": map[string]any{"content": "搜一下"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	frames := parseSSE(t, rec.Body.String())
	if len(frames) != 4 {
		t.Fatalf("帧数 = %d, want 4（tool×2 + message + done）; body=%q", len(frames), rec.Body.String())
	}

	var payload struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	for i, want := range []string{"search_articles", "get_article"} {
		if frames[i].event != "tool" {
			t.Fatalf("帧[%d] event = %q, want tool", i, frames[i].event)
		}
		if err := json.Unmarshal([]byte(frames[i].data), &payload); err != nil {
			t.Fatalf("解析 tool data: %v", err)
		}
		if payload.Type != "tool" || payload.Name != want {
			t.Fatalf("tool 帧 = %+v, want {type:tool name:%s}", payload, want)
		}
	}
	if frames[2].event != "message" || frames[3].event != "done" {
		t.Fatalf("工具帧后应为 message + done，got %+v", frames)
	}
}

func TestChat_NoAgentReturns503(t *testing.T) {
	srv := chatServer(nil) // 等价于未配置 AI_API_KEY 的进程

	rec := postJSON(t, srv, "/api/ai/chat", map[string]any{
		"message": map[string]any{"content": "你好"},
	})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Msg == "" {
		t.Fatalf("503 应有说明文案")
	}
	// 不进入 SSE：内容类型应是 JSON 而非 event-stream。
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json（未开始流）", ct)
	}
}

func TestChat_EmptyQuestionRejected(t *testing.T) {
	srv := chatServer(&agenttest.Fake{})

	for _, body := range []map[string]any{
		{"message": map[string]any{"content": ""}},
		{"message": map[string]any{"content": "   "}},
		{}, // 缺 message
	} {
		rec := postJSON(t, srv, "/api/ai/chat", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body=%v: status = %d, want 400", body, rec.Code)
		}
	}
}

func TestChat_BadJSONRejected(t *testing.T) {
	srv := chatServer(&agenttest.Fake{})

	req := httptest.NewRequest(http.MethodPost, "/api/ai/chat", strings.NewReader(`{"message":`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestChat_TooLargeBodyRejected(t *testing.T) {
	srv := chatServer(&agenttest.Fake{})

	big := strings.Repeat("a", 70<<10)
	rec := postJSON(t, srv, "/api/ai/chat", map[string]any{
		"message": map[string]any{"content": big},
	})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

// blockAgent 是阻塞替身：Stream 进入后通知测试，然后等 ctx 取消才返回。
// 用于验证「客户端断开 → request ctx 取消 → agent 取消 → handler 退出」链路。
type blockAgent struct {
	started chan struct{}
}

var _ agent.Service = (*blockAgent)(nil)

func (b *blockAgent) Chat(ctx context.Context, _ agent.Request) (*agent.Reply, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (b *blockAgent) Stream(ctx context.Context, _ agent.Request, _ func(agent.Chunk)) (*agent.Reply, error) {
	close(b.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestChat_ClientDisconnectCancelsAgent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ag := &blockAgent{started: make(chan struct{})}
	srv := chatServer(ag)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/chat",
		strings.NewReader(`{"message":{"content":"你好"}}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		srv.Handler().ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case <-ag.started:
	case <-time.After(5 * time.Second):
		t.Fatal("agent 未被调用")
	}
	cancel() // 模拟客户端断开：request ctx 取消应传导给 agent
	select {
	case <-done:
		// handler 在断开后立刻退出（没有泄漏的 goroutine/挂起流）
	case <-time.After(5 * time.Second):
		t.Fatal("客户端断开后 handler 未退出")
	}
}

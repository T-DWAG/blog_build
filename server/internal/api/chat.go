// 本文件实现 S12 任务 9：POST /api/ai/chat 公开 SSE 流式对话。
//
// 契约要点：
//   - 公开（无需鉴权），与 /api/ai/suggestions 同级；
//   - 请求体即 agent.Request（{message:{content}, history:[...]}），历史由
//     agent 内部裁剪为最近 4 条；当前问题为空 / 非法 JSON → 400（不进入 SSE）；
//   - 未配置密钥（s.ag == nil，即 agent.New 返回 ErrNoAPIKey 的进程）→ 503；
//   - 流开始前的错误用普通 JSON（WriteErr），流开始后的一切错误（预算超限等）
//     通过 SSE error 帧交付（HTTP 状态此时已锁定为 200）；
//   - 每个文本增量立即写一个 data: 帧并 Flush，保证前端边收边渲染；
//   - 客户端断开：把 request context 透传给 agent.Stream，ctx 取消即停止转发，
//     handler 随即退出（disconnect cancellation）；
//   - 安全分帧：data 行永远是单行 JSON，正文里的换行被 json.Marshal 转义为 \n，
//     不会破坏 SSE 的帧边界；
//   - 工具进度：agent 调工具时下发 tool 事件帧（data 为
//     {"type":"tool","name":"search_articles"}），前端据此维持「正在检索…」
//     中间态（见 S12-AI 任务 9）。
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/T-DWAG/blog_build/server/internal/agent"
)

// maxChatBodyBytes 限制公开对话接口的请求体大小：对话请求很小（历史最多取
// 最后 4 条），64KB 已非常宽松，纯属防滥用。
const maxChatBodyBytes = 64 << 10

// SSE error 帧的业务码（同时是语义化的 HTTP 状态码）：
// 预算超限 → 429；未配置密钥 → 503；其它 → 500。
const (
	chatCodeBudget = http.StatusTooManyRequests
	chatCodeNoKey  = http.StatusServiceUnavailable
	chatCodeErr    = http.StatusInternalServerError
)

// Chat POST /api/ai/chat。
//
// SSE 帧协议：
//
//	event: tool      data: {"type":"tool","name":"search_articles"}
//	event: message   data: {"content":"文本增量"}
//	event: done      data: {"reply":{"message":{...},"usage":{...}}}
//	event: error     data: {"code":429,"msg":"本月 AI 预算已用尽（…）"}
//
// 顺序保证：同一轮对话里，工具调用事件先于它触发的文本增量（agent.Stream
// 保证事件与图执行顺序一致）。
func (s *Server) Chat(c *gin.Context) {
	// 防滥用：限制请求体大小（超限 → 413）。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxChatBodyBytes)

	var req agent.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			WriteErr(c.Writer, http.StatusRequestEntityTooLarge, http.StatusRequestEntityTooLarge, "request too large")
		} else {
			WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
		}
		return
	}
	if strings.TrimSpace(req.Message.Content) == "" {
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "empty question")
		return
	}
	// 未配置密钥：AI 分身不可用（进程可正常启动，见 agent.New 契约）。
	if s.ag == nil {
		WriteErr(c.Writer, chatCodeNoKey, chatCodeNoKey, "AI 分身不可用：未配置 AI_API_KEY")
		return
	}
	// 若生产 agent 支持预算预检，在写出 SSE 头之前返回真实 HTTP 429。
	// 这是优化而非安全边界：Stream 内仍会原子登记，处理预检与登记之间的竞态。
	if checker, ok := s.ag.(interface{ CheckBudget(context.Context) error }); ok {
		if err := checker.CheckBudget(c.Request.Context()); err != nil {
			if errors.Is(err, agent.ErrBudget) {
				WriteErr(c.Writer, chatCodeBudget, chatCodeBudget, err.Error())
			} else {
				WriteErr(c.Writer, chatCodeErr, chatCodeErr, "internal error")
			}
			return
		}
	}

	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // 反代（nginx）不要缓冲 SSE
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		// 极不可能：gin 的 writer 与 httptest.ResponseRecorder 都实现 Flusher。
		WriteErr(c.Writer, chatCodeErr, chatCodeErr, "streaming unsupported")
		return
	}
	flusher.Flush() // 让 200 + 头立即到达客户端，错误再以 SSE 帧交付

	ctx := c.Request.Context()
	reply, err := s.ag.Stream(ctx, req, func(ch agent.Chunk) {
		if ch.Done || ctx.Err() != nil {
			return // 收尾帧或客户端已断开：不再写，等待 Stream 随 ctx 取消退出
		}
		if ch.ToolName != "" {
			// 工具调用帧：S12-AI 任务 9 要求转成 {"type":"tool","name":...}。
			_ = sseWrite(w, flusher, "tool", map[string]any{"type": "tool", "name": ch.ToolName})
			return
		}
		_ = sseWrite(w, flusher, "message", ch)
	})
	if err != nil {
		code, msg := chatErrMsg(err)
		_ = sseWrite(w, flusher, "error", map[string]any{"code": code, "msg": msg})
		return
	}
	if reply == nil {
		_ = sseWrite(w, flusher, "error", map[string]any{"code": chatCodeErr, "msg": "empty reply"})
		return
	}
	_ = sseWrite(w, flusher, "done", map[string]any{"reply": reply})
}

// chatErrMsg 把 agent 错误映射为 SSE error 帧的 code/msg：
// agent.ErrBudget（本月预算用尽）→ 429；其它 → 500。
func chatErrMsg(err error) (int, string) {
	if errors.Is(err, agent.ErrBudget) {
		return chatCodeBudget, err.Error()
	}
	return chatCodeErr, "internal error"
}

// sseWrite 写一条 SSE 帧：event 行 + 单行 JSON data + 空行，然后 Flush。
// JSON 编码保证 data 行内没有裸换行（安全分帧）。连接已断时返回错误（调用方忽略）。
func sseWrite(w http.ResponseWriter, flusher http.Flusher, event string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

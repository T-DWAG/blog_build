// Package agenttest 提供 agent.Service 的内存替身（Fake），供 API 层测试注入，
// 避免测试依赖真实模型/数据库/网络。
package agenttest

import (
	"context"
	"sync"

	"github.com/T-DWAG/blog_build/server/internal/agent"
)

// Fake 是 agent.Service 的内存替身。
//
// 用法（API 层测试）：
//
//	f := &agenttest.Fake{ReplyContent: "固定回复"}
//	srv := api.New(cfg, st, lim, f) // 后续任务注入
//
// 可配置行为：
//   - ReplyContent：Chat/Stream 成功时返回的回复文本；为空时用默认文案；
//   - ToolChunks：非空时 Stream 先逐个下发 Chunk{ToolName}（模拟工具调用帧）；
//   - Err：非 nil 时 Chat/Stream 直接返回该错误（不产生回复）；
//   - Calls：记录收到的每个请求，便于断言。
//
// 并发安全：Calls 的追加受内部锁保护。
type Fake struct {
	mu sync.Mutex

	ReplyContent string
	ToolChunks   []string
	Err          error
	Calls        []agent.Request
}

// 编译期断言：*Fake 实现 agent.Service。
var _ agent.Service = (*Fake)(nil)

// Chat 记录请求后返回配置的回复或错误。
func (f *Fake) Chat(ctx context.Context, req agent.Request) (*agent.Reply, error) {
	f.record(req)
	if f.Err != nil {
		return nil, f.Err
	}
	return f.reply(), nil
}

// Stream 记录请求后流式交付工具调用事件（若有）与回复文本（单个文本增量 +
// Done 分片），并返回最终回复。
func (f *Fake) Stream(ctx context.Context, req agent.Request, onChunk func(agent.Chunk)) (*agent.Reply, error) {
	f.record(req)
	if f.Err != nil {
		return nil, f.Err
	}
	r := f.reply()
	if onChunk != nil {
		for _, name := range f.ToolChunks {
			onChunk(agent.Chunk{ToolName: name})
		}
		onChunk(agent.Chunk{Content: r.Message.Content})
		onChunk(agent.Chunk{Done: true})
	}
	return r, nil
}

func (f *Fake) record(req agent.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, req)
}

func (f *Fake) reply() *agent.Reply {
	content := f.ReplyContent
	if content == "" {
		content = "fake reply"
	}
	return &agent.Reply{Message: agent.Message{Role: agent.RoleAssistant, Content: content}}
}

// 本文件覆盖 agenttest.Fake：验证它实现 agent.Service，且 Chat/Stream/错误
// 三种路径行为符合 API 层测试的预期。
package agenttest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/T-DWAG/blog_build/server/internal/agent"
)

func TestFake_Chat(t *testing.T) {
	f := &Fake{ReplyContent: "你好"}
	req := agent.Request{Message: agent.Message{Role: agent.RoleUser, Content: "在吗"}}

	reply, err := f.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if reply.Message.Role != agent.RoleAssistant || reply.Message.Content != "你好" {
		t.Fatalf("reply = %+v, want assistant 你好", reply.Message)
	}
	if len(f.Calls) != 1 || f.Calls[0].Message.Content != "在吗" {
		t.Fatalf("Calls 未记录请求: %+v", f.Calls)
	}

	// 默认文案兜底。
	if reply, _ := (&Fake{}).Chat(context.Background(), req); reply.Message.Content != "fake reply" {
		t.Fatalf("默认文案 = %q", reply.Message.Content)
	}
}

func TestFake_Stream(t *testing.T) {
	f := &Fake{ReplyContent: "流式文本"}
	req := agent.Request{Message: agent.Message{Role: agent.RoleUser, Content: "你好"}}

	var chunks []agent.Chunk
	reply, err := f.Stream(context.Background(), req, func(c agent.Chunk) { chunks = append(chunks, c) })
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if len(chunks) != 2 || chunks[0].Content != "流式文本" || !chunks[1].Done {
		t.Fatalf("chunks = %+v, want [流式文本, Done]", chunks)
	}
	if reply.Message.Content != "流式文本" {
		t.Fatalf("reply = %q", reply.Message.Content)
	}
}

func TestFake_Err(t *testing.T) {
	req := agent.Request{Message: agent.Message{Role: agent.RoleUser, Content: "你好"}}
	sentinel := errors.New("boom")

	f := &Fake{Err: sentinel, ReplyContent: "不应出现"}
	if _, err := f.Chat(context.Background(), req); !errors.Is(err, sentinel) {
		t.Fatalf("Chat: want sentinel, got %v", err)
	}
	if _, err := f.Stream(context.Background(), req, nil); !errors.Is(err, sentinel) {
		t.Fatalf("Stream: want sentinel, got %v", err)
	}
	if len(f.Calls) != 2 {
		t.Fatalf("Calls = %d, want 2（错误路径也应记录请求）", len(f.Calls))
	}
}

func TestFake_RecordedHistory(t *testing.T) {
	f := &Fake{}
	req := agent.Request{
		Message: agent.Message{Role: agent.RoleUser, Content: "现在的问题"},
		History: []agent.Message{
			{Role: agent.RoleUser, Content: "上一句"},
			{Role: agent.RoleAssistant, Content: "上一答"},
		},
	}
	if _, err := f.Chat(context.Background(), req); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(f.Calls[0].History) != 2 {
		t.Fatalf("history 应原样记录，got %d", len(f.Calls[0].History))
	}
	if !strings.Contains(f.Calls[0].History[1].Content, "上一答") {
		t.Fatalf("history 内容不符: %+v", f.Calls[0].History)
	}
}

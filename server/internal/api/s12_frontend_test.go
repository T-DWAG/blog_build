package api

import (
	"os"
	"strings"
	"testing"
)

// S12：AI 分身前端契约（S12-AI 验收 TestChatHTML_UsesAPI / TestSettingsHTML_NoEmbedding）。
// chat.html 是公开站页面（frontend/）；settings.html 是管理台 embed 页面（adminui/）。

func TestChatHTML_UsesAPI(t *testing.T) {
	b, err := os.ReadFile("../../../frontend/chat.html")
	if err != nil {
		t.Fatalf("read chat.html: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, "/api/ai/chat") {
		t.Fatal("chat.html 应包含 /api/ai/chat")
	}
	if !strings.Contains(s, "/api/ai/suggestions") {
		t.Fatal("chat.html 应包含 /api/ai/suggestions")
	}
	if !strings.Contains(s, "正在检索") {
		t.Fatal("chat.html 应含「正在检索…」中间态")
	}
}

func TestSettingsHTML_NoEmbedding(t *testing.T) {
	b, err := os.ReadFile("../adminui/settings.html")
	if err != nil {
		t.Fatalf("read settings.html: %v", err)
	}
	s := string(b)
	for _, forbid := range []string{
		"EMBEDDING", "embedding", "e_provider", "e_model", "e_dim",
	} {
		if strings.Contains(s, forbid) {
			t.Fatalf("settings.html 不应再含 embedding 表单（S12 移除），却出现 %q", forbid)
		}
	}
	// 仍然保留 AI 预算与建议问题配置。
	if !strings.Contains(s, "ai_budget") || !strings.Contains(s, "suggestions") {
		t.Fatal("settings.html 应保留 ai_budget 与 suggestions 配置")
	}
}

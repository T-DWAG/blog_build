package agent

import (
	"strings"
	"testing"
)

// TestSeedKnowledge_NonEmpty 种子内容非空，且覆盖四类必备主题。
func TestSeedKnowledge_NonEmpty(t *testing.T) {
	md := SeedKnowledge()
	if strings.TrimSpace(md) == "" {
		t.Fatal("SeedKnowledge() 为空，内置知识种子不能为空")
	}
	for _, section := range []string{
		"博主身份", // 博主身份
		"技术栈",  // 技术栈
		"项目概览", // 项目概览
		"回答规则", // 回答规则（不知道 / 引用来源）
	} {
		if !strings.Contains(md, section) {
			t.Fatalf("knowledge.md 缺少必备章节 %q", section)
		}
	}
}

// TestSeedKnowledge_NotMutablePersona 种子只固化稳定事实，不固化可变人设/风格
// （persona / embedding 等由管理台 settings 维护，见 store.SeedSettings 注释）。
func TestSeedKnowledge_NotMutablePersona(t *testing.T) {
	md := SeedKnowledge()
	for _, forbid := range []string{
		"人设：", "风格：", "说话风格：", "provider: ", "model: ",
	} {
		if strings.Contains(md, forbid) {
			t.Fatalf("knowledge.md 不应固化可变配置，却出现 %q", forbid)
		}
	}
}

package api

import (
	"os"
	"strings"
	"testing"
)

// S10：公开站文章三页改为运行时 fetch 接口数据，不再是硬编码/占位内容。

func TestPostsHTML_UsesAPI(t *testing.T) {
	b, err := os.ReadFile("../../../frontend/posts.html")
	if err != nil {
		t.Fatalf("read posts.html: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, "/api/articles") {
		t.Fatal("posts.html 应包含 /api/articles")
	}
	if strings.Contains(s, "singleflight：用一次查询合并一批并发请求") {
		t.Fatal("posts.html 不应再含硬编码文章标题行")
	}
}

func TestIndexHTML_UsesAPI(t *testing.T) {
	b, err := os.ReadFile("../../../frontend/index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `id="latest-articles"`) {
		t.Fatal("index.html 应包含 #latest-articles")
	}
	if !strings.Contains(s, "/api/articles") {
		t.Fatal("index.html 应包含 /api/articles")
	}
}

func TestArticleHTML_UsesAPI(t *testing.T) {
	b, err := os.ReadFile("../../../frontend/article.html")
	if err != nil {
		t.Fatalf("read article.html: %v", err)
	}
	if !strings.Contains(string(b), "/api/articles/") {
		t.Fatal("article.html 应包含 /api/articles/")
	}
}

// 补充：项目列表页原契约未排此步，本次一并接入，同样验证不留硬编码。
func TestProjectsHTML_UsesAPI(t *testing.T) {
	b, err := os.ReadFile("../../../frontend/projects.html")
	if err != nil {
		t.Fatalf("read projects.html: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, "/api/projects") {
		t.Fatal("projects.html 应包含 /api/projects")
	}
	if strings.Contains(s, "AI Agent 智能平台</h3>") {
		t.Fatal("projects.html 不应再含硬编码项目卡片")
	}
}

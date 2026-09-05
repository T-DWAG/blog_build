package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
)

func search(t *testing.T, srv *Server, q string) ([]any, []any) {
	t.Helper()
	rec := doJSON(t, srv, http.MethodGet, "/api/search?q="+url.QueryEscape(q), "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("search status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	data, _ := env.Data.(map[string]any)
	articles, _ := data["articles"].([]any)
	projects, _ := data["projects"].([]any)
	return articles, projects
}

func TestSearch_EmptyQ(t *testing.T) {
	const u = "s7_empty"
	srv, _ := newTestServer(t, u)

	for _, q := range []string{"", "   "} {
		articles, projects := search(t, srv, q)
		if len(articles) != 0 || len(projects) != 0 {
			t.Fatalf("q=%q got articles=%d projects=%d, want 0/0", q, len(articles), len(projects))
		}
	}
}

func TestSearch_EmptyDoesNotReturnDraft(t *testing.T) {
	const u = "s7_draft"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	// 一篇草稿，标题含关键词
	createArticle(t, srv, token, map[string]any{
		"slug": "draft-keyword", "title": "draft with golang", "content_md": "x",
		"status": "draft", "tags": []string{"go"},
	})
	articles, _ := search(t, srv, "golang")
	for _, it := range articles {
		m, _ := it.(map[string]any)
		if m["slug"] == "draft-keyword" {
			t.Fatal("draft appears in search results")
		}
	}
}

func TestSearch_ArticleTitleAndBody(t *testing.T) {
	const u = "s7_article"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	// published：标题命中
	createArticle(t, srv, token, map[string]any{
		"slug": "title-hit", "title": "singleflight deep dive", "content_md": "x",
		"status": "published", "tags": []string{"go"},
	})
	// published：正文命中
	createArticle(t, srv, token, map[string]any{
		"slug": "body-hit", "title": "cache", "content_md": "we talk about singleflight here",
		"status": "published", "tags": []string{"go"},
	})

	articles, _ := search(t, srv, "singleflight")
	if len(articles) != 2 {
		t.Fatalf("got %d articles, want 2 (title+body hit)", len(articles))
	}
}

func TestSearch_ProjectNameOnly(t *testing.T) {
	const u = "s7_project"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	createProject(t, srv, token, map[string]any{
		"name": "short link", "tags": []string{"go"},
		"detail_md": "only in detail: unique-keyword-xyz",
	})

	_, projects := search(t, srv, "unique-keyword-xyz")
	if len(projects) != 0 {
		t.Fatalf("got %d projects, want 0 (detail_md 不搜)", len(projects))
	}
	// name 命中
	_, projects2 := search(t, srv, "short link")
	if len(projects2) != 1 {
		t.Fatalf("got %d projects, want 1 (name 命中)", len(projects2))
	}
}

func TestSearch_UnpublishedProjectHidden(t *testing.T) {
	const u = "s7_unpub"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	createProject(t, srv, token, map[string]any{
		"name": "secret-project-hidden", "tags": []string{"go"}, "published": false,
	})
	_, projects := search(t, srv, "secret-project-hidden")
	if len(projects) != 0 {
		t.Fatalf("got %d projects, want 0 (未上线不搜)", len(projects))
	}
}

func TestTags_Aggregate(t *testing.T) {
	const u = "s7_tags"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	// 2 篇 published 带 go
	createArticle(t, srv, token, map[string]any{
		"slug": "t1", "title": "t1", "content_md": "x", "status": "published",
		"tags": []string{"go", "cache"},
	})
	createArticle(t, srv, token, map[string]any{
		"slug": "t2", "title": "t2", "content_md": "x", "status": "published",
		"tags": []string{"go"},
	})
	// 1 个已上线项目带 go
	createProject(t, srv, token, map[string]any{
		"name": "p1", "tags": []string{"go", "redis"},
	})

	rec := doJSON(t, srv, http.MethodGet, "/api/tags", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("tags status = %d", rec.Code)
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	items, _ := env.Data.([]any)
	goCount := 0
	for _, it := range items {
		m, _ := it.(map[string]any)
		if m["tag"] == "go" {
			goCount = int(m["count"].(float64))
		}
	}
	if goCount != 3 {
		t.Fatalf("go count = %d, want 3", goCount)
	}
}

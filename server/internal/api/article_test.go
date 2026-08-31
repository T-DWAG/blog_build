package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/T-DWAG/blog_build/server/internal/auth"
)

// adminToken 签发测试用 admin token。
func adminToken(t *testing.T, username string) string {
	t.Helper()
	token, err := auth.Issue(username, "test-secret")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return token
}

// slugCounter 生成全局唯一 slug 后缀，保证测试重复跑不撞唯一约束。
var slugCounter int

func uniqueSlug(prefix string) string {
	slugCounter++
	return prefix + "-" + strconv.Itoa(slugCounter)
}

func doJSON(t *testing.T, srv *Server, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

// createArticle 通过管理接口创建文章，返回其 id。
func createArticle(t *testing.T, srv *Server, token string, a map[string]any) int64 {
	t.Helper()
	rec := doJSON(t, srv, http.MethodPost, "/api/admin/articles", token, a)
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	data, _ := env.Data.(map[string]any)
	id, _ := data["id"].(float64)
	return int64(id)
}

func TestArticle_DraftNotPublic(t *testing.T) {
	const u = "s4_draft"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)
	draftSlug := uniqueSlug("draft-only")

	createArticle(t, srv, token, map[string]any{
		"slug": draftSlug, "title": "draft", "content_md": "x",
		"status": "draft", "tags": []string{"go"},
	})

	// 公开列表无该 slug
	rec := doJSON(t, srv, http.MethodGet, "/api/articles", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	items, _ := env.Data.([]any)
	for _, it := range items {
		m, _ := it.(map[string]any)
		if m["slug"] == draftSlug {
			t.Fatal("draft appears in public list")
		}
	}
	// GET /{slug} 404
	rec2 := doJSON(t, srv, http.MethodGet, "/api/articles/"+draftSlug, "", nil)
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("slug status = %d, want 404", rec2.Code)
	}
}

func TestArticle_PublishOrder(t *testing.T) {
	const u = "s4_order"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)
	pinnedSlug := uniqueSlug("pinned")

	createArticle(t, srv, token, map[string]any{
		"slug": uniqueSlug("a"), "title": "a", "content_md": "x", "status": "published",
		"tags": []string{"go"}, "is_pinned": false,
	})
	createArticle(t, srv, token, map[string]any{
		"slug": uniqueSlug("b"), "title": "b", "content_md": "x", "status": "published",
		"tags": []string{"go"}, "is_pinned": false,
	})
	createArticle(t, srv, token, map[string]any{
		"slug": pinnedSlug, "title": "pinned", "content_md": "x", "status": "published",
		"tags": []string{"go"}, "is_pinned": true,
	})

	rec := doJSON(t, srv, http.MethodGet, "/api/articles", "", nil)
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	items, _ := env.Data.([]any)
	if len(items) < 3 {
		t.Fatalf("got %d articles, want >=3", len(items))
	}
	first, _ := items[0].(map[string]any)
	if first["slug"] != pinnedSlug {
		t.Fatalf("first slug = %v, want %s", first["slug"], pinnedSlug)
	}
}

func TestArticle_TagFilter(t *testing.T) {
	const u = "s4_tag"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)
	goSlug := uniqueSlug("go-one")

	createArticle(t, srv, token, map[string]any{
		"slug": goSlug, "title": "go one", "content_md": "x",
		"status": "published", "tags": []string{"go", "cache"},
	})
	createArticle(t, srv, token, map[string]any{
		"slug": uniqueSlug("py-one"), "title": "py one", "content_md": "x",
		"status": "published", "tags": []string{"python"},
	})

	rec := doJSON(t, srv, http.MethodGet, "/api/articles?tag=go", "", nil)
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	items, _ := env.Data.([]any)
	if len(items) != 1 {
		t.Fatalf("tag=go got %d, want 1", len(items))
	}
	m, _ := items[0].(map[string]any)
	if m["slug"] != goSlug {
		t.Fatalf("got %v, want %s", m["slug"], goSlug)
	}
}

func TestArticle_CreateRequiresTitleTags(t *testing.T) {
	const u = "s4_validate"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	// 缺 title
	rec1 := doJSON(t, srv, http.MethodPost, "/api/admin/articles", token, map[string]any{
		"slug": "no-title", "content_md": "x", "tags": []string{"go"},
	})
	if rec1.Code != http.StatusBadRequest {
		t.Fatalf("no title status = %d, want 400", rec1.Code)
	}
	// tags 为空
	rec2 := doJSON(t, srv, http.MethodPost, "/api/admin/articles", token, map[string]any{
		"slug": "no-tags", "title": "x", "content_md": "y",
	})
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("no tags status = %d, want 400", rec2.Code)
	}
}

func TestArticle_Delete(t *testing.T) {
	const u = "s4_delete"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)
	slug := uniqueSlug("to-delete")

	id := createArticle(t, srv, token, map[string]any{
		"slug": slug, "title": "t", "content_md": "x",
		"status": "published", "tags": []string{"go"},
	})

	rec := doJSON(t, srv, http.MethodDelete, "/api/admin/articles/"+strconv.FormatInt(id, 10), token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body=%s", rec.Code, rec.Body.String())
	}
	// 删除后公开 GET slug → 404
	rec2 := doJSON(t, srv, http.MethodGet, "/api/articles/"+slug, "", nil)
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("after delete status = %d, want 404", rec2.Code)
	}
	// 删除不存在的 id → 404
	rec3 := doJSON(t, srv, http.MethodDelete, "/api/admin/articles/99999999", token, nil)
	if rec3.Code != http.StatusNotFound {
		t.Fatalf("delete missing status = %d, want 404", rec3.Code)
	}
}

func TestArticle_AdminNoToken(t *testing.T) {
	const u = "s4_notoken"
	srv, _ := newTestServer(t, u)

	rec := doJSON(t, srv, http.MethodPost, "/api/admin/articles", "", map[string]any{
		"slug": "x", "title": "x", "content_md": "y", "tags": []string{"go"},
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

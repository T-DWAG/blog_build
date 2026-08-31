package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/T-DWAG/blog_build/server/internal/auth"
)

func TestAdminPage_NoCookie_RedirectsLogin(t *testing.T) {
	srv, _ := newTestServer(t, "s9_page_nocookie")

	req := httptest.NewRequest(http.MethodGet, "/admin/articles", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "/admin/login") {
		t.Fatalf("location = %q, want /admin/login", rec.Header().Get("Location"))
	}
}

func TestAdminPage_WithCookie_200(t *testing.T) {
	const u = "s9_page_cookie"
	srv, _ := newTestServer(t, u)

	// 通过登录接口拿 cookie
	token, _ := auth.Issue(u, "test-secret")
	req := httptest.NewRequest(http.MethodGet, "/admin/articles", nil)
	req.AddCookie(&http.Cookie{Name: "admin_token", Value: token})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "文章管理") {
		t.Fatalf("body missing 文章管理: %s", rec.Body.String()[:200])
	}
}

func TestPreview_NoToken(t *testing.T) {
	srv, _ := newTestServer(t, "s9_preview_notoken")

	req := httptest.NewRequest(http.MethodPost, "/api/admin/preview", strings.NewReader(`{"content_md":"# h"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestPreview_OK(t *testing.T) {
	const u = "s9_preview_ok"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	rec := doJSON(t, srv, http.MethodPost, "/api/admin/preview", token, map[string]string{
		"content_md": "# hello",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	data, _ := env.Data.(map[string]any)
	html, _ := data["html"].(string)
	if !strings.Contains(html, "<h1>") {
		t.Fatalf("html missing <h1>: %s", html)
	}
}

func TestCORS_Localhost8000(t *testing.T) {
	srv, _ := newTestServer(t, "s9_cors_local")

	req := httptest.NewRequest(http.MethodGet, "/api/articles", nil)
	req.Header.Set("Origin", "http://localhost:8000")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:8000" {
		t.Fatalf("ACAO = %q, want http://localhost:8000", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_GithubPages(t *testing.T) {
	srv, _ := newTestServer(t, "s9_cors_pages")

	req := httptest.NewRequest(http.MethodGet, "/api/articles", nil)
	req.Header.Set("Origin", "https://T-DWAG.github.io")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://T-DWAG.github.io" {
		t.Fatalf("ACAO = %q, want https://T-DWAG.github.io", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_OtherOrigin(t *testing.T) {
	srv, _ := newTestServer(t, "s9_cors_evil")

	req := httptest.NewRequest(http.MethodGet, "/api/articles", nil)
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	acao := rec.Header().Get("Access-Control-Allow-Origin")
	if acao != "" && acao != "*" {
		t.Fatalf("ACAO = %q, want empty or not *", acao)
	}
}

func TestGuestbookHTML_UsesAPI(t *testing.T) {
	b, err := os.ReadFile("../../../frontend/guestbook.html")
	if err != nil {
		t.Fatalf("read guestbook: %v", err)
	}
	if !strings.Contains(string(b), "/api/messages") {
		t.Fatal("guestbook.html 应包含 /api/messages")
	}
}

func TestSearchHTML_UsesAPI(t *testing.T) {
	b, err := os.ReadFile("../../../frontend/search.html")
	if err != nil {
		t.Fatalf("read search: %v", err)
	}
	if !strings.Contains(string(b), "/api/search") {
		t.Fatal("search.html 应包含 /api/search")
	}
}

func TestPublicNav_NoAdmin(t *testing.T) {
	b, err := os.ReadFile("../../../frontend/index.html")
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if strings.Contains(string(b), `href="/admin"`) || strings.Contains(string(b), `href="admin"`) {
		t.Fatal("index.html 不应出现 /admin 链接")
	}
}

func TestRequireAdmin_Cookie(t *testing.T) {
	const u = "s9_cookie_api"
	srv, _ := newTestServer(t, u)
	token, _ := auth.Issue(u, "test-secret")

	// cookie 也能过 API 鉴权（Bearer 优先，cookie 兜底）
	req := httptest.NewRequest(http.MethodGet, "/api/admin/articles", nil)
	req.AddCookie(&http.Cookie{Name: "admin_token", Value: token})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (cookie 鉴权)", rec.Code)
	}
}

package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/T-DWAG/blog_build/server/internal/auth"
	"github.com/T-DWAG/blog_build/server/internal/config"
	"github.com/T-DWAG/blog_build/server/internal/ratelimit"
	"github.com/T-DWAG/blog_build/server/internal/store"
)

// newTestServer 连真实 DB，Migrate + 种子一个测试管理员，返回 Server。
// 每个测试用独立用户名，避免互相污染。
func newTestServer(t *testing.T, username string) (*Server, *store.Store) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:postgres@localhost:5432/blog?sslmode=disable"
	}
	ctx := context.Background()
	st, err := store.Open(ctx, url)
	if err != nil {
		t.Fatalf("open store: %v (先执行 docker compose up -d db)", err)
	}
	t.Cleanup(func() { st.Close() })

	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 清空业务表，保证测试间数据隔离（每次测试从干净状态开始）
	cleanupDB, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatalf("open cleanup db: %v", err)
	}
	t.Cleanup(func() { cleanupDB.Close() })
	if _, err := cleanupDB.ExecContext(ctx, `TRUNCATE articles, projects, messages`); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
	// 先删同名管理员，避免跨进程残留失败计数/锁定状态影响断言
	if _, err := cleanupDB.ExecContext(ctx, `DELETE FROM admin_users WHERE username = $1`, username); err != nil {
		t.Fatalf("cleanup admin: %v", err)
	}
	hash, err := auth.Hash("correct-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := st.SeedAdmin(ctx, username, hash); err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	cfg := config.Config{Addr: ":0", JWTSecret: "test-secret"}
	srv := New(cfg, st, ratelimit.NewWindow(time.Minute), nil)
	gin.SetMode(gin.TestMode)
	return srv, st
}

func postJSON(t *testing.T, srv *Server, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func login(t *testing.T, srv *Server, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	return postJSON(t, srv, "/api/admin/login", map[string]string{
		"username": username, "password": password,
	})
}

func TestLogin_OK(t *testing.T) {
	const u = "s3_ok"
	srv, _ := newTestServer(t, u)

	rec := login(t, srv, u, "correct-password")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	token, ok := env.Data.(map[string]any)["token"].(string)
	if !ok || token == "" {
		t.Fatalf("token empty: %v", env.Data)
	}
	// failed_attempts==0 由 store 包白盒测试 TestS3_RecordLoginOK_Clears 验证
}

func TestLogin_BadPassword(t *testing.T) {
	const u = "s3_bad"
	srv, _ := newTestServer(t, u)

	rec := login(t, srv, u, "wrong-password")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	// failed_attempts==1 由 store 包白盒测试 TestS3_RecordLoginFail_Increments 验证
}

func TestLogin_LockAfter5(t *testing.T) {
	const u = "s3_lock"
	srv, _ := newTestServer(t, u)

	// 连错 5 次
	for i := 0; i < 5; i++ {
		login(t, srv, u, "wrong-password")
	}
	// 锁定期内用正确密码也应拒绝
	rec := login(t, srv, u, "correct-password")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Msg != "locked" {
		t.Fatalf("msg = %q, want locked", env.Msg)
	}
}

func TestLogin_UnknownUser(t *testing.T) {
	srv, _ := newTestServer(t, "s3_unused_user")

	rec := login(t, srv, "no-such-user", "correct-password")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Msg != "unauthorized" {
		t.Fatalf("msg = %q, want unauthorized（与错密同文案）", env.Msg)
	}
}

func TestHealth_StillOpen(t *testing.T) {
	srv, _ := newTestServer(t, "s3_health")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

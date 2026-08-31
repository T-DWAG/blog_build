package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func submitMessage(t *testing.T, srv *Server, ip string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = ip + ":12345" // 模拟远端地址
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func reviewMessage(t *testing.T, srv *Server, token string, id int64, action string) *httptest.ResponseRecorder {
	t.Helper()
	rec := doJSON(t, srv, http.MethodPut, "/api/admin/messages/"+strconv.FormatInt(id, 10), token, map[string]string{
		"action": action,
	})
	return rec
}

func TestMessage_SubmitPending(t *testing.T) {
	const u = "s6_submit"
	srv, _ := newTestServer(t, u)

	rec := submitMessage(t, srv, "10.0.0.1", map[string]string{
		"nickname": "visitor", "content": "hello",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("submit status = %d, body=%s", rec.Code, rec.Body.String())
	}
	// 公开 GET 无该条（pending 不公开）
	rec2 := doJSON(t, srv, http.MethodGet, "/api/messages", "", nil)
	var env Envelope
	_ = json.Unmarshal(rec2.Body.Bytes(), &env)
	items, _ := env.Data.([]any)
	if len(items) != 0 {
		t.Fatalf("public list = %d items, want 0 (pending hidden)", len(items))
	}
}

func TestMessage_PublicJSONNoIP(t *testing.T) {
	const u = "s6_noip"
	srv, _ := newTestServer(t, u)

	rec := submitMessage(t, srv, "10.0.0.2", map[string]string{
		"nickname": "v", "content": "hi",
	})
	body := rec.Body.String()
	if strings.Contains(body, "ip") || strings.Contains(body, "user_agent") {
		t.Fatalf("public POST response leaks ip/ua: %s", body)
	}
}

func TestMessage_ApproveVisible(t *testing.T) {
	const u = "s6_approve"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	rec := submitMessage(t, srv, "10.0.0.3", map[string]string{
		"nickname": "v", "content": "hi",
	})
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	data, _ := env.Data.(map[string]any)
	id := int64(data["id"].(float64))

	reviewMessage(t, srv, token, id, "approved")

	rec2 := doJSON(t, srv, http.MethodGet, "/api/messages", "", nil)
	_ = json.Unmarshal(rec2.Body.Bytes(), &env)
	items, _ := env.Data.([]any)
	if len(items) != 1 {
		t.Fatalf("public list = %d, want 1 after approve", len(items))
	}
}

func TestMessage_RejectHidden(t *testing.T) {
	const u = "s6_reject"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	rec := submitMessage(t, srv, "10.0.0.4", map[string]string{
		"nickname": "v", "content": "hi",
	})
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	data, _ := env.Data.(map[string]any)
	id := int64(data["id"].(float64))

	reviewMessage(t, srv, token, id, "rejected")

	rec2 := doJSON(t, srv, http.MethodGet, "/api/messages", "", nil)
	_ = json.Unmarshal(rec2.Body.Bytes(), &env)
	items, _ := env.Data.([]any)
	if len(items) != 0 {
		t.Fatalf("public list = %d, want 0 after reject", len(items))
	}
}

func TestMessage_RateLimit(t *testing.T) {
	const u = "s6_rate"
	srv, _ := newTestServer(t, u)

	body := map[string]string{"nickname": "v", "content": "hi"}
	rec1 := submitMessage(t, srv, "10.0.0.5", body)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", rec1.Code)
	}
	rec2 := submitMessage(t, srv, "10.0.0.5", body)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", rec2.Code)
	}
}

func TestMessage_NicknameRequired(t *testing.T) {
	const u = "s6_nick"
	srv, _ := newTestServer(t, u)

	rec := submitMessage(t, srv, "10.0.0.6", map[string]string{
		"nickname": "", "content": "hi",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestMessage_AdminNoToken(t *testing.T) {
	const u = "s6_notoken"
	srv, _ := newTestServer(t, u)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/messages", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestMessage_AdminSeesIP(t *testing.T) {
	const u = "s6_adminip"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	// 用 X-Forwarded-For 模拟真实访客 IP
	b, _ := json.Marshal(map[string]string{"nickname": "v", "content": "hi"})
	req := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	req.RemoteAddr = "127.0.0.1:9999"
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("submit status = %d", rec.Code)
	}

	// 管理端能看到 IP
	rec2 := doJSON(t, srv, http.MethodGet, "/api/admin/messages", token, nil)
	body := rec2.Body.String()
	if !strings.Contains(body, "203.0.113.7") {
		t.Fatalf("admin list missing ip: %s", body)
	}
}

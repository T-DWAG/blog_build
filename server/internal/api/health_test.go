package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/T-DWAG/blog_build/server/internal/config"
	"github.com/T-DWAG/blog_build/server/internal/ratelimit"
	"github.com/T-DWAG/blog_build/server/internal/store"
)

func TestHealth(t *testing.T) {
	srv := New(config.Config{Addr: ":0"}, &store.Store{}, ratelimit.NewWindow(time.Minute), nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var env Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if env.Code != 0 {
		t.Fatalf("code = %d, want 0", env.Code)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env.Data)
	}
	if data["ok"] != true {
		t.Fatalf("data.ok = %v, want true", data["ok"])
	}
}

func TestHealth_NoAuthHeader(t *testing.T) {
	srv := New(config.Config{Addr: ":0"}, &store.Store{}, ratelimit.NewWindow(time.Minute), nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHealth_StillNoAuth(t *testing.T) {
	srv := New(config.Config{Addr: ":0"}, &store.Store{}, ratelimit.NewWindow(time.Minute), nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (healthz 不应要求鉴权)", rec.Code)
	}
}

func TestListen_RequiresStore(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("New with nil store 应 panic")
		}
	}()
	New(config.Config{Addr: ":0"}, nil, ratelimit.NewWindow(time.Minute), nil)
}

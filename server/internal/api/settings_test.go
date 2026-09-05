package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSettings_GetHasSeedKeys(t *testing.T) {
	const u = "s8_keys"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	rec := doJSON(t, srv, http.MethodGet, "/api/admin/settings", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	data, _ := env.Data.(map[string]any)
	for _, k := range []string{"persona", "ai", "embedding", "suggestions"} {
		if _, ok := data[k]; !ok {
			t.Fatalf("missing key %s in settings: %v", k, data)
		}
	}
	// 不应出现 site / build_state
	if _, ok := data["site"]; ok {
		t.Fatal("site should not be in settings")
	}
	if _, ok := data["build_state"]; ok {
		t.Fatal("build_state should not be writable/returned")
	}
}

func TestSettings_PutPersona(t *testing.T) {
	const u = "s8_put"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	newPersona := map[string]any{"role": "新人格", "style": "直白"}
	rec := doJSON(t, srv, http.MethodPut, "/api/admin/settings", token, map[string]any{
		"key": "persona", "value": newPersona,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("put status = %d, body=%s", rec.Code, rec.Body.String())
	}

	rec2 := doJSON(t, srv, http.MethodGet, "/api/admin/settings", token, nil)
	var env Envelope
	_ = json.Unmarshal(rec2.Body.Bytes(), &env)
	data, _ := env.Data.(map[string]any)
	persona, _ := data["persona"].(map[string]any)
	if persona["role"] != "新人格" {
		t.Fatalf("persona.role = %v, want 新人格", persona["role"])
	}
}

func TestSettings_RejectAPIKeyInAI(t *testing.T) {
	const u = "s8_apikey"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	rec := doJSON(t, srv, http.MethodPut, "/api/admin/settings", token, map[string]any{
		"key": "ai", "value": map[string]any{"api_key": "sk-xxx", "budget_per_month": 10},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (api_key 拒绝)", rec.Code)
	}
}

func TestSettings_NoToken(t *testing.T) {
	const u = "s8_notoken"
	srv, _ := newTestServer(t, u)

	rec1 := doJSON(t, srv, http.MethodGet, "/api/admin/settings", "", nil)
	if rec1.Code != http.StatusUnauthorized {
		t.Fatalf("GET status = %d, want 401", rec1.Code)
	}
	rec2 := doJSON(t, srv, http.MethodPut, "/api/admin/settings", "", map[string]any{
		"key": "persona", "value": map[string]any{},
	})
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("PUT status = %d, want 401", rec2.Code)
	}
}

func TestSuggestions_Public(t *testing.T) {
	const u = "s8_sugg"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	// 先写入建议列表
	doJSON(t, srv, http.MethodPut, "/api/admin/settings", token, map[string]any{
		"key": "suggestions", "value": []string{"问题A", "问题B"},
	})

	// 公开接口无 token 也能读
	rec := doJSON(t, srv, http.MethodGet, "/api/ai/suggestions", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	items, ok := env.Data.([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("suggestions = %v, want 2 items", env.Data)
	}
}

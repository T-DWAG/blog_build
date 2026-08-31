package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func createProject(t *testing.T, srv *Server, token string, p map[string]any) int64 {
	t.Helper()
	rec := doJSON(t, srv, http.MethodPost, "/api/admin/projects", token, p)
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	data, _ := env.Data.(map[string]any)
	id, _ := data["id"].(float64)
	return int64(id)
}

func TestProject_UnpublishedHidden(t *testing.T) {
	const u = "s5_hidden"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	createProject(t, srv, token, map[string]any{
		"name": "hidden-proj", "tags": []string{"go"}, "published": false,
	})

	rec := doJSON(t, srv, http.MethodGet, "/api/projects", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	items, _ := env.Data.([]any)
	for _, it := range items {
		m, _ := it.(map[string]any)
		if m["name"] == "hidden-proj" {
			t.Fatal("unpublished project appears in public list")
		}
	}
}

func TestProject_Sort(t *testing.T) {
	const u = "s5_sort"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	createProject(t, srv, token, map[string]any{
		"name": "low", "tags": []string{"go"}, "sort_order": 1,
	})
	createProject(t, srv, token, map[string]any{
		"name": "high", "tags": []string{"go"}, "sort_order": 10,
	})

	rec := doJSON(t, srv, http.MethodGet, "/api/projects", "", nil)
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	items, _ := env.Data.([]any)
	if len(items) < 2 {
		t.Fatalf("got %d projects, want >=2", len(items))
	}
	first, _ := items[0].(map[string]any)
	if first["name"] != "high" {
		t.Fatalf("first = %v, want high (sort_order 大在前)", first["name"])
	}
}

func TestProject_TagFilter(t *testing.T) {
	const u = "s5_tag"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	createProject(t, srv, token, map[string]any{
		"name": "go-proj", "tags": []string{"go"},
	})
	createProject(t, srv, token, map[string]any{
		"name": "py-proj", "tags": []string{"python"},
	})

	rec := doJSON(t, srv, http.MethodGet, "/api/projects?tag=go", "", nil)
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	items, _ := env.Data.([]any)
	if len(items) != 1 {
		t.Fatalf("tag=go got %d, want 1", len(items))
	}
	m, _ := items[0].(map[string]any)
	if m["name"] != "go-proj" {
		t.Fatalf("got %v, want go-proj", m["name"])
	}
}

func TestProject_CreateRequiresNameTags(t *testing.T) {
	const u = "s5_validate"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	// 缺 name
	rec1 := doJSON(t, srv, http.MethodPost, "/api/admin/projects", token, map[string]any{
		"tags": []string{"go"},
	})
	if rec1.Code != http.StatusBadRequest {
		t.Fatalf("no name status = %d, want 400", rec1.Code)
	}
	// tags 为空
	rec2 := doJSON(t, srv, http.MethodPost, "/api/admin/projects", token, map[string]any{
		"name": "x",
	})
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("no tags status = %d, want 400", rec2.Code)
	}
}

func TestProject_AdminNoToken(t *testing.T) {
	const u = "s5_notoken"
	srv, _ := newTestServer(t, u)

	rec := doJSON(t, srv, http.MethodPost, "/api/admin/projects", "", map[string]any{
		"name": "x", "tags": []string{"go"},
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestProject_PublishedDefaultTrue(t *testing.T) {
	const u = "s5_default"
	srv, _ := newTestServer(t, u)
	token := adminToken(t, u)

	// 省略 published → 默认上线（契约：*bool nil → true）
	id := createProject(t, srv, token, map[string]any{
		"name": "default-online", "tags": []string{"go"},
	})
	if id == 0 {
		t.Fatal("create returned id 0")
	}
	rec := doJSON(t, srv, http.MethodGet, "/api/projects", "", nil)
	var env Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	items, _ := env.Data.([]any)
	found := false
	for _, it := range items {
		m, _ := it.(map[string]any)
		if m["name"] == "default-online" {
			found = true
		}
	}
	if !found {
		t.Fatal("project without published field should default to online (visible)")
	}
}

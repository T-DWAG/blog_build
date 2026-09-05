package store

import (
	"context"
	"os"
	"testing"
)

// testStore 打开真实数据库（验收流程要求先 docker compose up -d db）。
func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:postgres@localhost:5432/blog?sslmode=disable"
	}
	ctx := context.Background()
	st, err := Open(ctx, url)
	if err != nil {
		t.Fatalf("open store: %v (先执行 docker compose up -d db)", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestS2_Migrate_CreatesTables(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 7 张表 to_regclass 非 null
	tables := []string{
		"admin_users", "articles", "projects", "messages",
		"knowledge_docs", "knowledge_chunks", "settings",
	}
	for _, name := range tables {
		var reg any
		err := st.db.QueryRowContext(ctx,
			`SELECT to_regclass($1)`, name).Scan(&reg)
		if err != nil {
			t.Fatalf("query to_regclass(%s): %v", name, err)
		}
		if reg == nil {
			t.Fatalf("table %s not created", name)
		}
	}
}

func TestS2_SeedAdmin_Idempotent(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	const username = "s2_seed_admin"
	if err := st.SeedAdmin(ctx, username, "hash-a"); err != nil {
		t.Fatalf("seed 1: %v", err)
	}
	if err := st.SeedAdmin(ctx, username, "hash-b"); err != nil {
		t.Fatalf("seed 2: %v", err)
	}
	var n int
	if err := st.db.QueryRowContext(ctx,
		`SELECT count(*) FROM admin_users WHERE username=$1`, username).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("admin_users rows = %d, want 1", n)
	}
}

func TestS2_SeedSettings_SeedsKeys(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := st.SeedSettings(ctx); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	keys := []string{"persona", "ai", "embedding", "build_state", "suggestions"}
	for _, k := range keys {
		var n int
		if err := st.db.QueryRowContext(ctx,
			`SELECT count(*) FROM settings WHERE key=$1`, k).Scan(&n); err != nil {
			t.Fatalf("query settings %s: %v", k, err)
		}
		if n != 1 {
			t.Fatalf("settings key %s absent", k)
		}
	}
	// 不应存在 site（全局信息写死在前端，不进库）
	var siteN int
	if err := st.db.QueryRowContext(ctx,
		`SELECT count(*) FROM settings WHERE key='site'`).Scan(&siteN); err != nil {
		t.Fatalf("query settings site: %v", err)
	}
	if siteN != 0 {
		t.Fatalf("settings key site should not exist, got %d", siteN)
	}
	// ai 无 API Key，budget_per_month=10
	var aiVal string
	if err := st.db.QueryRowContext(ctx,
		`SELECT value::text FROM settings WHERE key='ai'`).Scan(&aiVal); err != nil {
		t.Fatalf("read ai: %v", err)
	}
	if aiVal != `{"budget_per_month": 10}` && aiVal != `{"budget_per_month":10}` {
		t.Fatalf("ai value = %s, want budget_per_month=10", aiVal)
	}
}

func TestS2_Open_NoURL(t *testing.T) {
	ctx := context.Background()
	if _, err := Open(ctx, ""); err != ErrNoDatabaseURL {
		t.Fatalf("err = %v, want ErrNoDatabaseURL", err)
	}
}

func TestS2_SeedAdmin_EmptyHash(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := st.SeedAdmin(ctx, "s2_empty", ""); err != ErrEmptyPasswordHash {
		t.Fatalf("err = %v, want ErrEmptyPasswordHash", err)
	}
}

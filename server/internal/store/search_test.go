package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// cleanArticles 清空文章表，保证本测试从干净状态开始。
func cleanArticles(t *testing.T, st *Store) {
	t.Helper()
	if _, err := st.db.ExecContext(context.Background(), `TRUNCATE articles`); err != nil {
		t.Fatalf("truncate articles: %v", err)
	}
}

// seedArticle 直接 SQL 插入文章，便于精确控制 published_at / is_pinned 等排序字段。
func seedArticle(t *testing.T, st *Store, slug, title, summary, content, status string,
	pinned bool, tags []string, publishedAt *time.Time) int64 {
	t.Helper()
	var id int64
	err := st.db.QueryRowContext(context.Background(),
		`INSERT INTO articles (slug, title, summary, content_md, status, is_pinned, tags, published_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		slug, title, summary, content, status, pinned, tags, publishedAt).Scan(&id)
	if err != nil {
		t.Fatalf("seed article %s: %v", slug, err)
	}
	return id
}

func TestS12_SearchArticles_EmptyKeywordOrder(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cleanArticles(t, st)

	now := time.Now()
	seedArticle(t, st, "draft-x", "draft x", "", "x", StatusDraft, false, []string{"go"}, &now)
	seedArticle(t, st, "old-pinned", "old pinned", "", "x", StatusPublished, true, []string{"go"}, ptrTime(now.Add(-48*time.Hour)))
	seedArticle(t, st, "newest", "newest", "", "x", StatusPublished, false, []string{"go"}, ptrTime(now.Add(-1*time.Hour)))
	seedArticle(t, st, "nulled", "nulled", "", "x", StatusPublished, false, []string{"go"}, nil)
	seedArticle(t, st, "mid", "mid", "", "x", StatusPublished, false, []string{"go"}, ptrTime(now.Add(-2*time.Hour)))
	seedArticle(t, st, "oldest", "oldest", "", "x", StatusPublished, false, []string{"go"}, ptrTime(now.Add(-3*time.Hour)))

	got, err := st.SearchPublishedArticles(ctx, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	want := []string{"old-pinned", "newest", "mid", "oldest", "nulled"}
	if len(got) != len(want) {
		t.Fatalf("got %d articles, want %d", len(got), len(want))
	}
	for i, slug := range want {
		if got[i].Slug != slug {
			t.Fatalf("order[%d] = %s, want %s (all=%v)", i, got[i].Slug, slug, slugsOf(got))
		}
	}
}

func TestS12_SearchArticles_WhitespaceKeywordLikeEmpty(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cleanArticles(t, st)
	seedArticle(t, st, "w1", "w1", "", "x", StatusPublished, false, []string{"go"}, nil)
	seedArticle(t, st, "w2", "w2", "", "x", StatusPublished, false, []string{"go"}, nil)

	for _, q := range []string{"   ", "\t\n"} {
		got, err := st.SearchPublishedArticles(ctx, q)
		if err != nil {
			t.Fatalf("search q=%q: %v", q, err)
		}
		if len(got) != 2 {
			t.Fatalf("q=%q got %d, want 2 (全空格按空关键词处理)", q, len(got))
		}
	}
}

func TestS12_SearchArticles_MatchesAllFieldsCaseInsensitive(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cleanArticles(t, st)

	now := time.Now()
	seedArticle(t, st, "title-hit", "singleflight zzz deep dive", "", "x", StatusPublished, true, []string{"go"}, &now)
	seedArticle(t, st, "summary-hit", "plain", "unique zzz summary", "x", StatusPublished, false, []string{"go"}, &now)
	seedArticle(t, st, "body-hit", "plain", "", "we mention zzz in the body", StatusPublished, false, []string{"go"}, &now)
	seedArticle(t, st, "tag-hit", "plain", "", "x", StatusPublished, false, []string{"zzz-tag"}, &now)
	seedArticle(t, st, "draft-hit", "zzz draft only", "", "x", StatusDraft, false, []string{"go"}, &now)
	seedArticle(t, st, "clean", "nothing to see", "", "x", StatusPublished, false, []string{"go"}, &now)

	// 大写关键词 → 大小写不敏感；置顶的 title-hit 排最前；草稿不出现
	got, err := st.SearchPublishedArticles(ctx, "ZZZ")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	want := []string{"title-hit", "tag-hit", "body-hit", "summary-hit"}
	if len(got) != len(want) {
		t.Fatalf("got %d articles, want %d (all=%v)", len(got), len(want), slugsOf(got))
	}
	for i, slug := range want {
		if got[i].Slug != slug {
			t.Fatalf("order[%d] = %s, want %s", i, got[i].Slug, slug)
		}
	}

	// 不匹配任何已发布文章 → 空
	none, err := st.SearchPublishedArticles(ctx, "draft only")
	if err != nil {
		t.Fatalf("search draft: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("draft keyword matched %d published articles, want 0", len(none))
	}
}

func TestS12_SearchArticles_Limit10(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cleanArticles(t, st)

	for i := 1; i <= 12; i++ {
		seedArticle(t, st, fmt.Sprintf("p%02d", i), fmt.Sprintf("post %d", i), "", "x",
			StatusPublished, false, []string{"go"}, nil)
	}
	got, err := st.SearchPublishedArticles(ctx, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 10 {
		t.Fatalf("got %d articles, want 10 (limit)", len(got))
	}
	if got[0].Slug != "p12" || got[9].Slug != "p03" {
		t.Fatalf("expected newest 10 by id desc, got first=%s last=%s", got[0].Slug, got[9].Slug)
	}
}

func TestS12_SearchArticles_EscapesWildcards(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cleanArticles(t, st)
	seedArticle(t, st, "pct", "progress 100% complete", "", "x", StatusPublished, false, []string{"go"}, nil)
	seedArticle(t, st, "pct2", "progress 10percent", "", "x", StatusPublished, false, []string{"go"}, nil)
	seedArticle(t, st, "under", "snake_case_name", "", "x", StatusPublished, false, []string{"go"}, nil)
	seedArticle(t, st, "under2", "snakeXcase", "", "x", StatusPublished, false, []string{"go"}, nil)

	got, err := st.SearchPublishedArticles(ctx, "100%")
	if err != nil {
		t.Fatalf("search 100%%: %v", err)
	}
	if len(got) != 1 || got[0].Slug != "pct" {
		t.Fatalf("'%%' 应被转义为字面量, got %v", slugsOf(got))
	}
	got2, err := st.SearchPublishedArticles(ctx, "_")
	if err != nil {
		t.Fatalf("search _: %v", err)
	}
	if len(got2) != 1 || got2[0].Slug != "under" {
		t.Fatalf("'_' 应被转义为字面量, got %v", slugsOf(got2))
	}
}

// ptrTime 返回 time 指针，便于传 publishedAt。
func ptrTime(t time.Time) *time.Time { return &t }

// slugsOf 提取文章 slug 列表，便于断言输出。
func slugsOf(articles []Article) []string {
	out := make([]string, len(articles))
	for i, a := range articles {
		out[i] = a.Slug
	}
	return out
}

// 本文件覆盖 S12 任务 3-6 的三只直接 Eino 工具：
//   - 参数 schema 的必填/可选契约（slug 必填、query 可选留空兜底）；
//   - 只暴露已发布内容（草稿/未上线不可见）；
//   - 摘要为空时回退正文/详情截断（summary/detail fallback）；
//   - rune 级截断（中文/emoji 不被劈开）；
//   - 畸形/缺失参数不 panic、统一走 error。
//
// 与仓库其它包一致，本测试连真实 PostgreSQL（DATABASE_URL 缺省指向
// docker compose 的 db 服务），数据用唯一 slug/名称隔离并在 t.Cleanup 清理，
// 避免污染相邻测试。
package agent

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/tool"

	"github.com/T-DWAG/blog_build/server/internal/store"
)

// testStore 打开真实数据库并建表（验收流程要求先 docker compose up -d db）。
func testStore(t *testing.T) *store.Store {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:postgres@localhost:5432/blog?sslmode=disable"
	}
	ctx := context.Background()
	st, err := store.Open(ctx, url)
	if err != nil {
		t.Fatalf("open store: %v（先执行 docker compose up -d db）", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

// toolsSlugCounter 生成唯一 slug/名称，保证同一测试进程内不撞唯一约束。
// 加上进程级 run id，避免数据库保留上次异常退出的测试数据时关键词互相命中。
var toolsSlugCounter int
var toolsRunID = strconv.FormatInt(time.Now().UnixNano(), 36)

func uniqueSlug(prefix string) string {
	toolsSlugCounter++
	return prefix + "-" + toolsRunID + "-" + strconv.Itoa(toolsSlugCounter)
}

// seedArticle 通过公开 store API 插一篇文章，测试结束时按 id 删除。
func seedArticle(t *testing.T, st *store.Store, a *store.Article) store.Article {
	t.Helper()
	if err := st.InsertArticle(context.Background(), a); err != nil {
		t.Fatalf("seed article %s: %v", a.Slug, err)
	}
	got := *a
	t.Cleanup(func() { _ = st.DeleteArticle(context.Background(), got.ID) })
	return got
}

// seedProject 通过公开 store API 插一个项目，测试结束时按 id 删除。
func seedProject(t *testing.T, st *store.Store, p *store.Project) store.Project {
	t.Helper()
	if err := st.InsertProject(context.Background(), p); err != nil {
		t.Fatalf("seed project %s: %v", p.Name, err)
	}
	got := *p
	t.Cleanup(func() { _ = st.DeleteProject(context.Background(), got.ID) })
	return got
}

func newTools(t *testing.T, st *store.Store) *ToolSet {
	t.Helper()
	ts, err := NewTools(st)
	if err != nil {
		t.Fatalf("NewTools: %v", err)
	}
	return ts
}

// assertNoPanic 断言 fn 不 panic（panic 一律视为测试失败）。
func assertNoPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s 不应 panic，却 panic: %v", name, r)
		}
	}()
	fn()
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// TestNewTools_NilStore 构造入口对 nil store 报错而不是 panic。
func TestNewTools_NilStore(t *testing.T) {
	if _, err := NewTools(nil); err == nil {
		t.Fatal("nil store 应返回 error")
	}
}

// TestTruncateRunes_UTF8Safe rune 级截断：中文/emoji 不被劈成半个。
func TestTruncateRunes_UTF8Safe(t *testing.T) {
	if got := truncateRunes("你好世界", 10); got != "你好世界" {
		t.Fatalf("不超长不应截断: %q", got)
	}
	if got := truncateRunes("你好世界", 0); got != "你好世界" {
		t.Fatalf("max<=0 不应截断: %q", got)
	}

	got := truncateRunes("你好世界", 3)
	if got != "你好世…" {
		t.Fatalf("截断结果 = %q, want 你好世…", got)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("截断结果不是合法 UTF-8: %q", got)
	}
	if n := utf8.RuneCountInString(got); n != 4 {
		t.Fatalf("rune 数 = %d, want 4（3 字 + …）", n)
	}

	// emoji（多字节）不会被劈开
	gotE := truncateRunes("🚀🚀🚀", 2)
	if !utf8.ValidString(gotE) || !strings.HasPrefix(gotE, "🚀🚀") || !strings.HasSuffix(gotE, "…") {
		t.Fatalf("emoji 截断被劈开: %q", gotE)
	}
}

// TestTools_Schema_RequiredAndOptional 参数 schema 契约：
// get_article 的 slug 必填；search_articles / list_projects 参数可选。
func TestTools_Schema_RequiredAndOptional(t *testing.T) {
	st := testStore(t)
	ts := newTools(t, st)
	ctx := context.Background()

	required := func(tl tool.InvokableTool) []string {
		t.Helper()
		info, err := tl.Info(ctx)
		if err != nil {
			t.Fatalf("Info: %v", err)
		}
		js, err := info.ToJSONSchema()
		if err != nil {
			t.Fatalf("ToJSONSchema: %v", err)
		}
		if js == nil {
			t.Fatal("jsonschema 为空")
		}
		return js.Required
	}

	if req := required(ts.GetArticle); !containsStr(req, "slug") {
		t.Fatalf("get_article 的 slug 应为必填，required=%v", req)
	}
	if req := required(ts.SearchArticles); containsStr(req, "keyword") || containsStr(req, "limit") {
		t.Fatalf("search_articles 的 keyword/limit 应为可选（keyword 留空走兜底），required=%v", req)
	}
	if req := required(ts.ListProjects); len(req) != 0 {
		t.Fatalf("list_projects 参数应全可选，required=%v", req)
	}
}

// TestTools_SearchArticles_PublishedOnlyAndFallback 搜索只返回已发布文章，
// 摘要为空时回退到正文截断片段。
func TestTools_SearchArticles_PublishedOnlyAndFallback(t *testing.T) {
	st := testStore(t)
	ts := newTools(t, st)
	ctx := context.Background()

	token := uniqueSlug("marker")
	longBody := strings.Repeat("这是一段非常长的正文内容，用于验证摘要回退与字符截断。", 10)
	seedArticle(t, st, &store.Article{
		Slug: uniqueSlug("pub-sum"), Title: token + " eino 实战一", Summary: "手写摘要",
		ContentMD: longBody, Status: store.StatusPublished, Tags: []string{"go", "agent"},
	})
	seedArticle(t, st, &store.Article{
		Slug: uniqueSlug("pub-nosum"), Title: token + " eino 实战二", ContentMD: longBody,
		Status: store.StatusPublished, Tags: []string{"go"},
	})
	seedArticle(t, st, &store.Article{
		Slug: uniqueSlug("draft"), Title: token + " 草稿勿现", ContentMD: longBody,
		Status: store.StatusDraft, Tags: []string{"go"},
	})

	var out string
	var err error
	assertNoPanic(t, "search_articles", func() {
		out, err = ts.SearchArticles.InvokableRun(ctx, `{"keyword":"`+token+`"}`)
	})
	if err != nil {
		t.Fatalf("search_articles: %v", err)
	}
	var res SearchArticlesResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("解析返回: %v, raw=%s", err, out)
	}
	if res.Count != 2 || len(res.Articles) != 2 {
		t.Fatalf("count=%d articles=%d, want 2（草稿不可见）", res.Count, len(res.Articles))
	}
	for _, c := range res.Articles {
		if strings.Contains(c.Slug, "draft") {
			t.Fatalf("草稿不应出现在搜索结果: %+v", c)
		}
		if c.Slug == "" || c.Title == "" || c.PublishedAt == "" {
			t.Fatalf("卡片缺必填字段: %+v", c)
		}
		if strings.Contains(c.Slug, "pub-nosum") {
			// 无摘要 → 回退到正文截断片段
			if !strings.HasPrefix(c.Summary, "这是一段非常长的正文内容") || !strings.HasSuffix(c.Summary, "…") {
				t.Fatalf("摘要回退不符: %q", c.Summary)
			}
			if n := utf8.RuneCountInString(c.Summary); n > summaryMaxRunes+1 {
				t.Fatalf("回退摘要超长: %d runes", n)
			}
		} else if c.Summary != "手写摘要" {
			t.Fatalf("有摘要应原样返回: %+v", c)
		}
	}

	// 空关键词 → 最新文章兜底，不报错、不超默认上限
	assertNoPanic(t, "search_articles 空关键词", func() {
		out, err = ts.SearchArticles.InvokableRun(ctx, `{"keyword":""}`)
	})
	if err != nil {
		t.Fatalf("空关键词兜底: %v", err)
	}
	var empty SearchArticlesResult
	if err := json.Unmarshal([]byte(out), &empty); err != nil {
		t.Fatalf("解析返回: %v", err)
	}
	if len(empty.Articles) > searchDefaultLimit {
		t.Fatalf("空关键词返回 %d 条，超过默认上限 %d", len(empty.Articles), searchDefaultLimit)
	}
	for _, c := range empty.Articles {
		if c.Slug == "" {
			t.Fatalf("空关键词返回了缺 slug 的卡片: %+v", c)
		}
	}
}

// TestTools_SearchArticles_LimitClamp limit 的默认值、下限与上限夹取。
// 默认 10（S12-AI 任务 4：命中 10 篇），夹在 [1,10]。
func TestTools_SearchArticles_LimitClamp(t *testing.T) {
	st := testStore(t)
	ts := newTools(t, st)
	ctx := context.Background()

	token := uniqueSlug("clamp")
	for i := 0; i < 12; i++ {
		seedArticle(t, st, &store.Article{
			Slug: uniqueSlug("clamp-a"), Title: token + " " + strconv.Itoa(i),
			ContentMD: "x", Status: store.StatusPublished, Tags: []string{"go"},
		})
	}

	searchLen := func(args string) int {
		t.Helper()
		var out string
		var err error
		assertNoPanic(t, "search_articles "+args, func() {
			out, err = ts.SearchArticles.InvokableRun(ctx, args)
		})
		if err != nil {
			t.Fatalf("search_articles %s: %v", args, err)
		}
		var res SearchArticlesResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("解析返回: %v", err)
		}
		return len(res.Articles)
	}

	if n := searchLen(`{"keyword":"` + token + `"}`); n != searchDefaultLimit {
		t.Fatalf("默认 limit 应返回 %d 条，got %d", searchDefaultLimit, n)
	}
	if n := searchLen(`{"keyword":"` + token + `","limit":1}`); n != 1 {
		t.Fatalf("limit=1 应返回 1 条，got %d", n)
	}
	if n := searchLen(`{"keyword":"` + token + `","limit":999}`); n != searchMaxLimit {
		t.Fatalf("limit=999 应夹到上限 %d 条，got %d", searchMaxLimit, n)
	}
	if n := searchLen(`{"keyword":"` + token + `","limit":0}`); n != searchDefaultLimit {
		t.Fatalf("limit=0 应回落默认 %d 条，got %d", searchDefaultLimit, n)
	}
}

// TestTools_GetArticle_SuccessAndFallback 成功取详情；正文为空时回退摘要。
func TestTools_GetArticle_SuccessAndFallback(t *testing.T) {
	st := testStore(t)
	ts := newTools(t, st)
	ctx := context.Background()

	slug := uniqueSlug("detail")
	body := "# 标题\n\n正文内容 markdown **加粗**"
	seedArticle(t, st, &store.Article{
		Slug: slug, Title: "详情文章", Summary: "摘要", ContentMD: body,
		Status: store.StatusPublished, Tags: []string{"go", "eino"}, CoverURL: "https://example.com/c.png",
	})

	var out string
	var err error
	assertNoPanic(t, "get_article", func() {
		out, err = ts.GetArticle.InvokableRun(ctx, `{"slug":"`+slug+`"}`)
	})
	if err != nil {
		t.Fatalf("get_article: %v", err)
	}
	var d ArticleDetail
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("解析返回: %v", err)
	}
	if d.Slug != slug || d.Title != "详情文章" || d.ContentMD != body {
		t.Fatalf("详情不符: %+v", d)
	}
	if d.PublishedAt == "" {
		t.Fatal("published_at 应为 RFC3339 字符串")
	}
	if len(d.Tags) != 2 {
		t.Fatalf("tags = %v, want 2 个", d.Tags)
	}

	// 正文为空 → detail 兜底回退到摘要
	slug2 := uniqueSlug("detail-empty")
	seedArticle(t, st, &store.Article{
		Slug: slug2, Title: "空正文", Summary: "只有摘要", ContentMD: "",
		Status: store.StatusPublished, Tags: []string{"go"},
	})
	assertNoPanic(t, "get_article 空正文", func() {
		out, err = ts.GetArticle.InvokableRun(ctx, `{"slug":"`+slug2+`"}`)
	})
	if err != nil {
		t.Fatalf("get_article 空正文: %v", err)
	}
	var d2 ArticleDetail
	_ = json.Unmarshal([]byte(out), &d2)
	if d2.ContentMD != "只有摘要" {
		t.Fatalf("正文为空应回退摘要，got %q", d2.ContentMD)
	}
}

// TestTools_GetArticle_Errors 缺失/畸形参数与不可见内容一律 error，不 panic。
func TestTools_GetArticle_Errors(t *testing.T) {
	st := testStore(t)
	ts := newTools(t, st)
	ctx := context.Background()

	slug := uniqueSlug("err-art")
	seedArticle(t, st, &store.Article{
		Slug: slug, Title: "错误路径", ContentMD: "x",
		Status: store.StatusPublished, Tags: []string{"go"},
	})
	draftSlug := uniqueSlug("err-draft")
	seedArticle(t, st, &store.Article{
		Slug: draftSlug, Title: "草稿", ContentMD: "x",
		Status: store.StatusDraft, Tags: []string{"go"},
	})

	cases := []struct {
		name string
		args string
		want string // 期望错误信息包含的子串；空表示只要求 err != nil
	}{
		{"草稿不可见", `{"slug":"` + draftSlug + `"}`, "不存在或未发布"},
		{"未知 slug", `{"slug":"no-such-slug-zzz"}`, "不存在或未发布"},
		{"缺 slug", `{}`, "slug"},
		{"slug 为空串", `{"slug":""}`, "slug"},
		{"畸形 JSON", `{{{`, ""},
		{"JSON 数组", `[]`, ""},
		{"slug 类型错误", `{"slug":123}`, ""},
		{"顶层 null", `null`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out string
			var err error
			assertNoPanic(t, "get_article "+tc.name, func() {
				out, err = ts.GetArticle.InvokableRun(ctx, tc.args)
			})
			if err == nil {
				t.Fatalf("期望 error，却成功返回: %s", out)
			}
			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("错误信息应包含 %q: %v", tc.want, err)
			}
		})
	}
}

// TestTools_GetArticle_RuneTruncation max_chars 按字符截断正文，
// 超长附「已截断」提示 + 剩余字数（S12-AI 任务 5 / 验收 TestGetArticle_Truncate）。
func TestTools_GetArticle_RuneTruncation(t *testing.T) {
	st := testStore(t)
	ts := newTools(t, st)
	ctx := context.Background()

	// 断言截断后正文部分（提示之前）的 rune 数与提示内容。
	assertCut := func(t *testing.T, got string, max, remaining int, prefix string) {
		t.Helper()
		if !utf8.ValidString(got) {
			t.Fatalf("截断后不是合法 UTF-8: %q", got)
		}
		idx := strings.Index(got, "（已截断，剩余")
		if idx < 0 {
			t.Fatalf("超长正文应含「已截断」提示: %q", got)
		}
		body := strings.TrimRight(got[:idx], "\n")
		if n := utf8.RuneCountInString(body); n != max {
			t.Fatalf("正文部分 rune 数 = %d, want %d（按字符截断）", n, max)
		}
		if !strings.HasPrefix(body, prefix) {
			t.Fatalf("正文部分不符: %q", body)
		}
		wantNote := "剩余 " + strconv.Itoa(remaining) + " 字"
		if !strings.Contains(got[idx:], wantNote) {
			t.Fatalf("截断提示应含 %q: %q", wantNote, got[idx:])
		}
	}

	// 显式 max_chars=10：150 字符 → 10 字符 + 提示（剩余 140 字）
	slug := uniqueSlug("long")
	body := strings.Repeat("数据与工程", 30) // 150 个字符
	seedArticle(t, st, &store.Article{
		Slug: slug, Title: "长文", ContentMD: body,
		Status: store.StatusPublished, Tags: []string{"go"},
	})

	var out string
	var err error
	assertNoPanic(t, "get_article 截断", func() {
		out, err = ts.GetArticle.InvokableRun(ctx, `{"slug":"`+slug+`","max_chars":10}`)
	})
	if err != nil {
		t.Fatalf("get_article: %v", err)
	}
	var d ArticleDetail
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("解析返回: %v", err)
	}
	assertCut(t, d.ContentMD, 10, 140, "数据与工程数据与工程")
	if d.Title != "长文" || len(d.Tags) != 1 {
		t.Fatalf("截断后应保留 title/tags: %+v", d)
	}

	// 缺省 max_chars=0 → 默认 20000：短正文原样返回
	slug2 := uniqueSlug("short")
	seedArticle(t, st, &store.Article{
		Slug: slug2, Title: "短文", ContentMD: body,
		Status: store.StatusPublished, Tags: []string{"go"},
	})
	assertNoPanic(t, "get_article 默认不截断", func() {
		out, err = ts.GetArticle.InvokableRun(ctx, `{"slug":"`+slug2+`"}`)
	})
	if err != nil {
		t.Fatalf("get_article: %v", err)
	}
	var d2 ArticleDetail
	if err := json.Unmarshal([]byte(out), &d2); err != nil {
		t.Fatalf("解析返回: %v", err)
	}
	if d2.ContentMD != body {
		t.Fatalf("短正文不应截断: %q", d2.ContentMD)
	}

	// 缺省 max_chars=0 但正文超 20000 → 默认按 20000 字符截断 + 提示
	slug3 := uniqueSlug("huge")
	huge := strings.Repeat("字", defaultMaxContentRunes+7)
	seedArticle(t, st, &store.Article{
		Slug: slug3, Title: "超长", ContentMD: huge,
		Status: store.StatusPublished, Tags: []string{"go"},
	})
	assertNoPanic(t, "get_article 默认截断", func() {
		out, err = ts.GetArticle.InvokableRun(ctx, `{"slug":"`+slug3+`"}`)
	})
	if err != nil {
		t.Fatalf("get_article: %v", err)
	}
	var d3 ArticleDetail
	if err := json.Unmarshal([]byte(out), &d3); err != nil {
		t.Fatalf("解析返回: %v", err)
	}
	assertCut(t, d3.ContentMD, defaultMaxContentRunes, 7, strings.Repeat("字", 20))
}

// TestTools_ListProjects_PublishedOnlyAndDetail 只返回已上线项目；
// 默认不带详情、摘要空时回退详情截断；tag 过滤与 include_detail 契约。
func TestTools_ListProjects_PublishedOnlyAndDetail(t *testing.T) {
	st := testStore(t)
	ts := newTools(t, st)
	ctx := context.Background()

	token := uniqueSlug("proj")
	longDetail := strings.Repeat("项目B详情内容", 300) // 2100 字符 > 默认 2000
	seedProject(t, st, &store.Project{
		Name: token + " A", Summary: "项目A摘要", DetailMD: "详情A",
		Tags: []string{"go"}, SortOrder: 10, Published: true,
	})
	seedProject(t, st, &store.Project{
		Name: token + " B", DetailMD: longDetail,
		Tags: []string{"agent"}, SortOrder: 20, Published: true,
	})
	seedProject(t, st, &store.Project{
		Name: token + " hidden", Summary: "未上线", DetailMD: "x",
		Tags: []string{"go"}, SortOrder: 99, Published: false,
	})

	var out string
	var err error
	assertNoPanic(t, "list_projects 默认", func() {
		out, err = ts.ListProjects.InvokableRun(ctx, `{}`)
	})
	if err != nil {
		t.Fatalf("list_projects: %v", err)
	}
	var res ListProjectsResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("解析返回: %v, raw=%s", err, out)
	}
	if res.Count != 2 || len(res.Projects) != 2 {
		t.Fatalf("count=%d, want 2（未上线不可见）", res.Count)
	}
	fallbackOK := false
	for i, p := range res.Projects {
		if strings.Contains(p.Name, "hidden") {
			t.Fatalf("未上线项目不可见: %+v", p)
		}
		if p.DetailMD != "" {
			t.Fatalf("默认不应返回 detail_md: %+v", p)
		}
		if strings.HasPrefix(p.Summary, "项目B详情内容") {
			fallbackOK = true
		}
		// sort_order 必须返回，且列表按 sort_order 倒序（B=20 在 A=10 前）
		wantOrder := 20 - i*10
		if p.SortOrder != wantOrder {
			t.Fatalf("sort_order[%d] = %d, want %d（按 sort_order 倒序）", i, p.SortOrder, wantOrder)
		}
	}
	if !fallbackOK {
		t.Fatalf("无摘要项目应回退到详情截断: %+v", res.Projects)
	}

	// tag 过滤
	assertNoPanic(t, "list_projects tag", func() {
		out, err = ts.ListProjects.InvokableRun(ctx, `{"tag":"agent"}`)
	})
	if err != nil {
		t.Fatalf("list_projects tag: %v", err)
	}
	var filtered ListProjectsResult
	if err := json.Unmarshal([]byte(out), &filtered); err != nil {
		t.Fatalf("解析返回: %v", err)
	}
	if filtered.Count != 1 || len(filtered.Projects) != 1 || len(filtered.Projects[0].Tags) == 0 {
		t.Fatalf("tag 过滤结果 = %+v, want 仅 agent 项目", filtered)
	}

	// include_detail=true → 带 detail_md；短详情不截断，长详情默认截断到 2000
	assertNoPanic(t, "list_projects detail", func() {
		out, err = ts.ListProjects.InvokableRun(ctx, `{"include_detail":true}`)
	})
	if err != nil {
		t.Fatalf("list_projects detail: %v", err)
	}
	var detailed ListProjectsResult
	if err := json.Unmarshal([]byte(out), &detailed); err != nil {
		t.Fatalf("解析返回: %v", err)
	}
	for _, p := range detailed.Projects {
		if p.DetailMD == "" {
			t.Fatalf("include_detail=true 应返回 detail_md: %+v", p)
		}
		if strings.HasPrefix(p.Name, token+" A") {
			if p.DetailMD != "详情A" {
				t.Fatalf("短详情不应截断: %q", p.DetailMD)
			}
		}
		if strings.HasPrefix(p.Name, token+" B") {
			idx := strings.Index(p.DetailMD, "（已截断，剩余")
			if idx < 0 {
				t.Fatalf("长详情默认应含截断提示: %q", p.DetailMD)
			}
			if n := utf8.RuneCountInString(strings.TrimRight(p.DetailMD[:idx], "\n")); n != defaultMaxDetailRunes {
				t.Fatalf("默认详情截断 = %d 字符, want %d", n, defaultMaxDetailRunes)
			}
			if !strings.Contains(p.DetailMD[idx:], "剩余 100 字") {
				t.Fatalf("截断提示应含剩余字数: %q", p.DetailMD[idx:])
			}
		}
	}

	// include_detail + max_detail_runes → 长详情按字符截断
	assertNoPanic(t, "list_projects 截断", func() {
		out, err = ts.ListProjects.InvokableRun(ctx, `{"include_detail":true,"max_detail_runes":7}`)
	})
	if err != nil {
		t.Fatalf("list_projects 截断: %v", err)
	}
	var cut ListProjectsResult
	if err := json.Unmarshal([]byte(out), &cut); err != nil {
		t.Fatalf("解析返回: %v", err)
	}
	for _, p := range cut.Projects {
		if p.DetailMD == "详情A" {
			continue // 短详情不截断
		}
		idx := strings.Index(p.DetailMD, "（已截断，剩余")
		if idx < 0 {
			t.Fatalf("详情截断应含提示: %q", p.DetailMD)
		}
		body := strings.TrimRight(p.DetailMD[:idx], "\n")
		if !strings.HasPrefix(body, "项目B详情内容") {
			t.Fatalf("详情截断不符: %q", p.DetailMD)
		}
		if n := utf8.RuneCountInString(body); n != 7 {
			t.Fatalf("详情截断 = %d 字符, want 7", n)
		}
	}
}

// TestTools_NoPanicOnMalformedArgs 三只工具对畸形/缺失参数都不 panic，
// 且错误路径之后正常调用仍可用。
func TestTools_NoPanicOnMalformedArgs(t *testing.T) {
	st := testStore(t)
	ts := newTools(t, st)
	ctx := context.Background()

	bad := []string{
		"", // 空串不是合法 JSON
		"{{{",
		"not-json",
		`[]`,
		`42`,
		`null`,
		`{"keyword":123}`,
		`{"slug":123}`,
		`{"limit":"abc"}`,
	}
	tools := map[string]tool.InvokableTool{
		toolSearchArticles: ts.SearchArticles,
		toolGetArticle:     ts.GetArticle,
		toolListProjects:   ts.ListProjects,
	}
	for name, tl := range tools {
		for _, args := range bad {
			args := args
			assertNoPanic(t, name+" args="+args, func() {
				_, _ = tl.InvokableRun(ctx, args)
			})
		}
	}

	// 畸形参数调用后，合法调用仍正常
	var out string
	var err error
	assertNoPanic(t, "畸形后正常调用", func() {
		out, err = ts.SearchArticles.InvokableRun(ctx, `{}`)
	})
	if err != nil {
		t.Fatalf("畸形参数后正常调用失败: %v", err)
	}
	if !strings.Contains(out, `"articles"`) {
		t.Fatalf("正常调用返回异常: %s", out)
	}
}

// TestTools_ThreeRegistered 构造工具集，断言三只工具都注册且名字正确
// （S12-AI 验收 TestTools_ThreeRegistered：ToolsConfig.Tools 含
// search_articles / get_article / list_projects 三个）。
func TestTools_ThreeRegistered(t *testing.T) {
	st := testStore(t)
	ts := newTools(t, st)
	ctx := context.Background()

	tools := ts.All()
	if len(tools) != 3 {
		t.Fatalf("工具数 = %d, want 3", len(tools))
	}
	names := map[string]bool{}
	for _, tl := range tools {
		info, err := tl.Info(ctx)
		if err != nil {
			t.Fatalf("Info: %v", err)
		}
		names[info.Name] = true
	}
	for _, want := range []string{toolSearchArticles, toolGetArticle, toolListProjects} {
		if !names[want] {
			t.Fatalf("缺少工具 %s，已注册: %v", want, names)
		}
	}
}

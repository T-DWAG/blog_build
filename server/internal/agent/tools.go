// 本文件实现 S12 任务 3-6：AI 分身的三只「直接 Eino 工具」。
//
// 工具清单：
//   - search_articles：在已发布文章里按关键词搜索，返回摘要级卡片；
//   - get_article：按 slug 返回单篇已发布文章的完整详情（含 Markdown 正文）；
//   - list_projects：返回已上线项目列表，可按标签过滤、可选附带详情正文。
//
// 统一约束：
//   - 只暴露已发布内容：内部一律调用 store 的 published-only 方法，
//     草稿/未上线内容对模型不可见；
//   - 参数类型化：用 github.com/cloudwego/eino/components/tool/utils.InferTool
//     从 Go 结构体推断参数 JSON Schema，不经过 MCP；参数缺失或畸形 JSON
//     一律走 error 返回，绝不 panic；
//   - rune 级截断：长文本按字符（rune）截断，中文/emoji 不会被劈成半个；
//   - summary/detail 回退：摘要为空时回退到正文/详情截断片段。
package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/T-DWAG/blog_build/server/internal/store"
)

// 工具名。
const (
	toolSearchArticles = "search_articles"
	toolGetArticle     = "get_article"
	toolListProjects   = "list_projects"
)

// 文本截断与数量上限。
const (
	// summaryMaxRunes 是摘要回退时正文/详情最多保留的字符数（S12-AI 任务 4：
	// summary 为空 → 返回 content_md 前 200 字符兜底）。
	summaryMaxRunes = 200
	// searchDefaultLimit 是 search_articles 未传 limit 时的默认条数。
	// S12-AI 任务 4：空关键词与命中场景都返回 10 篇（与 store 单次上限一致）。
	searchDefaultLimit = 10
	// searchMaxLimit 是 search_articles 允许的最大条数（与 store 单次上限一致）。
	searchMaxLimit = 10
	// defaultMaxContentRunes 是 get_article 的 max_chars 默认值（S12-AI 零值表：
	// max_chars=0 → 默认 20000，按字符（rune）截断，绝不为空正文）。
	defaultMaxContentRunes = 20000
	// defaultMaxDetailRunes 是 list_projects 的 detail_md 每项默认截断长度
	// （S12-AI 任务 6：detail_md 按字符截断，每项默认 2000 字符）。
	defaultMaxDetailRunes = 2000
	// projectsDefaultMaxItems 是 list_projects 未传 max_items 时的默认条数。
	projectsDefaultMaxItems = 20
	// projectsMaxItems 是 list_projects 允许的最大条数。
	projectsMaxItems = 50
)

// truncationNoteFormat 是超长截断时附带的「已截断」提示（S12-AI 任务 5：
// 超长附「已截断」提示 + 剩余字数；剩余字数按字符（rune）计）。
const truncationNoteFormat = "\n\n（已截断，剩余 %d 字）"

// 工具描述会随参数 schema 一起发给模型，写具体有助于模型正确选工具。
const (
	searchArticlesDesc = "在已发布文章中搜索，关键词匹配标题/摘要/正文/标签（大小写不敏感）；" +
		"关键词为空时返回最新已发布文章（随便看看）。返回摘要级卡片，不含正文，" +
		"需要看正文请再调用 get_article。"
	getArticleDesc = "按 slug 返回单篇已发布文章的完整详情（含 Markdown 正文）。" +
		"slug 来自 search_articles 的返回；文章不存在或未发布时返回错误。" +
		"正文较长时可传 max_chars 限制返回长度（默认 20000 字符，超长附截断提示）。"
	listProjectsDesc = "返回已上线项目列表，可按标签过滤（如 go / agent）。" +
		"默认只返回摘要卡片；include_detail=true 时附带详情正文 detail_md" +
		"（每项默认截断 2000 字符）。"
)

// SearchArticlesArgs 是 search_articles 的类型化参数。
type SearchArticlesArgs struct {
	// Keyword 为空/全空格时返回最新已发布文章（兜底），非空时在
	// 标题/摘要/正文/标签上做大小写不敏感的子串匹配。
	// 注意：带 omitempty 表示 schema 层面可选（留空走兜底）。
	Keyword string `json:"keyword,omitempty" jsonschema_description:"搜索关键词（匹配标题/摘要/正文/标签，大小写不敏感）；留空返回最新已发布文章"`
	// Limit 默认 10，夹在 [1,10]。
	Limit int `json:"limit,omitempty" jsonschema_description:"最多返回条数（1-10，默认 10）"`
}

// GetArticleArgs 是 get_article 的类型化参数。Slug 必填。
type GetArticleArgs struct {
	Slug string `json:"slug" jsonschema:"required" jsonschema_description:"文章唯一标识 slug（来自 search_articles 的结果，必填）"`
	// MaxChars<=0 回落默认 20000；>0 按字符截断，超长附「已截断，剩余 N 字」提示。
	MaxChars int `json:"max_chars,omitempty" jsonschema_description:"正文最多保留字符数（默认 20000，按字符截断，超长附已截断提示）"`
}

// ListProjectsArgs 是 list_projects 的类型化参数。
type ListProjectsArgs struct {
	// Tag 非空时只返回含该标签的项目。
	Tag string `json:"tag,omitempty" jsonschema_description:"按标签过滤（如 go / agent），留空返回全部已上线项目"`
	// IncludeDetail=true 时附带详情正文 detail_md。
	IncludeDetail bool `json:"include_detail,omitempty" jsonschema_description:"是否附带详情正文 detail_md，默认 false（只返回摘要）"`
	// MaxDetailRunes<=0 回落默认 2000；>0 按字符截断。
	MaxDetailRunes int `json:"max_detail_runes,omitempty" jsonschema_description:"include_detail=true 时详情最多保留字符数（默认 2000，按字符截断）"`
	// MaxItems 默认 20，夹在 [1,50]。
	MaxItems int `json:"max_items,omitempty" jsonschema_description:"最多返回条数（1-50，默认 20）"`
}

// ArticleCard 是 search_articles 返回的单条卡片（摘要级，不含正文）。
type ArticleCard struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Tags        []string `json:"tags"`
	PublishedAt string   `json:"published_at"`
}

// SearchArticlesResult 是 search_articles 的返回。
type SearchArticlesResult struct {
	Count    int           `json:"count"`
	Articles []ArticleCard `json:"articles"`
}

// ArticleDetail 是 get_article 的返回（含正文）。
type ArticleDetail struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	ContentMD   string   `json:"content_md"`
	Tags        []string `json:"tags"`
	CoverURL    string   `json:"cover_url"`
	PublishedAt string   `json:"published_at"`
}

// ProjectCard 是 list_projects 返回的单条项目卡片。
// SortOrder 是管理台排序权重（大在前），S12-AI 任务 6 要求返回。
type ProjectCard struct {
	Name      string   `json:"name"`
	Summary   string   `json:"summary"`
	DetailMD  string   `json:"detail_md,omitempty"` // 仅 include_detail=true 时输出
	Tags      []string `json:"tags"`
	RepoURL   string   `json:"repo_url,omitempty"`
	HomeURL   string   `json:"home_url,omitempty"`
	DemoURL   string   `json:"demo_url,omitempty"`
	SortOrder int      `json:"sort_order"`
}

// ListProjectsResult 是 list_projects 的返回。
type ListProjectsResult struct {
	Count    int           `json:"count"`
	Projects []ProjectCard `json:"projects"`
}

// ToolSet 持有三只直接 Eino 工具，供 agent 编排（agent.go）一次性挂载。
type ToolSet struct {
	SearchArticles tool.InvokableTool
	GetArticle     tool.InvokableTool
	ListProjects   tool.InvokableTool
}

// NewTools 用 utils.InferTool 从类型化参数结构体构造三只工具。
// st 不能为 nil；返回 error 表示参数 schema 推断失败（正常不会发生）。
func NewTools(st *store.Store) (*ToolSet, error) {
	if st == nil {
		return nil, errors.New("agent: store is nil")
	}

	search, err := utils.InferTool(toolSearchArticles, searchArticlesDesc,
		func(ctx context.Context, in SearchArticlesArgs) (*SearchArticlesResult, error) {
			return runSearchArticles(ctx, st, in)
		})
	if err != nil {
		return nil, fmt.Errorf("agent: infer %s: %w", toolSearchArticles, err)
	}

	detail, err := utils.InferTool(toolGetArticle, getArticleDesc,
		func(ctx context.Context, in GetArticleArgs) (*ArticleDetail, error) {
			return runGetArticle(ctx, st, in)
		})
	if err != nil {
		return nil, fmt.Errorf("agent: infer %s: %w", toolGetArticle, err)
	}

	projects, err := utils.InferTool(toolListProjects, listProjectsDesc,
		func(ctx context.Context, in ListProjectsArgs) (*ListProjectsResult, error) {
			return runListProjects(ctx, st, in)
		})
	if err != nil {
		return nil, fmt.Errorf("agent: infer %s: %w", toolListProjects, err)
	}

	return &ToolSet{
		SearchArticles: search,
		GetArticle:     detail,
		ListProjects:   projects,
	}, nil
}

// All 返回全部工具切片，便于后续把整套工具挂到 ToolsNode / BindTools。
func (ts *ToolSet) All() []tool.InvokableTool {
	return []tool.InvokableTool{ts.SearchArticles, ts.GetArticle, ts.ListProjects}
}

// runSearchArticles 是 search_articles 的实现。
// 只查已发布文章（store.SearchPublishedArticles 内部已过滤 status='published'）。
func runSearchArticles(ctx context.Context, st *store.Store, in SearchArticlesArgs) (*SearchArticlesResult, error) {
	limit := clampInt(in.Limit, searchDefaultLimit, 1, searchMaxLimit)
	articles, err := st.SearchPublishedArticles(ctx, in.Keyword)
	if err != nil {
		return nil, fmt.Errorf("search_articles: %w", err)
	}
	if len(articles) > limit {
		articles = articles[:limit]
	}
	cards := make([]ArticleCard, 0, len(articles))
	for _, a := range articles {
		cards = append(cards, ArticleCard{
			Slug:        a.Slug,
			Title:       a.Title,
			Summary:     summaryOrFallback(a.Summary, a.ContentMD, summaryMaxRunes),
			Tags:        normStrings(a.Tags),
			PublishedAt: fmtTime(a.PublishedAt),
		})
	}
	return &SearchArticlesResult{Count: len(cards), Articles: cards}, nil
}

// runGetArticle 是 get_article 的实现。
// 只返回已发布文章（store.GetPublishedArticleBySlug 内部已过滤 status='published'）。
// max_chars<=0 回落默认 20000（S12-AI 零值表：绝不因 0 返回空正文）。
func runGetArticle(ctx context.Context, st *store.Store, in GetArticleArgs) (*ArticleDetail, error) {
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		return nil, errors.New("get_article: 缺少必填参数 slug")
	}
	a, err := st.GetPublishedArticleBySlug(ctx, slug)
	if errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("get_article: 文章不存在或未发布（slug=%s）", slug)
	}
	if err != nil {
		return nil, fmt.Errorf("get_article: %w", err)
	}
	// detail 兜底：正文为空时退回摘要（summary/detail fallback）。
	content := a.ContentMD
	if strings.TrimSpace(content) == "" {
		content = a.Summary
	}
	return &ArticleDetail{
		Slug:        a.Slug,
		Title:       a.Title,
		Summary:     a.Summary,
		ContentMD:   truncateContent(content, in.MaxChars, defaultMaxContentRunes),
		Tags:        normStrings(a.Tags),
		CoverURL:    a.CoverURL,
		PublishedAt: fmtTime(a.PublishedAt),
	}, nil
}

// runListProjects 是 list_projects 的实现。
// 只返回已上线项目（store.ListPublishedProjects 内部已过滤 published=TRUE）。
func runListProjects(ctx context.Context, st *store.Store, in ListProjectsArgs) (*ListProjectsResult, error) {
	maxItems := clampInt(in.MaxItems, projectsDefaultMaxItems, 1, projectsMaxItems)
	projects, err := st.ListPublishedProjects(ctx, strings.TrimSpace(in.Tag))
	if err != nil {
		return nil, fmt.Errorf("list_projects: %w", err)
	}
	if len(projects) > maxItems {
		projects = projects[:maxItems]
	}
	cards := make([]ProjectCard, 0, len(projects))
	for _, p := range projects {
		card := ProjectCard{
			Name:      p.Name,
			Summary:   summaryOrFallback(p.Summary, p.DetailMD, summaryMaxRunes),
			Tags:      normStrings(p.Tags),
			RepoURL:   p.RepoURL,
			HomeURL:   p.HomeURL,
			DemoURL:   p.DemoURL,
			SortOrder: p.SortOrder,
		}
		if in.IncludeDetail {
			// detail_md 非空才带，且每项默认按 2000 字符截断（超长附截断提示）。
			card.DetailMD = truncateContent(p.DetailMD, in.MaxDetailRunes, defaultMaxDetailRunes)
		}
		cards = append(cards, card)
	}
	return &ListProjectsResult{Count: len(cards), Projects: cards}, nil
}

// truncateRunes 按字符（rune）截断，超出部分以 … 结尾；max<=0 返回原文。
// 先数再切，保证多字节字符（中文/emoji）不会被劈成半个。
func truncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max]) + "…"
}

// truncateContent 是正文/详情的截断入口（S12-AI 零值表 + 任务 5/6）：
// max<=0 时回落 defMax（正文 20000 / 详情 2000），按字符（rune）截断，
// 超长时在末尾附「已截断，剩余 N 字」提示（N 为剩余字符数）。
func truncateContent(s string, max, defMax int) string {
	if max <= 0 {
		max = defMax
	}
	n := utf8.RuneCountInString(s)
	if n <= max {
		return s
	}
	body := string([]rune(s)[:max])
	return body + fmt.Sprintf(truncationNoteFormat, n-max)
}

// summaryOrFallback 摘要非空时原样返回（schema 限长 VARCHAR(300)）；
// 摘要为空时回退到正文/详情截断片段；两者皆空返回占位文案。
func summaryOrFallback(summary, body string, maxRunes int) string {
	if s := strings.TrimSpace(summary); s != "" {
		return s
	}
	if s := truncateRunes(strings.TrimSpace(body), maxRunes); s != "" {
		return s
	}
	return "（无摘要）"
}

// fmtTime 把 *time.Time 格式化为 RFC3339；nil → ""。
func fmtTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// clampInt 把 v 夹在 [min,max]；v<=0 时取 def。
func clampInt(v, def, min, max int) int {
	if v <= 0 {
		v = def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// normStrings 把 nil 切片规范成空切片，避免输出 JSON 出现 null。
func normStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

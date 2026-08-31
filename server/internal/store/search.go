package store

import (
	"context"
	"strings"
)

// SearchResult 是搜索返回的分组结果。
type SearchResult struct {
	Articles []Article `json:"articles"`
	Projects []Project `json:"projects"`
}

// Normalized 把 nil 切片转成空切片，保证 JSON 输出 [] 而非 null。
func (r *SearchResult) Normalized() *SearchResult {
	if r.Articles == nil {
		r.Articles = []Article{}
	}
	if r.Projects == nil {
		r.Projects = []Project{}
	}
	return r
}

// escapeLike 转义 LIKE 通配符，防止用户输入被当作模式。
func escapeLike(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(s)
}

// Search 在「已发布文章的标题/正文」和「已上线项目的标题」里搜索。
// 关键词为空（或全空格）时不查库，返回空结果。
func (s *Store) Search(ctx context.Context, q string) (*SearchResult, error) {
	res := &SearchResult{}
	q = strings.TrimSpace(q)
	if q == "" {
		return res.Normalized(), nil
	}
	pattern := "%" + escapeLike(q) + "%"

	// 文章：published AND (title ILIKE OR content_md ILIKE)
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+articleCols+` FROM articles
		 WHERE status = $1 AND (title ILIKE $2 OR content_md ILIKE $2)
		 ORDER BY id DESC`,
		StatusPublished, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		res.Articles = append(res.Articles, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 项目：published AND name ILIKE（不含 detail_md）
	prows, err := s.db.QueryContext(ctx,
		`SELECT `+projectCols+` FROM projects
		 WHERE published = TRUE AND name ILIKE $1
		 ORDER BY id DESC`,
		pattern)
	if err != nil {
		return nil, err
	}
	defer prows.Close()
	for prows.Next() {
		p, err := scanProject(prows)
		if err != nil {
			return nil, err
		}
		res.Projects = append(res.Projects, *p)
	}
	if err := prows.Err(); err != nil {
		return nil, err
	}

	return res.Normalized(), nil
}

// TagCount 是标签聚合结果。
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// ListTagCounts 统计已发布文章与已上线项目里出现过的标签及次数。
func (s *Store) ListTagCounts(ctx context.Context) ([]TagCount, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tag, count(*) AS cnt FROM (
		     SELECT unnest(tags) AS tag FROM articles WHERE status = $1
		     UNION ALL
		     SELECT unnest(tags) AS tag FROM projects WHERE published = TRUE
		 ) t
		 GROUP BY tag ORDER BY cnt DESC, tag`,
		StatusPublished)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := []TagCount{}
	for rows.Next() {
		var tc TagCount
		if err := rows.Scan(&tc.Tag, &tc.Count); err != nil {
			return nil, err
		}
		tags = append(tags, tc)
	}
	return tags, rows.Err()
}

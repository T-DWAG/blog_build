package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

// 文章状态
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

// Article 对应 articles 表的一行。
type Article struct {
	ID          int64      `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Summary     string     `json:"summary"`
	ContentMD   string     `json:"content_md"`
	Status      string     `json:"status"`
	IsPinned    bool       `json:"is_pinned"`
	Tags        []string   `json:"tags"`
	CoverURL    string     `json:"cover_url"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

const defaultPageSize = 20

// scanTags 把查询里 to_jsonb(tags)::text 的输出解析成 []string。
// pgx stdlib 默认把 TEXT[] 当字符串返回，无法直接 Scan 进 []string，故用 JSON 中转。
func scanTags(s string) ([]string, error) {
	var tags []string
	if err := json.Unmarshal([]byte(s), &tags); err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, nil
}

const articleCols = `id, slug, title, summary, content_md, status, is_pinned,
	to_jsonb(tags)::text, cover_url, published_at, created_at, updated_at`

// scanArticle 按 articleCols 的列序扫描一行。
func scanArticle(scanner interface{ Scan(...any) error }) (*Article, error) {
	var a Article
	var tagsJSON string
	err := scanner.Scan(&a.ID, &a.Slug, &a.Title, &a.Summary, &a.ContentMD, &a.Status,
		&a.IsPinned, &tagsJSON, &a.CoverURL, &a.PublishedAt, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	tags, err := scanTags(tagsJSON)
	if err != nil {
		return nil, err
	}
	a.Tags = tags
	return &a, nil
}

// ListPublishedArticles 已发布文章列表：置顶优先，再按发布时间倒序。
// tag 非空则只返回含该标签的文章。page<1 视为 1。
func (s *Store) ListPublishedArticles(ctx context.Context, tag string, page int) ([]Article, error) {
	if page < 1 {
		page = 1
	}
	query := `SELECT ` + articleCols + `
	          FROM articles
	          WHERE status = $1`
	args := []any{StatusPublished}
	if tag != "" {
		query += ` AND tags @> $2`
		args = append(args, []string{tag})
	}
	query += ` ORDER BY is_pinned DESC, published_at DESC NULLS LAST, id DESC
	           LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)
	args = append(args, defaultPageSize, (page-1)*defaultPageSize)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	articles := []Article{}
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		articles = append(articles, *a)
	}
	return articles, rows.Err()
}

// GetPublishedArticleBySlug 按 slug 取已发布文章；非 published → ErrNotFound。
func (s *Store) GetPublishedArticleBySlug(ctx context.Context, slug string) (*Article, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+articleCols+`
		 FROM articles WHERE slug = $1 AND status = $2`,
		slug, StatusPublished)
	a, err := scanArticle(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// ListArticlesAdmin 全部状态的文章，id 倒序。
func (s *Store) ListArticlesAdmin(ctx context.Context) ([]Article, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+articleCols+` FROM articles ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	articles := []Article{}
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		articles = append(articles, *a)
	}
	return articles, rows.Err()
}

// InsertArticle 新增文章。Title 为空或 Tags 为空 → ErrValidation。
// Status 为空 → draft；published 且 PublishedAt 为空 → now。
func (s *Store) InsertArticle(ctx context.Context, a *Article) error {
	if a.Title == "" || len(a.Tags) == 0 {
		return ErrValidation
	}
	if a.Status == "" {
		a.Status = StatusDraft
	}
	if a.Status != StatusDraft && a.Status != StatusPublished {
		return ErrValidation
	}
	publishedAt := a.PublishedAt
	if a.Status == StatusPublished && publishedAt == nil {
		now := time.Now()
		publishedAt = &now
	}
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO articles (slug, title, summary, content_md, status, is_pinned, tags, cover_url, published_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, created_at, updated_at`,
		a.Slug, a.Title, a.Summary, a.ContentMD, a.Status, a.IsPinned, a.Tags, a.CoverURL, publishedAt).
		Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return err
	}
	a.PublishedAt = publishedAt
	return nil
}

// UpdateArticle 更新文章。无行 → ErrNotFound。
// draft→published 且 PublishedAt 为空 → now；已发布不改 PublishedAt。
func (s *Store) UpdateArticle(ctx context.Context, a *Article) error {
	if a.Title == "" || len(a.Tags) == 0 {
		return ErrValidation
	}
	if a.Status != StatusDraft && a.Status != StatusPublished {
		return ErrValidation
	}
	// 从库中取当前状态
	var curStatus string
	var curPublishedAt *time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT status, published_at FROM articles WHERE id = $1`, a.ID).
		Scan(&curStatus, &curPublishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	publishedAt := curPublishedAt
	if curStatus == StatusDraft && a.Status == StatusPublished && publishedAt == nil {
		now := time.Now()
		publishedAt = &now
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE articles
		 SET slug=$1, title=$2, summary=$3, content_md=$4, status=$5, is_pinned=$6,
		     tags=$7, cover_url=$8, published_at=$9, updated_at=now()
		 WHERE id=$10`,
		a.Slug, a.Title, a.Summary, a.ContentMD, a.Status, a.IsPinned, a.Tags, a.CoverURL, publishedAt, a.ID)
	if err != nil {
		return err
	}
	a.PublishedAt = publishedAt
	return nil
}

// DeleteArticle 删除文章。0 行 → ErrNotFound。不删文件。
func (s *Store) DeleteArticle(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM articles WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

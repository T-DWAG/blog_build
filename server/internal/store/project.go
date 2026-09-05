package store

import (
	"context"
	"time"
)

// Project 对应 projects 表的一行。
type Project struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CoverURL  string    `json:"cover_url"`
	Summary   string    `json:"summary"`
	RepoURL   string    `json:"repo_url"`
	HomeURL   string    `json:"home_url"`
	DemoURL   string    `json:"demo_url"`
	DetailMD  string    `json:"detail_md"`
	Tags      []string  `json:"tags"`
	SortOrder int       `json:"sort_order"`
	Published bool      `json:"published"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const projectCols = `id, name, cover_url, summary, repo_url, home_url, demo_url, detail_md,
	to_jsonb(tags)::text, sort_order, published, created_at, updated_at`

// scanProject 按 projectCols 列序扫描一行。
func scanProject(scanner interface{ Scan(...any) error }) (*Project, error) {
	var p Project
	var tagsJSON string
	err := scanner.Scan(&p.ID, &p.Name, &p.CoverURL, &p.Summary, &p.RepoURL, &p.HomeURL,
		&p.DemoURL, &p.DetailMD, &tagsJSON, &p.SortOrder, &p.Published, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	tags, err := scanTags(tagsJSON)
	if err != nil {
		return nil, err
	}
	p.Tags = tags
	return &p, nil
}

// ListPublishedProjects 已上线项目列表：sort_order 大的在前，同值按 id 倒序。
// tag 非空则只返回含该标签的项目。
func (s *Store) ListPublishedProjects(ctx context.Context, tag string) ([]Project, error) {
	query := `SELECT ` + projectCols + ` FROM projects WHERE published = TRUE`
	args := []any{}
	if tag != "" {
		query += ` AND tags @> $1`
		args = append(args, []string{tag})
	}
	query += ` ORDER BY sort_order DESC, id DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, *p)
	}
	return projects, rows.Err()
}

// ListProjectsAdmin 全部项目（含未上线），id 倒序。
func (s *Store) ListProjectsAdmin(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+projectCols+` FROM projects ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, *p)
	}
	return projects, rows.Err()
}

// InsertProject 新增项目。Name 或 Tags 为空 → ErrValidation。
func (s *Store) InsertProject(ctx context.Context, p *Project) error {
	if p.Name == "" || len(p.Tags) == 0 {
		return ErrValidation
	}
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO projects (name, cover_url, summary, repo_url, home_url, demo_url,
		                      detail_md, tags, sort_order, published)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, created_at, updated_at`,
		p.Name, p.CoverURL, p.Summary, p.RepoURL, p.HomeURL, p.DemoURL,
		p.DetailMD, p.Tags, p.SortOrder, p.Published).
		Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return err
	}
	return nil
}

// UpdateProject 更新项目。无行 → ErrNotFound；Name 或 Tags 为空 → ErrValidation。
func (s *Store) UpdateProject(ctx context.Context, p *Project) error {
	if p.Name == "" || len(p.Tags) == 0 {
		return ErrValidation
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE projects
		 SET name=$1, cover_url=$2, summary=$3, repo_url=$4, home_url=$5, demo_url=$6,
		     detail_md=$7, tags=$8, sort_order=$9, published=$10, updated_at=now()
		 WHERE id=$11`,
		p.Name, p.CoverURL, p.Summary, p.RepoURL, p.HomeURL, p.DemoURL,
		p.DetailMD, p.Tags, p.SortOrder, p.Published, p.ID)
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

// DeleteProject 删除项目。0 行 → ErrNotFound。
func (s *Store) DeleteProject(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = $1`, id)
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

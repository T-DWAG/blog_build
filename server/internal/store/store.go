package store

import (
	"context"
	"database/sql"
	"errors"

	_ "github.com/jackc/pgx/v5/stdlib" // 注册 pgx 驱动
)

// 哨兵错误
var (
	ErrNoDatabaseURL    = errors.New("store: no database url")
	ErrEmptyPasswordHash = errors.New("store: empty password hash")
)

// Store 持有数据库连接池。
type Store struct {
	db *sql.DB
}

// Open 建立数据库连接并 Ping 验证可达。
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	if databaseURL == "" {
		return nil, ErrNoDatabaseURL
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close 关闭连接池。
func (s *Store) Close() error {
	return s.db.Close()
}

// SeedAdmin 以 username/passwordHash 插入管理员，已存在则不覆盖。
func (s *Store) SeedAdmin(ctx context.Context, username, passwordHash string) error {
	if passwordHash == "" {
		return ErrEmptyPasswordHash
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO admin_users (username, password_hash) VALUES ($1, $2)
		 ON CONFLICT (username) DO NOTHING`,
		username, passwordHash)
	return err
}

// SeedSettings 写入 5 个种子配置 key，已存在则不覆盖。
// ai 不存任何 API Key；budget_per_month=10；build_state 为默认空构建态。
// 结构见契约《数据》：site 不进库，全局信息写死在前端。
func (s *Store) SeedSettings(ctx context.Context) error {
	seeds := []struct {
		key   string
		value string
	}{
		{"persona", `{"role":"站主的 AI 分身，以第一人称回答","style":"简洁、克制，以证据说话","fallback":"素材里没有的内容，明确说不知道，不编造"}`},
		{"ai", `{"budget_per_month":10}`},
		{"embedding", `{"provider":"siliconflow","model":"bge-m3","dim":1024}`},
		{"build_state", `{"building":false,"last_build_at":"","last_status":""}`},
		{"suggestions", `["他是做什么的？","他的技术栈？","他做过哪些项目？","如何联系他？"]`},
	}
	for _, seed := range seeds {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO settings (key, value) VALUES ($1, $2::jsonb)
			 ON CONFLICT (key) DO NOTHING`,
			seed.key, seed.value)
		if err != nil {
			return err
		}
	}
	return nil
}

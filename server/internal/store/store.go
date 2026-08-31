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
// 目的：保证 key 存在（代码读取时不缺 key），不锁死业务值。
// persona / embedding / suggestions 只占位，具体值由站主在管理台（S08）表单填写；
// ai 的 budget_per_month=10 为契约锁定默认；build_state 契约保留。
// API Key 一律不进库，走环境变量。
func (s *Store) SeedSettings(ctx context.Context) error {
	seeds := []struct {
		key   string
		value string
	}{
		{"persona", `{}`},
		{"ai", `{"budget_per_month":10}`},
		{"embedding", `{}`},
		{"build_state", `{"building":false,"last_build_at":"","last_status":""}`},
		{"suggestions", `[]`},
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

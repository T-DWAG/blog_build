package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // 注册 pgx 驱动
)

// 哨兵错误
var (
	ErrNoDatabaseURL     = errors.New("store: no database url")
	ErrEmptyPasswordHash = errors.New("store: empty password hash")
	ErrNotFound          = errors.New("store: not found")
	ErrValidation        = errors.New("store: validation failed")
)

// Admin 对应 admin_users 表的一行。
type Admin struct {
	ID             int64      `json:"id"`
	Username       string     `json:"username"`
	PasswordHash   string     `json:"-"`
	FailedAttempts int        `json:"-"`
	LockedUntil    *time.Time `json:"-"`
}

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

// GetAdminByUsername 按用户名取管理员，无行 → ErrNotFound。
func (s *Store) GetAdminByUsername(ctx context.Context, username string) (*Admin, error) {
	var a Admin
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, failed_attempts, locked_until
		 FROM admin_users WHERE username = $1`,
		username).Scan(&a.ID, &a.Username, &a.PasswordHash, &a.FailedAttempts, &a.LockedUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// RecordLoginFail 失败次数 +1；加后 >=5 则锁定 10 分钟。
func (s *Store) RecordLoginFail(ctx context.Context, adminID int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE admin_users
		 SET failed_attempts = failed_attempts + 1,
		     locked_until = CASE
		         WHEN failed_attempts + 1 >= 5 THEN now() + interval '10 minutes'
		         ELSE locked_until END
		 WHERE id = $1`,
		adminID)
	return err
}

// RecordLoginOK 登录成功：清零失败次数、解除锁定、记录登录时间。
func (s *Store) RecordLoginOK(ctx context.Context, adminID int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE admin_users
		 SET failed_attempts = 0, locked_until = NULL, last_login_at = now()
		 WHERE id = $1`,
		adminID)
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

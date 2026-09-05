package store

import (
	"context"
	_ "embed"
	"strings"
)

//go:embed schema.sql
var schemaSQL string

// Migrate 整段执行 schema.sql。
// pgx 驱动不支持一条 Exec 多条语句，这里按分号切分逐条执行。
func (s *Store) Migrate(ctx context.Context) error {
	statements := strings.Split(schemaSQL, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

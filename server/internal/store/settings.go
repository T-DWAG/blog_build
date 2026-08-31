package store

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
)

// settings key 常量。site 不进库；build_state 已废弃不提供读写。
const (
	KeyPersona     = "persona"
	KeyAI          = "ai"
	KeyEmbedding   = "embedding"
	KeySuggestions = "suggestions"
)

// WritableSettingKeys 是站主可通过设置接口修改的 key 集合。
var WritableSettingKeys = map[string]bool{
	KeyPersona:     true,
	KeyAI:          true,
	KeyEmbedding:   true,
	KeySuggestions: true,
}

// GetSettings 返回全部可写 key 的当前值。
func (s *Store) GetSettings(ctx context.Context) (map[string]json.RawMessage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, value FROM settings WHERE key = ANY($1)`,
		[]string{KeyPersona, KeyAI, KeyEmbedding, KeySuggestions})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]json.RawMessage{}
	for rows.Next() {
		var k string
		var v json.RawMessage
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// PutSetting 写入一个可写 key。key 不可写 / value 为 null / AI 配置含密钥 → ErrValidation。
func (s *Store) PutSetting(ctx context.Context, key string, value json.RawMessage) error {
	if !WritableSettingKeys[key] {
		return ErrValidation
	}
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return ErrValidation
	}
	if key == KeyAI && containsSecretKey(value) {
		return ErrValidation
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value, updated_at)
		 VALUES ($1, $2, now())
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		key, value)
	return err
}

// containsSecretKey 检查 JSON 顶层是否含 api_key / apiKey / secret 字段。
func containsSecretKey(raw json.RawMessage) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	for k := range m {
		lk := strings.ToLower(k)
		if lk == "api_key" || lk == "apikey" || strings.Contains(lk, "secret") {
			return true
		}
	}
	return false
}

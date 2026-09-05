package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// settings key 常量。site 不进库；build_state 已废弃不提供读写。
// ai_usage 是运行时 AI 用量计数（非可写设置），由 IncrAIUsage 维护。
const (
	KeyPersona     = "persona"
	KeyAI          = "ai"
	KeyEmbedding   = "embedding"
	KeySuggestions = "suggestions"
	KeyAIUsage     = "ai_usage"
)

// WritableSettingKeys 是站主可通过设置接口修改的 key 集合。
var WritableSettingKeys = map[string]bool{
	KeyPersona:     true,
	KeyAI:          true,
	KeyEmbedding:   true,
	KeySuggestions: true,
}

// GetSettings 返回全部可写 key 的当前值。
// ai_usage 是运行时计数，不属于可写设置，因此不会出现在返回里。
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

// AIUsage 是 ai_usage 的用量快照，供上层（agent/chat）展示与预算判断。
type AIUsage struct {
	Month  string `json:"month"`  // 统计月份 YYYY-MM
	Count  int    `json:"count"`  // 当月已登记次数
	Budget int    `json:"budget"` // 当月预算（ai.budget_per_month，缺省 10）
}

// defaultAIBudget 是预算契约：ai 设置缺失/未配置时按 S02 锁定的默认值。
const defaultAIBudget = 10

// CheckAIUsage 返回当前月份用量并原子判断本次对话是否仍有额度，但不递增。
// API 层在写出 SSE 头之前调用它，因此预算用尽能返回真实 HTTP 429；
// 真正占用额度仍由 IncrAIUsage 在 agent 入口完成，避免只检查不登记。
func (s *Store) CheckAIUsage(ctx context.Context) (*AIUsage, error) {
	budget := defaultAIBudget
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE((value->>'budget_per_month')::int, $1) FROM settings WHERE key = $2`,
		defaultAIBudget, KeyAI).Scan(&budget)
	if errors.Is(err, sql.ErrNoRows) {
		budget = defaultAIBudget
	} else if err != nil {
		return nil, err
	}

	month, count := "", 0
	err = s.db.QueryRowContext(ctx,
		`SELECT value->>'month', COALESCE((value->>'count')::int, 0) FROM settings WHERE key = $1`,
		KeyAIUsage).Scan(&month, &count)
	if errors.Is(err, sql.ErrNoRows) {
		month, count = "", 0
	} else if err != nil {
		return nil, err
	}

	curMonth := time.Now().Format("2006-01")
	if month != curMonth {
		count = 0
	}
	usage := &AIUsage{Month: curMonth, Count: count, Budget: budget}
	if count >= budget {
		return usage, ErrBudgetExceeded
	}
	return usage, nil
}

// IncrAIUsage 原子地登记 n 次 AI 调用（n 通常为 1），返回登记后的用量快照。
//
// 存储契约（S12-AI 任务 7）：ai_usage 存于 settings 表 key='ai_usage'，值形如
// {"month":"2026-09","count":3}；跨月自动重置。它是运行时计数，不属于可写设置
// —— GetSettings 不返回它，PutSetting 也无法写入（见 WritableSettingKeys）。
//
// 语义：
//   - 跨月自动重置：事务内先读 ai_usage，若 month 不是当前月，count 从 0 起算，
//     再记本次增量（首个跨月调用把 month 置为当前月）。
//   - 预算硬上限：budget_per_month 取自 ai 设置（缺省 10）。登记后 count 若超过
//     预算，本次不落库并返回 ErrBudgetExceeded；返回的 AIUsage 快照仍带当月已用
//     次数与预算，供上层提示「本月预算已用尽」。
//   - 零预算：budget_per_month=0 表示当月零配额（S12-AI 零值表 + 验收
//     TestChat_BudgetZero：入口第一步即 ErrBudget、不调模型）——任何 n>0 的增量
//     都直接 ErrBudgetExceeded 且不落库。
//   - 并发安全：事务内对 ai / ai_usage 两行 SELECT ... FOR UPDATE 加锁串行化，
//     避免并发增量互相覆盖或把用量冲过预算。
//
// 建议调用方式（等价于 S12-AI 流程「used>=budget → ErrBudget 不调模型」的原子版）：
// 对话入口第一步调用 IncrAIUsage(ctx, 1)，ErrBudgetExceeded 即拒绝并映射为 ErrBudget。
//
// n<=0 → ErrValidation。
func (s *Store) IncrAIUsage(ctx context.Context, n int) (*AIUsage, error) {
	if n <= 0 {
		return nil, ErrValidation
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 读预算并锁 ai 行（与 ai_usage 的读写保持同一临界区）
	budget := defaultAIBudget
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE((value->>'budget_per_month')::int, $1) FROM settings WHERE key = $2 FOR UPDATE`,
		defaultAIBudget, KeyAI).Scan(&budget)
	if errors.Is(err, sql.ErrNoRows) {
		budget = defaultAIBudget
	} else if err != nil {
		return nil, err
	}

	// 读当前用量并锁 ai_usage 行
	month := ""
	count := 0
	err = tx.QueryRowContext(ctx,
		`SELECT value->>'month', COALESCE((value->>'count')::int, 0) FROM settings WHERE key = $1 FOR UPDATE`,
		KeyAIUsage).Scan(&month, &count)
	if errors.Is(err, sql.ErrNoRows) {
		month, count = "", 0
	} else if err != nil {
		return nil, err
	}

	curMonth := time.Now().Format("2006-01")
	if month != curMonth {
		count = 0
	}
	if count+n > budget {
		return &AIUsage{Month: curMonth, Count: count, Budget: budget}, ErrBudgetExceeded
	}
	count += n

	usage := AIUsage{Month: curMonth, Count: count, Budget: budget}
	raw, err := json.Marshal(map[string]any{"month": usage.Month, "count": usage.Count})
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, now())
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		KeyAIUsage, raw); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &usage, nil
}

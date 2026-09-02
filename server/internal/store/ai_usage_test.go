package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"testing"
	"time"
)

// cleanAISettings 删除 ai / ai_usage 设置行并重新种子，保证从干净状态开始。
func cleanAISettings(t *testing.T, st *Store) {
	t.Helper()
	ctx := context.Background()
	if _, err := st.db.ExecContext(ctx,
		`DELETE FROM settings WHERE key IN ('ai','ai_usage')`); err != nil {
		t.Fatalf("clean ai settings: %v", err)
	}
	if err := st.SeedSettings(ctx); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
}

// setAIBudget 把 ai 预算改为 budget（模拟管理台修改 budget_per_month）。
func setAIBudget(t *testing.T, st *Store, budget int) {
	t.Helper()
	if _, err := st.db.ExecContext(context.Background(),
		`UPDATE settings SET value = $1::jsonb WHERE key = 'ai'`,
		`{"budget_per_month":`+strconv.Itoa(budget)+`}`); err != nil {
		t.Fatalf("set ai budget: %v", err)
	}
}

// writeAIUsage 直接写入一条 ai_usage 记录（用于模拟跨月残留/满额）。
func writeAIUsage(t *testing.T, st *Store, month string, count int) {
	t.Helper()
	if _, err := st.db.ExecContext(context.Background(),
		`INSERT INTO settings (key, value) VALUES ('ai_usage', $1::jsonb)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		`{"month":"`+month+`","count":`+strconv.Itoa(count)+`}`); err != nil {
		t.Fatalf("write ai_usage: %v", err)
	}
}

// readUsageCount 直读库里 ai_usage 的 count，用于验证是否落库；无记录视为 0。
func readUsageCount(t *testing.T, st *Store) int {
	t.Helper()
	var n int
	err := st.db.QueryRowContext(context.Background(),
		`SELECT COALESCE((value->>'count')::int, 0) FROM settings WHERE key = 'ai_usage'`).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0
	}
	if err != nil {
		t.Fatalf("read ai_usage count: %v", err)
	}
	return n
}

func TestS12_IncrAIUsage_Increments(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cleanAISettings(t, st)

	u1, err := st.IncrAIUsage(ctx, 1)
	if err != nil {
		t.Fatalf("incr 1: %v", err)
	}
	if u1.Count != 1 || u1.Budget != 10 {
		t.Fatalf("after first = %+v, want count=1 budget=10", u1)
	}
	wantMonth := time.Now().Format("2006-01")
	if u1.Month != wantMonth {
		t.Fatalf("month = %s, want %s", u1.Month, wantMonth)
	}

	u2, err := st.IncrAIUsage(ctx, 2)
	if err != nil {
		t.Fatalf("incr 2: %v", err)
	}
	if u2.Count != 3 {
		t.Fatalf("count = %d, want 3", u2.Count)
	}
	if readUsageCount(t, st) != 3 {
		t.Fatalf("stored count != 3")
	}
}

func TestS12_IncrAIUsage_MonthlyReset(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cleanAISettings(t, st)

	// 残留上个月的满额用量
	writeAIUsage(t, st, "2000-01", 9)
	u, err := st.IncrAIUsage(ctx, 1)
	if err != nil {
		t.Fatalf("incr across month: %v", err)
	}
	if u.Count != 1 {
		t.Fatalf("跨月应重置后从 1 起算, got count=%d", u.Count)
	}
	if u.Month != time.Now().Format("2006-01") {
		t.Fatalf("month = %s, want current month", u.Month)
	}
	if readUsageCount(t, st) != 1 {
		t.Fatalf("stored count after reset != 1")
	}
}

func TestS12_IncrAIUsage_BudgetExceeded(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cleanAISettings(t, st)
	setAIBudget(t, st, 3)

	for i := 0; i < 3; i++ {
		if _, err := st.IncrAIUsage(ctx, 1); err != nil {
			t.Fatalf("incr %d: %v", i+1, err)
		}
	}
	u, err := st.IncrAIUsage(ctx, 1)
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("err = %v, want ErrBudgetExceeded", err)
	}
	if u == nil || u.Count != 3 || u.Budget != 3 {
		t.Fatalf("exceeded snapshot = %+v, want count=3 budget=3", u)
	}
	// 超额调用不落库
	if readUsageCount(t, st) != 3 {
		t.Fatalf("over-budget increment should not be recorded, stored=%d", readUsageCount(t, st))
	}
}

func TestS12_IncrAIUsage_BudgetResetAcrossMonth(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cleanAISettings(t, st)
	setAIBudget(t, st, 2)

	// 上月已满额，但跨月后重置，1 次调用应当放行
	writeAIUsage(t, st, "2000-01", 2)
	u, err := st.IncrAIUsage(ctx, 1)
	if err != nil {
		t.Fatalf("跨月后应放行, err = %v", err)
	}
	if u.Count != 1 {
		t.Fatalf("count = %d, want 1", u.Count)
	}
}

func TestS12_IncrAIUsage_ConcurrentAtomic(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cleanAISettings(t, st)
	setAIBudget(t, st, 100) // 放大预算，只测并发计数原子性

	const workers = 12
	results := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			_, err := st.IncrAIUsage(ctx, 1)
			results <- err
		}()
	}
	for i := 0; i < workers; i++ {
		if err := <-results; err != nil {
			t.Fatalf("concurrent incr: %v", err)
		}
	}
	if got := readUsageCount(t, st); got != workers {
		t.Fatalf("stored count = %d, want %d (并发增量不丢)", got, workers)
	}
}

func TestS12_IncrAIUsage_Validation(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cleanAISettings(t, st)
	for _, n := range []int{0, -1} {
		if _, err := st.IncrAIUsage(ctx, n); !errors.Is(err, ErrValidation) {
			t.Fatalf("incr(%d) err = %v, want ErrValidation", n, err)
		}
	}
}

func TestS12_IncrAIUsage_BudgetZero(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cleanAISettings(t, st)
	setAIBudget(t, st, 0) // 零预算 = 当月零配额

	u, err := st.IncrAIUsage(ctx, 1)
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("budget=0 首次增量 err = %v, want ErrBudgetExceeded", err)
	}
	if u == nil || u.Budget != 0 || u.Count != 0 {
		t.Fatalf("snapshot = %+v, want count=0 budget=0", u)
	}
	// 不落库，库里无 count 残留
	if got := readUsageCount(t, st); got != 0 {
		t.Fatalf("stored count = %d, want 0 (零预算不登记)", got)
	}
}

func TestS12_AIUsage_NotWritableSetting(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cleanAISettings(t, st)

	// 设置接口不可写 ai_usage
	if err := st.PutSetting(ctx, KeyAIUsage, []byte(`{"month":"2026-09","count":1}`)); !errors.Is(err, ErrValidation) {
		t.Fatalf("PutSetting(ai_usage) err = %v, want ErrValidation", err)
	}
	// 设置接口读不到 ai_usage
	got, err := st.GetSettings(ctx)
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if _, ok := got[KeyAIUsage]; ok {
		t.Fatal("ai_usage should not be exposed by GetSettings")
	}
}

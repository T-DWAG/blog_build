package store

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/T-DWAG/blog_build/server/internal/auth"
)

// newStoreTestStore 复用 S2 的 testStore，并种一个管理员。
var adminCounter int

// seedTestAdmin 种一个唯一用户名的管理员，返回 (admin, username)。
func seedTestAdmin(t *testing.T, st *Store, prefix string) (*Admin, string) {
	t.Helper()
	adminCounter++
	username := prefix + "-" + strconv.Itoa(adminCounter)
	hash, err := auth.Hash("correct-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := st.SeedAdmin(context.Background(), username, hash); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	admin, err := st.GetAdminByUsername(context.Background(), username)
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	return admin, username
}

func TestS3_RecordLoginFail_Increments(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	admin, u := seedTestAdmin(t, st, "s3_fail_inc")

	for i := 0; i < 2; i++ {
		if err := st.RecordLoginFail(ctx, admin.ID); err != nil {
			t.Fatalf("record fail: %v", err)
		}
	}
	got, err := st.GetAdminByUsername(ctx, u)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.FailedAttempts != 2 {
		t.Fatalf("failed_attempts = %d, want 2", got.FailedAttempts)
	}
	if got.LockedUntil != nil {
		t.Fatalf("locked_until should be nil before 5 fails, got %v", *got.LockedUntil)
	}
}

func TestS3_RecordLoginFail_LocksAt5(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	admin, u := seedTestAdmin(t, st, "s3_fail_lock")

	for i := 0; i < 5; i++ {
		if err := st.RecordLoginFail(ctx, admin.ID); err != nil {
			t.Fatalf("record fail %d: %v", i, err)
		}
	}
	got, err := st.GetAdminByUsername(ctx, u)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.FailedAttempts != 5 {
		t.Fatalf("failed_attempts = %d, want 5", got.FailedAttempts)
	}
	if got.LockedUntil == nil {
		t.Fatal("locked_until should be set after 5 fails")
	}
	if !got.LockedUntil.After(time.Now()) {
		t.Fatalf("locked_until %v should be in the future", *got.LockedUntil)
	}
}

func TestS3_RecordLoginOK_Clears(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	admin, u := seedTestAdmin(t, st, "s3_ok_clear")

	for i := 0; i < 3; i++ {
		if err := st.RecordLoginFail(ctx, admin.ID); err != nil {
			t.Fatalf("record fail: %v", err)
		}
	}
	if err := st.RecordLoginOK(ctx, admin.ID); err != nil {
		t.Fatalf("record ok: %v", err)
	}
	got, err := st.GetAdminByUsername(ctx, u)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.FailedAttempts != 0 {
		t.Fatalf("failed_attempts = %d, want 0", got.FailedAttempts)
	}
	if got.LockedUntil != nil {
		t.Fatalf("locked_until should be nil after OK, got %v", *got.LockedUntil)
	}
}

// Package auth — audit_retention_sweeper_test.go: AL-7.2 sweeper unit
// tests + reverse grep 约束 守 (跟 ap-2 expires_sweeper_test.go 同模式).
//
// Pins:
//
//	REG-AL7-001 TestAL_RunOnceArchivesExpired — 3 expired + 2 fresh → archived count=3
//	REG-AL7-002 TestAL_RunOnceSoftArchiveNotRealDelete — UPDATE archived_at, row stays
//	REG-AL7-003 TestAL_RunOnceIdempotent — second tick count==0
//	REG-AL7-004 TestAL_StartCtxShutdown — Start goroutine ctx-aware
//	REG-AL7-005 TestAL_NilSafeCtor — Store nil = no-op
//	REG-AL7-006 TestAL_SweeperReason_ByteIdentical — reasons.Unknown
package auth

import (
	"context"
	"testing"
	"time"

	"borgee-server/internal/agent/reasons"
	"borgee-server/internal/store"
)

// al7TestStore builds a memory store with admin_actions table including
// archived_at column (跟 al_7_1 migration v=33 shape match).
func al7TestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.DB().AutoMigrate(&store.AdminAction{}); err != nil {
		t.Fatal(err)
	}
	// ADMIN-SPA-SHAPE-FIX D4: store.AdminAction now has ArchivedAt field, so
	// AutoMigrate creates the archived_at column directly — no manual ALTER
	// needed (历史 patch reverted post-D4 struct surface).
	return s
}

// seedAction inserts an admin_actions row with the given created_at (ms).
func seedAction(t *testing.T, s *store.Store, id string, createdAtMs int64) {
	t.Helper()
	if err := s.DB().Exec(`INSERT INTO admin_actions
		(id, actor_id, target_user_id, action, metadata, created_at)
		VALUES (?, 'system', 'u-1', 'permission_expired', '', ?)`,
		id, createdAtMs).Error; err != nil {
		t.Fatalf("seed action %s: %v", id, err)
	}
}

// REG-AL7-001 — RunOnce archives rows older than RetentionDays cutoff.
func TestAL_RunOnceArchivesExpired(t *testing.T) {
	t.Parallel()
	s := al7TestStore(t)
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	clk := func() time.Time { return now }
	dayMs := int64(24 * 60 * 60 * 1000)

	// 3 expired (>14d old), 2 fresh (<14d).
	seedAction(t, s, "old-1", now.UnixMilli()-15*dayMs)
	seedAction(t, s, "old-2", now.UnixMilli()-30*dayMs)
	seedAction(t, s, "old-3", now.UnixMilli()-100*dayMs)
	seedAction(t, s, "fresh-1", now.UnixMilli()-1*dayMs)
	seedAction(t, s, "fresh-2", now.UnixMilli()-7*dayMs)

	sw := &RetentionSweeper{Store: s, Now: clk}
	count, err := sw.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if count != 3 {
		t.Errorf("count: got %d, want 3 (3 expired, 2 fresh)", count)
	}
}

// REG-AL7-002 — soft-archive: row stays in table, archived_at set;
// not real DELETE. 约束 ①.
func TestAL_RunOnceSoftArchiveNotRealDelete(t *testing.T) {
	t.Parallel()
	s := al7TestStore(t)
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	clk := func() time.Time { return now }
	dayMs := int64(24 * 60 * 60 * 1000)
	seedAction(t, s, "old-soft", now.UnixMilli()-30*dayMs)

	sw := &RetentionSweeper{Store: s, Now: clk}
	if _, err := sw.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	// Row still in table.
	var n int64
	s.DB().Raw(`SELECT COUNT(*) FROM admin_actions WHERE id='old-soft'`).Row().Scan(&n)
	if n != 1 {
		t.Errorf("row deleted (got count=%d, want 1) — must be soft-archive (UPDATE)", n)
	}

	// archived_at set to current 'now'.
	var archived *int64
	s.DB().Raw(`SELECT archived_at FROM admin_actions WHERE id='old-soft'`).Row().Scan(&archived)
	if archived == nil || *archived != now.UnixMilli() {
		t.Errorf("archived_at: got %v, want %d (now)", archived, now.UnixMilli())
	}
}

// REG-AL7-003 — second tick is no-op (already-archived rows excluded
// by WHERE archived_at IS NULL).
func TestAL_RunOnceIdempotent(t *testing.T) {
	t.Parallel()
	s := al7TestStore(t)
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	clk := func() time.Time { return now }
	dayMs := int64(24 * 60 * 60 * 1000)
	seedAction(t, s, "old-idem", now.UnixMilli()-30*dayMs)

	sw := &RetentionSweeper{Store: s, Now: clk}
	if c, _ := sw.RunOnce(context.Background()); c != 1 {
		t.Fatalf("first tick: got %d, want 1", c)
	}
	if c, _ := sw.RunOnce(context.Background()); c != 0 {
		t.Errorf("second tick: got %d, want 0 (already-archived rows excluded)", c)
	}
}

// REG-AL7-004 — Start ctx-aware shutdown (smoke).
func TestAL_StartCtxShutdown(t *testing.T) {
	t.Parallel()
	s := al7TestStore(t)
	sw := &RetentionSweeper{Store: s, Interval: 100 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	sw.Start(ctx)
	cancel()
	time.Sleep(50 * time.Millisecond)
	// no panic = ctx-aware shutdown healthy.
}

// REG-AL7-005 — nil-safe ctor (Store nil = no-op).
func TestAL_NilSafeCtor(t *testing.T) {
	t.Parallel()
	var sw *RetentionSweeper
	sw.Start(context.Background())
	if c, err := sw.RunOnce(context.Background()); c != 0 || err != nil {
		t.Errorf("nil sweeper: got count=%d err=%v, want 0/nil", c, err)
	}
	sw2 := &RetentionSweeper{}
	sw2.Start(context.Background())
	if c, err := sw2.RunOnce(context.Background()); c != 0 || err != nil {
		t.Errorf("nil-Store sweeper: got count=%d err=%v, want 0/nil", c, err)
	}
}

// REG-AL7-006 — SweeperReason byte-identical 跟 reasons.Unknown 同源.
// AL-1a reason 对齐第 15 处 (改 = 改 reasons.go + 此处 + 14 处 byte-
// identical 站点).
func TestAL_SweeperReason_ByteIdentical(t *testing.T) {
	t.Parallel()
	if SweeperReason != reasons.Unknown {
		t.Errorf("SweeperReason mismatch: got %q, want %q (= reasons.Unknown)",
			SweeperReason, reasons.Unknown)
	}
	if SweeperReason != "unknown" {
		t.Errorf("SweeperReason 字面 mismatch: got %q, want %q", SweeperReason, "unknown")
	}
	if RetentionDays != 14 {
		t.Errorf("RetentionDays mismatch: got %d, want 14 (蓝图 admin-model.md §3 字面单一来源)", RetentionDays)
	}
	if ActionAuditRetentionOverride != "audit_retention_override" {
		t.Errorf("ActionAuditRetentionOverride mismatch: got %q, want %q",
			ActionAuditRetentionOverride, "audit_retention_override")
	}
}

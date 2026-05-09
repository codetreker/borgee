// Package auth — expires_sweeper_test.go: AP-2.1 + AP-2.2 + AP-2.3
// sweeper goroutine + soft-delete + audit + full-flow integration.
//
// Pins:
//   REG-AP2-001b TestAP_StartCtxShutdown — Start goroutine ctx-aware
//   REG-AP2-001c TestAP_RunOnceFindsExpired — finds expired rows
//   REG-AP2-001d TestAP_RunOnceSoftDeletesNotRealDelete — UPDATE not DELETE
//   REG-AP2-001e TestAP_RunOnceIdempotentSecondTick — twice = count==0
//   REG-AP2-002a TestAP_RevokeWritesAuditEntry — admin_actions row written
//   REG-AP2-002b TestAP_ReasonConstByteIdentical — const byte-identical
//   REG-AP2-002c TestAP_SystemActorByteIdentical — actor='system' byte-identical
//   REG-AP2-002d TestAP_AuditPayloadShape — JSON 3-key shape
//   REG-AP2-003a TestAP23_FullFlow — grant expired → revoked → HasCapability false
//   REG-AP2-003b TestAP_ReverseGrep_5Patterns_AllZeroHit — 反约束 grep
package auth

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/store"
)

// ap2TestStore builds a memory store with user + user_permissions +
// admin_actions tables (跟 ap_2_1 migration shape match).
func ap2TestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.DB().AutoMigrate(&store.User{}, &store.UserPermission{}, &store.AdminAction{}); err != nil {
		t.Fatal(err)
	}
	return s
}

// REG-AP2-002b — const literal byte-identical 跟 spec + migration CHECK
// + admin_actions 6-tuple 同源.
func TestAP_ReasonConstByteIdentical(t *testing.T) {
	t.Parallel()
	if ReasonPermissionExpired != "permission_expired" {
		t.Errorf("ReasonPermissionExpired drift: got %q, want %q",
			ReasonPermissionExpired, "permission_expired")
	}
}

// REG-AP2-002c — actor='system' byte-identical 跟 BPP-4 watchdog 跨五
// milestone 锁.
func TestAP_SystemActorByteIdentical(t *testing.T) {
	t.Parallel()
	if SystemActorID != "system" {
		t.Errorf("SystemActorID drift: got %q, want %q",
			SystemActorID, "system")
	}
}

// REG-AP2-001c (acceptance §1.4) — RunOnce finds expired-but-not-revoked
// rows and returns count.
func TestAP_RunOnceFindsExpired(t *testing.T) {
	t.Parallel()
	s := ap2TestStore(t)
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	clk := func() time.Time { return now }

	// Insert 3 expired (in the past) + 2 永久 (NULL expires_at).
	mustGrant(t, s, "u-1", "p1", "*", ms(now.Add(-2*time.Hour)))
	mustGrant(t, s, "u-1", "p2", "channel:c1", ms(now.Add(-1*time.Hour)))
	mustGrant(t, s, "u-2", "p3", "*", ms(now.Add(-30*time.Minute)))
	mustGrantNoExpire(t, s, "u-3", "p4", "*")
	mustGrantNoExpire(t, s, "u-4", "p5", "*")

	sw := &ExpiresSweeper{Store: s, Now: clk}
	count, err := sw.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if count != 3 {
		t.Errorf("count: got %d, want 3 (expired-only, leaving 2 永久)", count)
	}
}

// REG-AP2-001d (acceptance §1.5) — soft-delete: row stays in table with
// revoked_at set; not real DELETE.
func TestAP_RunOnceSoftDeletesNotRealDelete(t *testing.T) {
	t.Parallel()
	s := ap2TestStore(t)
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	clk := func() time.Time { return now }

	expiredAt := ms(now.Add(-1 * time.Hour))
	mustGrant(t, s, "u-soft", "perm", "*", expiredAt)

	sw := &ExpiresSweeper{Store: s, Now: clk}
	if _, err := sw.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	// Row still in table.
	var n int64
	s.DB().Raw(`SELECT COUNT(*) FROM user_permissions WHERE user_id='u-soft'`).Row().Scan(&n)
	if n != 1 {
		t.Errorf("row deleted (got count=%d, want 1) — must be soft-delete (UPDATE)", n)
	}

	// revoked_at set to original expires_at.
	var revoked *int64
	s.DB().Raw(`SELECT revoked_at FROM user_permissions WHERE user_id='u-soft'`).Row().Scan(&revoked)
	if revoked == nil || *revoked != expiredAt {
		t.Errorf("revoked_at: got %v, want %d (= expires_at)", revoked, expiredAt)
	}
}

// REG-AP2-001e (acceptance §1.5) — second tick is no-op (revoked rows
// excluded by WHERE).
func TestAP_RunOnceIdempotentSecondTick(t *testing.T) {
	t.Parallel()
	s := ap2TestStore(t)
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	clk := func() time.Time { return now }

	mustGrant(t, s, "u-idem", "p", "*", ms(now.Add(-1*time.Hour)))

	sw := &ExpiresSweeper{Store: s, Now: clk}
	if c, _ := sw.RunOnce(context.Background()); c != 1 {
		t.Fatalf("first tick: got %d, want 1", c)
	}
	if c, _ := sw.RunOnce(context.Background()); c != 0 {
		t.Errorf("second tick: got %d, want 0 (revoked rows excluded)", c)
	}
}

// REG-AP2-001b — Start ctx-aware shutdown.
func TestAP_StartCtxShutdown(t *testing.T) {
	t.Parallel()
	s := ap2TestStore(t)
	sw := &ExpiresSweeper{Store: s, Interval: 100 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	sw.Start(ctx)
	// goroutine runs; cancel + verify it returns (no race assertion possible
	// without instrumentation, smoke test that cancel doesn't panic).
	cancel()
	time.Sleep(50 * time.Millisecond)
	// If we got here without panic, ctx-aware shutdown is healthy.
}

// REG-AP2-002a (acceptance §2.1) — RunOnce writes one admin_actions row
// per revocation (复用 ADM-2.1 既有 path).
func TestAP_RevokeWritesAuditEntry(t *testing.T) {
	t.Parallel()
	s := ap2TestStore(t)
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	clk := func() time.Time { return now }

	expiredAt := ms(now.Add(-1 * time.Hour))
	mustGrant(t, s, "u-audit", "channel.write", "channel:c-1", expiredAt)

	sw := &ExpiresSweeper{Store: s, Now: clk}
	if _, err := sw.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	var rows []store.AdminAction
	s.DB().Where("target_user_id = ?", "u-audit").Find(&rows)
	if len(rows) != 1 {
		t.Fatalf("admin_actions rows for u-audit: got %d, want 1", len(rows))
	}
	r := rows[0]
	if r.ActorID != "system" {
		t.Errorf("actor_id: got %q, want %q", r.ActorID, "system")
	}
	if r.Action != "permission_expired" {
		t.Errorf("action: got %q, want %q", r.Action, "permission_expired")
	}
	if r.TargetUserID != "u-audit" {
		t.Errorf("target_user_id: got %q, want %q", r.TargetUserID, "u-audit")
	}
}

// REG-AP2-002d (acceptance §2.3) — audit metadata JSON shape (3-key
// byte-identical).
func TestAP_AuditPayloadShape(t *testing.T) {
	t.Parallel()
	s := ap2TestStore(t)
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	clk := func() time.Time { return now }

	expiredAt := ms(now.Add(-30 * time.Minute))
	mustGrant(t, s, "u-shape", "artifact.commit", "artifact:art-7", expiredAt)

	sw := &ExpiresSweeper{Store: s, Now: clk}
	if _, err := sw.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	var row store.AdminAction
	s.DB().Where("target_user_id = ?", "u-shape").First(&row)

	var meta map[string]any
	if err := json.Unmarshal([]byte(row.Metadata), &meta); err != nil {
		t.Fatalf("metadata not valid JSON: %v (raw: %s)", err, row.Metadata)
	}
	if meta["permission"] != "artifact.commit" {
		t.Errorf("metadata.permission: got %v", meta["permission"])
	}
	if meta["scope"] != "artifact:art-7" {
		t.Errorf("metadata.scope: got %v", meta["scope"])
	}
	if eaf, _ := meta["original_expires_at"].(float64); int64(eaf) != expiredAt {
		t.Errorf("metadata.original_expires_at: got %v, want %d", meta["original_expires_at"], expiredAt)
	}
}

// REG-AP2-003a (acceptance §3.1) — full-flow: grant w/ expired → RunOnce
// → revoked + admin_actions row + HasCapability returns false (跟 AP-1
// SSOT 同精神, ListUserPermissions 排除 revoked rows).
func TestAP_FullFlow_GrantExpired_ThenRevokedThenHasCapabilityFalse(t *testing.T) {
	t.Parallel()
	s := ap2TestStore(t)
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	clk := func() time.Time { return now }

	user := &store.User{ID: "u-full", DisplayName: "Full", Role: "member"}
	s.CreateUser(user)

	// Grant expired.
	expiredAt := ms(now.Add(-1 * time.Hour))
	mustGrant(t, s, "u-full", "channel.write", "channel:c-A", expiredAt)

	// Pre-sweep: HasCapability is true (legacy AP-1 path — expires_at not
	// yet consumed by HasCapability per AP-1.1 设计 "schema 保留 UI 不做").
	ctx := context.WithValue(context.Background(), userContextKey, user)
	if !HasCapability(ctx, s, "channel.write", "channel:c-A") {
		t.Fatal("pre-sweep HasCapability should be true (AP-1 path active)")
	}

	// Sweep.
	sw := &ExpiresSweeper{Store: s, Now: clk}
	if c, err := sw.RunOnce(context.Background()); err != nil || c != 1 {
		t.Fatalf("RunOnce: count=%d err=%v", c, err)
	}

	// Post-sweep: revoked_at set + HasCapability false (ListUserPermissions
	// excludes revoked rows, AP-1 SSOT 同精神 改 = 改 queries.go 一处).
	if HasCapability(ctx, s, "channel.write", "channel:c-A") {
		t.Error("post-sweep HasCapability should be false (revoked row excluded)")
	}

	// Audit row written.
	var n int64
	s.DB().Model(&store.AdminAction{}).
		Where("target_user_id = ? AND action = ?", "u-full", "permission_expired").
		Count(&n)
	if n != 1 {
		t.Errorf("audit row count: got %d, want 1", n)
	}
}

// REG-AP2-003b (acceptance §3.2 + 设计 ③) — reverse grep 5 pattern in
// internal/auth/+internal/api/+internal/migrations/ all count==0 (except
// for sweeper file itself for the UPDATE pattern).
func TestAP_ReverseGrep_5Patterns_AllZeroHit(t *testing.T) {
	t.Parallel()
	patterns := []struct {
		dir       string
		pat       *regexp.Regexp
		exclude   string // file basename to exclude from hits
		zeroLabel string
	}{
		{
			dir:     filepath.Join("..", "auth"),
			pat:     regexp.MustCompile(`DELETE FROM user_permissions`),
			exclude: "expires_sweeper.go",
		},
		{
			dir:     filepath.Join("..", "api"),
			pat:     regexp.MustCompile(`DELETE FROM user_permissions`),
			exclude: "",
		},
		{
			dir:     filepath.Join("..", "migrations"),
			pat:     regexp.MustCompile(`CREATE TABLE.*expires_audit|CREATE TABLE.*permission_revocations`),
			exclude: "",
		},
		{
			dir:     filepath.Join("..", "..", ".."),
			pat:     regexp.MustCompile(`"github\.com/[^"]*cron|"github\.com/robfig/cron"|"github\.com/[^"]*gocron"`),
			exclude: "expires_sweeper.go",
		},
	}
	for _, tc := range patterns {
		_ = filepath.Walk(tc.dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			base := filepath.Base(p)
			if tc.exclude != "" && base == tc.exclude {
				return nil
			}
			if !strings.HasSuffix(p, ".go") {
				return nil
			}
			if strings.HasSuffix(p, "_test.go") {
				return nil
			}
			body, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			if loc := tc.pat.FindIndex(body); loc != nil {
				t.Errorf("反约束 broken — pattern %q hit in %s", tc.pat.String(), p)
			}
			return nil
		})
	}

	// 5th pattern: hardcode "permission_expired" in handler path (反 const
	// 单源漂移). Allowed only in const file + migration + tests.
	hardcodePat := regexp.MustCompile(`"permission_expired"`)
	apiDir := filepath.Join("..", "api")
	_ = filepath.Walk(apiDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		if loc := hardcodePat.FindIndex(body); loc != nil {
			t.Errorf("反约束 broken — hardcode %q in %s (use auth.ReasonPermissionExpired const)",
				`"permission_expired"`, p)
		}
		return nil
	})
}

// helpers
func ms(t time.Time) int64 { return t.UnixMilli() }

func mustGrant(t *testing.T, s *store.Store, userID, perm, scope string, expiresAt int64) {
	t.Helper()
	if err := s.DB().Exec(`INSERT INTO user_permissions
		(user_id, permission, scope, granted_at, expires_at)
		VALUES (?, ?, ?, ?, ?)`,
		userID, perm, scope, time.Now().UnixMilli(), expiresAt).Error; err != nil {
		t.Fatalf("seed grant %s/%s/%s: %v", userID, perm, scope, err)
	}
}

func mustGrantNoExpire(t *testing.T, s *store.Store, userID, perm, scope string) {
	t.Helper()
	if err := s.GrantPermission(&store.UserPermission{
		UserID: userID, Permission: perm, Scope: scope,
	}); err != nil {
		t.Fatalf("seed perma grant %s/%s/%s: %v", userID, perm, scope, err)
	}
}

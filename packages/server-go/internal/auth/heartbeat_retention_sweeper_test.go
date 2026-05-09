// Package auth — heartbeat_retention_sweeper_test.go: HB-5.2 sweeper
// unit tests + reverse grep 反向约束 守 (跟 al-7 audit_retention_sweeper_
// test.go 同模式).
//
// Pins:
//   REG-HB5-001 TestHB_RunOnceArchivesExpired
//   REG-HB5-002 TestHB_RunOnceSoftArchiveNotRealDelete
//   REG-HB5-003 TestHB_RunOnceIdempotent
//   REG-HB5-004 TestHB_StartCtxShutdown
//   REG-HB5-005 TestHB_NilSafeCtor
//   REG-HB5-006 TestHB_SweeperReason_ByteIdentical + TestHB_NoHeartbeatRetentionQueue
package auth

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/agent/reasons"
	"borgee-server/internal/store"
)

// hb5TestStore builds a memory store with agent_state_log table including
// archived_at column (跟 hb_5_1 migration v=34 shape match).
func hb5TestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.DB().Exec(`CREATE TABLE IF NOT EXISTS agent_state_log (
  id          INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  agent_id    TEXT    NOT NULL,
  from_state  TEXT    NOT NULL,
  to_state    TEXT    NOT NULL,
  reason      TEXT    NOT NULL DEFAULT '',
  task_id     TEXT    NOT NULL DEFAULT '',
  ts          INTEGER NOT NULL,
  archived_at INTEGER
)`).Error; err != nil {
		t.Fatal(err)
	}
	return s
}

// seedStateRow inserts an agent_state_log row with given ts (ms).
func seedStateRow(t *testing.T, s *store.Store, agentID string, tsMs int64) {
	t.Helper()
	if err := s.DB().Exec(`INSERT INTO agent_state_log
		(agent_id, from_state, to_state, reason, task_id, ts)
		VALUES (?, '', 'online', '', '', ?)`,
		agentID, tsMs).Error; err != nil {
		t.Fatalf("seed state row %s: %v", agentID, err)
	}
}

// REG-HB5-001 — RunOnce archives rows older than HeartbeatRetentionDays cutoff.
func TestHB_RunOnceArchivesExpired(t *testing.T) {
	s := hb5TestStore(t)
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	clk := func() time.Time { return now }
	dayMs := int64(24 * 60 * 60 * 1000)

	// 3 expired (>30d), 2 fresh (<30d).
	seedStateRow(t, s, "agent-old-1", now.UnixMilli()-31*dayMs)
	seedStateRow(t, s, "agent-old-2", now.UnixMilli()-60*dayMs)
	seedStateRow(t, s, "agent-old-3", now.UnixMilli()-100*dayMs)
	seedStateRow(t, s, "agent-fresh-1", now.UnixMilli()-7*dayMs)
	seedStateRow(t, s, "agent-fresh-2", now.UnixMilli()-29*dayMs)

	sw := &HeartbeatRetentionSweeper{Store: s, Now: clk}
	count, err := sw.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if count != 3 {
		t.Errorf("count: got %d, want 3 (3 expired, 2 fresh)", count)
	}
}

// REG-HB5-002 — soft-archive: row stays + archived_at set; not real DELETE.
func TestHB_RunOnceSoftArchiveNotRealDelete(t *testing.T) {
	s := hb5TestStore(t)
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	clk := func() time.Time { return now }
	dayMs := int64(24 * 60 * 60 * 1000)
	seedStateRow(t, s, "agent-soft", now.UnixMilli()-60*dayMs)

	sw := &HeartbeatRetentionSweeper{Store: s, Now: clk}
	if _, err := sw.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	var n int64
	s.DB().Raw(`SELECT COUNT(*) FROM agent_state_log WHERE agent_id='agent-soft'`).Row().Scan(&n)
	if n != 1 {
		t.Errorf("row deleted (got %d, want 1) — must be soft-archive", n)
	}

	var archived *int64
	s.DB().Raw(`SELECT archived_at FROM agent_state_log WHERE agent_id='agent-soft'`).Row().Scan(&archived)
	if archived == nil || *archived != now.UnixMilli() {
		t.Errorf("archived_at: got %v, want %d", archived, now.UnixMilli())
	}
}

// REG-HB5-003 — second tick is no-op.
func TestHB_RunOnceIdempotent(t *testing.T) {
	s := hb5TestStore(t)
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	clk := func() time.Time { return now }
	dayMs := int64(24 * 60 * 60 * 1000)
	seedStateRow(t, s, "agent-idem", now.UnixMilli()-60*dayMs)

	sw := &HeartbeatRetentionSweeper{Store: s, Now: clk}
	if c, _ := sw.RunOnce(context.Background()); c != 1 {
		t.Fatalf("first tick: got %d, want 1", c)
	}
	if c, _ := sw.RunOnce(context.Background()); c != 0 {
		t.Errorf("second tick: got %d, want 0", c)
	}
}

// REG-HB5-004 — Start ctx-aware shutdown.
func TestHB_StartCtxShutdown(t *testing.T) {
	s := hb5TestStore(t)
	sw := &HeartbeatRetentionSweeper{Store: s, Interval: 100 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	sw.Start(ctx)
	cancel()
	time.Sleep(50 * time.Millisecond)
}

// REG-HB5-005 — nil-safe ctor.
func TestHB_NilSafeCtor(t *testing.T) {
	var sw *HeartbeatRetentionSweeper
	sw.Start(context.Background())
	if c, err := sw.RunOnce(context.Background()); c != 0 || err != nil {
		t.Errorf("nil sweeper: got count=%d err=%v", c, err)
	}
	sw2 := &HeartbeatRetentionSweeper{}
	sw2.Start(context.Background())
	if c, err := sw2.RunOnce(context.Background()); c != 0 || err != nil {
		t.Errorf("nil-Store sweeper: got count=%d err=%v", c, err)
	}
}

// REG-HB5-006 — const byte-identical 跟 reasons.Unknown 同源 (AL-1a 守护链
// 第 17 处 — AL-7 #15 + AL-8 #16 不变不脱节).
func TestHB_SweeperReason_ByteIdentical(t *testing.T) {
	if HeartbeatSweeperReason != reasons.Unknown {
		t.Errorf("HeartbeatSweeperReason 脱节: got %q, want %q",
			HeartbeatSweeperReason, reasons.Unknown)
	}
	if HeartbeatSweeperReason != "unknown" {
		t.Errorf("HeartbeatSweeperReason 字面脱节: got %q", HeartbeatSweeperReason)
	}
	if HeartbeatRetentionDays != 30 {
		t.Errorf("HeartbeatRetentionDays 脱节: got %d, want 30", HeartbeatRetentionDays)
	}
	if HeartbeatTargetLabel != "heartbeat" {
		t.Errorf("HeartbeatTargetLabel 脱节: got %q", HeartbeatTargetLabel)
	}
	// 原则 ② — 复用 AL-7 既有 ActionAuditRetentionOverride const, 不另起.
	if ActionAuditRetentionOverride != "audit_retention_override" {
		t.Errorf("ActionAuditRetentionOverride 脱节: got %q", ActionAuditRetentionOverride)
	}
}

// REG-HB5-006b — 原则 ④ + 原则 ⑤ 反向 grep: cron framework + retention
// queue tokens 0 hit in this file.
func TestHB_NoCronFrameworkImport(t *testing.T) {
	body, err := os.ReadFile("heartbeat_retention_sweeper.go")
	if err != nil {
		t.Fatalf("read sweeper: %v", err)
	}
	for _, pat := range []string{`"github.com/robfig/cron`, `"github.com/go-co-op/gocron`, `gocron.`} {
		if strings.Contains(string(body), pat) {
			t.Errorf("反向约束 broken — cron import %q in heartbeat_retention_sweeper.go", pat)
		}
	}
}

// REG-HB5-006c — 原则 ⑤ AST 守护链延伸第 9 处 forbidden-token 0 hit.
//
// Scans internal/auth + internal/api production *.go (excluding tests)
// for heartbeat retention queue / dead-letter tokens (跟 AL-7 守护链延伸
// 第 7 处 + AL-8 第 8 处 同模式).
func TestHB_NoHeartbeatRetentionQueue(t *testing.T) {
	forbidden := []string{
		"pendingHeartbeatRetention",
		"heartbeatRetentionRetryQueue",
		"deadLetterHeartbeatRetention",
	}
	dirs := []string{
		filepath.Join("..", "auth"),
		filepath.Join("..", "api"),
	}
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			body, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			for _, tok := range forbidden {
				if strings.Contains(string(body), tok) {
					t.Errorf("AST 守护链延伸第 9 处 broken — forbidden token %q in %s", tok, p)
				}
			}
			return nil
		})
	}
}

// REG-HB5-cov — explicit-set retentionDays/interval/now branches.
func TestHB_SweeperFieldOverrides(t *testing.T) {
	t.Parallel()
	customNow := func() time.Time { return time.UnixMilli(1700000000000) }
	s := &HeartbeatRetentionSweeper{
		Interval:      5 * time.Hour,
		RetentionDays: 60,
		Now:           customNow,
	}
	if got := s.interval(); got != 5*time.Hour {
		t.Errorf("interval override: got %v, want 5h", got)
	}
	if got := s.retentionDays(); got != 60 {
		t.Errorf("retentionDays override: got %d, want 60", got)
	}
	if got := s.now(); got.UnixMilli() != 1700000000000 {
		t.Errorf("now override: got %v, want 1700000000000", got.UnixMilli())
	}
}

// REG-HB5-cov v3 — Now=nil branch (covers default time.Now() path).
func TestHB_NowDefaultBranch(t *testing.T) {
	t.Parallel()
	s := &HeartbeatRetentionSweeper{} // Now=nil
	got := s.now()
	if got.IsZero() {
		t.Error("default now() should return non-zero time")
	}
}

// REG-HB5-cov v3 — Start with nil store (early return covers nil-safe path).
func TestHB_StartNilStoreReturns(t *testing.T) {
	t.Parallel()
	s := &HeartbeatRetentionSweeper{} // Store=nil
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.Start(ctx) // should return immediately, not panic
}

// REG-HB5-cov v3 — Start nil receiver (covers s==nil branch).
func TestHB_StartNilReceiver(t *testing.T) {
	t.Parallel()
	var s *HeartbeatRetentionSweeper
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.Start(ctx) // should return immediately
}

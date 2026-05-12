// Package auth — heartbeat_retention_sweeper.go: HB-5.2 archived_at
// soft-archive sweeper for agent_state_log, using time.Ticker instead of a
// scheduler framework and best-effort logging.
//
// Blueprint: agent-lifecycle.md §2.3 forward-only state log + AL-7 #533
// archived_at retention pattern. Spec: docs/implementation/modules/
// hb-5-spec.md (v0) §0 principle 1 + §1 HB-5.2.
//
// What this does (one round-trip closes the HB-5 retention loop):
//
//   - On each tick (1h DefaultRetentionInterval reused from AL-7) UPDATE
//     agent_state_log SET archived_at = now WHERE ts < (now -
//     HeartbeatRetentionDays*24h) AND archived_at IS NULL.
//   - Soft-archives only: UPDATE, never DELETE (reverse-grep reference
//     DELETE FROM agent_state_log has zero production hits; this matches the
//     AL-1 and AL-7 forward-only rule).
//   - Does not add a separate archive table: agent_state_log.archived_at is
//     the single source (reverse-grep references such as heartbeat_archive_table
//     have zero hits).
//   - Does not add a scheduler framework: time.Ticker matches AP-2 / AL-7 sweepers.
//   - Reuses the AL-1a six-reason set: HeartbeatSweeperReason = reasons.Unknown
//     stays byte-identical with AL-7 SweeperReason.
//
// Public surface (nil-safe like AL-7 RetentionSweeper):
//   - HeartbeatRetentionSweeper{Store, Logger, RetentionDays, Interval, Now}
//   - (s *HeartbeatRetentionSweeper) Start(ctx) — goroutine 1h ticker.
//   - (s *HeartbeatRetentionSweeper) RunOnce(ctx) (count int, err error).
//
// Constraints (hb-5-spec.md §0 + principles 1, 4, 5, and 6):
//   - Do not delete rows: write archived_at with UPDATE, never DELETE.
//   - Do not split data into a new table: reuse agent_state_log.
//   - Do not introduce a scheduler framework: use time.Ticker only.
//   - Do not open a retention queue: forbidden-token scans stay at zero hits.
//   - Keep the 30-day heartbeat retention literal single-sourced:
//     HeartbeatRetentionDays = 30.
package auth

import (
	"context"
	"log/slog"
	"time"

	"borgee-server/internal/agent/reasons"
	"borgee-server/internal/store"
)

// HeartbeatRetentionDays is the default heartbeat/agent_state_log
// retention window in days. Blueprint hb-5-spec.md §0.6 pins the 30d literal
// because heartbeat rows are more frequent than audit rows and should cover a
// rolling month. Admin override (POST
// /admin-api/v1/heartbeat-retention/override) writes one admin_actions
// row reusing AL-7 'audit_retention_override' action with metadata
// target='heartbeat' to keep the enum aligned.
const HeartbeatRetentionDays = 30

// HeartbeatTargetLabel is the byte-identical metadata.target literal
// written by HB-5 admin override; it is the counterpart to the AL-7 audit
// override target='admin_actions'. HB-5 intentionally reuses the AL-7 action.
const HeartbeatTargetLabel = "heartbeat"

// HeartbeatSweeperReason is the AL-1a 6-dict byte-identical const
// referenced by the heartbeat retention sweeper. This is another entry in the
// AL-1a reason alignment chain. HB-5 does not create a new reason dictionary;
// it reuses reasons.Unknown.
const HeartbeatSweeperReason = reasons.Unknown

// HeartbeatRetentionSweeper periodically archives expired agent_state_log
// rows by UPDATE archived_at = now (forward-only soft-archive, never a real delete).
//
// All fields optional (nil-safe). Pattern mirrors AL-7 RetentionSweeper
// #533 for cross-milestone consistency.
type HeartbeatRetentionSweeper struct {
	Store         *store.Store
	Logger        *slog.Logger
	RetentionDays int
	Interval      time.Duration
	Now           func() time.Time
}

func (s *HeartbeatRetentionSweeper) interval() time.Duration {
	if s.Interval <= 0 {
		return DefaultRetentionInterval
	}
	return s.Interval
}

func (s *HeartbeatRetentionSweeper) retentionDays() int {
	if s.RetentionDays <= 0 {
		return HeartbeatRetentionDays
	}
	return s.RetentionDays
}

func (s *HeartbeatRetentionSweeper) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Start launches the sweeper goroutine with nil-safe, ctx-aware shutdown like
// AL-7 RetentionSweeper.
func (s *HeartbeatRetentionSweeper) Start(ctx context.Context) {
	if s == nil || s.Store == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(s.interval())
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := s.RunOnce(ctx); err != nil && s.Logger != nil {
					s.Logger.Warn("hb5.heartbeat_retention_sweeper.run_once_failed",
						"error", err.Error(),
						"reason", HeartbeatSweeperReason)
				}
			}
		}
	}()
}

// RunOnce performs one full sweep cycle synchronously. Returns the
// number of rows archived. Idempotent — second call within the same
// instant returns count==0 (already-archived rows excluded by WHERE
// archived_at IS NULL).
//
// Principle 1: UPDATE, not DELETE (forward-only soft-archive). Reverse-grep
// reference `DELETE FROM agent_state_log` has zero hits in production *.go files.
func (s *HeartbeatRetentionSweeper) RunOnce(ctx context.Context) (int, error) {
	if s == nil || s.Store == nil {
		return 0, nil
	}
	nowMs := s.now().UnixMilli()
	cutoff := nowMs - int64(s.retentionDays())*24*60*60*1000

	res := s.Store.DB().WithContext(ctx).Exec(
		`UPDATE agent_state_log SET archived_at = ?
		 WHERE ts < ? AND archived_at IS NULL`,
		nowMs, cutoff)
	if res.Error != nil {
		return 0, res.Error
	}
	return int(res.RowsAffected), nil
}

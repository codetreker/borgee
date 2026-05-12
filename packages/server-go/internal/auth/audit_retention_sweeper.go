// Package auth — audit_retention_sweeper.go: AL-7.2 archived_at
// soft-archive sweeper, using time.Ticker instead of cron and best-effort
// logging.
//
// Blueprint: admin-model.md §3 retention + ADM-2.1 #484 forward-only audit
// retention close-out. Spec: docs/implementation/modules/al-7-spec.md
// (v0 3fa2db0) §0 principle 1 + §1 AL-7.2.
//
// What this does (one round-trip closes the AL-7 retention loop):
//
//   - On each tick (1h DefaultRetentionInterval) UPDATE admin_actions
//     SET archived_at = now WHERE created_at < (now - RetentionDays*24h)
//     AND archived_at IS NULL.
//   - Soft-archives only: UPDATE, never DELETE (reverse-grep reference
//     `DELETE FROM admin_actions` has zero production hits; this matches the
//     ADM-2.1 and AP-2 forward-only rule).
//   - Does not add a separate archive table: admin_actions.archived_at is the
//     single source (reverse-grep reference
//     `audit_archive_table\|audit_history_log\|al7_archive_log` has zero hits).
//   - Does not add a scheduler framework: time.Ticker matches AP-2
//     ExpiresSweeper; scheduler imports have zero hits in this file.
//   - Reuses the AL-1a six-reason set: SweeperReason = reasons.Unknown remains
//     byte-identical with that chain.
//
// Public surface (nil-safe like AP-2 ExpiresSweeper):
//   - RetentionSweeper{Store, Logger, RetentionDays, Interval, Now} — config
//   - (s *RetentionSweeper) Start(ctx) — goroutine 1h ticker, ctx-aware
//     shutdown, nil-safe for Store and Logger.
//   - (s *RetentionSweeper) RunOnce(ctx) (count int, err error) — single
//     synchronous sweep entry point for tests.
//
// Constraints (al-7-spec.md §0 + principles 1, 4, 5, and 6):
//   - Do not delete rows: write archived_at with UPDATE, never DELETE.
//   - Do not split data into a new table: reuse admin_actions.
//   - Do not introduce a scheduler framework: use time.Ticker only.
//   - Do not open a retention queue: forbidden-token scans stay at zero hits.
//   - Keep the 14-day literal single-sourced: RetentionDays = 14.
package auth

import (
	"context"
	"log/slog"
	"time"

	"borgee-server/internal/agent/reasons"
	"borgee-server/internal/store"
)

// RetentionDays is the default audit retention window in days. Blueprint
// admin-model.md §3 pins the 14d literal. Admin override (POST /admin-api/v1/audit-
// retention/override) writes one admin_actions row and updates the
// in-memory effective window via the handler — not via mutating this
// const (compile-time single source for the literal).
const RetentionDays = 14

// RetentionMinDays / RetentionMaxDays clamp the admin override endpoint range
// (1d minimum rejects zero or negative values; 365d maximum is a 1y cap).
const (
	RetentionMinDays = 1
	RetentionMaxDays = 365
)

// DefaultRetentionInterval is the sweeper tick. The 1h interval matches AP-2
// ExpiresSweeper; blueprint §3 does not require real-time retention, only an
// asynchronous soft-archive stamp.
const DefaultRetentionInterval = 1 * time.Hour

// SweeperReason is the AL-1a 6-dict byte-identical const referenced by
// the retention sweeper. This is another entry in the AL-1a reason alignment
// chain. Do not create a new reason dictionary here: the sweeper is
// best-effort and intentionally does not distinguish sub-reasons.
const SweeperReason = reasons.Unknown

// ActionAuditRetentionOverride is the admin_actions.action literal kept
// byte-identical with the al_7_1 migration CHECK 12-tuple; changing it requires
// changing both this const and the migration CHECK.
const ActionAuditRetentionOverride = "audit_retention_override"

// RetentionSweeper periodically archives expired admin_actions rows by
// UPDATE archived_at = now (forward-only soft-archive, never a real delete).
//
// All fields optional (nil-safe — Logger nil = silent; Now nil =
// time.Now; Interval 0 = DefaultRetentionInterval; RetentionDays 0 =
// RetentionDays const). Pattern mirrors AP-2 ExpiresSweeper #525 for
// cross-milestone consistency.
type RetentionSweeper struct {
	Store         *store.Store
	Logger        *slog.Logger
	RetentionDays int
	Interval      time.Duration
	Now           func() time.Time
}

func (s *RetentionSweeper) interval() time.Duration {
	if s.Interval <= 0 {
		return DefaultRetentionInterval
	}
	return s.Interval
}

func (s *RetentionSweeper) retentionDays() int {
	if s.RetentionDays <= 0 {
		return RetentionDays
	}
	return s.RetentionDays
}

func (s *RetentionSweeper) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Start launches the sweeper goroutine. Returns immediately. Goroutine
// runs RunOnce on each tick until ctx cancellation, then returns.
// Pattern mirrors AP-2 ExpiresSweeper #525 nil-safe ctx-aware shutdown.
func (s *RetentionSweeper) Start(ctx context.Context) {
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
					s.Logger.Warn("al7.retention_sweeper.run_once_failed",
						"error", err.Error(),
						"reason", SweeperReason)
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
// reference `DELETE FROM admin_actions` has zero hits in production *.go files.
func (s *RetentionSweeper) RunOnce(ctx context.Context) (int, error) {
	if s == nil || s.Store == nil {
		return 0, nil
	}
	nowMs := s.now().UnixMilli()
	cutoff := nowMs - int64(s.retentionDays())*24*60*60*1000

	// Step — soft-archive: UPDATE archived_at = now WHERE created_at < cutoff
	// AND archived_at IS NULL. Principle 1: not DELETE.
	res := s.Store.DB().WithContext(ctx).Exec(
		`UPDATE admin_actions SET archived_at = ?
		 WHERE created_at < ? AND archived_at IS NULL`,
		nowMs, cutoff)
	if res.Error != nil {
		return 0, res.Error
	}
	return int(res.RowsAffected), nil
}

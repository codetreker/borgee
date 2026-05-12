// Package auth — expires_sweeper.go: AP-2 expires_at sweeper
// goroutine + soft-delete revoke + audit log.
//
// AP-1.1 #493 schema reserved `user_permissions.expires_at INTEGER NULL`
// (NULL = permanent). AP-2 (v0) closes the runtime loop — periodic
// sweeper goroutine scans for expired-but-not-yet-revoked grants, writes
// `revoked_at = expires_at` (NOT real DELETE; it follows the same
// forward-only audit model as AL-1 #492 state_log and ADM-2.1 #484
// admin_actions), and emits one `admin_actions` audit row per revocation
// instead of creating an expires_audit table.
//
// Spec: docs/implementation/modules/ap-2-spec.md (v0, cfa3869)
// §0 design points 1, 2, and 3 + §1 AP-2.1 and AP-2.2.
// Stance checklist: docs/qa/ap-2-stance-checklist.md.
// Acceptance: docs/qa/acceptance-templates/ap-2.md §1.1-§3.3.
//
// Public surface:
//   - ExpiresSweeper{Store, Logger, Interval, Now} — config struct
//   - (s *ExpiresSweeper) Start(ctx) — starts a goroutine with a 1h ticker,
//     ctx-aware shutdown, and nil-safe behavior like the AL-1b agent_status sweeper.
//   - (s *ExpiresSweeper) RunOnce(ctx) (count int, err error) — single
//     synchronous sweep entry point used by tests and by Start's loop.
//
// Constraints (ap-2-spec.md §3 + design points 1, 3, 7, and 8):
//   - Do not delete rows: UPDATE user_permissions SET revoked_at = ?.
//     Reverse-grep reference `DELETE FROM user_permissions` remains zero-hit
//     in internal/auth and internal/api outside this file.
//   - Do not create an expires_audit table: reuse admin_actions (the existing
//     ADM-2.1 #484 path).
//   - Do not add a cron framework: use time.Ticker like the AL-1b
//     agent_status sweeper; reverse-grep reference `cron|gocron` stays zero-hit.
//   - Do not route through admin override behavior: automated revokes use the
//     actor='system' literal, while active admin revokes use the separate ADM-3+ path.
//   - Do not use time.Sleep: use the ticker.
package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"borgee-server/internal/store"
)

// ReasonPermissionExpired is the byte-identical action const written to
// admin_actions.action when the sweeper revokes an expired grant.
// It is kept aligned with the ap_2_1_user_permissions_revoked migration v=30
// admin_actions CHECK 6-tuple; changing it requires changing both this const
// and the migration CHECK.
const ReasonPermissionExpired = "permission_expired"

// SystemActorID is the actor_id literal written by automated server-side
// processes (sweeper, watchdog). It stays byte-identical with the BPP-4
// watchdog system actor and other automated audit writers.
const SystemActorID = "system"

// DefaultSweeperInterval is the periodic sweep tick. Blueprint §5 requires a
// periodic sweep, not real-time behavior. The 1h interval matches the AL-1b
// agent_status stale-detect cadence and can become configurable in v2+.
const DefaultSweeperInterval = 1 * time.Hour

// ExpiresSweeper periodically revokes user_permissions rows whose
// expires_at has passed. Design point 1: forward-only soft-delete via
// revoked_at + audit row.
//
// All fields optional (nil-safe — Logger nil = silent; Now nil =
// time.Now; Interval 0 = DefaultSweeperInterval). Pattern mirrors
// AL-1b agent_status sweeper (#458) for cross-milestone consistency.
type ExpiresSweeper struct {
	Store    *store.Store
	Logger   *slog.Logger
	Interval time.Duration
	Now      func() time.Time
}

func (s *ExpiresSweeper) interval() time.Duration {
	if s.Interval <= 0 {
		return DefaultSweeperInterval
	}
	return s.Interval
}

func (s *ExpiresSweeper) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Start launches the sweeper goroutine. Returns immediately. Goroutine
// runs RunOnce on each tick until ctx cancellation, then returns.
// Pattern mirrors AL-1b agent_status sweeper #458 nil-safe ctx-aware
// shutdown.
func (s *ExpiresSweeper) Start(ctx context.Context) {
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
					s.Logger.Warn("ap2.expires_sweeper.run_once_failed",
						"error", err.Error())
				}
			}
		}
	}()
}

// expiredRow is the projection used by RunOnce. We need only the
// columns required for audit metadata + the UPDATE WHERE.
type expiredRow struct {
	ID         uint
	UserID     string `gorm:"column:user_id"`
	Permission string
	Scope      string
	ExpiresAt  *int64 `gorm:"column:expires_at"`
}

// RunOnce performs one full sweep cycle synchronously. Returns the
// number of rows revoked. Idempotent — second call within the same
// instant returns count==0 (revoked rows are excluded by WHERE).
//
// Acceptance §1.4 — testable sync entry point.
func (s *ExpiresSweeper) RunOnce(ctx context.Context) (int, error) {
	if s == nil || s.Store == nil {
		return 0, nil
	}
	nowMs := s.now().UnixMilli()

	// Step 1 — find expired-but-not-yet-revoked rows.
	var rows []expiredRow
	if err := s.Store.DB().WithContext(ctx).
		Raw(`SELECT id, user_id, permission, scope, expires_at
		     FROM user_permissions
		     WHERE expires_at IS NOT NULL
		       AND expires_at < ?
		       AND revoked_at IS NULL`, nowMs).
		Scan(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	// Step 2 — soft-delete: write revoked_at = expires_at (forward-only,
	// never a real row delete). Design point 1: UPDATE not DELETE.
	revoked := 0
	for _, r := range rows {
		var revokedAt int64
		if r.ExpiresAt != nil {
			revokedAt = *r.ExpiresAt
		} else {
			// Defensive — WHERE clause already filters NULL, but cover.
			revokedAt = nowMs
		}
		if err := s.Store.DB().WithContext(ctx).
			Exec(`UPDATE user_permissions SET revoked_at = ?
			      WHERE id = ? AND revoked_at IS NULL`,
				revokedAt, r.ID).Error; err != nil {
			return revoked, err
		}

		// Step 3 — write audit row through ADM-2.1 InsertAdminAction. Design
		// point 2: do not create an expires_audit table. Design point 4: keep
		// actor='system' and action 'permission_expired' byte-identical with the
		// admin_actions CHECK 6-tuple.
		meta, err := json.Marshal(map[string]any{
			"permission":          r.Permission,
			"scope":               r.Scope,
			"original_expires_at": revokedAt,
		})
		if err != nil {
			return revoked, err
		}
		if _, err := s.Store.InsertAdminAction(
			SystemActorID, r.UserID, ReasonPermissionExpired, string(meta),
		); err != nil {
			return revoked, err
		}
		revoked++
	}
	return revoked, nil
}

// Package auth — expires_sweeper.go: AP-2 设计 ① expires_at sweeper
// goroutine + soft-delete revoke + audit log.
//
// AP-1.1 #493 schema reserved `user_permissions.expires_at INTEGER NULL`
// (NULL = 永久). AP-2 (战马C v0) closes the runtime loop — periodic
// sweeper goroutine scans for expired-but-not-yet-revoked grants, writes
// `revoked_at = expires_at` (NOT real DELETE — forward-only audit, 跟
// AL-1 #492 state_log + ADM-2.1 #484 admin_actions 同精神), and emits
// one `admin_actions` audit row per revocation (复用 ADM-2.1 既有 path,
// 不另起 expires_audit 表).
//
// Spec: docs/implementation/modules/ap-2-spec.md (战马C v0, cfa3869)
// §0 设计 ①②③ + §1 拆段 AP-2.1 + AP-2.2.
// 约定: docs/qa/ap-2-stance-checklist.md (8 设计).
// Acceptance: docs/qa/acceptance-templates/ap-2.md §1.1-§3.3.
//
// Public surface:
//   - ExpiresSweeper{Store, Logger, Interval, Now} — config struct
//   - (s *ExpiresSweeper) Start(ctx) — goroutine 启动 (1h ticker, ctx-aware
//     shutdown 跟 AL-1b agent_status sweeper 同精神 nil-safe)
//   - (s *ExpiresSweeper) RunOnce(ctx) (count int, err error) — 单次扫
//     描入口 (testable 同步 path, Start 内部循环走此)
//
// 约束 (ap-2-spec.md §3 + 设计 ①③⑦⑧):
//   - 不真删 row — UPDATE user_permissions SET revoked_at = ? (反向 grep
//     `DELETE FROM user_permissions` 在 internal/auth/+internal/api/ 除
//     此文件 count==0)
//   - 不另起 expires_audit 表 — 复用 admin_actions (ADM-2.1 #484 既有 path)
//   - 不引入 cron 框架 — 用 time.Ticker (跟 AL-1b agent_status sweeper
//     同模式; 反向 grep `cron|gocron` count==0)
//   - admin god-mode 不入此 path — actor='system' 字面 (跟 BPP-4 watchdog
//     actor='system' 跨五 milestone 锁; admin 主动 revoke 走 ADM-3+
//     单独 path)
//   - 不 time.Sleep — 用 ticker (反向 grep time.Sleep 在此文件 count==0)
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
// 跟 ap_2_1_user_permissions_revoked migration v=30 admin_actions CHECK
// 6-tuple 同源 (改 = 改两处: const + migration CHECK).
const ReasonPermissionExpired = "permission_expired"

// SystemActorID is the actor_id literal written by automated server-side
// processes (sweeper, watchdog). Cross-milestone byte-identical 跟 BPP-4
// watchdog system actor 同源 (跨五 milestone 锁: AP-2 / BPP-4 / AL-1
// state_log system writer / DL-4 push GC / future automated audit 写者).
const SystemActorID = "system"

// DefaultSweeperInterval is the cron tick (蓝图 §5 字面 "周期性 sweep,
// 不要求实时"). 1h interval — 跟 AL-1b agent_status stale-detect 周期
// 同精神 (业务 SLA + 运维成本平衡, v2+ 可调).
const DefaultSweeperInterval = 1 * time.Hour

// ExpiresSweeper periodically revokes user_permissions rows whose
// expires_at has passed. 设计 ①: forward-only soft-delete via
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
	// row 不真删). 设计 ①: UPDATE not DELETE.
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

		// Step 3 — write audit row (复用 ADM-2.1 InsertAdminAction). 设计 ②:
		// 不另起 expires_audit 表; actor='system' 字面 (设计 ④); action
		// 'permission_expired' const 字面 byte-identical 跟 admin_actions
		// CHECK 6-tuple 同源.
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

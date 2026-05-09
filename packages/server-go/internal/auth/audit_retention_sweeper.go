// Package auth — audit_retention_sweeper.go: AL-7.2 设计 ① archived_at
// soft-archive sweeper + 设计 ④ time.Ticker (no cron) + 设计 ⑤ best-effort.
//
// Blueprint: admin-model.md §3 retention + ADM-2.1 #484 forward-only audit
// 终结收尾. Spec: docs/implementation/modules/al-7-spec.md (战马D v0
// 3fa2db0) §0 设计 ① + §1 拆段 AL-7.2.
//
// What this does (one round-trip closes the AL-7 retention loop):
//
//   - On each tick (1h DefaultRetentionInterval) UPDATE admin_actions
//     SET archived_at = now WHERE created_at < (now - RetentionDays*24h)
//     AND archived_at IS NULL.
//   - 不真删 — UPDATE not DELETE (grep 检查 `DELETE FROM admin_actions`
//     在 production 0 hit; forward-only 跟 ADM-2.1 + AP-2 设计沿用).
//   - 不另起 archive 表 — admin_actions.archived_at 列单源
//     (grep 检查 `audit_archive_table\|audit_history_log\|al7_archive_log`
//     0 hit, 设计 ① 守).
//   - 不引入 scheduler 框架 — time.Ticker (跟 AP-2 ExpiresSweeper 同模式;
//     grep 检查 scheduler import 在此文件 0 hit, 设计 ④).
//   - reason 复用 AL-1a 6-dict — SweeperReason = reasons.Unknown
//     byte-identical (AL-1a 锁链第 15 处, 设计 ②).
//
// Public surface (跟 AP-2 ExpiresSweeper 同模式 nil-safe):
//   - RetentionSweeper{Store, Logger, RetentionDays, Interval, Now} — config
//   - (s *RetentionSweeper) Start(ctx) — goroutine 1h ticker, ctx-aware
//     shutdown, nil-safe (Store/Logger nil 跟 AP-2 ExpiresSweeper 同精神).
//   - (s *RetentionSweeper) RunOnce(ctx) (count int, err error) — 单次扫
//     描入口 (testable 同步 path).
//
// 反约束 (al-7-spec.md §0 + 设计 ①④⑤⑥):
//   - 不真删 row — UPDATE archived_at, 不 DELETE (grep 检查 测试守).
//   - 不裂表 — 复用 admin_actions (grep 检查 测试守).
//   - 不引入 scheduler 框架 — time.Ticker only.
//   - 不开 retention queue — AST 锁链延伸第 7 处 forbidden token 0 hit.
//   - retention 14d 字面单源 — RetentionDays = 14 const (grep 检查
//     hardcode 非 14 字面 0 hit).
package auth

import (
	"context"
	"log/slog"
	"time"

	"borgee-server/internal/agent/reasons"
	"borgee-server/internal/store"
)

// RetentionDays is the default audit retention window in days. 蓝图
// admin-model.md §3 字面 14d. Admin override (POST /admin-api/v1/audit-
// retention/override) writes one admin_actions row and updates the
// in-memory effective window via the handler — not via mutating this
// const (compile-time SSOT, 设计 ⑥ 字面单源).
const RetentionDays = 14

// RetentionMinDays / RetentionMaxDays clamp range for admin override
// endpoint (1d min — reject 0/负数; 365d max — 1y cap).
const (
	RetentionMinDays = 1
	RetentionMaxDays = 365
)

// DefaultRetentionInterval is the sweeper tick (跟 AP-2 ExpiresSweeper
// 同精神 1h, 蓝图 §3 retention 不要求 real-time, sweeper 异步软删戳
// 即可).
const DefaultRetentionInterval = 1 * time.Hour

// SweeperReason is the AL-1a 6-dict byte-identical const referenced by
// the retention sweeper. AL-1a reason 锁链第 15 处 (HB-3 v2 #14 承袭不
// 漂). 设计 ②: 不另起 reason 字典 — 复用 reasons.Unknown (sweeper 走
// best-effort, 不区分细分原因, 跟 BPP-7/BPP-8 SDK reason 一致).
const SweeperReason = reasons.Unknown

// ActionAuditRetentionOverride is the admin_actions.action 字面 byte-
// identical 跟 al_7_1 migration CHECK 12-tuple 同源 (改 = 改两处:
// const + migration CHECK).
const ActionAuditRetentionOverride = "audit_retention_override"

// RetentionSweeper periodically archives expired admin_actions rows by
// UPDATE archived_at = now (forward-only soft-archive, 不真删).
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
// 设计 ①: UPDATE not DELETE (forward-only soft-archive). grep 检查
// `DELETE FROM admin_actions` 在 production *.go 0 hit.
func (s *RetentionSweeper) RunOnce(ctx context.Context) (int, error) {
	if s == nil || s.Store == nil {
		return 0, nil
	}
	nowMs := s.now().UnixMilli()
	cutoff := nowMs - int64(s.retentionDays())*24*60*60*1000

	// Step — soft-archive: UPDATE archived_at = now WHERE created_at < cutoff
	// AND archived_at IS NULL. 设计 ①: not DELETE.
	res := s.Store.DB().WithContext(ctx).Exec(
		`UPDATE admin_actions SET archived_at = ?
		 WHERE created_at < ? AND archived_at IS NULL`,
		nowMs, cutoff)
	if res.Error != nil {
		return 0, res.Error
	}
	return int(res.RowsAffected), nil
}

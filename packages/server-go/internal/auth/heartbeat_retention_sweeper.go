// Package auth — heartbeat_retention_sweeper.go: HB-5.2 设计 ① archived_at
// soft-archive sweeper for agent_state_log + 设计 ④ time.Ticker (no scheduler
// framework) + 设计 ⑤ best-effort.
//
// Blueprint: agent-lifecycle.md §2.3 forward-only state log + AL-7 #533
// archived_at retention 模式延伸. Spec: docs/implementation/modules/
// hb-5-spec.md (战马D v0) §0 设计 ① + §1 拆段 HB-5.2.
//
// What this does (one round-trip closes the HB-5 retention loop):
//
//   - On each tick (1h DefaultRetentionInterval reused from AL-7) UPDATE
//     agent_state_log SET archived_at = now WHERE ts < (now -
//     HeartbeatRetentionDays*24h) AND archived_at IS NULL.
//   - 不真删 — UPDATE not DELETE (grep 检查 DELETE FROM agent_state_log
//     在 production 0 hit; forward-only 跟 AL-1 + AL-7 设计沿用).
//   - 不另起 archive 表 — agent_state_log.archived_at 列单源 (grep 检查
//     heartbeat_archive_table 等 0 hit, 设计 ① 守).
//   - 不引入 scheduler 框架 — time.Ticker (跟 AP-2 / AL-7 sweeper 同模式).
//   - reason 复用 AL-1a 6-dict — HeartbeatSweeperReason = reasons.Unknown
//     byte-identical 跟 AL-7 SweeperReason 同源 (AL-1a 锁链第 17 处, 设计 ②).
//
// Public surface (跟 AL-7 RetentionSweeper 同模式 nil-safe):
//   - HeartbeatRetentionSweeper{Store, Logger, RetentionDays, Interval, Now}
//   - (s *HeartbeatRetentionSweeper) Start(ctx) — goroutine 1h ticker.
//   - (s *HeartbeatRetentionSweeper) RunOnce(ctx) (count int, err error).
//
// 反约束 (hb-5-spec.md §0 + 设计 ①④⑤⑥):
//   - 不真删 row — UPDATE archived_at, 不 DELETE.
//   - 不裂表 — 复用 agent_state_log.
//   - 不引入 scheduler 框架 — time.Ticker only.
//   - 不开 retention queue — AST 锁链延伸第 9 处 forbidden token 0 hit.
//   - heartbeat retention 30d 字面单源 — HeartbeatRetentionDays = 30.
package auth

import (
	"context"
	"log/slog"
	"time"

	"borgee-server/internal/agent/reasons"
	"borgee-server/internal/store"
)

// HeartbeatRetentionDays is the default heartbeat/agent_state_log
// retention window in days. 蓝图 hb-5-spec.md §0.6 字面 30d (心跳频次
// 高于 audit, 30d cover 1 month rolling). Admin override (POST
// /admin-api/v1/heartbeat-retention/override) writes one admin_actions
// row reusing AL-7 'audit_retention_override' action with metadata
// target='heartbeat' (反向 enum 漂移; 设计 ② 守).
const HeartbeatRetentionDays = 30

// HeartbeatTargetLabel is the byte-identical metadata.target literal
// written by HB-5 admin override (跟 AL-7 audit override target='admin_
// actions' 二选一字面区分). 设计 ②: HB-5 复用 AL-7 既有 action.
const HeartbeatTargetLabel = "heartbeat"

// HeartbeatSweeperReason is the AL-1a 6-dict byte-identical const
// referenced by the heartbeat retention sweeper. AL-1a reason 锁链第
// 17 处 (AL-7 SweeperReason #15 + AL-8 #16 承袭不漂). 设计 ②: HB-5 不
// 另起 reason 字典 — 复用 reasons.Unknown.
const HeartbeatSweeperReason = reasons.Unknown

// HeartbeatRetentionSweeper periodically archives expired agent_state_log
// rows by UPDATE archived_at = now (forward-only soft-archive, 不真删).
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

// Start launches the sweeper goroutine. nil-safe ctx-aware shutdown 跟
// AL-7 RetentionSweeper 同模式.
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
// 设计 ①: UPDATE not DELETE (forward-only soft-archive). grep 检查
// `DELETE FROM agent_state_log` 在 production *.go 0 hit.
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

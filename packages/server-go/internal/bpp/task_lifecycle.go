// Package bpp — task_lifecycle.go: BPP-2.2 source-of-truth for the
// task_started / task_finished plugin-upstream frame validation +
// AL-1b busy/idle state-machine source.
//
// busy 状态由 task_started/finished frame 单一来源驱动, 不开 PATCH
// /api/v1/agents/:id/state — 跟 AL-1b #482 BPP single source 原则同源
// (蓝图 §2.3 R3). online = session-level 走 WS conn lifecycle, 跟
// task-level (busy) 正交 — 反向 grep `presence_sessions.*busy|
// presence.*task_id` count==0 (acceptance §4.2).
//
// AL-1b client busy/idle UI: 服务端派生 (option a) — server 在收到
// task_started/finished frame 后, 直接复用既有 RT-* AgentRosterUpdated /
// presence push 通道把派生 state 推给 client; 不另起独立的
// AgentTaskStateChangedFrame (frame 数量少一个 不冗余, busy/idle 是
// task lifecycle 的算法结果不是独立信号). 因此 bppEnvelopeWhitelist
// 留 11 frame 不动, 5-frame 共序锁定字段数各 frame 自报 (各自 _test 已锁定).
//
// Blueprint出处: docs/blueprint/current/plugin-protocol.md §1.6 (失联与故障状态 +
// "工作中状态需要 plugin 主动心跳上报 — 缺心跳按未知") + §2.2 (data
// plane Plugin → Borgee). agent-lifecycle.md §2.3 字面: "busy / idle
// source 必须是 plugin 上行的 task_started / task_finished frame, 没
// BPP 就只能 stub, stub 一旦上 v1 要拆掉 = 白写"; §11 文案守 (野马硬
// 条件 不准用模糊文案糊弄).
//
// Spec brief: docs/implementation/modules/bpp-2-spec.md (战马E #460 v0)
// §0 原则 ② + §1 拆段 BPP-2.2.
// 原则: docs/qa/bpp-2-stance-checklist.md §2 原则 ② 反向约束 checkbox.
// Content lock: docs/qa/bpp-2-content-lock.md §1 ③ 3 outcome enum + §1 ④
// 6 reason 字典字面跟随 AL-1a #249 + §1 ⑤ subject 文案锁定.
//
// What this file does:
//   1. Validate TaskStartedFrame: subject MUST be non-empty after
//      strings.TrimSpace; empty rejects with TaskErrCodeSubjectEmpty.
//   2. Validate TaskFinishedFrame: outcome ∈ 3-enum; when
//      outcome=='failed', reason MUST be in AL-1a #249 6-set.
//   3. Expose ValidateTaskStarted / ValidateTaskFinished free functions
//      so the api package (or future BPP listener) can validate before
//      side-effecting AL-1b state.
//
// 反向约束 (acceptance §2 + content-lock §2):
//   - subject 必带非空 + reject 默认值 fallback — 反向 grep CI lint
//     count==0 (acceptance §4.4).
//   - outcome 字典外值 (中间态) reject — 反向 grep CI lint count==0
//     (acceptance §4.8).
//   - reason 字典字面跟随 AL-1a #249 6 项, 不另起 (改 = 改四处:
//     #249 + AL-3 #305 + AL-4 #321 + #427 + BPP-2.2 = 第四+).
package bpp

import (
	"errors"
	"fmt"
	"strings"

	"borgee-server/internal/agent/reasons"
)

// TaskOutcome enum — content-lock §1 ③ byte-identical 跟蓝图 §1.6
// 失联与故障状态 outcome 字面跟随. 改 = 改三处: spec §0 原则 ② +
// acceptance §2.2 + this enum.
const (
	TaskOutcomeCompleted = "completed"
	TaskOutcomeFailed    = "failed"
	TaskOutcomeCancelled = "cancelled"
)

// validTaskOutcomes is the 3-enum membership set. Reverse grep CI lint
// rejects 中间态 ('partial' / 'paused' / 'pending' / 'starting')
// count==0 (acceptance §4.8 + content-lock §2 ⑧ 中间态严闭).
var validTaskOutcomes = map[string]bool{
	TaskOutcomeCompleted: true,
	TaskOutcomeFailed:    true,
	TaskOutcomeCancelled: true,
}

// validTaskReasons — REFACTOR-REASONS: 单一来源 迁到 internal/agent/reasons.
// 直接调 reasons.IsValid(s); 不再 inline map. 改字面 = 改 reasons.ALL 一处
// 即 8 处单测同步挂.
//
// 历史: 此处原 inline 6 字面 byte-identical 跟 agent/state.go Reason*
// (#249/#305/#321/#380/#454/#458/#481/#492 八处单测守护链), REFACTOR-REASONS
// 一 PR dedupe 到 internal/agent/reasons 单一来源 包.
func validTaskReason(s string) bool { return reasons.IsValid(s) }

// TaskErrCode* — error code literals byte-identical 跟 content-lock
// §1 ⑥ 同源 (跟 anchor.create_owner_only #360 / dm.workspace_not_supported
// #407 / iteration.target_not_in_channel #409 / bpp.semantic_op_unknown
// 命名同模式).
const (
	TaskErrCodeSubjectEmpty     = "bpp.task_subject_empty"
	TaskErrCodeOutcomeUnknown   = "bpp.task_outcome_unknown"
	TaskErrCodeReasonUnknown    = "bpp.task_reason_unknown"
	TaskErrCodeFinishedNoReason = "bpp.task_finished_no_reason"

	// ThinkingErrCodeSubjectRequired — RT-3 ⭐ wire-level reason code for
	// thinking subject 反向约束 (rt-3-spec.md §0.2 + 蓝图 §1.1 ⭐). Surfaced
	// at endpoints where thinking state is exposed to clients (跟 chn-3
	// content-lock 5 源 byte-identical 守门模式一致). 改 = 改三处: 此 const
	// + acceptance §2.3 + content-lock §3.
	ThinkingErrCodeSubjectRequired = "thinking.subject_required"
)

// errSubjectEmpty / errOutcomeUnknown / errReasonUnknown / errFinishedNoReason
// are sentinels callers can errors.Is against to map to wire-level
// error codes (跟 errSemanticOpUnknown / errArtifactConflict 同模式).
var (
	errSubjectEmpty     = errors.New("bpp: task_started subject empty")
	errOutcomeUnknown   = errors.New("bpp: task_finished outcome unknown")
	errReasonUnknown    = errors.New("bpp: task_finished reason unknown (not in AL-1a 6 dict)")
	errFinishedNoReason = errors.New("bpp: task_finished outcome=failed requires non-empty reason")
)

// IsTaskSubjectEmpty / IsTaskOutcomeUnknown / IsTaskReasonUnknown /
// IsTaskFinishedNoReason — sentinel matchers (跟 IsSemanticOpUnknown
// 同模式).
func IsTaskSubjectEmpty(err error) bool   { return errors.Is(err, errSubjectEmpty) }
func IsTaskOutcomeUnknown(err error) bool { return errors.Is(err, errOutcomeUnknown) }
func IsTaskReasonUnknown(err error) bool  { return errors.Is(err, errReasonUnknown) }
func IsTaskFinishedNoReason(err error) bool {
	return errors.Is(err, errFinishedNoReason)
}

// ValidateTaskStarted enforces 原则 ② subject 必带非空反向约束 (蓝图 §11
// 文案守 + content-lock §1 ⑤). Empty / whitespace-only Subject returns
// errSubjectEmpty wrapped with the offending agent_id for log warn.
//
// Reverse grep CI lint guards the反向约束 — this validator is the only
// sanctioned path; any fallback elsewhere violates spec §0 原则 ②.
func ValidateTaskStarted(frame TaskStartedFrame) error {
	if strings.TrimSpace(frame.Subject) == "" {
		return fmt.Errorf("%w: agent_id=%q task_id=%q",
			errSubjectEmpty, frame.AgentID, frame.TaskID)
	}
	return nil
}

// ValidateTaskFinished enforces 原则 ② outcome 3-态 + reason 字典跟随
// AL-1a 6 项 (content-lock §1 ③④). Validation order:
//   1. outcome ∈ {completed, failed, cancelled} else errOutcomeUnknown.
//   2. when outcome=='failed': reason non-empty AND in AL-1a 6 dict.
//      Empty reason on failed → errFinishedNoReason; non-empty but
//      字典外 → errReasonUnknown.
//   3. when outcome ∈ {completed, cancelled}: reason MUST be empty
//      (反向约束: 不允许"跑完了但顺便报个 reason" 漏 — 字典污染防御).
func ValidateTaskFinished(frame TaskFinishedFrame) error {
	if !validTaskOutcomes[frame.Outcome] {
		return fmt.Errorf("%w: outcome=%q (3-enum: completed/failed/cancelled)",
			errOutcomeUnknown, frame.Outcome)
	}
	if frame.Outcome == TaskOutcomeFailed {
		if frame.Reason == "" {
			return fmt.Errorf("%w: outcome=failed agent_id=%q task_id=%q",
				errFinishedNoReason, frame.AgentID, frame.TaskID)
		}
		if !validTaskReason(frame.Reason) {
			return fmt.Errorf("%w: reason=%q (AL-1a 6-dict: api_key_invalid/quota_exceeded/network_unreachable/runtime_crashed/runtime_timeout/unknown)",
				errReasonUnknown, frame.Reason)
		}
		return nil
	}
	// completed / cancelled — reason must be empty (反字典污染).
	if frame.Reason != "" {
		return fmt.Errorf("%w: outcome=%q must NOT carry reason (reason=%q)",
			errOutcomeUnknown, frame.Outcome, frame.Reason)
	}
	return nil
}

// Package bpp — task_lifecycle.go: BPP-2.2 source-of-truth for the
// task_started / task_finished plugin-upstream frame validation +
// AL-1b busy/idle state-machine source.
//
// Busy state is driven by task_started/task_finished frames only; there is no
// PATCH /api/v1/agents/:id/state path. online is session-level WebSocket
// lifecycle, while busy is task-level state. Reverse grep for
// `presence_sessions.*busy|presence.*task_id` must return 0 hits (acceptance §4.2).
//
// AL-1b client busy/idle UI is server-derived: after task_started/finished,
// the server reuses existing RT-* AgentRosterUpdated / presence push paths to
// publish derived state to clients. busy/idle is derived from task lifecycle,
// not an independent signal.
//
// Blueprint: docs/blueprint/current/plugin-protocol.md §1.6 and §2.2 plus
// agent-lifecycle.md §2.3. Spec brief: docs/implementation/modules/bpp-2-spec.md
// §0 + §1 BPP-2.2. Content lock: docs/qa/bpp-2-content-lock.md §1 ③④⑤.
//
// What this file does:
//  1. Validate TaskStartedFrame: subject MUST be non-empty after
//     strings.TrimSpace; empty rejects with TaskErrCodeSubjectEmpty.
//  2. Validate TaskFinishedFrame: outcome ∈ 3-enum; when
//     outcome=='failed', reason MUST be in AL-1a #249 6-set.
//  3. Expose ValidateTaskStarted / ValidateTaskFinished free functions
//     so the api package (or future BPP listener) can validate before
//     side-effecting AL-1b state.
//
// Negative constraints (acceptance §2 + content-lock §2):
//   - subject must be non-empty; default-value fallback rejects — reverse grep CI
//     count==0 (acceptance §4.4).
//   - enum-out outcome values, including intermediate states, reject — reverse grep CI count==0
//     (acceptance §4.8).
//   - reason values reuse the AL-1a #249 6-value reason set.
package bpp

import (
	"errors"
	"fmt"
	"strings"

	"borgee-server/internal/agent/reasons"
)

// TaskOutcome enum — content-lock §1 ③ byte-identical with blueprint §1.6.
// Changes must be coordinated with spec §0, acceptance §2.2, and this enum.
const (
	TaskOutcomeCompleted = "completed"
	TaskOutcomeFailed    = "failed"
	TaskOutcomeCancelled = "cancelled"
)

// validTaskOutcomes is the 3-enum membership set. Reverse grep CI lint
// rejects intermediate states ('partial' / 'paused' / 'pending' / 'starting')
// count==0 (acceptance §4.8 + content-lock §2 ⑧ 中间态严闭).
var validTaskOutcomes = map[string]bool{
	TaskOutcomeCompleted: true,
	TaskOutcomeFailed:    true,
	TaskOutcomeCancelled: true,
}

// validTaskReasons — REFACTOR-REASONS moved the single source to
// internal/agent/reasons. Call reasons.IsValid(s) instead of keeping an inline map.
//
// History: this file previously kept an inline 6-literal set, byte-identical
// with agent/state.go Reason* across the eight test-lock points
// (#249/#305/#321/#380/#454/#458/#481/#492). REFACTOR-REASONS deduped it into
// the internal/agent/reasons single source package.
func validTaskReason(s string) bool { return reasons.IsValid(s) }

// TaskErrCode* error code literals are byte-identical with content-lock §1 ⑥.
// Naming follows anchor.create_owner_only #360, dm.workspace_not_supported #407,
// iteration.target_not_in_channel #409, and bpp.semantic_op_unknown.
const (
	TaskErrCodeSubjectEmpty     = "bpp.task_subject_empty"
	TaskErrCodeOutcomeUnknown   = "bpp.task_outcome_unknown"
	TaskErrCodeReasonUnknown    = "bpp.task_reason_unknown"
	TaskErrCodeFinishedNoReason = "bpp.task_finished_no_reason"

	// ThinkingErrCodeSubjectRequired is the RT-3 wire-level reason code for the
	// thinking-subject negative constraint (rt-3-spec.md §0.2 + blueprint §1.1).
	// It surfaces at endpoints where thinking state is exposed to clients,
	// matching the chn-3 content-lock pattern. Any change must update this const,
	// acceptance §2.3, and content-lock §3 together.
	ThinkingErrCodeSubjectRequired = "thinking.subject_required"
)

// errSubjectEmpty / errOutcomeUnknown / errReasonUnknown / errFinishedNoReason
// are sentinels callers can errors.Is against to map to wire-level
// error codes, matching the errSemanticOpUnknown / errArtifactConflict pattern.
var (
	errSubjectEmpty     = errors.New("bpp: task_started subject empty")
	errOutcomeUnknown   = errors.New("bpp: task_finished outcome unknown")
	errReasonUnknown    = errors.New("bpp: task_finished reason unknown (not in AL-1a 6 dict)")
	errFinishedNoReason = errors.New("bpp: task_finished outcome=failed requires non-empty reason")
)

// IsTaskSubjectEmpty / IsTaskOutcomeUnknown / IsTaskReasonUnknown /
// IsTaskFinishedNoReason are sentinel matchers, matching the IsSemanticOpUnknown
// pattern.
func IsTaskSubjectEmpty(err error) bool   { return errors.Is(err, errSubjectEmpty) }
func IsTaskOutcomeUnknown(err error) bool { return errors.Is(err, errOutcomeUnknown) }
func IsTaskReasonUnknown(err error) bool  { return errors.Is(err, errReasonUnknown) }
func IsTaskFinishedNoReason(err error) bool {
	return errors.Is(err, errFinishedNoReason)
}

// ValidateTaskStarted enforces principle ②: subject must be non-empty
// (blueprint §11 wording guard + content-lock §1 ⑤). Empty / whitespace-only Subject returns
// errSubjectEmpty wrapped with the offending agent_id for log warn.
//
// Reverse grep CI lint guards the negative constraint: this validator is the
// only sanctioned path; any fallback elsewhere violates spec §0 principle ②.
func ValidateTaskStarted(frame TaskStartedFrame) error {
	if strings.TrimSpace(frame.Subject) == "" {
		return fmt.Errorf("%w: agent_id=%q task_id=%q",
			errSubjectEmpty, frame.AgentID, frame.TaskID)
	}
	return nil
}

// ValidateTaskFinished enforces principle ②: the 3-state outcome enum and the
// AL-1a 6-value reason dictionary (content-lock §1 ③④). Validation order:
//  1. outcome ∈ {completed, failed, cancelled} else errOutcomeUnknown.
//  2. when outcome=='failed': reason non-empty AND in AL-1a 6 dict.
//     Empty reason on failed → errFinishedNoReason; non-empty but
//     outside the dictionary → errReasonUnknown.
//  3. when outcome ∈ {completed, cancelled}: reason MUST be empty
//     (negative constraint: do not allow a completed/cancelled task to carry a
//     reason; this prevents reason dictionary pollution).
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
	// completed / cancelled — reason must be empty to avoid dictionary pollution.
	if frame.Reason != "" {
		return fmt.Errorf("%w: outcome=%q must NOT carry reason (reason=%q)",
			errOutcomeUnknown, frame.Outcome, frame.Reason)
	}
	return nil
}

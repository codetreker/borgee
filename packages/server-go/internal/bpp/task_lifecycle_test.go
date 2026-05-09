// Package bpp_test — task_lifecycle_test.go: BPP-2.2 acceptance tests.
//
// Stance pins exercised (bpp-2-spec.md §0 设计 ② + acceptance §2 +
// content-lock §1 ③④⑤):
//   - subject 必带非空 (反默认值 fallback)
//   - outcome 3 态严闭 ('partial' / 'paused' / 'pending' / 'starting'
//     等中间态 reject)
//   - reason 字典字面承袭 AL-1a #249 6 项 (改 = 改四处单测锁: #249 +
//     AL-3 #305 + AL-4 #321 + #427 + 此 = 第四+)
//   - completed/cancelled 时 reason 必空 (反字典污染)
package bpp_test

import (
	"encoding/json"
	"strings"
	"testing"

	agentpkg "borgee-server/internal/agent"
	"borgee-server/internal/bpp"
)

// TestBPP_TaskStartedFrameFieldOrder pins 6-field byte-identical
// envelope order. JSON key order follows struct declaration order.
func TestBPP_TaskStartedFrameFieldOrder(t *testing.T) {
	t.Parallel()
	frame := bpp.TaskStartedFrame{
		Type:      bpp.FrameTypeBPPTaskStarted,
		TaskID:    "task-A",
		AgentID:   "agent-X",
		ChannelID: "ch-Y",
		Subject:   "Drafting PRD section 2",
		StartedAt: 1700000000000,
	}
	b, err := json.Marshal(&frame)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"task_started","task_id":"task-A","agent_id":"agent-X","channel_id":"ch-Y","subject":"Drafting PRD section 2","started_at":1700000000000}`
	if string(b) != want {
		t.Fatalf("TaskStarted envelope byte-identity broken:\n got: %s\nwant: %s", b, want)
	}
}

// TestBPP_TaskFinishedFrameFieldOrder pins 7-field byte-identical
// envelope order.
func TestBPP_TaskFinishedFrameFieldOrder(t *testing.T) {
	t.Parallel()
	frame := bpp.TaskFinishedFrame{
		Type:       bpp.FrameTypeBPPTaskFinished,
		TaskID:     "task-A",
		AgentID:    "agent-X",
		ChannelID:  "ch-Y",
		Outcome:    "failed",
		Reason:     "api_key_invalid",
		FinishedAt: 1700000000001,
	}
	b, err := json.Marshal(&frame)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"task_finished","task_id":"task-A","agent_id":"agent-X","channel_id":"ch-Y","outcome":"failed","reason":"api_key_invalid","finished_at":1700000000001}`
	if string(b) != want {
		t.Fatalf("TaskFinished envelope byte-identity broken:\n got: %s\nwant: %s", b, want)
	}
}

// TestBPP_ValidateTaskStarted_AcceptsNonEmpty pins acceptance §2.1
// happy path — subject with content passes validation.
func TestBPP_ValidateTaskStarted_AcceptsNonEmpty(t *testing.T) {
	frame := bpp.TaskStartedFrame{
		Type:      bpp.FrameTypeBPPTaskStarted,
		TaskID:    "task-A",
		AgentID:   "agent-X",
		ChannelID: "ch-Y",
		Subject:   "Drafting PRD section 2",
		StartedAt: 1700000000000,
	}
	if err := bpp.ValidateTaskStarted(frame); err != nil {
		t.Errorf("non-empty subject rejected: %v", err)
	}
}

// TestBPP_ValidateTaskStarted_RejectsEmpty pins acceptance §2.1 反断
// + content-lock §1 ⑤ — empty / whitespace-only Subject MUST reject
// with errSubjectEmpty (蓝图 §11 文案守 字面禁默认值 fallback).
func TestBPP_ValidateTaskStarted_RejectsEmpty(t *testing.T) {
	for _, bad := range []string{"", " ", "\t", "\n", "   \t  \n "} {
		frame := bpp.TaskStartedFrame{
			Type:      bpp.FrameTypeBPPTaskStarted,
			AgentID:   "agent-X",
			Subject:   bad,
			StartedAt: 1700000000000,
		}
		err := bpp.ValidateTaskStarted(frame)
		if err == nil {
			t.Errorf("subject=%q accepted — should reject (野马 §11 文案守)", bad)
			continue
		}
		if !bpp.IsTaskSubjectEmpty(err) {
			t.Errorf("subject=%q rejected with wrong sentinel: got %v", bad, err)
		}
	}
}

// TestBPP_ValidateTaskFinished_AcceptsThreeOutcomes pins acceptance
// §2.2 — 3 outcome enum 全过 (completed/failed/cancelled byte-identical).
func TestBPP_ValidateTaskFinished_AcceptsThreeOutcomes(t *testing.T) {
	cases := []struct {
		outcome string
		reason  string
	}{
		{bpp.TaskOutcomeCompleted, ""},
		{bpp.TaskOutcomeFailed, agentpkg.ReasonAPIKeyInvalid},
		{bpp.TaskOutcomeCancelled, ""},
	}
	for _, c := range cases {
		frame := bpp.TaskFinishedFrame{
			Type:       bpp.FrameTypeBPPTaskFinished,
			TaskID:     "task-A",
			AgentID:    "agent-X",
			Outcome:    c.outcome,
			Reason:     c.reason,
			FinishedAt: 1700000000000,
		}
		if err := bpp.ValidateTaskFinished(frame); err != nil {
			t.Errorf("outcome=%q rejected: %v", c.outcome, err)
		}
	}
}

// TestBPP_ValidateTaskFinished_RejectsMiddleStates pins acceptance
// §2.2 反断 + content-lock §2 ⑧ — 中间态 ('partial' / 'paused' /
// 'pending' / 'starting') MUST reject (3 态严闭).
func TestBPP_ValidateTaskFinished_RejectsMiddleStates(t *testing.T) {
	for _, bad := range []string{
		"partial", "paused", "pending", "starting", "running",
		"in_progress", "Completed", "FAILED", "", "done",
	} {
		frame := bpp.TaskFinishedFrame{
			Type:       bpp.FrameTypeBPPTaskFinished,
			Outcome:    bad,
			FinishedAt: 1700000000000,
		}
		err := bpp.ValidateTaskFinished(frame)
		if err == nil {
			t.Errorf("outcome=%q accepted — should reject (3-enum 严闭)", bad)
			continue
		}
		if !bpp.IsTaskOutcomeUnknown(err) {
			t.Errorf("outcome=%q wrong sentinel: %v", bad, err)
		}
	}
}

// TestBPP_ValidateTaskFinished_FailedRequiresAL1aReason pins
// acceptance §2.2 + content-lock §1 ④ — outcome=='failed' requires
// reason in AL-1a 6-dict (改 = 改四处+ 单测锁: #249 + AL-3 + AL-4 +
// #427 + 此).
func TestBPP_ValidateTaskFinished_FailedRequiresAL1aReason(t *testing.T) {
	// 6 AL-1a reasons all accepted on failed.
	for _, reason := range []string{
		agentpkg.ReasonAPIKeyInvalid,
		agentpkg.ReasonQuotaExceeded,
		agentpkg.ReasonNetworkUnreachable,
		agentpkg.ReasonRuntimeCrashed,
		agentpkg.ReasonRuntimeTimeout,
		agentpkg.ReasonUnknown,
	} {
		frame := bpp.TaskFinishedFrame{
			Type:    bpp.FrameTypeBPPTaskFinished,
			Outcome: bpp.TaskOutcomeFailed,
			Reason:  reason,
		}
		if err := bpp.ValidateTaskFinished(frame); err != nil {
			t.Errorf("AL-1a reason=%q rejected: %v", reason, err)
		}
	}

	// failed + empty reason → errFinishedNoReason.
	err := bpp.ValidateTaskFinished(bpp.TaskFinishedFrame{
		Type:    bpp.FrameTypeBPPTaskFinished,
		Outcome: bpp.TaskOutcomeFailed,
		Reason:  "",
	})
	if !bpp.IsTaskFinishedNoReason(err) {
		t.Errorf("failed+empty reason wrong sentinel: %v", err)
	}

	// failed + 字典外 reason → errReasonUnknown.
	for _, bad := range []string{
		"made_up_reason", "ApiKeyInvalid", "rate_limited", "wrong_password",
	} {
		err := bpp.ValidateTaskFinished(bpp.TaskFinishedFrame{
			Type:    bpp.FrameTypeBPPTaskFinished,
			Outcome: bpp.TaskOutcomeFailed,
			Reason:  bad,
		})
		if err == nil {
			t.Errorf("reason=%q accepted on failed — should reject (AL-1a 6-dict)", bad)
			continue
		}
		if !bpp.IsTaskReasonUnknown(err) {
			t.Errorf("reason=%q wrong sentinel: %v", bad, err)
		}
	}
}

// TestBPP_ValidateTaskFinished_CompletedRejectsReason pins
// acceptance §2.2 反断 — outcome ∈ {completed, cancelled} MUST have
// empty reason (反字典污染).
func TestBPP_ValidateTaskFinished_CompletedRejectsReason(t *testing.T) {
	for _, outcome := range []string{bpp.TaskOutcomeCompleted, bpp.TaskOutcomeCancelled} {
		err := bpp.ValidateTaskFinished(bpp.TaskFinishedFrame{
			Type:    bpp.FrameTypeBPPTaskFinished,
			Outcome: outcome,
			Reason:  agentpkg.ReasonUnknown, // any non-empty reason
		})
		if err == nil {
			t.Errorf("outcome=%q with reason accepted — should reject (反字典污染)", outcome)
		}
	}
}

// TestTaskLifecycle_ErrorCodeLiteralsByteIdentical pins content-lock §1 ⑥
// 错误码字面 byte-identical.
func TestTaskLifecycle_ErrorCodeLiteralsByteIdentical(t *testing.T) {
	cases := []struct{ got, want string }{
		{bpp.TaskErrCodeSubjectEmpty, "bpp.task_subject_empty"},
		{bpp.TaskErrCodeOutcomeUnknown, "bpp.task_outcome_unknown"},
		{bpp.TaskErrCodeReasonUnknown, "bpp.task_reason_unknown"},
		{bpp.TaskErrCodeFinishedNoReason, "bpp.task_finished_no_reason"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("error code drift: got %q, want %q", c.got, c.want)
		}
	}
}

// Compile-time guard: keep strings import alive (used for whitespace
// detection by package code).
var _ = strings.TrimSpace

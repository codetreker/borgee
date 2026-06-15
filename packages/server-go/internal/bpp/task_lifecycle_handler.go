// Package bpp — task_lifecycle_handler.go: RT-3 server-derived hook —
// Plugin-upstream task_started / task_finished frames → server fanout
// AgentTaskStateChangedFrame (busy/idle) via Hub.PushAgentTaskStateChanged.
//
// Blueprint reference: docs/blueprint/current/realtime.md §1.1 (live-feel /
// thinking must include subject) + agent-lifecycle.md §2.3 (busy/idle source =
// plugin upstream frame).
// Spec: docs/implementation/modules/rt-3-spec.md §0 设计 ②+③ + §1 RT-3.2.
//
// Wire-up uses the same pattern as BPP-3 #489, AL-2b #481 AckFrameAdapter,
// and BPP-5/6:
//
//   server.go boot:
//     hub := ws.NewHub(...)
//     pusher := bpp.NewHubAgentTaskPusher(hub)
//     handler := bpp.NewTaskLifecycleHandler(pusher, ownerResolver, logger)
//     pfd.Register(bpp.FrameTypeBPPTaskStarted,  handler.StartedAdapter())
//     pfd.Register(bpp.FrameTypeBPPTaskFinished, handler.FinishedAdapter())
//
// Design (matching spec §0):
//   ① BroadcastToChannel multi-device fanout (Hub.PushAgentTaskStateChanged
//     internal): user-id routing goes through channel member subscription and
//     does not split by device-id, matching P1MultiDeviceWebSocket #197.
//   ② thinking subject must be non-empty. The handler uses ValidateTaskStarted
//     as the single source of truth (same errSubjectEmpty source as BPP-2.2
//     task_lifecycle.go). The derived path fails closed: empty subject rejects
//     and does not push any fallback literal such as 'AI is thinking' or
//     defaultSubject into ws push.
//   ③ task_started → busy + Subject passthrough; task_finished → idle + empty
//     Subject + reason passthrough. idle+failed uses the AL-1a 6-dict;
//     completed/cancelled use an empty reason. ValidateTaskFinished prevents
//     reason dictionary pollution.
//
// Constraints:
//   - Do not add a device-only push channel. Reuse hub.cursors shared ordering
//     and channel member subscription for automatic multi-device fanout.
//   - Admin users do not trigger this push. The handler is only triggered by a
//     plugin upstream frame; reverse grep `admin.*PushAgentTaskStateChanged`
//     must return 0 hits.
//   - Do not add schema/migration changes. RT-3 is 0 schema, matching RT-4 and
//     DM-9 intent.

package bpp

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
)

// AgentTaskPusher is the test seam for Hub.PushAgentTaskStateChanged.
// Production wires *ws.Hub via HubAgentTaskPusher; tests inject a fake.
//
// The interface is a strict superset of the busy/idle derivation —
// state ('busy' | 'idle'), subject (non-empty for busy, "" for idle),
// reason (AL-1a 6-dict for idle+failed; "" otherwise).
type AgentTaskPusher interface {
	PushAgentTaskStateChanged(
		agentID string,
		channelID string,
		state string,
		subject string,
		reason string,
		changedAt int64,
	) (cursor int64, sent bool)
}

// ChannelMemberFetcher returns user_ids of members for a channel.
// WIRE-1 wire-3 — fan-out target for AgentTaskNotifier push. DL-4 uses two
// tracks: Hub.PushAgentTaskStateChanged already covers ws, while notifier adds
// mobile background service-worker push.
type ChannelMemberFetcher interface {
	ListChannelMemberUserIDs(channelID string) ([]string, error)
}

// AgentTaskPushNotifier is the test seam for push.AgentTaskNotifier.
// Wraps NotifyAgentTask call (returns attempts count, observability only).
//
// Design ②: thinking subject must be non-empty for busy state; reason passes
// through for idle+failed using the AL-1a 6-dict, avoiding reason dictionary
// pollution.
type AgentTaskPushNotifier interface {
	NotifyAgentTask(targetUserID, agentID, state, subject, reason string, changedAt int64) int
}

// TaskErrCodeCrossOwnerReject — wire-level error code, named with the same
// pattern as BPP-3 AckErrCodeCrossOwnerReject / BPP-6
// ColdStartErrCodeCrossOwnerReject.
const TaskErrCodeCrossOwnerReject = "bpp.task_lifecycle_cross_owner_reject"

// errTaskCrossOwnerReject — cross-owner ACL failure on a task_started /
// task_finished frame, matching the BPP-3 errAckCrossOwnerReject / BPP-6
// errColdStartCrossOwnerReject pattern. Callers errors.Is against it to map
// to the wire-level error code.
var errTaskCrossOwnerReject = errors.New("bpp: task_lifecycle cross-owner reject")

// IsTaskCrossOwnerReject — sentinel matcher, mirroring IsAckCrossOwnerReject /
// IsColdStartCrossOwnerReject.
func IsTaskCrossOwnerReject(err error) bool {
	return errors.Is(err, errTaskCrossOwnerReject)
}

// TaskLifecycleHandler routes plugin-upstream task_started /
// task_finished frames through the ValidateTask* single source of truth, then fans out
// AgentTaskStateChangedFrame via the AgentTaskPusher seam.
//
// Construction is pure — no boot-time side effects. server.go wires
// instances + registers Started/FinishedAdapter() returns onto the
// PluginFrameDispatcher boundary (BPP-3 #489).
//
// WIRE-1 wire-3: members + notifier may be nil. Production injects
// ListChannelMembers + push.NewAgentTaskNotifier; tests can pass only pusher.
// Fanout is nil-safe.
type TaskLifecycleHandler struct {
	pusher   AgentTaskPusher
	resolver OwnerResolver         // cross-owner gate: OwnerOf(agent) == sess owner
	members  ChannelMemberFetcher  // nil-safe: 跳 push fanout
	notifier AgentTaskPushNotifier // nil-safe: 跳 push 调用
	logger   *slog.Logger
}

// NewTaskLifecycleHandler constructs the RT-3 server-side derived
// fanout handler. logger may be nil (defaults to discard).
//
// resolver is the cross-owner ACL gate (mirrors NewAckDispatcher /
// NewColdStartHandler): both pusher and resolver MUST be non-nil; a nil
// argument is a server boot bug (panics — defense-in-depth, prevents
// 0-coverage routes from silently entering production).
func NewTaskLifecycleHandler(pusher AgentTaskPusher, resolver OwnerResolver, logger *slog.Logger) *TaskLifecycleHandler {
	if pusher == nil {
		panic("bpp: NewTaskLifecycleHandler pusher must not be nil")
	}
	if resolver == nil {
		panic("bpp: NewTaskLifecycleHandler resolver must not be nil")
	}
	return &TaskLifecycleHandler{pusher: pusher, resolver: resolver, logger: logger}
}

// SetPushFanout wires WIRE-1 wire-3 push fanout through the DL-4 push gateway.
// If either members or notifier is nil, fanout is skipped to avoid leaks.
//
// Constraint: do not add four constructor parameters to NewTaskLifecycleHandler.
// Keep the existing BPP-3 wire-up pattern byte-identical and use this setter to
// make the extra wire-up step explicit.
func (h *TaskLifecycleHandler) SetPushFanout(members ChannelMemberFetcher, notifier AgentTaskPushNotifier) {
	h.members = members
	h.notifier = notifier
}

// StartedAdapter returns the BPP-3 FrameDispatcher for task_started.
func (h *TaskLifecycleHandler) StartedAdapter() FrameDispatcher {
	return &taskStartedAdapter{handler: h}
}

// FinishedAdapter returns the BPP-3 FrameDispatcher for task_finished.
func (h *TaskLifecycleHandler) FinishedAdapter() FrameDispatcher {
	return &taskFinishedAdapter{handler: h}
}

// checkOwner enforces the cross-owner ACL gate for the adapter Dispatch path,
// mirroring AgentConfigAckDispatcher:174-183 / ColdStartHandler:138-152: the
// authenticated plugin owner (sess.OwnerUserID) must own agentID. OwnerOf err
// is treated as a reject (frame for an unknown / disconnected agent). On
// mismatch or err, returns errTaskCrossOwnerReject so the caller never reaches
// HandleStarted / HandleFinished, push, or fanout. The typed HandleStarted /
// HandleFinished entries stay session-free.
func (h *TaskLifecycleHandler) checkOwner(agentID string, sess PluginSessionContext) error {
	owner, err := h.resolver.OwnerOf(agentID)
	if err != nil {
		return fmt.Errorf("%w: agent_id=%q resolve failed: %v",
			errTaskCrossOwnerReject, agentID, err)
	}
	if owner != sess.OwnerUserID {
		if h.logger != nil {
			h.logger.Warn(TaskErrCodeCrossOwnerReject,
				"agent_id", agentID,
				"owner", owner,
				"sess_owner", sess.OwnerUserID)
		}
		return fmt.Errorf("%w: agent_id=%q owner=%q sess_owner=%q",
			errTaskCrossOwnerReject, agentID, owner, sess.OwnerUserID)
	}
	return nil
}

// HandleStarted is the test-friendly typed entry. Validation errors
// are wrapped with errSubjectEmpty etc. (errors.Is compatible). On
// success, fanout AgentTaskStateChangedFrame{state: 'busy', subject:
// frame.Subject, empty reason} via pusher.
func (h *TaskLifecycleHandler) HandleStarted(frame TaskStartedFrame) error {
	if err := ValidateTaskStarted(frame); err != nil {
		// Design ②: thinking subject must be non-empty. Fail closed; do not push a fallback.
		// Caller (FrameDispatcher) logs warn via the dispatcher boundary.
		return err
	}
	// task_started → busy. Subject passes through from plugin upstream; the
	// validator has already enforced non-empty. Reason is "" because busy has no
	// reason; AL-1a reasons are only used for idle+failed.
	h.pusher.PushAgentTaskStateChanged(
		frame.AgentID,
		frame.ChannelID,
		"busy",
		frame.Subject,
		"",
		frame.StartedAt,
	)
	// WIRE-1 wire-3: DL-4 push gateway fanout for mobile background delivery.
	// Nil-safe: if members or notifier is nil, skip fanout for test paths.
	h.fanoutPush(frame.AgentID, frame.ChannelID, "busy", frame.Subject, "", frame.StartedAt)
	return nil
}

// HandleFinished is the test-friendly typed entry for task_finished.
// On success, fanout AgentTaskStateChangedFrame{state: 'idle', empty subject,
// reason: frame.Reason} (failed → AL-1a reason; completed/cancelled
// → reason "" via ValidateTaskFinished dictionary-pollution guard).
func (h *TaskLifecycleHandler) HandleFinished(frame TaskFinishedFrame) error {
	if err := ValidateTaskFinished(frame); err != nil {
		return err
	}
	// task_finished → idle. Subject must be empty to avoid idle field pollution;
	// reason passes through. The validator has already enforced AL-1a 6-dict for
	// outcome=failed and "" for completed/cancelled.
	h.pusher.PushAgentTaskStateChanged(
		frame.AgentID,
		frame.ChannelID,
		"idle",
		"",
		frame.Reason,
		frame.FinishedAt,
	)
	h.fanoutPush(frame.AgentID, frame.ChannelID, "idle", "", frame.Reason, frame.FinishedAt)
	return nil
}

// fanoutPush invokes AgentTaskNotifier per channel member for mobile
// background push (DL-4 #485 push gateway). Nil-safe: if members or notifier is
// nil, skip fanout to avoid leaks and boot panics.
//
// Design: hub.PushAgentTaskStateChanged uses the ws live connection for
// foreground clients, while notifier uses service-worker push for mobile
// background / closed tabs. This two-track fanout matches DL-4.6 mention.
func (h *TaskLifecycleHandler) fanoutPush(agentID, channelID, state, subject, reason string, ts int64) {
	if h.members == nil || h.notifier == nil {
		return
	}
	userIDs, err := h.members.ListChannelMemberUserIDs(channelID)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("rt3.task_push_fanout_members_err",
				"channel_id", channelID, "error", err)
		}
		return
	}
	for _, uid := range userIDs {
		if uid == "" || uid == agentID {
			continue // Skip the agent itself and empty user ids.
		}
		_ = h.notifier.NotifyAgentTask(uid, agentID, state, subject, reason, ts)
	}
}

// taskStartedAdapter implements FrameDispatcher for task_started.
type taskStartedAdapter struct{ handler *TaskLifecycleHandler }

func (a *taskStartedAdapter) Dispatch(raw json.RawMessage, sess PluginSessionContext) error {
	var frame TaskStartedFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return fmt.Errorf("bpp.task_started_decode: %w", err)
	}
	// cross-owner gate (mirror AckDispatcher:174-183): the authenticated plugin
	// owner (sess.OwnerUserID) must own frame.AgentID, else reject before any
	// HandleStarted / push / fanout. OwnerOf err is treated as a reject.
	if err := a.handler.checkOwner(frame.AgentID, sess); err != nil {
		return err
	}
	if err := a.handler.HandleStarted(frame); err != nil {
		// Surface sentinel for caller errors.Is mapping (e.g. log warn
		// + metrics tag bpp.task_subject_empty).
		if errors.Is(err, errSubjectEmpty) && a.handler.logger != nil {
			a.handler.logger.Warn("rt.subject_required",
				"agent_id", frame.AgentID,
				"task_id", frame.TaskID,
				"channel_id", frame.ChannelID)
		}
		return err
	}
	return nil
}

// taskFinishedAdapter implements FrameDispatcher for task_finished.
type taskFinishedAdapter struct{ handler *TaskLifecycleHandler }

func (a *taskFinishedAdapter) Dispatch(raw json.RawMessage, sess PluginSessionContext) error {
	var frame TaskFinishedFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return fmt.Errorf("bpp.task_finished_decode: %w", err)
	}
	// cross-owner gate (mirror AckDispatcher:174-183): reject before any
	// HandleFinished / push / fanout when sess owner != frame.AgentID owner.
	if err := a.handler.checkOwner(frame.AgentID, sess); err != nil {
		return err
	}
	return a.handler.HandleFinished(frame)
}

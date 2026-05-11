// Package bpp — task_lifecycle_handler.go: RT-3 ⭐ server派生 hook —
// Plugin-upstream task_started / task_finished frames → server fanout
// AgentTaskStateChangedFrame (busy/idle) via Hub.PushAgentTaskStateChanged.
//
// Blueprint锚: docs/blueprint/current/realtime.md §1.1 ⭐ (活物感 / thinking 必带
// subject) + agent-lifecycle.md §2.3 (busy/idle source = plugin 上行 frame).
// Spec: docs/implementation/modules/rt-3-spec.md §0 设计 ②+③ + §1 RT-3.2.
//
// Wire (跟 BPP-3 #489 + AL-2b #481 AckFrameAdapter + BPP-5/6 同模式):
//
//   server.go boot:
//     hub := ws.NewHub(...)
//     pusher := bpp.NewHubAgentTaskPusher(hub)
//     handler := bpp.NewTaskLifecycleHandler(pusher, ownerResolver, logger)
//     pfd.Register(bpp.FrameTypeBPPTaskStarted,  handler.StartedAdapter())
//     pfd.Register(bpp.FrameTypeBPPTaskFinished, handler.FinishedAdapter())
//
// 设计 (跟 spec §0):
//   ① BroadcastToChannel multi-device fanout (Hub.PushAgentTaskStateChanged
//     internal — user-id 路由经 channel member subscription, 反 device-id
//     拆分; P1MultiDeviceWebSocket #197 模式).
//   ② thinking subject 必带非空 — handler 走 ValidateTaskStarted 单一来源
//     (BPP-2.2 task_lifecycle.go errSubjectEmpty 同源, 改 = 改 validator
//     一处). 派生路径 fail-closed: empty subject reject + 不 push 任何
//     fallback 字面 (反 'AI is thinking' / defaultSubject 进 ws push).
//   ③ task_started → busy + Subject 透传; task_finished → idle + Subject 空 +
//     reason 透传 (idle+failed 时 AL-1a 6-dict; completed/cancelled 时空,
//     反字典污染 — ValidateTaskFinished 守门).
//
// 约束:
//   - 不另起 device-only push channel — 复用 hub.cursors 共序 + channel
//     member subscription 自动 multi-device fanout.
//   - admin god-mode 不下发 — handler 仅由 plugin upstream frame 触发,
//     反向 grep `admin.*PushAgentTaskStateChanged` 0 hit (CI 守门).
//   - 不挂 schema/migration — RT-3 是 0 schema (跟 RT-4 / DM-9 同精神).

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
// state ('busy' | 'idle'), subject (busy 时 non-empty, idle 时 ""),
// reason (idle+failed 时 AL-1a 6-dict; 否则 "").
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
// WIRE-1 wire-3 — fan-out target for AgentTaskNotifier push (DL-4 ws +
// service worker push 双轨 fanout, Hub.PushAgentTaskStateChanged 已走 ws,
// notifier 补 mobile background push).
type ChannelMemberFetcher interface {
	ListChannelMemberUserIDs(channelID string) ([]string, error)
}

// AgentTaskPushNotifier is the test seam for push.AgentTaskNotifier.
// Wraps NotifyAgentTask call (returns attempts count, observability only).
//
// 设计 ② thinking subject 必带非空 (busy 态), reason 透传 (idle+failed
// 时 AL-1a 6-dict, 反字典污染).
type AgentTaskPushNotifier interface {
	NotifyAgentTask(targetUserID, agentID, state, subject, reason string, changedAt int64) int
}

// TaskLifecycleHandler routes plugin-upstream task_started /
// task_finished frames through ValidateTask* 单一来源 and then fans out
// AgentTaskStateChangedFrame via the AgentTaskPusher seam.
//
// Construction is pure — no boot-time side effects. server.go wires
// instances + registers Started/FinishedAdapter() returns onto the
// PluginFrameDispatcher boundary (BPP-3 #489).
//
// WIRE-1 wire-3: members + notifier 可 nil (production 真注入 ListChannelMembers
// + push.NewAgentTaskNotifier; test path 仅传 pusher 不破). Nil-safe fanout.
type TaskLifecycleHandler struct {
	pusher   AgentTaskPusher
	members  ChannelMemberFetcher  // nil-safe: 跳 push fanout
	notifier AgentTaskPushNotifier // nil-safe: 跳 push 调用
	logger   *slog.Logger
}

// NewTaskLifecycleHandler constructs the RT-3 server-side derived
// fanout handler. logger may be nil (defaults to discard).
func NewTaskLifecycleHandler(pusher AgentTaskPusher, logger *slog.Logger) *TaskLifecycleHandler {
	if pusher == nil {
		panic("bpp: NewTaskLifecycleHandler pusher must not be nil")
	}
	return &TaskLifecycleHandler{pusher: pusher, logger: logger}
}

// SetPushFanout wires WIRE-1 wire-3 push fanout (RT-3 AgentTaskNotifier 真接
// DL-4 push gateway). members + notifier 任一 nil → 跳 fanout (反 leak).
//
// 约束: 不在 NewTaskLifecycleHandler 加 4 参数 — 保持 BPP-3 既有 wire 模式
// byte-identical (战马 review 友好); 走 setter 加 wire-up 步骤透明.
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

// HandleStarted is the test-friendly typed entry. Validation errors
// are wrapped with errSubjectEmpty etc. (errors.Is compatible). On
// success, fanout AgentTaskStateChangedFrame{state: 'busy', subject:
// frame.Subject, reason: ''} via pusher.
func (h *TaskLifecycleHandler) HandleStarted(frame TaskStartedFrame) error {
	if err := ValidateTaskStarted(frame); err != nil {
		// 设计 ② thinking subject 必带非空 — fail-closed, 反 fallback push.
		// Caller (FrameDispatcher) logs warn via the dispatcher boundary.
		return err
	}
	// task_started → busy. subject 透传 plugin 上行 (validator 已守非空).
	// reason 是 "" (busy 态语义无 reason; AL-1a reason 仅 idle+failed 用).
	h.pusher.PushAgentTaskStateChanged(
		frame.AgentID,
		frame.ChannelID,
		"busy",
		frame.Subject,
		"",
		frame.StartedAt,
	)
	// WIRE-1 wire-3 — DL-4 push gateway fanout 真接 (mobile background).
	// nil-safe: members 或 notifier 任一 nil 跳 fanout (test path).
	h.fanoutPush(frame.AgentID, frame.ChannelID, "busy", frame.Subject, "", frame.StartedAt)
	return nil
}

// HandleFinished is the test-friendly typed entry for task_finished.
// On success, fanout AgentTaskStateChangedFrame{state: 'idle', subject:
// '', reason: frame.Reason} (failed → AL-1a reason; completed/cancelled
// → reason "" via ValidateTaskFinished 字典污染防御).
func (h *TaskLifecycleHandler) HandleFinished(frame TaskFinishedFrame) error {
	if err := ValidateTaskFinished(frame); err != nil {
		return err
	}
	// task_finished → idle. subject 必空 (反 idle 字段污染); reason 透传
	// (validator 已守 outcome=failed 时 AL-1a 6-dict, completed/cancelled 时 "").
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
// background push (DL-4 #485 push gateway). nil-safe: members 或 notifier
// 任一 nil → 跳 (反 leak / 反 boot panic).
//
// 设计: hub.PushAgentTaskStateChanged 走 ws live conn (前台 client),
// notifier 走 service worker push (后台 mobile / closed tab). 双轨 fanout
// 跟 DL-4.6 mention 同模式.
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
			continue // 跳 agent 自己 (反 self-push) + 空字符串
		}
		_ = h.notifier.NotifyAgentTask(uid, agentID, state, subject, reason, ts)
	}
}

// taskStartedAdapter implements FrameDispatcher for task_started.
type taskStartedAdapter struct{ handler *TaskLifecycleHandler }

func (a *taskStartedAdapter) Dispatch(raw json.RawMessage, _ PluginSessionContext) error {
	var frame TaskStartedFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return fmt.Errorf("bpp.task_started_decode: %w", err)
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

func (a *taskFinishedAdapter) Dispatch(raw json.RawMessage, _ PluginSessionContext) error {
	var frame TaskFinishedFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return fmt.Errorf("bpp.task_finished_decode: %w", err)
	}
	return a.handler.HandleFinished(frame)
}

// Package bpp_test — agent_config_ack_dispatcher_test.go: AL-2b ack
// dispatcher 验证锁 (4 test 跟任务字面对应).
//
// 锚: docs/qa/acceptance-templates/al-2b.md §1.2 (status enum 3 态) +
// §2.5 (cross-owner reject) + §3.2 (reason 字典承袭 AL-1a 6 项).
package bpp_test

import (
	"errors"
	"fmt"
	"testing"

	agentpkg "borgee-server/internal/agent"
	"borgee-server/internal/bpp"
)

// stubHandler is a test-double for AgentConfigAckHandler that records
// calls + lets tests inject a return error.
type stubHandler struct {
	calls   int
	gotF    bpp.AgentConfigAckFrame
	gotS    bpp.AckSessionContext
	wantErr error
}

func (h *stubHandler) HandleAck(f bpp.AgentConfigAckFrame, s bpp.AckSessionContext) error {
	h.calls++
	h.gotF = f
	h.gotS = s
	return h.wantErr
}

// stubResolver is a test-double for OwnerResolver — maps agent_id to
// owner UUID via an in-memory map; missing agent_id returns an error.
type stubResolver struct {
	owners map[string]string
}

func (r *stubResolver) OwnerOf(agentID string) (string, error) {
	o, ok := r.owners[agentID]
	if !ok {
		return "", fmt.Errorf("agent %q not found", agentID)
	}
	return o, nil
}

func newDispatcherWith(t *testing.T, owners map[string]string) (*bpp.AckDispatcher, *stubHandler) {
	t.Helper()
	h := &stubHandler{}
	r := &stubResolver{owners: owners}
	return bpp.NewAckDispatcher(h, r), h
}

// TestAL_AckDispatcher_StatusEnum_FailClosed pins acceptance §1.2 — 3
// 态 enum byte-identical, 枚举外值 reject (跟 al_2b_frames_test.go
// isValidAckStatus 同源, 此处 prod 路径).
func TestAL_AckDispatcher_StatusEnum_FailClosed(t *testing.T) {
	t.Parallel()

	d, _ := newDispatcherWith(t, map[string]string{"agent-A": "user-1"})
	sess := bpp.AckSessionContext{OwnerUserID: "user-1", PluginID: "plugin-X"}

	// 白名单 3 态合法 (resolver 校验后 handler 接).
	for _, st := range []string{
		bpp.AgentConfigAckStatusApplied,
		bpp.AgentConfigAckStatusRejected,
		bpp.AgentConfigAckStatusStale,
	} {
		f := bpp.AgentConfigAckFrame{
			Type: bpp.FrameTypeBPPAgentConfigAck, AgentID: "agent-A",
			Status: st, Reason: "",
		}
		if err := d.Dispatch(f, sess); err != nil {
			t.Errorf("status=%q rejected unexpectedly: %v", st, err)
		}
	}

	// 枚举外值 reject + sentinel match.
	for _, bad := range []string{
		"unknown", "ok", "fail", "",
		"APPLIED",   // 大小写漂
		"applying",  // 中间态漂
		"completed", // CV-4 状态漂入
	} {
		f := bpp.AgentConfigAckFrame{
			Type: bpp.FrameTypeBPPAgentConfigAck, AgentID: "agent-A",
			Status: bad,
		}
		err := d.Dispatch(f, sess)
		if err == nil {
			t.Errorf("status=%q accepted — should reject (acceptance §1.2 fail-closed)", bad)
			continue
		}
		if !bpp.IsAckStatusUnknown(err) {
			t.Errorf("status=%q sentinel mismatch: got %v, want errAckStatusUnknown", bad, err)
		}
	}
}

// TestAL_AckDispatcher_ReasonAL1aDict pins acceptance §3.2 — Reason
// 字典承袭 AL-1a 6 项 byte-identical. 跟 BPP-2.2 task_finished failed
// reason 同源 (改 = 改 8 处单测锁).
func TestAL_AckDispatcher_ReasonAL1aDict(t *testing.T) {
	t.Parallel()

	d, _ := newDispatcherWith(t, map[string]string{"agent-A": "user-1"})
	sess := bpp.AckSessionContext{OwnerUserID: "user-1", PluginID: "plugin-X"}

	// AL-1a 6 项字面合法 (rejected + reason 走字典).
	for _, r := range []string{
		agentpkg.ReasonAPIKeyInvalid,
		agentpkg.ReasonQuotaExceeded,
		agentpkg.ReasonNetworkUnreachable,
		agentpkg.ReasonRuntimeCrashed,
		agentpkg.ReasonRuntimeTimeout,
		agentpkg.ReasonUnknown,
	} {
		f := bpp.AgentConfigAckFrame{
			Type: bpp.FrameTypeBPPAgentConfigAck, AgentID: "agent-A",
			Status: bpp.AgentConfigAckStatusRejected, Reason: r,
		}
		if err := d.Dispatch(f, sess); err != nil {
			t.Errorf("reason=%q rejected unexpectedly: %v", r, err)
		}
	}

	// 字典外 reject (rejected + 非空 Reason).
	for _, bad := range []string{
		"timeout",          // AL-1a 是 runtime_timeout 不是 timeout
		"forbidden",        // 不在字典
		"runtime_oom",      // 不在字典
		"network_blocked",  // 不在字典
		"completed",        // CV-4 漂入
	} {
		f := bpp.AgentConfigAckFrame{
			Type: bpp.FrameTypeBPPAgentConfigAck, AgentID: "agent-A",
			Status: bpp.AgentConfigAckStatusStale, Reason: bad,
		}
		err := d.Dispatch(f, sess)
		if err == nil {
			t.Errorf("reason=%q accepted — should reject (AL-1a 6-dict)", bad)
			continue
		}
		if !bpp.IsAckReasonUnknown(err) {
			t.Errorf("reason=%q sentinel mismatch: got %v, want errAckReasonUnknown", bad, err)
		}
	}

	// applied 态 + 空 Reason 合法 (反约束: Reason 仅 rejected/stale 必填).
	f := bpp.AgentConfigAckFrame{
		Type: bpp.FrameTypeBPPAgentConfigAck, AgentID: "agent-A",
		Status: bpp.AgentConfigAckStatusApplied, Reason: "",
	}
	if err := d.Dispatch(f, sess); err != nil {
		t.Errorf("applied + empty reason rejected: %v", err)
	}
}

// TestAL_AckDispatcher_CrossOwnerReject pins acceptance §2.5 — frame
// agent_id 跟 sess.OwnerUserID owner 不匹配 → reject (REG-INV-002
// fail-closed 扫描器复用).
func TestAL_AckDispatcher_CrossOwnerReject(t *testing.T) {
	t.Parallel()

	d, h := newDispatcherWith(t, map[string]string{
		"agent-A": "user-1",
		"agent-B": "user-2",
	})

	// session is user-1 但 frame 引用 agent-B (user-2 拥有) → cross-owner.
	sess := bpp.AckSessionContext{OwnerUserID: "user-1", PluginID: "plugin-X"}
	f := bpp.AgentConfigAckFrame{
		Type: bpp.FrameTypeBPPAgentConfigAck, AgentID: "agent-B",
		Status: bpp.AgentConfigAckStatusApplied,
	}
	err := d.Dispatch(f, sess)
	if err == nil {
		t.Fatal("cross-owner accepted — should reject (REG-INV-002)")
	}
	if !bpp.IsAckCrossOwnerReject(err) {
		t.Errorf("sentinel mismatch: got %v, want errAckCrossOwnerReject", err)
	}
	if h.calls != 0 {
		t.Errorf("handler called on cross-owner reject — handler.calls=%d, want 0", h.calls)
	}

	// resolver 找不到 agent_id → 也是 cross-owner reject (soft reject).
	f.AgentID = "agent-Z"
	if err := d.Dispatch(f, sess); err == nil {
		t.Fatal("missing agent_id accepted — should reject")
	} else if !bpp.IsAckCrossOwnerReject(err) {
		t.Errorf("missing agent sentinel mismatch: got %v", err)
	}
}

// TestAL_AckDispatcher_HappyPath_DelegatesToHandler pins the seam
// contract: 校验全过 → handler.HandleAck 调一次, 参数 byte-identical
// 跟入参. handler 错误透传 (跟 BPP-2.1 ActionHandler 同模式).
func TestAL_AckDispatcher_HappyPath_DelegatesToHandler(t *testing.T) {
	t.Parallel()

	d, h := newDispatcherWith(t, map[string]string{"agent-A": "user-1"})
	sess := bpp.AckSessionContext{OwnerUserID: "user-1", PluginID: "plugin-X"}
	f := bpp.AgentConfigAckFrame{
		Type:          bpp.FrameTypeBPPAgentConfigAck,
		Cursor:        42,
		AgentID:       "agent-A",
		SchemaVersion: 3,
		Status:        bpp.AgentConfigAckStatusApplied,
		Reason:        "",
		AppliedAt:     1700000000001,
	}

	if err := d.Dispatch(f, sess); err != nil {
		t.Fatalf("happy path failed: %v", err)
	}
	if h.calls != 1 {
		t.Errorf("handler.calls=%d, want 1", h.calls)
	}
	if h.gotF != f {
		t.Errorf("handler frame mismatch: got %+v, want %+v", h.gotF, f)
	}
	if h.gotS != sess {
		t.Errorf("handler sess mismatch: got %+v, want %+v", h.gotS, sess)
	}

	// handler 错误透传.
	wantErr := errors.New("handler boom")
	h.wantErr = wantErr
	if err := d.Dispatch(f, sess); !errors.Is(err, wantErr) {
		t.Errorf("handler error not transparent: got %v, want %v", err, wantErr)
	}
}

// TestAL_AckDispatcher_NilArgsPanic pins boot-time defense (跟 BPP-2.1
// RegisterHandler nil panic 同模式 — prevents 0-coverage routes).
func TestAL_AckDispatcher_NilArgsPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("NewAckDispatcher(nil, nil) did not panic")
		}
	}()
	_ = bpp.NewAckDispatcher(nil, nil)
}

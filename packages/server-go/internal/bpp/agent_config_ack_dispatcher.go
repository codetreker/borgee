// Package bpp — agent_config_ack_dispatcher.go: AL-2b ack frame 入站
// dispatcher source-of-truth.
//
// Blueprint锚: docs/blueprint/current/plugin-protocol.md §1.5 (热更新 + 幂等
// reload + ack 回执) + §2.2 (data plane, Plugin → Server).
// Spec brief: docs/implementation/modules/al-2b-spec.md (烈马 #465 v0)
// + docs/implementation/modules/al-2b.2-server-hook-spec.md §1
// (`internal/bpp/agent_config_ack_dispatcher.go` 落点).
// Acceptance: docs/qa/acceptance-templates/al-2b.md §1.2 + §2.5 + §3.2.
//
// What this file does:
//   1. Validate AgentConfigAckFrame.Status ∈ 3-enum {applied, rejected,
//      stale}; enum 外 reject with AckErrCodeStatusUnknown (跟 BPP-2.2
//      task_outcome_unknown / BPP-2.3 config_field_disallowed 错码命名
//      同模式).
//   2. When Status ∈ {rejected, stale} 且 Reason 非空: 校验 Reason ∈
//      AL-1a 6 字典 byte-identical 同源 (跟 BPP-2.2 task_finished failed
//      reason 第 7 处, 此处第 8 处跟链 — 改 = 改 8 处单测锁:
//      AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 +
//      AL-4 #387/#461 + BPP-2.2 #485 + AL-2b #481, 不另起字典).
//   3. cross-owner reject — sess.AgentUserID (BPP-1 connect 时已认证
//      的 plugin owner) 跟 frame.AgentID 的 owner 不匹配 → reject +
//      log warn `bpp.ack_cross_owner_reject` (跟 anchor #360 owner-only
//      + REG-INV-002 fail-closed 扫描器同模式).
//   4. ActionHandler-style interface seam (跟 BPP-2.1 dispatcher.go
//      ActionHandler / cv-4.2 IterationStatePusher 同模式) — bpp 包
//      不 import internal/api, api 包注册 AgentConfigAckHandler.
//
// 反约束 (acceptance §3.2 + §4 grep 检查, 跟 al-2b-spec §3 byte-identical):
//   - admin god-mode 不入业务路径 — CI grep 检查 守 (al-2b-spec §3
//     第 3 行) count==0.
//   - cursor 唯一可信序 — CI grep 检查 守 (al-2b-spec §3 第 7 行) count==0.
//   - AL-2a 轮询路径下线 drift 防双轨 — CI grep 检查 守 (al-2b-spec §3
//     第 6 行) count==0.
//   - reason 字典不另起 — 复用 internal/agent/state.go::Reason* SSOT,
//     drift 则 BPP-2.2 task_finished + AL-2b ack 同破.
//   - bpp 包零 internal/api 依赖 — interface seam (跟 BPP-2.1 同模式).
package bpp

import (
	"errors"
	"fmt"

	"borgee-server/internal/agent/reasons"
)

// AckErrCode* — error code literals byte-identical 跟 BPP-2.2
// task_outcome_unknown / BPP-2.3 config_field_disallowed 命名同模式.
const (
	AckErrCodeStatusUnknown    = "bpp.ack_status_unknown"
	AckErrCodeReasonUnknown    = "bpp.ack_reason_unknown"
	AckErrCodeCrossOwnerReject = "bpp.ack_cross_owner_reject"
)

// errAckStatusUnknown / errAckReasonUnknown / errAckCrossOwnerReject
// — sentinels callers can errors.Is against to map to wire-level error
// codes (跟 BPP-2.1 errSemanticOpUnknown / BPP-2.2 errOutcomeUnknown
// 同模式).
var (
	errAckStatusUnknown    = errors.New("bpp: agent_config_ack status unknown (3-enum: applied/rejected/stale)")
	errAckReasonUnknown    = errors.New("bpp: agent_config_ack reason unknown (not in AL-1a 6 dict)")
	errAckCrossOwnerReject = errors.New("bpp: agent_config_ack cross-owner reject")
)

// IsAckStatusUnknown / IsAckReasonUnknown / IsAckCrossOwnerReject —
// sentinel matchers (跟 BPP-2.1 IsSemanticOpUnknown / BPP-2.2
// IsTaskOutcomeUnknown 同模式).
func IsAckStatusUnknown(err error) bool { return errors.Is(err, errAckStatusUnknown) }
func IsAckReasonUnknown(err error) bool { return errors.Is(err, errAckReasonUnknown) }
func IsAckCrossOwnerReject(err error) bool {
	return errors.Is(err, errAckCrossOwnerReject)
}

// validAckStatuses — 3-enum membership set byte-identical 跟 acceptance
// §1.2 CHECK enum (跟 al_2b_frames_test.go::isValidAckStatus 同源, 此处
// 提到 prod 路径).
var validAckStatuses = map[string]bool{
	AgentConfigAckStatusApplied:  true,
	AgentConfigAckStatusRejected: true,
	AgentConfigAckStatusStale:    true,
}

// validAL1aReason — REFACTOR-REASONS: SSOT 迁到 internal/agent/reasons.
// 改字面 = 改 reasons.ALL 一处即 8 处单测同步挂.
//
// 历史: 此处原 inline 6 字面 byte-identical 跟 agent/state.go Reason*
// (#249/#305/#321/#380/#454/#458/#481/#492 八处单测锁链), REFACTOR-REASONS
// 一 PR dedupe 到 internal/agent/reasons SSOT 包.
func validAL1aReason(s string) bool { return reasons.IsValid(s) }

// AckSessionContext is the per-plugin-connection context the
// AckDispatcher passes to the registered handler. Carries the
// authenticated plugin owner UUID (resolved via BPP-1 connect handshake)
// + the plugin id (audit trail).
//
// cross-owner reject 用 OwnerUserID 跟 frame.AgentID 的 owner 比对;
// 跟 BPP-2.1 SessionContext 同结构, 但分独立类型避免误用 (ack 不走
// semantic action 路径).
type AckSessionContext struct {
	OwnerUserID string // resolved via BPP-1 connect handshake
	PluginID    string // for audit / log only
}

// AgentConfigAckHandler is the seam between the bpp package and the api
// package for processing a validated AgentConfigAckFrame. The api
// package implements one handler that:
//   - Looks up agent_configs.schema_version SSOT 当前值;
//   - Compares against frame.SchemaVersion (mismatch → log stale, skip
//     last_applied_at 更新);
//   - For Status==applied: UPDATE agent_configs.last_applied_at;
//   - For Status∈{rejected,stale}: log warn (best-effort, 不 block).
//
// 反约束: bpp 包不 import internal/api — handler 是 interface 注入
// (跟 BPP-2.1 ActionHandler / cv-4.2 IterationStatePusher 同模式).
type AgentConfigAckHandler interface {
	HandleAck(frame AgentConfigAckFrame, sess AckSessionContext) error
}

// OwnerResolver resolves an agent_id to its owner user UUID for cross-
// owner ACL. The api package wires this to the agents table (跟 既有
// REST handler owner-only ACL 同闸 — anchor #360 / DM-2 #372 同模式).
//
// Returns ("", error) when agent_id 不存在; the dispatcher treats this
// as a soft reject (frame from disconnected agent — log warn but don't
// crash).
type OwnerResolver interface {
	OwnerOf(agentID string) (string, error)
}

// AckDispatcher routes validated AgentConfigAckFrame instances to the
// registered AgentConfigAckHandler. Validation order:
//
//  1. frame.Status ∈ validAckStatuses (3-enum). enum 外 → errAckStatusUnknown.
//  2. when Status ∈ {rejected, stale} 且 Reason 非空: Reason ∈
//     validAL1aReasons (AL-1a 6-dict). 字典外 → errAckReasonUnknown.
//  3. cross-owner check: resolver.OwnerOf(frame.AgentID) == sess.OwnerUserID.
//     mismatch → errAckCrossOwnerReject.
//  4. Delegate to handler.HandleAck(frame, sess).
//
// 反约束 (acceptance §4):
//   - admin god-mode 不入此路径 (handler.HandleAck 走 owner-only ACL,
//     CI grep 检查 守 al-2b-spec §3 第 3 行).
//   - 不接 raw HTTP / REST endpoint (interface seam, dispatcher 零
//     internal/api import — 跟 BPP-2.1 同模式).
type AckDispatcher struct {
	handler  AgentConfigAckHandler
	resolver OwnerResolver
}

// NewAckDispatcher creates a dispatcher wired to the given handler +
// owner resolver. Both MUST be non-nil; nil arguments are a server boot
// bug (panics — defense-in-depth, prevents 0-coverage routes from
// silently entering production).
func NewAckDispatcher(h AgentConfigAckHandler, r OwnerResolver) *AckDispatcher {
	if h == nil {
		panic("bpp: NewAckDispatcher handler must not be nil")
	}
	if r == nil {
		panic("bpp: NewAckDispatcher resolver must not be nil")
	}
	return &AckDispatcher{handler: h, resolver: r}
}

// Dispatch validates a plugin-upstream AgentConfigAckFrame and routes
// it to the registered handler. See type doc for validation order.
//
// Returns wrapped sentinel errors so callers can errors.Is to map to
// wire-level error codes (跟 BPP-2.1 Dispatch / BPP-2.2 ValidateTaskFinished
// 同模式).
func (d *AckDispatcher) Dispatch(frame AgentConfigAckFrame, sess AckSessionContext) error {
	// 1. Status 3-enum.
	if !validAckStatuses[frame.Status] {
		return fmt.Errorf("%w: status=%q (3-enum: applied/rejected/stale)",
			errAckStatusUnknown, frame.Status)
	}

	// 2. Reason 字典 (仅 rejected/stale 且 Reason 非空时校验).
	if frame.Status != AgentConfigAckStatusApplied && frame.Reason != "" {
		if !validAL1aReason(frame.Reason) {
			return fmt.Errorf("%w: reason=%q (AL-1a 6-dict: api_key_invalid/quota_exceeded/network_unreachable/runtime_crashed/runtime_timeout/unknown)",
				errAckReasonUnknown, frame.Reason)
		}
	}

	// 3. cross-owner check.
	owner, err := d.resolver.OwnerOf(frame.AgentID)
	if err != nil {
		return fmt.Errorf("%w: agent_id=%q resolve failed: %v",
			errAckCrossOwnerReject, frame.AgentID, err)
	}
	if owner != sess.OwnerUserID {
		return fmt.Errorf("%w: agent_id=%q owner=%q sess_owner=%q",
			errAckCrossOwnerReject, frame.AgentID, owner, sess.OwnerUserID)
	}

	// 4. Delegate.
	return d.handler.HandleAck(frame, sess)
}

// Package api — capability_grant.go: BPP-3.2.1 server-side handler for
// `request_capability_grant` semantic op (蓝图 auth-permissions.md §1.3
// 主入口字面 + bpp-3.2-spec.md §1 设计 ① + bpp-3.2-stance §1).
//
// Flow:
//   1. plugin SDK 收 BPP-3.1 PermissionDeniedFrame after AP-1 abac.go::
//      HasCapability false (POST /artifacts/:id/commits 等端点).
//   2. plugin 上行 SemanticActionFrame{Action: "request_capability_grant",
//      Payload: {agent_id, attempted_action, required_capability,
//      current_scope, request_id}}.
//   3. BPP-2.1 Dispatcher 路由到本 handler (CapabilityGrantHandler).
//   4. handler:
//      - 解析 payload 5 字段 (validate non-empty + capability ∈ AP-1
//        Capabilities const 白名单);
//      - lookup agent.OwnerID (GetUserByID);
//      - lookup owner's system channel (`type='system' AND created_by=ownerID`,
//        idempotent — CM-onboarding #203 既有 channel);
//      - 构 DM body byte-identical 跟 content-lock §1 字面:
//        `"{agent_name} 想 {attempted_action} 但缺权限 {required_capability}"`;
//      - 构 quick_action JSON byte-identical 跟 content-lock §2:
//        `{action,agent_id,capability,scope,request_id}` (action 锁
//        默认 'grant', client UI 渲染三按钮);
//      - INSERT message (sender_id='system', quick_action=JSON).
//
// 反向检查 (bpp-3.2-stance §1):
//   - DM 走 DM-2 messages + quick_action 既有 path, 不开新 channel 类型 /
//     不写新 system_message_kind enum (grep 检查 守).
//   - capability 必走 auth.Capabilities const, 不 hardcode 字面 (跟 AP-1
//     反向检查 #1 同源).
//   - admin god-mode 不入此路径 — agent 必有 OwnerID, admin 自己无 owner
//     不会触发此 op.

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"


	"borgee-server/internal/idgen"

	"borgee-server/internal/auth"
	"borgee-server/internal/bpp"
	"borgee-server/internal/store"
)

// CapabilityGrantDMTemplate is the DM body byte-identical lock 跟蓝图
// auth-permissions.md §1.3 字面 + bpp-3.2-content-lock.md §1.
//
// 改 = 改三处: 蓝图 §1.3 + spec §0 设计 ① + content-lock §1 + 此 const
// (grep 检查 "agent.*尝试.*权限\\|agent.*请求.*授权" count==0 守近义词漂禁).
const CapabilityGrantDMTemplate = "%s 想 %s 但缺权限 %s"

// CapabilityGrantDefaultAction is the default quick_action `action` value
// when server emits the system DM. Client renders three buttons (授权 /
// 拒绝 / 稍后 byte-identical content-lock §3); the JSON-stored action
// here is the *primary* one — clients overlay reject/snooze choices via
// the UI three-button render path.
const CapabilityGrantDefaultAction = "grant"

// CapabilityGrantErrCode* — error code literals byte-identical 跟
// bpp-3.2-content-lock.md §4.
const (
	CapabilityGrantErrCodeCapabilityDisallowed = "bpp.grant_capability_disallowed"
)

// errCapabilityDisallowed sentinel — caller maps to wire-level error
// code CapabilityGrantErrCodeCapabilityDisallowed via errors.Is.
var errCapabilityDisallowed = errors.New("bpp: grant capability disallowed (not in AP-1 Capabilities)")

// IsCapabilityDisallowed lets callers map the package-private sentinel
// to the wire-level error code (跟 bpp.IsSemanticOpUnknown 同模式).
func IsCapabilityDisallowed(err error) bool {
	return errors.Is(err, errCapabilityDisallowed)
}

// CapabilityGrantPayload is the parsed shape of SemanticActionFrame.Payload
// for op=='request_capability_grant'. byte-identical 跟 bpp-3.2-spec.md
// §1 设计 ① + content-lock §2.
//
// Field semantics:
//   - AgentID: 触发 frame 的 agent UUID
//   - AttemptedAction: AP-1 abac.go 403 body 字段 byte-identical (e.g.
//     "artifact.commit"); 跨 PR mismatch 守 (改 = 改五处, content-lock §5)
//   - RequiredCapability: ∈ auth.Capabilities 14 项 const (反向检查: 字面
//     hardcode 0 hit, 跟 AP-1 反向检查 #1 同源)
//   - CurrentScope: ∈ {*, channel:<id>, artifact:<id>} v1 三层
//   - RequestID: AP-1 调用方 trace UUID, plugin 端按此 key 做 retry cache
//     dedup (BPP-3.2.3 后续)
type CapabilityGrantPayload struct {
	AgentID            string `json:"agent_id"`
	AttemptedAction    string `json:"attempted_action"`
	RequiredCapability string `json:"required_capability"`
	CurrentScope       string `json:"current_scope"`
	RequestID          string `json:"request_id"`
}

// CapabilityGrantHandler implements bpp.ActionHandler for
// `request_capability_grant`. Registered at server boot via
// Dispatcher.RegisterHandler (跟 BPP-2.1 ActionHandler 同模式).
type CapabilityGrantHandler struct {
	Store *store.Store
	Now   func() time.Time
	NewID func() string
}

// HandleAction routes plugin → owner DM. Returns nil result blob (the
// plugin doesn't await a result; the owner DM + downstream
// agent_config_update push is the side effect).
//
// Validation (in order):
//  1. payload JSON valid + non-empty fields (agent_id, attempted_action,
//     required_capability, current_scope, request_id);
//  2. required_capability ∈ auth.Capabilities (枚举外 → errCapabilityDisallowed);
//  3. agent exists + has OwnerID (else: silent drop, log warn — agent
//     could have been deleted between AP-1 reject and frame upload);
//  4. owner has a system channel (CM-onboarding #203 既有), else return
//     error (caller should retry after re-bootstrapping owner).
func (h *CapabilityGrantHandler) HandleAction(frame bpp.SemanticActionFrame, sess bpp.SessionContext) ([]byte, error) {
	var p CapabilityGrantPayload
	if err := json.Unmarshal([]byte(frame.Payload), &p); err != nil {
		return nil, fmt.Errorf("bpp.grant_payload_malformed: %v", err)
	}
	// Trim + non-empty guard (跟 BPP-2.2 task subject 同模式 fail-loud).
	for name, v := range map[string]string{
		"agent_id":            p.AgentID,
		"attempted_action":    p.AttemptedAction,
		"required_capability": p.RequiredCapability,
		"current_scope":       p.CurrentScope,
		"request_id":          p.RequestID,
	} {
		if strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("bpp.grant_payload_field_empty: field=%q", name)
		}
	}
	// Capability 必走 AP-1 const 白名单 (反向检查 #1, grep 检查 守 hardcode 0 hit).
	if !auth.IsValidCapability(p.RequiredCapability) {
		return nil, fmt.Errorf("%w: capability=%q (AP-1 Capabilities 14 项)",
			errCapabilityDisallowed, p.RequiredCapability)
	}

	// Lookup agent + owner.
	agent, err := h.Store.GetUserByID(p.AgentID)
	if err != nil {
		return nil, fmt.Errorf("bpp.grant_agent_not_found: %v", err)
	}
	if agent.OwnerID == nil || *agent.OwnerID == "" {
		// Silent drop: orphan agent (e.g. agent.OwnerID 在 AP-1 reject 后
		// 被删). Log warn, no DM (跟 BPP-3.1 PushPermissionDenied plugin
		// offline frame 丢同精神).
		return nil, fmt.Errorf("bpp.grant_agent_no_owner: agent_id=%q", p.AgentID)
	}
	ownerID := *agent.OwnerID

	// Owner's system channel (idempotent — CM-onboarding #203 既有, 写
	// welcome 同 channel; 跟 store/welcome.go::CreateWelcomeChannelForUser
	// type='system' + created_by=ownerID 同源).
	var sysCh store.Channel
	if err := h.Store.DB().Where("created_by = ? AND type = ? AND deleted_at IS NULL",
		ownerID, "system").First(&sysCh).Error; err != nil {
		return nil, fmt.Errorf("bpp.grant_owner_system_channel_missing: owner_id=%q: %v",
			ownerID, err)
	}

	// 构 DM body 字面锁 (content-lock §1).
	body := fmt.Sprintf(CapabilityGrantDMTemplate,
		agent.DisplayName, p.AttemptedAction, p.RequiredCapability)

	// 构 quick_action JSON 字面锁 (content-lock §2).
	qa := struct {
		Action     string `json:"action"`
		AgentID    string `json:"agent_id"`
		Capability string `json:"capability"`
		Scope      string `json:"scope"`
		RequestID  string `json:"request_id"`
	}{
		Action:     CapabilityGrantDefaultAction,
		AgentID:    p.AgentID,
		Capability: p.RequiredCapability,
		Scope:      p.CurrentScope,
		RequestID:  p.RequestID,
	}
	qaBytes, err := json.Marshal(qa)
	if err != nil {
		return nil, fmt.Errorf("bpp.grant_quick_action_marshal: %v", err)
	}
	qaStr := string(qaBytes)

	// INSERT message via raw SQL (跟 store/welcome.go::CreateWelcomeChannel
	// ForUser system message 同模式 — sender_id='system', quick_action
	// 列由 migration v=7 提供).
	msgID := h.newID()
	now := h.now().UnixMilli()
	if err := h.Store.DB().Exec(`
		INSERT INTO messages (id, channel_id, sender_id, content, content_type, created_at, quick_action)
		VALUES (?, ?, 'system', ?, 'text', ?, ?)
	`, msgID, sysCh.ID, body, now, qaStr).Error; err != nil {
		return nil, fmt.Errorf("bpp.grant_dm_insert: %v", err)
	}
	return nil, nil
}

func (h *CapabilityGrantHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *CapabilityGrantHandler) newID() string {
	if h.NewID != nil {
		return h.NewID()
	}
	return idgen.NewID()
}

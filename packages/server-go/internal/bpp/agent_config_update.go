// Package bpp — agent_config_update.go: BPP-2.3 source-of-truth for
// the agent_config_update fields whitelist + idempotent reload validation.
//
// Blueprint锚: docs/blueprint/current/plugin-protocol.md §1.5 (配置热更新按字段
// 分类生效 + "plugin 必须支持幂等 reload, runtime 不缓存 agent 定义 —
// 每次 inference 前读最新 config") + §1.4 表字面 (Borgee 管: name/avatar/
// prompt/model/capabilities/enabled / Runtime 管: API key / 温度参数
// / token 上限 / 限速 / retry — 设计 ① "Borgee 不带 runtime").
//
// Spec brief: docs/implementation/modules/bpp-2-spec.md (战马E #460 v0)
// §0 设计 ③ + §1 拆段 BPP-2.3.
// Stance: docs/qa/bpp-2-stance-checklist.md §3 设计 ③ 反约束 checkbox.
// Content lock: docs/qa/bpp-2-content-lock.md §1 ② 6 fields 白名单字面.
//
// What this file does:
//   1. ConfigField enum lock — 6 项 byte-identical 跟蓝图 §1.4 表字面
//      (左列 "归 Borgee 管" 完整列表).
//   2. ValidateConfigPayload — parses the AgentConfigUpdateFrame.Payload
//      JSON (opaque on wire) + asserts every key ∈ valid whitelist;
//      runtime 调优字段 reject with ConfigErrCodeFieldDisallowed.
//   3. Track per-agent ConfigRev for idempotent reload — same
//      (agent_id, config_rev) pushed twice is a no-op (蓝图 §1.5 字面).
//
// 反约束 (acceptance §3 + content-lock §2):
//   - runtime 调优字段不入 frame payload — grep 检查 CI lint count==0
//     (acceptance §4.5).
//   - config 单源 server→plugin (plugin 不上行 config) — grep 检查
//     CI lint count==0 (acceptance §4.3).
//   - fields 白名单严闭 — 字典外值 reject + log warn
//     `bpp.config_field_disallowed`.
package bpp

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ConfigField enum — content-lock §1 ② byte-identical 跟蓝图 §1.4 表
// 左列字面 "归 Borgee 管 (用户选择项)" 完整列表. 改 = 改三处: 蓝图 §1.4
// + spec §0 设计 ③ + this enum.
const (
	ConfigFieldName         = "name"
	ConfigFieldAvatar       = "avatar"
	ConfigFieldPrompt       = "prompt"
	ConfigFieldModel        = "model"
	ConfigFieldCapabilities = "capabilities"
	ConfigFieldEnabled      = "enabled"
)

// ValidConfigFields is the 6-项白名单 set. Membership is the only
// gate at the dispatcher boundary — runtime 调优字段 MUST reject.
//
// 反约束 (acceptance §4.5 + content-lock §2 ⑤): runtime 调优字段
// (蓝图 §1.4 右列字面) 不入 frame payload.
var ValidConfigFields = map[string]bool{
	ConfigFieldName:         true,
	ConfigFieldAvatar:       true,
	ConfigFieldPrompt:       true,
	ConfigFieldModel:        true,
	ConfigFieldCapabilities: true,
	ConfigFieldEnabled:      true,
}

// ConfigErrCode* — error code literals byte-identical 跟 content-lock
// §1 ⑥ 同源.
const (
	ConfigErrCodeFieldDisallowed  = "bpp.config_field_disallowed"
	ConfigErrCodePayloadMalformed = "bpp.config_payload_malformed"
)

// errConfigFieldDisallowed / errConfigPayloadMalformed — sentinels.
var (
	errConfigFieldDisallowed  = errors.New("bpp: config field disallowed (not in 6-whitelist)")
	errConfigPayloadMalformed = errors.New("bpp: config payload not valid JSON object")
)

// IsConfigFieldDisallowed / IsConfigPayloadMalformed — sentinel matchers.
func IsConfigFieldDisallowed(err error) bool {
	return errors.Is(err, errConfigFieldDisallowed)
}
func IsConfigPayloadMalformed(err error) bool {
	return errors.Is(err, errConfigPayloadMalformed)
}

// ValidateConfigPayload parses frame.Blob (opaque on wire) as a
// flat JSON object + asserts every top-level key ∈ ValidConfigFields.
//
// Returns the parsed map on success (caller may consume the typed
// values). On reject:
//   - JSON parse failure → errConfigPayloadMalformed
//   - any key ∉ ValidConfigFields → errConfigFieldDisallowed (carries
//     the offending key for log warn).
//
// 反约束: runtime 调优字段 (蓝图 §1.4 右列字面) reject —设计 ③ guard
// at the BPP-2.3 frame ingress.
func ValidateConfigPayload(frame AgentConfigUpdateFrame) (map[string]any, error) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(frame.Blob), &parsed); err != nil {
		return nil, fmt.Errorf("%w: %v", errConfigPayloadMalformed, err)
	}
	for key := range parsed {
		if !ValidConfigFields[key] {
			return nil, fmt.Errorf("%w: field=%q (6-whitelist: name/avatar/prompt/model/capabilities/enabled)",
				errConfigFieldDisallowed, key)
		}
	}
	return parsed, nil
}

// ConfigRevTracker is the per-agent idempotency guard for plugin
// config reload (蓝图 §1.5 字面 "同一 update payload 重复推送不应有
// 副作用"). Stores the last-applied config_rev per agent_id; ShouldApply
// returns true only when the incoming rev is strictly greater than the
// last seen rev.
//
// 反约束: stale rev (incoming ≤ last) returns false WITHOUT error —
// it's a legitimate retry / network double-send, not a protocol
// violation. The caller logs at debug level + drops the frame.
//
// Thread-safety: ConfigRevTracker is NOT goroutine-safe. The BPP
// listener guarantees per-plugin-connection serialization (single
// reader per WS), which is the Borgee BPP-1 invariant — concurrent
// agent_config_update for the same agent_id from different plugin
// connections is itself a protocol violation (one runtime per agent,
// AL-4.1 #398 schema UNIQUE(agent_id) 设计 ① 字面承袭).
type ConfigRevTracker struct {
	last map[string]int64
}

// NewConfigRevTracker creates an empty per-agent rev tracker.
func NewConfigRevTracker() *ConfigRevTracker {
	return &ConfigRevTracker{last: make(map[string]int64)}
}

// ShouldApply returns true iff the incoming (agent_id, config_rev)
// pair represents a forward step (rev > last seen). On true, the
// tracker records the new rev. On false (stale or duplicate rev),
// the tracker leaves state unchanged — caller treats as no-op.
//
// 反约束: rev MUST be strictly increasing (蓝图 §1.5 字面 "幂等 reload"
// — same payload twice = no-op). Equal rev returns false (idempotent
// retry guard). Negative rev returns false (defensive — never seen
// in practice but spec doesn't forbid; we treat as stale).
func (t *ConfigRevTracker) ShouldApply(agentID string, configRev int64) bool {
	last := t.last[agentID]
	if configRev <= last {
		return false
	}
	t.last[agentID] = configRev
	return true
}

// LastRev returns the last-applied config_rev for agent_id, or 0 if
// the tracker has never seen this agent. Test seam — production code
// should use ShouldApply, not introspect state.
func (t *ConfigRevTracker) LastRev(agentID string) int64 {
	return t.last[agentID]
}

// Package bpp — heartbeat_decay.go: HB-3 v2.1 decay 三档 derive helper.
//
// Blueprint锚: docs/blueprint/current/plugin-protocol.md §1.6 (失联非 binary).
// Spec brief: docs/implementation/modules/hb-3-v2-spec.md §0.1 + §1
// HB-3 v2.1.
//
// 设计 (跟 stance §1+§4 byte-identical):
//
//   - **0 schema 改** — DecayState 从 last_heartbeat_at 反向 derive,
//     不裂表 / 不另起 sequence (grep 检查 守 production 不引入新表).
//   - **threshold byte-identical 跟 BPP-4** — StaleThreshold = 30 *
//     time.Second (跟 srvbpp/BPP-7 SDK HeartbeatInterval 同源).
//   - **enum 字面单源** — DecayState const 字面锁 (`fresh / stale /
//     dead`); grep 检查 hardcode 在 hb_3_v2*.go 外 0 hit.
//
// 反约束:
//   - nil-safe — DeriveDecayState(now, 0) 返 dead (永久不活); 负
//     lastHeartbeatAt 当 0 处理.
//   - 不挂 IO / 不挂 store dep — 纯 fn.

package bpp

import "time"

// DecayState — 3 字面单源 byte-identical 跟 spec §1 字面 + acceptance
// §1 enum 字面.
type DecayState string

const (
	// DecayStateFresh — last heartbeat ≤ StaleThreshold (plugin healthy).
	DecayStateFresh DecayState = "fresh"
	// DecayStateStale — StaleThreshold < last heartbeat ≤ DeadThreshold.
	DecayStateStale DecayState = "stale"
	// DecayStateDead — last heartbeat > DeadThreshold (plugin gone).
	DecayStateDead DecayState = "dead"
)

// StaleThreshold — same wall-clock value as BPP-4 #499 watchdog stale
// threshold (30s) and BPP-7 SDK HeartbeatInterval (30s). 改 = 改三处
// 同步 (BPP-4 watchdog const + BPP-7 SDK const + 此 const).
const StaleThreshold = 30 * time.Second

// DeadThreshold — fully-failed plugin. > DeadThreshold means the
// next bucket transition fires the BPP-8 RecordHeartbeatTimeout audit.
const DeadThreshold = 60 * time.Second

// DeriveDecayState — pure function. now and lastHeartbeatAt are Unix
// milliseconds. Negative or zero lastHeartbeatAt counts as "no heartbeat
// ever" → dead.
//
// 设计 ① — 反向 derive 不查表 / 不挂 store / 不挂 IO. Reverse-monotonic
// safe (now < lastHeartbeatAt → fresh, since the future-dated heartbeat
// is treated as healthy by virtue of < StaleThreshold).
func DeriveDecayState(now, lastHeartbeatAt int64) DecayState {
	if lastHeartbeatAt <= 0 {
		return DecayStateDead
	}
	delta := now - lastHeartbeatAt
	if delta < 0 {
		// future-dated heartbeat — clamp to 0 for fresh.
		delta = 0
	}
	d := time.Duration(delta) * time.Millisecond
	switch {
	case d <= StaleThreshold:
		return DecayStateFresh
	case d <= DeadThreshold:
		return DecayStateStale
	default:
		return DecayStateDead
	}
}

// IsCrossBucketTransition returns true iff the two states are in
// different decay buckets — used by the watchdog wire (HB-3 v2.2) to
// decide whether to fire BPP-8 RecordHeartbeatTimeout audit. Same-bucket
// transitions are silently no-op (设计 ⑦ — 同档不重复触, 防止 audit
// log 被 high-frequency noise 淹没).
func IsCrossBucketTransition(from, to DecayState) bool {
	return from != to
}

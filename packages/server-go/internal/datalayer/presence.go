// DL-1 — PresenceStore interface (蓝图 §4 B 第 2 条).
//
// 设计 ① (DL-1 spec §0): IsOnline / Sessions byte-identical 跟蓝图.
// v1 实现 InMemoryPresence 走 AL-3 #324 既有 presence.PresenceTracker
// (内部 in-memory map) byte-identical 不破 — 跟 G2.5 contract 锁同源.
//
// RT-3 ⭐ 设计 ② (rt-3-spec.md §0.2): PresenceState 4 态 enum SSOT —
// online / away / offline / thinking (蓝图 §1.4 活物感 4 态). 单源 const
// grep 检查 count==4 hit (反 5 态漂 / 反 false-loading indicator 漂入).
//
// 切换路径 (留 v3+):
//   - InMemoryPresence (v1) → presence.PresenceTracker
//   - DistributedPresence  → Redis / NATS pub-sub (留 DL-3 阈值哨触发)
package datalayer

import "context"

// PresenceState — RT-3 ⭐ 4 态 enum SSOT (蓝图 §1.4 活物感).
// 跟 reasons.IsValid #496 / AP-4-enum #591 / NAMING-1 #614 enum SSOT 模式承袭.
//
// 反约束 (rt-3-spec.md §0.2 + content-lock §3):
//   - 4 态封闭枚举, 不另起第 5 态 (反 type-T-indicator 漂入: t-y-p-i-n-g
//     / c-o-m-p-o-s-i-n-g / 输 入 中 等同义词跨 enum 漂)
//   - grep 检查 `PresenceStateOnline|PresenceStateAway|PresenceStateOffline|
//     PresenceStateThinking` count==4 hit (单源)
//   - thinking 态必带 subject (走 bpp.ValidateTaskStarted SSOT, 反空字符串 reject)
type PresenceState string

const (
	// PresenceStateOnline — 用户/agent 至少 1 live session (跟 IsOnline 同源).
	PresenceStateOnline PresenceState = "online"
	// PresenceStateAway — 5min 无活动 (last-seen 阈值, RT-3 client UI 派生).
	PresenceStateAway PresenceState = "away"
	// PresenceStateOffline — 0 live session (跟 IsOnline 反义).
	PresenceStateOffline PresenceState = "offline"
	// PresenceStateThinking — agent 在执行任务 (走 bpp.task_started frame
	// + Subject 字段必带). 反"假 loading" 漂 — Subject 反空字符串守门
	// (rt-3-spec.md §0.2 + 蓝图 §1.1 ⭐ 关键纪律).
	PresenceStateThinking PresenceState = "thinking"
)

// PresenceStore is the SSOT interface for "is user X reachable?" queries.
// v1 walls thru AL-3 PresenceTracker; v3+ swap underlying implementation
// without touching consumers (handlers go thru this seam, not directly).
type PresenceStore interface {
	// IsOnline reports whether the user/agent has at least one live session.
	// 跟 G2.5 contract 同源 (presence.PresenceTracker.IsOnline).
	IsOnline(ctx context.Context, userID string) (bool, error)

	// Sessions returns the live session ids for the user. Empty slice means
	// offline. Stable order is not required (跟 #310 锁同精神).
	Sessions(ctx context.Context, userID string) ([]string, error)
}

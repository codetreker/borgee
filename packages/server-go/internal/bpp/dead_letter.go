// Package bpp — dead_letter.go: BPP-4.2 server→plugin push 失败 audit
// log (best-effort 立场: log warn, **不入持久队列**, RT-1.3 cursor
// replay 兜底).
//
// Blueprint锚: docs/blueprint/current/plugin-protocol.md §1.5 (runtime 不缓存)
// + RT-1.3 #296 cursor replay (重连后 plugin 主动拉缺失 frame).
// Spec brief: docs/implementation/modules/bpp-4-spec.md §0.3 + §1
// BPP-4.2. Acceptance: docs/qa/acceptance-templates/bpp-4.md §2.
//
// 立场 (跟 stance §3 byte-identical):
//   - **ack best-effort 不重发** (蓝图 §1.5 立场承袭). server→plugin push
//     失败 (sent=false, plugin offline) → log warn + audit hint, **不入
//     队列**. plugin 重连后走 RT-1.3 cursor replay 主动拉, server 端不
//     主动重发.
//   - **dead-letter audit log schema byte-identical 跟 HB-1/HB-2 audit**
//     (5 字段: actor / action / target / when / scope). 改 = 改三处单测锁
//     (跟 HB-4 §1.5 release gate 第 4 行 "审计日志格式锁定" 守门同源).
//
// 反约束 (acceptance §4.3):
//   - 反向 grep `pendingAcks\|retryQueue\|deadLetterQueue\|ackTimeout.*resend`
//     0 hit (CI lint 守门, 防偷偷下沉 v2 retry 路径).
//   - 反向 grep `time.*Ticker.*resend\|retry.*frame.*backoff` 0 hit (本
//     dead_letter.go 文件里 0 ticker, 0 retry; 仅 log + return).

package bpp

import (
	"log/slog"
)

// DeadLetterAuditEntry — 5-field audit log schema byte-identical 跟
// HB-1 install-butler audit (docs/implementation/modules/hb-1-spec.md §4
// 反约束第 7) + HB-2 host-bridge IPC audit (docs/implementation/modules/
// hb-2-spec.md §4 反约束第 5) 三处同源.
//
// 改 = 改三处:
//   1. 此 struct 字段名 + JSON tag (BPP-4)
//   2. HB-1 install-butler audit struct (待 HB-1 实施)
//   3. HB-2 host-bridge IPC audit struct (待 HB-2 实施)
//
// HB-4 §1.5 release gate 第 4 行 "审计日志格式锁定 JSON schema (含
// actor / action / target / when / scope)" 守门同源.
type DeadLetterAuditEntry struct {
	Actor  string `json:"actor"`  // "server" (BPP-4 dead-letter 唯一 actor)
	Action string `json:"action"` // "frame_drop"
	Target string `json:"target"` // "<agent_id>"
	When   int64  `json:"when"`   // Unix ms
	Scope  string `json:"scope"`  // "<frame_type>:cursor=<cursor>"
}

// LogFrameDroppedPluginOffline — 单源 dead-letter 入口. 调用方
// (al_2b_2_agent_config_push.go 等 push 失败路径) sent=false 时调.
//
// **不**入持久队列, 不重发, 不挂 timer — 仅 log warn + audit hint.
// 重连后 plugin 走 RT-1.3 #296 cursor replay 主动拉缺失 frame.
//
// log key `bpp.frame_dropped_plugin_offline` byte-identical 跟
// content-lock §1.③ 单源锁 (改 = 改三处: 此函数 + content-lock + acceptance).
func LogFrameDroppedPluginOffline(logger *slog.Logger, entry DeadLetterAuditEntry) {
	if logger == nil {
		return
	}
	logger.Warn("bpp.frame_dropped_plugin_offline",
		"actor", entry.Actor,
		"action", entry.Action,
		"target", entry.Target,
		"when", entry.When,
		"scope", entry.Scope)
}

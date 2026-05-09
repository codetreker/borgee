// Package datalayer — must_persist_kinds.go: DL-2 §3 必落 kind enum SSOT.
//
// Spec: docs/implementation/modules/dl-2-spec.md §0 设计 ② + 蓝图 §3.4
// 隐私契约必落 4 类.
//
// 设计:
//   - 4 类必落 kind (perm.grant / perm.revoke / impersonate.* / agent.state /
//     admin.force_*) 永不 retention sweeper 删 (隐私契约 = 永久审计).
//   - SSOT 单源, 反 inline 字面漂 (grep 检查 `mustPersistKinds`/`MustPersistKind`
//     count==1 hit, 跟 reasons.IsValid #496 / AP-4-enum #591 同精神承袭).

package datalayer

import "strings"

// MustPersistKindPrefixes is the canonical set of event kind prefixes that
// MUST persist forever (never reaped by retention sweeper).
//
// 蓝图 §3.4 隐私契约 4 类:
//  1. 权限授予/撤销 — `perm.grant`, `perm.revoke`
//  2. 模拟会话 — `impersonate.start`, `impersonate.end`
//  3. agent 上下线状态切换 — `agent.state` (busy/idle/error/offline)
//  4. admin 强删/禁用 — `admin.force_delete`, `admin.force_disable`
var MustPersistKindPrefixes = []string{
	"perm.",
	"impersonate.",
	"agent.state",
	"admin.force_",
}

// IsMustPersistKind reports whether the kind matches any must-persist prefix.
// Sweeper consults this before issuing DELETE — must-persist rows skip retention.
func IsMustPersistKind(kind string) bool {
	for _, p := range MustPersistKindPrefixes {
		if strings.HasPrefix(kind, p) {
			return true
		}
	}
	return false
}

// DefaultRetentionDays for events not in MustPersistKindPrefixes and without
// an explicit retention_days override. Per spec §0 设计 ②:
//   - default: 90 days
//   - per-channel events (channel.*, message.*): 30 days
//   - agent_task / artifact: 60 days
//
// retentionDaysForKind returns the effective default for a given kind.
// Caller may still override via row-level retention_days column.
func RetentionDaysForKind(kind string) int {
	if IsMustPersistKind(kind) {
		// must-persist: sentinel -1 means "never reap"
		return -1
	}
	switch {
	case strings.HasPrefix(kind, "channel.") || strings.HasPrefix(kind, "message."):
		return 30
	case strings.HasPrefix(kind, "agent_task.") || strings.HasPrefix(kind, "artifact."):
		return 60
	default:
		return 90
	}
}

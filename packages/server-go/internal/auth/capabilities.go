// Package auth — capabilities.go: AP-1 设计 ③ capability const 白名单
// (≤30, byte-identical 跟 spec §1 ③ + 蓝图 auth-permissions.md §1).
//
// 单源协议: 所有 endpoint authz 必须用本文件 const, 严禁 hardcode 字面
// permission name. 反约束 grep 锁:
//
//   git grep -nE 'HasCapability\("[a-z_]+"' packages/server-go/internal/api/
//   # 期望 0 hit (应改为 HasCapability(ctx, auth.CommitArtifact, scope))
//
// Spec锚: docs/implementation/modules/ap-1-spec.md §1 设计 ③ + §2 反约束 #1.
// 蓝图锚: docs/blueprint/current/auth-permissions.md §1 (ABAC + UI bundle 混合).
//
// admin god-mode capability 不在此白名单 — admin 走 /admin-api/* 单独
// middleware (admin.RequireAdmin), ADM-0 §1.3 红线 + spec §1 设计 ③ 字面.
package auth

// v1 capability 字面白名单 (spec §1 设计 ③ byte-identical).
//
// 改 = 改三处+: spec §1 ③ + 蓝图 auth-permissions.md §1 + acceptance
// `docs/qa/acceptance-templates/ap-1.md` §字面锁 + 此 const.
const (
	// channel scope (`*` / `channel:<id>`)
	ReadChannel   = "channel.read"
	WriteChannel  = "channel.write"
	DeleteChannel = "channel.delete"

	// artifact scope (`*` / `channel:<id>` / `artifact:<id>`)
	ReadArtifact     = "artifact.read"
	WriteArtifact    = "artifact.write"
	CommitArtifact   = "artifact.commit"
	IterateArtifact  = "artifact.iterate"
	RollbackArtifact = "artifact.rollback"

	// messaging
	MentionUser = "user.mention"
	ReadDM      = "dm.read"
	SendDM      = "dm.send"

	// channel admin (channel-scoped)
	ManageMembers = "channel.manage_members"
	InviteUser    = "channel.invite"
	ChangeRole    = "channel.change_role"
)

// ALL is the canonical ordered slice of capability literals — single
// source of truth for AP-4-enum (spec §0 设计 ①). Order is byte-identical
// to const block above (channel scope → artifact scope → messaging →
// channel admin); change one place to add a capability — init() rebuilds
// Capabilities map automatically; reflect-lint test守 ALL ↔ const drift.
//
// 反约束: 不准直接 mutate `Capabilities` map (grep 检查
// `Capabilities\[".*"\]\s*=` packages/server-go/internal/auth/ 仅 init() 1 hit).
var ALL = []string{
	ReadChannel,
	WriteChannel,
	DeleteChannel,
	ReadArtifact,
	WriteArtifact,
	CommitArtifact,
	IterateArtifact,
	RollbackArtifact,
	MentionUser,
	ReadDM,
	SendDM,
	ManageMembers,
	InviteUser,
	ChangeRole,
}

// Capabilities is the canonical full list (membership lookup + future
// CI lint reflection). AP-4-enum 设计 ① — auto-built from ALL via init();
// no direct literal init. Adding a new capability = add to ALL only.
var Capabilities map[string]bool

func init() {
	Capabilities = make(map[string]bool, len(ALL))
	for _, c := range ALL {
		Capabilities[c] = true
	}
}

// IsValidCapability is the single-source helper for handler-side
// capability validity checks (spec §0 设计 ③ — reasons.IsValid #496 同模式).
// Handlers must call this; direct `auth.Capabilities[name]` access in
// `internal/api/` is reverse-grep banned (count==0).
func IsValidCapability(name string) bool {
	return Capabilities[name]
}

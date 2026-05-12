// Package auth — capabilities.go: AP-1 design 3 capability const allowlist
// (≤30, kept in sync with spec §1 ③ + blueprint auth-permissions.md §1).
//
// Single-source rule: all endpoint authz must use these consts instead of
// hardcoded permission names. Grep check:
//
//   git grep -nE 'HasCapability\("[a-z_]+"' packages/server-go/internal/api/
//   # 期望 0 hit (应改为 HasCapability(ctx, auth.CommitArtifact, scope))
//
// Spec 依据: docs/implementation/modules/ap-1-spec.md §1 设计 ③ + §2 约束 #1.
// 蓝图依据: docs/blueprint/current/auth-permissions.md §1 (ABAC + UI bundle 混合).
//
// Admin god-mode capability is not in this allowlist. Admin requests use
// separate /admin-api/* middleware (admin.RequireAdmin), per ADM-0 §1.3
// and spec §1 design 3.
package auth

// v1 capability literal allowlist (spec §1 design 3).
//
// Changes must stay aligned with spec §1 ③, blueprint auth-permissions.md §1,
// `docs/qa/acceptance-templates/ap-1.md`, and this const block.
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
// source of truth for AP-4-enum (spec §0 design 1). Order matches the
// const block above (channel scope → artifact scope → messaging →
// channel admin); change one place to add a capability — init() rebuilds
// Capabilities map automatically; the reflect-lint test catches ALL ↔ const drift.
//
// Constraint: do not mutate `Capabilities` map directly (reverse grep
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

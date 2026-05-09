// Package api — chn_9_visibility.go: CHN-9 channel privacy 三态 const
// + IsValidVisibility predicate (single-source).
//
// Blueprint: channel-model.md §2 不变量 + §1.4 红线. Spec:
// docs/implementation/modules/chn-9-spec.md (战马D v0). 0 schema 改 —
// channels.visibility TEXT 列复用 CHN-1.1 #267 既有, 加第 3 enum
// `creator_only` 跟既有 `private`/`public` 共三态.
//
// 三向锁 (chn-9-content-lock.md §3): server const 跟 client lib/visibility.ts
// VISIBILITY_* 跟 DB 字面 byte-identical. 改一处 = 改三处.
//
// 反约束 (chn-9-spec.md §0):
//   - 设计 ① 0 schema — channels 表不动, 仅 app 层扩 enum 校验.
//   - 设计 ② 三向锁 byte-identical (server + client + DB).
//   - 设计 ③ owner-only — visibility PATCH 走既有 channel.manage_visibility
//     permission (CHN-1.2 ACL byte-identical 不动); admin god-mode 不挂.
//     creator_only 不 leak (ListChannelsWithUnread `visibility = 'public'`
//     filter byte-identical 不动 — 反向 unit 守门).
package api

// VisibilityCreatorOnly is the strictest tier (CHN-9 新增): only the
// creator + admin can see the channel; non-creator members 也看不到
// (跟 ChannelMembersModal CHN-1.2 既有 channel.manage_visibility 权限
// 路径同源 ACL).
const VisibilityCreatorOnly = "creator_only"

// VisibilityMembers is the legacy `private` tier — channel members
// only. Alias of CHN-1 既有字面 'private' for backward compat.
const VisibilityMembers = "private"

// VisibilityOrgPublic is the legacy `public` tier — same-org peers
// can preview. Alias of CHN-1 既有字面 'public' for backward compat.
const VisibilityOrgPublic = "public"

// VisibilityValid is the byte-identical 3-tuple of accepted enum
// values. 跟 client VISIBILITY_VALID byte-identical.
var VisibilityValid = []string{
	VisibilityCreatorOnly,
	VisibilityMembers,
	VisibilityOrgPublic,
}

// IsValidVisibility reports whether the given visibility string is one
// of the three accepted enum values. Single-source predicate; 调用方
// 禁止 inline `s == "public" || s == "private"` (grep 检查项 — handler
// 走此谓词).
func IsValidVisibility(s string) bool {
	switch s {
	case VisibilityCreatorOnly, VisibilityMembers, VisibilityOrgPublic:
		return true
	default:
		return false
	}
}

// VisibilityRejectMessage is the byte-identical user-facing reject
// string returned by handlers when body.visibility is invalid. Single-
// source so vitest reflect can lock 跟 client side.
const VisibilityRejectMessage = "Visibility must be 'creator_only', 'private', or 'public'"

// Package api — chn_9_visibility.go: CHN-9 channel privacy three-state const
// + IsValidVisibility predicate (single-source).
//
// Blueprint: channel-model.md §2 invariant + §1.4 boundary. Spec:
// docs/implementation/modules/chn-9-spec.md. No schema change: the
// channels.visibility TEXT column reuses CHN-1.1 #267 and adds `creator_only`
// beside the existing `private` / `public` values.
//
// Three-way invariant (chn-9-content-lock.md §3): server consts, client
// lib/visibility.ts VISIBILITY_*, and DB literals stay byte-identical. Changing
// one requires updating all three.
//
// Constraints (chn-9-spec.md §0):
//   - Design ① no schema change: keep the channels table unchanged; extend enum
//     validation in application code only.
//   - Design ② three-way byte-identical lock (server + client + DB).
//   - Design ③ owner-only: visibility PATCH uses the existing
//     channel.manage_visibility permission, preserving CHN-1.2 ACL behavior; no
//     admin route is mounted. creator_only must not leak through
//     ListChannelsWithUnread; the `visibility = 'public'` filter remains
//     byte-identical and is covered by reverse unit tests.
package api

// VisibilityCreatorOnly is the strictest tier: only the creator + admin can see
// the channel; non-creator members cannot. It shares the existing CHN-1.2
// channel.manage_visibility ACL path used by ChannelMembersModal.
const VisibilityCreatorOnly = "creator_only"

// VisibilityMembers is the legacy `private` tier — channel members
// only. Alias of the CHN-1 literal 'private' for backward compatibility.
const VisibilityMembers = "private"

// VisibilityOrgPublic is the legacy `public` tier — same-org peers
// can preview. Alias of the CHN-1 literal 'public' for backward compatibility.
const VisibilityOrgPublic = "public"

// VisibilityValid is the byte-identical 3-tuple of accepted enum
// values. Byte-identical with client VISIBILITY_VALID.
var VisibilityValid = []string{
	VisibilityCreatorOnly,
	VisibilityMembers,
	VisibilityOrgPublic,
}

// IsValidVisibility reports whether the given visibility string is one
// of the three accepted enum values. Single-source predicate; callers must not
// inline `s == "public" || s == "private"`. Grep checks require handlers to use
// this predicate.
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
// source so Vitest reflection can lock it with the client side.
const VisibilityRejectMessage = "Visibility must be 'creator_only', 'private', or 'public'"

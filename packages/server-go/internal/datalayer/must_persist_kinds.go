// Package datalayer — must_persist_kinds.go: DL-2 §3 canonical must-persist kind enum.
//
// Spec: docs/implementation/modules/dl-2-spec.md §0 principle ② + blueprint
// §3.4 privacy contract for 4 must-persist categories.
//
// Policy:
//   - 4 must-persist kind categories (perm.grant / perm.revoke / impersonate.* /
//     agent.state / admin.force_*) are never deleted by the retention sweeper
//     because the privacy contract requires permanent audit.
//   - This centralized list avoids inline literal drift. Search for
//     `mustPersistKinds`/`MustPersistKind` must find one definition, matching the
//     reasons.IsValid #496 / AP-4-enum #591 pattern.

package datalayer

import "strings"

// MustPersistKindPrefixes is the canonical set of event kind prefixes that
// MUST persist forever (never reaped by retention sweeper).
//
// Blueprint §3.4 privacy contract categories:
//  1. permission grant/revoke — `perm.grant`, `perm.revoke`
//  2. impersonation sessions — `impersonate.start`, `impersonate.end`
//  3. agent state transitions — `agent.state` (busy/idle/error/offline)
//  4. admin force-delete/disable — `admin.force_delete`, `admin.force_disable`
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
// an explicit retention_days override. Per spec §0 principle ②:
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

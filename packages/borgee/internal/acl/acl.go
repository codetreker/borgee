// Package acl implements the HB-2 IPC request gate: path normalization,
// cross-agent ACL, and grants validation. Every IPC call enters through it.
//
// hb-2-spec.md §4 negative constraints: #2 rejects path escapes, #4 enforces
// cross-agent ACL, and #7 rejects write-class IPC.
package acl

import (
	"context"
	"path/filepath"
	"strings"

	"borgee/internal/grants"
	"borgee/internal/reasons"
)

// Action is an IPC request action. Only the read-only set is allowed; write
// classes are rejected.
type Action string

const (
	ActionListFiles      Action = "list_files"
	ActionReadFile       Action = "read_file"
	ActionNetworkEgress  Action = "network_egress"
)

// readOnlyActions is the negative-constraint #7 allowlist; every action outside
// this set is rejected.
var readOnlyActions = map[Action]bool{
	ActionListFiles:     true,
	ActionReadFile:      true,
	ActionNetworkEgress: true,
}

// IsReadOnly is the reverse-enumeration source for tests covering rejected
// write forms (write_file / delete_file / chmod / chown / mkdir / rmdir / mv /
// cp ...).
func IsReadOnly(a Action) bool {
	return readOnlyActions[a]
}

// Gate decides whether an IPC request is allowed without starting real IO;
// tests can inject a mock consumer.
type Gate struct {
	Grants grants.Consumer
}

// New constructs a gate with a caller-provided consumer. v0(C) used the mock;
// the landed HB-3 path uses SQL.
func New(c grants.Consumer) *Gate {
	return &Gate{Grants: c}
}

// Decision is the ACL decision result.
type Decision struct {
	Allow  bool
	Reason reasons.Reason // denial reason (OK when Allow=true)
	Scope  string         // matched grant scope for audit target/scope
}

// Decide is the main entrypoint and evaluates (handshakeAgentID,
// requestAgentID, action, target).
//
// handshakeAgentID is the agent_id registered during IPC handshake and held by
// the daemon. requestAgentID is the agent_id in the current request payload. A
// mismatch yields cross_agent_reject (negative constraint #4).
func (g *Gate) Decide(ctx context.Context, handshakeAgentID, requestAgentID string, action Action, target string) Decision {
	// 1. Reject write-class actions (negative constraint #7), guarded by reverse-enumeration tests.
	if !IsReadOnly(action) {
		return Decision{Allow: false, Reason: reasons.IOFailed}
	}
	// 2. Enforce cross-agent ACL (negative constraint #4).
	if handshakeAgentID == "" || requestAgentID == "" || handshakeAgentID != requestAgentID {
		return Decision{Allow: false, Reason: reasons.CrossAgentReject}
	}
	// 3. Normalize paths and reject traversal (negative constraint #2; file actions only).
	scope := target
	if action == ActionListFiles || action == ActionReadFile {
		clean, ok := normalizePath(target)
		if !ok {
			return Decision{Allow: false, Reason: reasons.PathOutsideGrants}
		}
		scope = "fs:" + clean
	} else if action == ActionNetworkEgress {
		// network_egress: scope = "egress:<host>"; caller has already normalized the URL.
		scope = "egress:" + target
	}
	// 4. Look up grants through the read-only consumer; negative constraint #3 forbids caching.
	mc, ok := g.Grants.(interface {
		LookupRaw(context.Context, string, string) (grants.Grant, bool, bool, error)
	})
	if ok {
		_, exists, expired, err := mc.LookupRaw(ctx, requestAgentID, scope)
		if err != nil {
			return Decision{Allow: false, Reason: reasons.IOFailed}
		}
		if !exists {
			return Decision{Allow: false, Reason: reasons.GrantNotFound}
		}
		if expired {
			return Decision{Allow: false, Reason: reasons.GrantExpired}
		}
		return Decision{Allow: true, Reason: reasons.OK, Scope: scope}
	}
	gr, ok2, err := g.Grants.Lookup(ctx, requestAgentID, scope)
	if err != nil {
		return Decision{Allow: false, Reason: reasons.IOFailed}
	}
	if !ok2 {
		return Decision{Allow: false, Reason: reasons.GrantNotFound}
	}
	return Decision{Allow: true, Reason: reasons.OK, Scope: gr.Scope}
}

// normalizePath rejects traversal: no .. segment, absolute paths only, and no
// NUL byte. It does not resolve symlinks; runtime IO remains guarded by the OS
// layer, Landlock / sandbox-exec, and the sandbox build-tag split from
// hb-2-spec.md §5.5.
func normalizePath(p string) (string, bool) {
	if p == "" || strings.ContainsRune(p, 0) {
		return "", false
	}
	if !filepath.IsAbs(p) {
		return "", false
	}
	clean := filepath.Clean(p)
	// Clean can collapse .. across roots (Linux Clean("/a/../b") = "/b"), so
	// scan the original path before accepting the cleaned result.
	for _, seg := range strings.Split(p, string(filepath.Separator)) {
		if seg == ".." {
			return "", false
		}
	}
	return clean, true
}

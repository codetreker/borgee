// Package presence holds the contract that gates G2.5 (Phase 2 exit) and
// will be implemented in full at Phase 4 / AL-3 (agent-lifecycle.md §3 +
// §2.3 four-state model).
//
// This file is the Phase 2 exit placeholder for announcement #268's
// G2.5 tracking row. The interface signature is fixed here so RT-* and BPP-*
// can wire against a stable shape during Phase 3, while the real impl
// (presence map, session expiry, BPP `session.connected` frame trigger)
// lands at AL-3.
//
// Path contract: internal/presence/contract.go (G2.5 grep anchor).
// Symbol contract: PresenceTracker.IsOnline + PresenceTracker.Sessions.
package presence

// PresenceTracker is the authoritative read API for "is agent X reachable
// right now". Phase 4 / AL-3 will provide the implementation backed by the
// /ws hub + BPP `session.connected` / `session.disconnected` frames.
//
// Phase 2 callers (RT-0 / CM-4.3b offline detection) MUST depend on this
// interface, not on any concrete map — see phase-2-exit-gate.md §G2.5.
type PresenceTracker interface {
	// IsOnline reports whether the user/agent has at least one live session.
	// MUST be O(1) and safe to call from the hot path of message routing.
	IsOnline(userID string) bool

	// Sessions returns the live session ids for the user. Empty slice means
	// offline. The caller MUST NOT mutate the returned slice.
	Sessions(userID string) []string
}

// DL-1 — PresenceStore interface (blueprint §4 B item 2).
//
// Principle ① (DL-1 spec §0): IsOnline / Sessions preserve the exact
// blueprint. v1 InMemoryPresence uses the existing AL-3 #324
// presence.PresenceTracker (internal in-memory map) without changing behavior,
// matching the G2.5 contract lock.
//
// RT-3 ⭐ principle ② (rt-3-spec.md §0.2): PresenceState is the canonical set for the
// 4-state enum: online / away / offline / thinking (blueprint §1.4 live-presence
// feel). Search count==4 protects against adding a 5th state or a
// misleading loading-state synonym.
//
// Implementation swap path (v3+):
//   - InMemoryPresence (v1) → presence.PresenceTracker
//   - DistributedPresence  → Redis / NATS pub-sub (triggered by DL-3 threshold monitor)
package datalayer

import "context"

// PresenceState — RT-3 ⭐ canonical 4-state enum (blueprint §1.4 live-presence feel).
// This matches the centralized enum pattern used by reasons.IsValid #496,
// AP-4-enum #591, and NAMING-1 #614.
//
// Negative constraints (rt-3-spec.md §0.2 + content-lock §3):
//   - Closed 4-state enum; do not add a 5th text-entry state or other
//     cross-enum synonyms.
//   - Search for `PresenceStateOnline|PresenceStateAway|PresenceStateOffline|
//     PresenceStateThinking`; count==4 confirms no extra enum constant.
//   - thinking state requires subject through bpp.ValidateTaskStarted; empty
//     subject is rejected.
type PresenceState string

const (
	// PresenceStateOnline — user/agent has at least 1 live session (same source as IsOnline).
	PresenceStateOnline PresenceState = "online"
	// PresenceStateAway — 5min inactivity (last-seen threshold, derived by RT-3 client UI).
	PresenceStateAway PresenceState = "away"
	// PresenceStateOffline — 0 live sessions (opposite of IsOnline).
	PresenceStateOffline PresenceState = "offline"
	// PresenceStateThinking — agent is executing a task through the bpp.task_started
	// frame and must include Subject. This avoids misleading loading-state drift; empty Subject
	// is rejected (rt-3-spec.md §0.2 + blueprint §1.1 ⭐ discipline).
	PresenceStateThinking PresenceState = "thinking"
)

// PresenceStore is the canonical interface for "is user X reachable?" queries.
// v1 routes through AL-3 PresenceTracker; v3+ can swap the underlying
// implementation without touching consumers because handlers use this interface.
type PresenceStore interface {
	// IsOnline reports whether the user/agent has at least one live session.
	// Same source as the G2.5 contract (presence.PresenceTracker.IsOnline).
	IsOnline(ctx context.Context, userID string) (bool, error)

	// Sessions returns the live session ids for the user. Empty slice means
	// offline. Stable order is not required, matching the #310 lock.
	Sessions(ctx context.Context, userID string) ([]string, error)
}

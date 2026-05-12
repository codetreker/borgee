// Package presence — writer.go: AL-3.2 write-side contract for the
// PresenceTracker. The read-side interface (`PresenceTracker` in
// contract.go) was locked at G2.5 (#277) so Phase 3 callers
// could wire against a stable shape; this file adds the write-side
// without touching that locked file.
//
// Why a separate interface (`PresenceWriter`):
//   - Read callers (RT-0 / DM-2 fallback / sidebar rendering) only ever
//     consume `IsOnline / Sessions` — passing them a writer would let
//     hot-path code mistakenly mutate state. The split keeps the read
//     contract a strict subset.
//   - The /ws hub is the only legal writer. Anywhere else calling
//     `TrackOnline / TrackOffline` is an architectural drift; the
//     acceptance check in al-3.md §2 verifies this.
//
// Implementation: SessionsTracker (in tracker.go) satisfies BOTH
// interfaces via the same *gorm.DB handle, so wiring is a single
// constructor at boot.
package presence

// PresenceWriter is the authoritative write API for the presence
// session table. Only the WS hub lifecycle hook (AL-3.2) calls these.
//
// Idempotency:
//   - TrackOnline with the same sessionID is a no-op via the
//     UNIQUE(session_id) constraint laid down in migration v=12 — a
//     duplicate insert returns the underlying driver's UNIQUE error,
//     which the hub treats as "already tracked". This makes retries
//     after transient network blips safe without dedup logic.
//   - TrackOffline with an unknown sessionID is a no-op (zero rows
//     affected, nil error) so panic-driven `defer` cleanups at the
//     top of `HandleClient` don't blow up if Register hadn't run yet.
type PresenceWriter interface {
	// TrackOnline records a fresh session for the user. agentID may be
	// nil (human session) or a non-nil string (agent session — written
	// to the partial index column so DM-2 fallback's
	// `IsOnline(agent.id)` OR-query path resolves).
	//
	// The (userID, agentID) pair lets a single session represent both
	// "this human is online" and, if the connection is for an agent
	// runtime, "this agent's runtime is reachable" — DM-2.2 mention
	// fallback, RT-0 routing, and sidebar rendering all read off the same
	// row via the OR-shaped IsOnline query.
	TrackOnline(userID, sessionID string, agentID *string) error

	// TrackOffline removes the session row. multi-session last-wins:
	// closing one of N sessions for a user does NOT flip them offline;
	// only the close of the last live session removes the final row,
	// at which point IsOnline reports false (the table is empty for
	// that user_id, so the LIMIT-1 EXISTS query returns 0).
	TrackOffline(sessionID string) error
}

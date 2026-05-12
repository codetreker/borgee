// Package presence — tracker.go: AL-3.1 (#301 spec brief) read-side
// implementation of the PresenceTracker interface locked by #277
// (`contract.go`).
//
// Scope of this PR (AL-3.1):
//   - Reads: `IsOnline(userID)` + `Sessions(userID)` query the
//     `presence_sessions` table that migration v=12 lays down.
//   - Writes: `TrackOnline` / `TrackOffline` are NOT in this PR. They
//     land with AL-3.2 (`internal/ws/hub.go` lifecycle hook) where the
//     write surface needs to coordinate with the WS connection lifecycle
//     (defer-based teardown, ctx-cancel, panic). Adding the writes here
//     without the hook ties them to nothing.
//
// Interface signature contract (#277):
//   - `IsOnline(userID string) bool` — same byte-level signature as the
//     stub. The DB-backed impl below satisfies the same interface; no
//     existing caller (RT-0 / CM-4.3b / DM-2 fallback) recompiles.
//   - `Sessions(userID string) []string` — same. Returns the session_id
//     list; empty slice (not nil-only) when user is offline so callers
//     can `len(...) == 0` without nil-guard noise.
//
// Negative constraints (#301 spec §0):
//   - presence reads MUST NOT scan `events` / `messages` / any
//     cursor-bearing table. They live in `presence_sessions` only —
//     presence is transient state, unlike RT-1's replayable sequence.
//   - The agent_id-keyed lookup uses `idx_presence_sessions_agent_id`
//     (created in v=12 partial index); if the agent has no row at all
//     the query returns false (cross-org default-private, #301 §4).
package presence

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// timeNow is the package-level clock indirection. Tests override it
// to drive deterministic timestamps; production callers use time.Now.
var timeNow = time.Now

// SessionsTracker is the DB-backed PresenceTracker implementation.
// Constructed with a *gorm.DB handle (typically `store.Store.DB()`).
//
// Concurrency: gorm's *DB is goroutine-safe for queries; the read path
// here is stateless (no in-memory cache) so it composes with the AL-3.2
// hub writes without locking. The write path (AL-3.2) will likewise go
// straight at the DB; the only shared invariant is the UNIQUE(session_id)
// constraint which the schema enforces.
type SessionsTracker struct {
	db *gorm.DB
}

// NewSessionsTracker wires a tracker against the given gorm handle.
// Returns an error only if db is nil — keeping the constructor explicit
// makes plumbing test failures (forgot to pass store.DB()) loud at boot.
func NewSessionsTracker(db *gorm.DB) (*SessionsTracker, error) {
	if db == nil {
		return nil, errors.New("presence: nil *gorm.DB")
	}
	return &SessionsTracker{db: db}, nil
}

// IsOnline returns true iff at least one row in `presence_sessions`
// matches the given user_id OR agent_id. The OR shape lets callers pass
// either a `users.id` (human owner / agent owner) or an `agents.id`
// (when checking an agent specifically) — DM-2 mention fallback uses
// the agent_id path; RT-0 / sidebar rendering uses user_id.
//
// Index path: `idx_presence_sessions_user_id` (full) +
// `idx_presence_sessions_agent_id` (partial WHERE agent_id IS NOT NULL).
// Both are O(log N); the LIMIT 1 + EXISTS shape avoids a full count.
func (t *SessionsTracker) IsOnline(userID string) bool {
	if userID == "" {
		return false
	}
	var present int64
	err := t.db.Raw(`SELECT 1 FROM presence_sessions
		WHERE user_id = ? OR agent_id = ?
		LIMIT 1`, userID, userID).Scan(&present).Error
	if err != nil {
		return false
	}
	return present == 1
}

// Sessions returns the live session_id list for the user. Same OR
// matching as IsOnline so callers see the union of human + agent rows.
// The returned slice is freshly allocated; callers may retain it.
//
// Empty (not nil) when offline so callers can range without nil-guard.
// `MUST NOT mutate` is documented on the interface for parity with the
// stub — this impl returns a fresh slice anyway, but the docstring
// keeps the contract truthful for future cache-backed impls.
func (t *SessionsTracker) Sessions(userID string) []string {
	if userID == "" {
		return []string{}
	}
	var ids []string
	err := t.db.Raw(`SELECT session_id FROM presence_sessions
		WHERE user_id = ? OR agent_id = ?
		ORDER BY connected_at ASC`, userID, userID).Scan(&ids).Error
	if err != nil {
		return []string{}
	}
	if ids == nil {
		return []string{}
	}
	return ids
}

// TrackOnline (AL-3.2) inserts a presence_sessions row for the new WS
// session. agentID is nil for human sessions, non-nil for agent runtime
// connections — the partial index `idx_presence_sessions_agent_id`
// (laid down in v=12 WHERE agent_id IS NOT NULL) accelerates DM-2
// fallback's `IsOnline(agent.id)` OR-query path.
//
// Idempotency: a duplicate sessionID returns the driver's UNIQUE
// constraint violation; the hub treats that as "already tracked" and
// continues. The `nowMs` parameter is injected (not `time.Now()` here)
// so testutil/clock can drive the connected_at / last_heartbeat_at
// columns deterministically — same pattern as CM-4.0 and AL-3.1 tests.
func (t *SessionsTracker) TrackOnline(userID, sessionID string, agentID *string) error {
	return t.trackOnlineAt(userID, sessionID, agentID, nowMs())
}

// trackOnlineAt is the clock-injectable inner form. Tests reach for it
// when they need to pin connected_at to a specific epoch; production
// callers go through TrackOnline which uses the package-level clock.
func (t *SessionsTracker) trackOnlineAt(userID, sessionID string, agentID *string, nowMillis int64) error {
	if userID == "" || sessionID == "" {
		return errors.New("presence: TrackOnline requires non-empty userID + sessionID")
	}
	return t.db.Exec(`INSERT INTO presence_sessions
		(session_id, user_id, agent_id, connected_at, last_heartbeat_at)
		VALUES (?, ?, ?, ?, ?)`,
		sessionID, userID, agentID, nowMillis, nowMillis).Error
}

// TrackOffline (AL-3.2) deletes the presence_sessions row for the
// given sessionID. multi-session last-wins: closing one of N sessions
// for a user removes only that row; IsOnline still reports true while
// at least one row remains. Only the close of the last live session
// drops the final row and flips the user offline.
//
// Unknown sessionID is a soft no-op (0 rows affected, nil error) so
// panic-driven `defer TrackOffline` cleanups at the top of HandleClient
// don't blow up if Register hadn't run yet (AL-3.2 acceptance §2.1).
func (t *SessionsTracker) TrackOffline(sessionID string) error {
	if sessionID == "" {
		return errors.New("presence: TrackOffline requires non-empty sessionID")
	}
	return t.db.Exec(`DELETE FROM presence_sessions WHERE session_id = ?`, sessionID).Error
}

// nowMs returns the current time in Unix milliseconds. Indirection
// kept package-private so tests in the same package can shadow it via
// `trackOnlineAt` rather than a swappable global — keeps production
// callers honest.
func nowMs() int64 { return timeNow().UnixMilli() }

// Compile-time assertion that SessionsTracker satisfies the #277-locked
// PresenceTracker interface. Drift on either method signature trips a
// build error — this is the contract drift test the AL-3 acceptance §1.2
// row pins (`var _ PresenceTracker = (*sessionsImpl)(nil)`).
var _ PresenceTracker = (*SessionsTracker)(nil)

// Compile-time assertion that SessionsTracker satisfies the AL-3.2
// PresenceWriter contract. The hub depends on the interface (not the
// struct) so future cache-backed impls plug in without touching the
// hub lifecycle hook.
var _ PresenceWriter = (*SessionsTracker)(nil)

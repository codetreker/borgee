// Package presence — tracker_test.go: AL-3.2 write-side coverage for
// SessionsTracker. Read-side coverage is in the migration suite (the
// schema-shape tests there pin the DB contract); this file pins the
// write semantics the WS hub depends on:
//
//   - TrackOnline writes the agent_id column when supplied (the partial
//     index path DM-2.2 fallback's IsOnline(agent.id) hits, #310 lock).
//   - multi-session last-wins: closing one of N sessions keeps IsOnline
//     true; only the close of the last session flips it false (#302
//     §2.2 — the hub-level invariant the Untrack defer chain enforces).
//   - TrackOffline on an unknown sessionID is a soft no-op so panic-
//     driven `defer TrackOffline` cleanups at the top of HandleClient
//     don't blow up if Register hadn't run yet (AL-3.2 acceptance §2.1).
package presence

import (
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTestDB opens an in-memory SQLite + applies the v=12 schema body
// inline (we can't import the migrations package because that would
// pull in this package — circular). The DDL must stay byte-identical
// with `internal/migrations/al_3_1_presence_sessions.go`; if either
// drifts, the migration suite + this suite both flag it.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS presence_sessions (
		  id                INTEGER PRIMARY KEY AUTOINCREMENT,
		  session_id        TEXT    NOT NULL UNIQUE,
		  user_id           TEXT    NOT NULL,
		  agent_id          TEXT,
		  connected_at      INTEGER NOT NULL,
		  last_heartbeat_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_presence_sessions_user_id ON presence_sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_presence_sessions_agent_id ON presence_sessions(agent_id) WHERE agent_id IS NOT NULL`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	return db
}

func TestTrackOnline_WritesRow(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	tr, err := NewSessionsTracker(db)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	if err := tr.TrackOnline("user-A", "sess-1", nil); err != nil {
		t.Fatalf("TrackOnline: %v", err)
	}
	if !tr.IsOnline("user-A") {
		t.Fatal("IsOnline(user-A) should be true after TrackOnline")
	}
	if got := tr.Sessions("user-A"); len(got) != 1 || got[0] != "sess-1" {
		t.Fatalf("Sessions(user-A): got %v, want [sess-1]", got)
	}
}

// TestTrackOnline_AgentIDPartialIndex pins #310 partial index path:
// TrackOnline(agentID=x) writes a row keyed by user_id=human-owner +
// agent_id=x; subsequent IsOnline(x) hits the OR-shaped query via the
// agent_id branch (the partial index covers it) and returns true.
// This keeps AL-3.2 writes aligned with DM-2.2 fallback reads.
func TestTrackOnline_AgentIDPartialIndex(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	tr, _ := NewSessionsTracker(db)
	agentID := "agent-bot"
	if err := tr.TrackOnline("human-owner", "sess-bot", &agentID); err != nil {
		t.Fatalf("TrackOnline(agentID): %v", err)
	}
	// IsOnline reachable via either the user_id (owner) or agent_id branch.
	if !tr.IsOnline("human-owner") {
		t.Fatal("IsOnline(human-owner) via user_id branch should be true")
	}
	if !tr.IsOnline(agentID) {
		t.Fatal("IsOnline(agent-bot) via agent_id partial-index branch should be true — DM-2.2 fallback contract")
	}
}

// TestTrackOffline_MultiSessionLastWins pins #302 §2.2: closing one of
// N concurrent sessions for the same user keeps IsOnline true; only the
// close of the last session flips offline. Deviation here would let
// multi-device users (web tab + mobile + plugin) appear offline whenever
// any tab closed.
func TestTrackOffline_MultiSessionLastWins(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	tr, _ := NewSessionsTracker(db)
	for _, sid := range []string{"sess-web", "sess-mobile", "sess-plugin"} {
		if err := tr.TrackOnline("user-A", sid, nil); err != nil {
			t.Fatalf("TrackOnline %s: %v", sid, err)
		}
	}
	if !tr.IsOnline("user-A") {
		t.Fatal("user-A should be online with 3 sessions")
	}
	// Close two of three; user must still be online.
	for _, sid := range []string{"sess-web", "sess-mobile"} {
		if err := tr.TrackOffline(sid); err != nil {
			t.Fatalf("TrackOffline %s: %v", sid, err)
		}
	}
	if !tr.IsOnline("user-A") {
		t.Fatal("user-A should STILL be online with 1 session remaining (last-wins invariant)")
	}
	// Close the last session; now offline.
	if err := tr.TrackOffline("sess-plugin"); err != nil {
		t.Fatalf("TrackOffline last: %v", err)
	}
	if tr.IsOnline("user-A") {
		t.Fatal("user-A should be offline after all sessions closed")
	}
}

// TestTrackOffline_UnknownSessionIsSoftNoop pins the panic-safety
// invariant: a defer-driven TrackOffline at the top of HandleClient
// MUST NOT error if Register hadn't run (e.g. panic before the row was
// inserted). Returning nil keeps the lifecycle hook simple — no
// "did Register succeed?" branching needed.
func TestTrackOffline_UnknownSessionIsSoftNoop(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	tr, _ := NewSessionsTracker(db)
	if err := tr.TrackOffline("never-registered"); err != nil {
		t.Fatalf("TrackOffline on unknown sessionID should be soft no-op, got: %v", err)
	}
}

// TestTrackOnline_DuplicateSessionIDIsUnique pins UNIQUE(session_id):
// the hub treats a duplicate insert as "already tracked" and continues.
// This lets retries after transient blips stay idempotent.
func TestTrackOnline_DuplicateSessionIDIsUnique(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	tr, _ := NewSessionsTracker(db)
	if err := tr.TrackOnline("user-A", "sess-1", nil); err != nil {
		t.Fatalf("first TrackOnline: %v", err)
	}
	err := tr.TrackOnline("user-B", "sess-1", nil)
	if err == nil {
		t.Fatal("duplicate session_id should violate UNIQUE constraint")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unique") {
		t.Fatalf("expected UNIQUE error, got: %v", err)
	}
}

// TestTrackOnline_RejectsEmptyArgs pins the fail-fast guard: empty
// userID or sessionID is a programmer error (forgot to wire something),
// not a runtime condition — error rather than silent ignore so tests
// trip immediately.
func TestTrackOnline_RejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	tr, _ := NewSessionsTracker(db)
	if err := tr.TrackOnline("", "sess-1", nil); err == nil {
		t.Error("empty userID should error")
	}
	if err := tr.TrackOnline("user-A", "", nil); err == nil {
		t.Error("empty sessionID should error")
	}
	if err := tr.TrackOffline(""); err == nil {
		t.Error("empty sessionID on TrackOffline should error")
	}
}

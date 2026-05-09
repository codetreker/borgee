package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runAL31 runs migration v=12 (AL-3.1) on a memory DB. v=12 is a clean
// CREATE — no upstream tables required, so we don't seed anything.
func runAL31(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(presenceSessions)
	if err := e.Run(0); err != nil {
		t.Fatalf("run al_3_1: %v", err)
	}
}

// TestAL_CreatesPresenceSessionsTable pins acceptance §1 (al-3.md):
// presence_sessions has the contract columns with the right NOT NULL /
// nullable shape. Drift here breaks IsOnline correctness or schema
// equivalence with the AL-3.2 hub writer.
func TestAL_CreatesPresenceSessionsTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL31(t, db)

	cols := pragmaColumns(t, db, "presence_sessions")
	if len(cols) == 0 {
		t.Fatal("presence_sessions table not created")
	}

	// PK axis (#302 §1.1 三轴断言): `id` is the PK; session_id is UNIQUE
	// (not PK) so the row-level PRIMARY KEY stays an INTEGER AUTOINCREMENT
	// and session_id keeps its UNIQUE-but-mutable-shape contract.
	idCol, ok := cols["id"]
	if !ok {
		t.Fatalf("presence_sessions missing id column (have %v)", keys(cols))
	}
	if !idCol.pk {
		t.Error("presence_sessions.id must be PRIMARY KEY")
	}
	if sid, ok := cols["session_id"]; ok && sid.pk {
		t.Error("presence_sessions.session_id must NOT be PK (UNIQUE only — #302 §1.1)")
	}

	// Required NOT NULL columns.
	for _, name := range []string{"session_id", "user_id", "connected_at", "last_heartbeat_at"} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("presence_sessions missing %q (have %v)", name, keys(cols))
		}
		if !c.notNull {
			t.Errorf("presence_sessions.%s must be NOT NULL", name)
		}
	}

	// agent_id is nullable (NULL = human session, non-NULL = agent session).
	agentID, ok := cols["agent_id"]
	if !ok {
		t.Fatalf("presence_sessions missing agent_id (have %v)", keys(cols))
	}
	if agentID.notNull {
		t.Error("presence_sessions.agent_id must be nullable (NULL = human session)")
	}

	// Reverse-assert the反约束: cursor column MUST NOT exist (presence is
	// transient state, not a cursor-ordered event stream — RT-1 拆死).
	if _, has := cols["cursor"]; has {
		t.Error("presence_sessions.cursor exists — 反约束 broken (presence != RT-1 events)")
	}
	// last_seen_at MUST NOT exist (Phase 5+ stance argument; AL-3.1 spec
	// says last_heartbeat_at, not last_seen_at).
	if _, has := cols["last_seen_at"]; has {
		t.Error("presence_sessions.last_seen_at exists — spec uses last_heartbeat_at (#302 §1.1)")
	}
}

// TestAL_RejectsDuplicateSessionID pins UNIQUE(session_id). The
// constraint is acceptance-critical: a second TrackOnline call with the
// same session_id (e.g. retry after network blip) must reject so the
// hub's write path stays idempotent without dedup logic.
func TestAL_RejectsDuplicateSessionID(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL31(t, db)

	insert := func(sessionID, userID string) error {
		return db.Exec(`INSERT INTO presence_sessions
			(session_id, user_id, connected_at, last_heartbeat_at)
			VALUES (?, ?, ?, ?)`,
			sessionID, userID, 1700000000000, 1700000000000).Error
	}
	if err := insert("sess-1", "user-A"); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := insert("sess-1", "user-B"); err == nil {
		t.Fatal("duplicate session_id was accepted — UNIQUE(session_id) constraint missing")
	}
}

// TestAL_AllowsMultiSessionPerUser pins #301 spec §0 设计 ③: one user
// can have many concurrent sessions (web tab + mobile + plugin). The
// schema MUST NOT have UNIQUE(user_id); only session_id is unique.
func TestAL_AllowsMultiSessionPerUser(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL31(t, db)

	insert := func(sessionID string) error {
		return db.Exec(`INSERT INTO presence_sessions
			(session_id, user_id, connected_at, last_heartbeat_at)
			VALUES (?, 'user-A', ?, ?)`,
			sessionID, 1700000000000, 1700000000000).Error
	}
	if err := insert("sess-web"); err != nil {
		t.Fatalf("first session: %v", err)
	}
	if err := insert("sess-mobile"); err != nil {
		t.Fatalf("second session for same user (multi-end legal): %v", err)
	}
	if err := insert("sess-plugin"); err != nil {
		t.Fatalf("third session for same user: %v", err)
	}

	var count int64
	if err := db.Raw(`SELECT COUNT(*) FROM presence_sessions WHERE user_id = 'user-A'`).Scan(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Fatalf("multi-session count: got %d, want 3", count)
	}
}

// TestAL_HasUserIDIndex pins acceptance §1.1 — IsOnline O(1) lookup
// requires `idx_presence_sessions_user_id`. Verified via sqlite_master
// rather than EXPLAIN QUERY PLAN to keep the assertion deterministic.
func TestAL_HasUserIDIndex(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL31(t, db)

	for _, idx := range []string{
		"idx_presence_sessions_user_id",
		"idx_presence_sessions_agent_id",
	} {
		var name string
		err := db.Raw(`SELECT name FROM sqlite_master
			WHERE type='index' AND name=?`, idx).Scan(&name).Error
		if err != nil || name != idx {
			t.Errorf("missing index %s (got %q, err=%v)", idx, name, err)
		}
	}
}

// TestAgentPresenceSessions_Idempotent pins forward-only safety: re-running v=12 against
// a DB that already has it must be a no-op (CREATE TABLE IF NOT EXISTS
// + CREATE INDEX IF NOT EXISTS guards). This is what migrations_test
// expects of every migration body.
func TestAgentPresenceSessions_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL31(t, db)
	// Second engine, fresh registry — body must succeed.
	e := New(db)
	e.Register(presenceSessions)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run al_3_1: %v", err)
	}
}

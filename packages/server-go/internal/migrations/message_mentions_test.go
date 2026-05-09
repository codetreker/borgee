package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runDM21 runs migration v=15 (DM-2.1) on a memory DB. v=15 is a clean
// CREATE — message_mentions logical-FKs into messages / users via
// message_id / target_user_id, but SQLite FK enforcement is off, so we
// don't seed upstream tables. Tests that exercise routing logic (#311
// acceptance §1.1-§1.3) live in DM-2.2 (server parser path), not here.
func runDM21(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(messageMentions)
	if err := e.Run(0); err != nil {
		t.Fatalf("run dm_2_1: %v", err)
	}
}

// TestDM_CreatesMessageMentionsTable pins acceptance §1.0.a (dm-2.md):
// message_mentions has the contract columns with the right NOT NULL /
// PK shape. Drift here breaks mention routing or the dedup contract.
func TestDM_CreatesMessageMentionsTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runDM21(t, db)

	cols := pragmaColumns(t, db, "message_mentions")
	if len(cols) == 0 {
		t.Fatal("message_mentions table not created")
	}

	idCol, ok := cols["id"]
	if !ok {
		t.Fatalf("message_mentions missing id (have %v)", keys(cols))
	}
	if !idCol.pk {
		t.Error("message_mentions.id must be PRIMARY KEY")
	}

	for _, name := range []string{"message_id", "target_user_id", "created_at"} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("message_mentions missing %q (have %v)", name, keys(cols))
		}
		if !c.notNull {
			t.Errorf("message_mentions.%s must be NOT NULL", name)
		}
	}
}

// TestDM_NoCursorOrFanoutOwnerColumns pins acceptance §1.0.e —
// 反约束 column list. cursor / fanout_to_owner_id / cc_owner_id /
// target_kind / read_at must NOT exist. spec §0 设计 ③ (mention 永不
// 抄送 owner) + 设计 ⑥ (user / agent 同语义, 不分叉 target_kind).
func TestDM_NoCursorOrFanoutOwnerColumns(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runDM21(t, db)

	cols := pragmaColumns(t, db, "message_mentions")
	for _, forbidden := range []string{
		"cursor",            // RT-1 envelope cursor 拆死
		"fanout_to_owner_id", // 设计 ③ 永不抄送 owner
		"cc_owner_id",       // 设计 ③ 永不抄送 owner (#293 §4.a 反约束)
		"owner_id",          // 设计 ③ 任何 owner 路由列禁
		"target_kind",       // 设计 ⑥ user / agent 同语义不分叉
		"read_at",           // mention 阅读态留 Phase 5+
		"acknowledged_at",   // mention 阅读态留 Phase 5+
	} {
		if _, has := cols[forbidden]; has {
			t.Errorf("message_mentions.%s exists — 反约束 broken (spec §0 / acceptance §1.0.e)", forbidden)
		}
	}
}

// TestDM_RejectsDuplicateMentionPerMessage pins acceptance §1.0.b —
// UNIQUE(message_id, target_user_id) — 同 message 同 target 二次 INSERT
// reject (dedup, 设计 ⑥ agent=同事 同语义). 重复 `@<id>` 同 message
// 只持久化一行.
func TestDM_RejectsDuplicateMentionPerMessage(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runDM21(t, db)

	insert := func(messageID, targetUserID string) error {
		return db.Exec(`INSERT INTO message_mentions
			(message_id, target_user_id, created_at)
			VALUES (?, ?, ?)`,
			messageID, targetUserID, 1700000000000).Error
	}
	if err := insert("msg-A", "user-1"); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := insert("msg-A", "user-1"); err == nil {
		t.Fatal("duplicate (message_id, target_user_id) accepted — UNIQUE constraint missing")
	}
	// Same target, different message → legal.
	if err := insert("msg-B", "user-1"); err != nil {
		t.Fatalf("same target diff message rejected: %v", err)
	}
	// Same message, different target → legal.
	if err := insert("msg-A", "user-2"); err != nil {
		t.Fatalf("same message diff target rejected: %v", err)
	}
}

// TestDM_AllowsMultiTargetPerMessage pins #312 spec §1 DM-2.1 —
// single message 多 `@` 不同 target 合法 (§4 反约束 batch mention 留
// Phase 5+, 但 单 message 多 target 是基础语义). schema MUST NOT have
// UNIQUE(message_id) — only the (message_id, target_user_id) pair.
func TestDM_AllowsMultiTargetPerMessage(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runDM21(t, db)

	insert := func(targetUserID string) error {
		return db.Exec(`INSERT INTO message_mentions
			(message_id, target_user_id, created_at)
			VALUES ('msg-multi', ?, ?)`,
			targetUserID, 1700000000000).Error
	}
	for _, t2 := range []string{"user-1", "user-2", "agent-1"} {
		if err := insert(t2); err != nil {
			t.Fatalf("insert %s: %v", t2, err)
		}
	}

	var count int64
	if err := db.Raw(`SELECT COUNT(*) FROM message_mentions WHERE message_id = 'msg-multi'`).Scan(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Fatalf("multi-target count: got %d, want 3", count)
	}
}

// TestDM_HasTargetUserIDIndex pins acceptance §1.0.c — mention 路由
// 热路径要 idx_message_mentions_target_user_id (fanout 时按 target 查).
// Verified via sqlite_master rather than EXPLAIN QUERY PLAN to keep
// the assertion deterministic (跟 #310 TestAL31_HasUserIDIndex 同模式).
func TestDM_HasTargetUserIDIndex(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runDM21(t, db)

	const idx = "idx_message_mentions_target_user_id"
	var name string
	err := db.Raw(`SELECT name FROM sqlite_master
		WHERE type='index' AND name=?`, idx).Scan(&name).Error
	if err != nil || name != idx {
		t.Errorf("missing index %s (got %q, err=%v)", idx, name, err)
	}
}

// TestDM_MentionsPKMonotonic pins acceptance §1.0.a id PK AUTOINCREMENT
// — global strictly increasing across all messages. Same shape as
// TestCV21_CommentsTablePKMonotonic for anchor_comments; gives audit
// log a stable order assumption regardless of message_id grouping.
func TestDM_MentionsPKMonotonic(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runDM21(t, db)

	insert := func(messageID, targetUserID string) error {
		return db.Exec(`INSERT INTO message_mentions
			(message_id, target_user_id, created_at)
			VALUES (?, ?, ?)`,
			messageID, targetUserID, 1700000000000).Error
	}
	// Interleave messages to prove PK is global, not per-message.
	pairs := [][2]string{
		{"msg-A", "user-1"},
		{"msg-B", "user-1"},
		{"msg-A", "user-2"},
		{"msg-B", "user-2"},
	}
	for _, p := range pairs {
		if err := insert(p[0], p[1]); err != nil {
			t.Fatalf("%s/%s: %v", p[0], p[1], err)
		}
	}

	type row struct {
		ID        int64  `gorm:"column:id"`
		MessageID string `gorm:"column:message_id"`
	}
	var rows []row
	if err := db.Raw(`SELECT id, message_id FROM message_mentions ORDER BY id ASC`).Scan(&rows).Error; err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].ID <= rows[i-1].ID {
			t.Errorf("PK not strictly increasing at row %d: %d after %d (message_id %q)",
				i, rows[i].ID, rows[i-1].ID, rows[i].MessageID)
		}
	}
}

// TestMessageMentions_Idempotent pins acceptance §1.0.d forward-only safety:
// re-running v=15 is no-op (CREATE TABLE IF NOT EXISTS + CREATE INDEX
// IF NOT EXISTS guards). Same as every migration body in the registry.
func TestMessageMentions_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runDM21(t, db)
	e := New(db)
	e.Register(messageMentions)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run dm_2_1: %v", err)
	}
}

package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runAL14 runs migration v=25 (AL-1.4 agent_state_log) on a memory DB.
// Logical FK to users (agent rows); SQLite FK enforcement off, no upstream
// seed needed for schema-layer tests.
func runAL14(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(agentStateLog)
	if err := e.Run(0); err != nil {
		t.Fatalf("run al_1_4: %v", err)
	}
}

// TestAL_CreatesAgentStateLogTable pins schema 7 列 + AUTOINCREMENT PK +
// NOT NULL shape. Drift here breaks server reducer audit append path.
func TestAL_CreatesAgentStateLogTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL14(t, db)

	cols := pragmaColumns(t, db, "agent_state_log")
	if len(cols) == 0 {
		t.Fatal("agent_state_log table not created")
	}

	// 6 NOT NULL columns + 1 nullable (none — all NOT NULL DEFAULT ''/0).
	for _, name := range []string{"id", "agent_id", "from_state", "to_state", "reason", "task_id", "ts"} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("agent_state_log missing %q (have %v)", name, keys(cols))
		}
		if !c.notNull {
			t.Errorf("agent_state_log.%s must be NOT NULL", name)
		}
	}
	if idCol := cols["id"]; !idCol.pk {
		t.Error("agent_state_log.id must be PRIMARY KEY (AUTOINCREMENT)")
	}
}

// TestAL_InsertAndAutoIncrement pins PK AUTOINCREMENT 单调序; 多次
// INSERT 同 agent 行 id 严格递增 (反向: 重复 id 由 SQLite 自动拒).
func TestAL_InsertAndAutoIncrement(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL14(t, db)

	insert := func(agentID, from, to, reason, taskID string, ts int64) error {
		return db.Exec(`INSERT INTO agent_state_log
			(agent_id, from_state, to_state, reason, task_id, ts)
			VALUES (?, ?, ?, ?, ?, ?)`,
			agentID, from, to, reason, taskID, ts).Error
	}

	if err := insert("a1", "", "online", "", "", 1700000000000); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := insert("a1", "online", "busy", "", "task-1", 1700000001000); err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if err := insert("a1", "busy", "idle", "", "task-1", 1700000002000); err != nil {
		t.Fatalf("third insert: %v", err)
	}

	// Verify 3 rows + monotonic ids.
	var rows []struct {
		ID int64
		To string `gorm:"column:to_state"`
	}
	if err := db.Raw(`SELECT id, to_state FROM agent_state_log WHERE agent_id = ? ORDER BY id ASC`, "a1").
		Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].ID <= rows[i-1].ID {
			t.Errorf("id not monotonic: %d <= %d", rows[i].ID, rows[i-1].ID)
		}
	}
}

// TestAgentStateLog_NoDomainBleed pins acceptance §反约束 — 列名反向断言:
// updated_at (forward-only audit 不可改写) / cursor (RT-1 envelope frame
// 路径不下沉) / org_id (派生不冗余) / task_state (state 已在 to_state)
// 等不挂.
func TestAgentStateLog_NoDomainBleed(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL14(t, db)

	cols := pragmaColumns(t, db, "agent_state_log")
	for _, forbidden := range []string{
		// 设计 ① forward-only — 反向 updated_at / modified_at 引诱 UPDATE 路径
		"updated_at",
		"modified_at",
		// 设计 ② state machine 单源 — 反向 task_state / state 单字段
		// (语义在 to_state, 双字段会引诱 redundant write)
		"state",
		"task_state",
		// 跟 al_3_1 / al_4_1 / cv_*_1 / dm_2_1 / cv_4_1 / chn_3_1 / al_2a_1 /
		// adm_2_1 / adm_2_2 同模式 — RT-1 envelope cursor 不下沉 schema
		"cursor",
		// 派生 users.org_id, 跟 admin_actions 设计 ⑥ 同精神
		"org_id",
		// owner_id 不在此表 (派生 users.owner_id, 反向不重复持有)
		"owner_id",
	} {
		if _, has := cols[forbidden]; has {
			t.Errorf("agent_state_log.%s exists — 反约束 broken (蓝图 §2.3 + AL-1a/AL-1b 设计沿用)", forbidden)
		}
	}
}

// TestAL_HasIndex pins acceptance §数据契约 — idx_agent_state_log_agent_id_ts
// (owner GET /api/v1/agents/:id/state-log 热路径). 跟 admin_actions / chn_3_1 /
// cv_4_1 同模式显式命名.
func TestAL_HasIndex(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL14(t, db)

	var name string
	err := db.Raw(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`,
		"idx_agent_state_log_agent_id_ts").Scan(&name).Error
	if err != nil || name != "idx_agent_state_log_agent_id_ts" {
		t.Errorf("missing idx_agent_state_log_agent_id_ts (got %q, err=%v)", name, err)
	}
}

// TestAgentStateLog_AcceptsAL1aReasonValues pins 设计 ④ — error 转移 reason 复用
// AL-1a 6 reason byte-identical (改 = 改 7 处单测锁链).
func TestAgentStateLog_AcceptsAL1aReasonValues(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL14(t, db)

	insert := func(reason string) error {
		return db.Exec(`INSERT INTO agent_state_log
			(agent_id, from_state, to_state, reason, task_id, ts)
			VALUES ('a1', 'online', 'error', ?, '', 1700000000000)`, reason).Error
	}
	for _, r := range []string{
		"api_key_invalid", "quota_exceeded", "network_unreachable",
		"runtime_crashed", "runtime_timeout", "unknown",
	} {
		if err := insert(r); err != nil {
			t.Errorf("reason=%q rejected: %v", r, err)
		}
	}
	// schema 不挂 reason CHECK (server-side ValidateTransition 走) — 反向断言
	// 跟 cv_4_1 #405 TestCV41_AcceptsAL1aReasonValues 同模式 (其他字典外
	// reason 在 server 层拒, 不是 schema).
}

// TestAgentStateLog_Idempotent pins forward-only safety: re-running v=25 is no-op
// (CREATE TABLE IF NOT EXISTS + CREATE INDEX IF NOT EXISTS guards).
func TestAgentStateLog_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL14(t, db)
	e := New(db)
	e.Register(agentStateLog)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run al_1_4: %v", err)
	}
}

package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runAL1B1 runs migration v=21 (AL-1b.1) on a memory DB. v=21 is a clean
// CREATE — agent_status logical-FKs into agents via agent_id, but SQLite
// FK enforcement is off, so we don't seed upstream tables. Tests that
// exercise real BPP frame → state machine behaviour live in AL-1b.2
// (server path), not here (acceptance §2.* / §3.*).
func runAL1B1(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(al1b1AgentStatus)
	if err := e.Run(0); err != nil {
		t.Fatalf("run al_1b_1: %v", err)
	}
}

// TestAL_CreatesAgentStatusTable pins acceptance §1.1 (al-1b.md):
// agent_status has the contract columns with the right NOT NULL /
// nullable shape. Drift here breaks AL-1b.2 server state machine路径
// (BPP task_started/task_finished frame → state transition). 跟 AL-3.1
// #310 TestAL31_CreatesPresenceSessionsTable + AL-4.1 #398
// TestAL41_CreatesAgentRuntimesTable 同模式.
func TestAL_CreatesAgentStatusTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL1B1(t, db)

	cols := pragmaColumns(t, db, "agent_status")
	if len(cols) == 0 {
		t.Fatal("agent_status table not created")
	}

	idCol, ok := cols["agent_id"]
	if !ok {
		t.Fatalf("agent_status missing agent_id (have %v)", keys(cols))
	}
	if !idCol.pk {
		t.Error("agent_status.agent_id must be PRIMARY KEY (1 row per agent, 设计 ① 拆三路径)")
	}

	for _, name := range []string{
		"state",
		"created_at",
		"updated_at",
	} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("agent_status missing %q (have %v)", name, keys(cols))
		}
		if !c.notNull {
			t.Errorf("agent_status.%s must be NOT NULL", name)
		}
	}

	// last_task_id / last_task_started_at / last_task_finished_at — nullable
	// (idle 态时 5min 无 frame 判时无 task; busy 态时 finished_at 空; idle
	// 态时 started_at 仍保留上次的 — 表 last_task_* 是 last-known, 跟 BPP
	// frame 触发更新).
	for _, name := range []string{
		"last_task_id",
		"last_task_started_at",
		"last_task_finished_at",
	} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("agent_status missing %q (have %v)", name, keys(cols))
		}
		if c.notNull {
			t.Errorf("agent_status.%s must be nullable (idle 态时可空, BPP frame 触发填)", name)
		}
	}
}

// TestAgentStatus_NoDomainBleed pins acceptance §1.5 — 设计 ① "拆三路径".
// 反向断言列名: AL-3 presence 列 (is_online / presence) 全无 + AL-4
// runtime 列 (last_error_reason / endpoint_url / process_kind) 全无 +
// 设计 ② "BPP 单源" 反人工伪造列 (source / set_by) 全无 + RT-1 envelope
// cursor 拆死. 跟 al_4_1 TestAL41_NoLLMOrPresenceColumns + cv_3_1
// TestCV31_NoCascadeDelete 同模式 反约束防御.
func TestAgentStatus_NoDomainBleed(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL1B1(t, db)

	cols := pragmaColumns(t, db, "agent_status")
	for _, forbidden := range []string{
		// 设计 ① 反 AL-3 — busy/idle ≠ presence, 不替代两表两路径.
		"is_online",
		"presence",
		// 设计 ① 反 AL-4 — busy/idle 是 task-level, 不混 process-level.
		"last_error_reason",
		"endpoint_url",
		"process_kind",
		// 设计 ② 反人工伪造 — busy/idle state machine 单 source = BPP frame.
		"source",
		"set_by",
		// RT-1 envelope cursor 拆死 (跟 al_3_1 / al_4_1 / cv_*_1 / dm_2_1 同模式).
		"cursor",
	} {
		if _, has := cols[forbidden]; has {
			t.Errorf("agent_status.%s exists — 反约束 broken (acceptance §1.5 + spec §0 设计 ①②)", forbidden)
		}
	}
}

// TestAL_AcceptsBusyIdleEnum pins acceptance §1.2 — state CHECK
// ('busy','idle') 2 态. 设计 ③ 文案三态: schema 仅 2 态, client UI 合并
// AL-1a 三态 (online/offline/error) + AL-3 presence 显示 5-state. 反约束:
// 'online' / 'offline' / 'error' / 'running' / 'active' / '' 等枚举外值
// reject (跟 AL-1a 三态拆死, 跟 AL-4 process-level 4 态拆死).
func TestAL_AcceptsBusyIdleEnum(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL1B1(t, db)

	insert := func(agentID, state string) error {
		return db.Exec(`INSERT INTO agent_status
			(agent_id, state, created_at, updated_at)
			VALUES (?, ?, 1700000000000, 1700000000000)`,
			agentID, state).Error
	}
	// 白名单 2 态合法.
	for _, ok := range []string{"busy", "idle"} {
		if err := insert("agent-"+ok, ok); err != nil {
			t.Errorf("state=%q rejected — CHECK should accept: %v", ok, err)
		}
	}
	// 枚举外值 reject — AL-1a 三态 + AL-4 4 态 + 同义词漂.
	for _, bad := range []string{
		"online", "offline", "error", // AL-1a 三态拆死
		"running", "stopped", "registered", // AL-4 process-level 拆死
		"active", "working", "idling", "", // 同义词漂 + 空
	} {
		if err := insert("agent-bad-"+bad, bad); err == nil {
			t.Errorf("state=%q accepted — CHECK ('busy','idle') missing or wrong", bad)
		}
	}
}

// TestAL_HasStateIndex pins acceptance §1.5 — INDEX
// idx_agent_status_state (busy 列表 lookup 热路径). 跟 AL-3.1
// idx_presence_sessions_user_id / AL-4.1 idx_agent_runtimes_agent_id
// 同模式 — 显式命名让 EXPLAIN QUERY PLAN 可读 + 反查 grep 可断.
func TestAL_HasStateIndex(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL1B1(t, db)

	const idx = "idx_agent_status_state"
	var name string
	err := db.Raw(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&name).Error
	if err != nil || name != idx {
		t.Errorf("missing index %s (got %q, err=%v)", idx, name, err)
	}
}

// TestAL_NoCascadeDelete pins acceptance §1.5 — 蓝图 §2.3 字面 "保留
// 状态历史". agent 删后 agent_status row 留账 (admin 审计路径). 反向断言:
// CREATE TABLE 字面不含 ON DELETE CASCADE / ON DELETE SET NULL — 跟
// al_3_1 / al_4_1 / cv_2_1 / dm_2_1 同模式逻辑 FK (SQLite FK 默认禁用,
// 此处 schema 字面双闸).
func TestAL_NoCascadeDelete(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL1B1(t, db)

	var sql string
	err := db.Raw(`SELECT sql FROM sqlite_master WHERE type='table' AND name='agent_status'`).Scan(&sql).Error
	if err != nil {
		t.Fatalf("query schema: %v", err)
	}
	for _, forbidden := range []string{
		"ON DELETE CASCADE",
		"ON DELETE SET NULL",
		"REFERENCES agents", // 反向硬 FK — 蓝图 §2.3 留账保留语义
	} {
		if containsCI(sql, forbidden) {
			t.Errorf("agent_status schema contains %q — 反约束 broken (蓝图 §2.3 保留状态历史 + 跟 al_3_1/al_4_1 同逻辑 FK 模式)", forbidden)
		}
	}
}

// TestAgentStatus_Idempotent pins acceptance §1.4 forward-only safety:
// re-running v=21 is no-op (CREATE TABLE IF NOT EXISTS + CREATE INDEX
// IF NOT EXISTS guards). Same as every migration body in the registry.
func TestAgentStatus_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL1B1(t, db)
	e := New(db)
	e.Register(al1b1AgentStatus)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run al_1b_1: %v", err)
	}
}

// containsCI does case-insensitive substring match for SQL schema text
// (sqlite_master.sql may normalize casing across SQLite versions; reuse
// pattern from cv_3_1 / cv_4_1 schema text scans).
func containsCI(haystack, needle string) bool {
	hLen := len(haystack)
	nLen := len(needle)
	if nLen == 0 || nLen > hLen {
		return false
	}
	for i := 0; i+nLen <= hLen; i++ {
		match := true
		for j := 0; j < nLen; j++ {
			a := haystack[i+j]
			b := needle[j]
			if a >= 'a' && a <= 'z' {
				a -= 32
			}
			if b >= 'a' && b <= 'z' {
				b -= 32
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

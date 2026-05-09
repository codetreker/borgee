package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runAL41 runs migration v=16 (AL-4.1) on a memory DB. v=16 is a clean
// CREATE — agent_runtimes logical-FKs into agents via agent_id, but
// SQLite FK enforcement is off, so we don't seed upstream tables. Tests
// that exercise real start/stop / heartbeat behaviour live in AL-4.2
// (server path), not here (acceptance §2.* / §3.*).
func runAL41(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(agentRuntimes)
	if err := e.Run(0); err != nil {
		t.Fatalf("run al_4_1: %v", err)
	}
}

// TestAL_CreatesAgentRuntimesTable pins acceptance §1.1 (al-4.md):
// agent_runtimes has the contract columns with the right NOT NULL /
// nullable shape. Drift here breaks AL-4.2 server registry路径 or
// AL-1a #249 三态机 reason 复用. 跟 AL-3.1 #310
// TestAL31_CreatesPresenceSessionsTable 同模式.
func TestAL_CreatesAgentRuntimesTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL41(t, db)

	cols := pragmaColumns(t, db, "agent_runtimes")
	if len(cols) == 0 {
		t.Fatal("agent_runtimes table not created")
	}

	idCol, ok := cols["id"]
	if !ok {
		t.Fatalf("agent_runtimes missing id (have %v)", keys(cols))
	}
	if !idCol.pk {
		t.Error("agent_runtimes.id must be PRIMARY KEY")
	}

	for _, name := range []string{
		"agent_id",
		"endpoint_url",
		"process_kind",
		"status",
		"created_at",
		"updated_at",
	} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("agent_runtimes missing %q (have %v)", name, keys(cols))
		}
		if !c.notNull {
			t.Errorf("agent_runtimes.%s must be NOT NULL", name)
		}
	}

	// last_error_reason / last_heartbeat_at — nullable (registered 态时
	// 还没有心跳, error 态时才有 reason).
	for _, name := range []string{"last_error_reason", "last_heartbeat_at"} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("agent_runtimes missing %q (have %v)", name, keys(cols))
		}
		if c.notNull {
			t.Errorf("agent_runtimes.%s must be nullable (registered 态时空, error 态时填)", name)
		}
	}
}

// TestAL_NoLLMOrPresenceColumns pins acceptance §1.5 — 设计 ① "Borgee
// 不带 runtime" + 设计 ③ runtime status ≠ presence. 反向断言列名:
// llm_provider / model_name / api_key / prompt_template 全无 (蓝图
// 设计 #7 字面 — 那是 plugin 内部事); is_online 全无 (跟 AL-3
// presence_sessions 拆死, 设计 ③ 字面).
func TestAL_NoLLMOrPresenceColumns(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL41(t, db)

	cols := pragmaColumns(t, db, "agent_runtimes")
	for _, forbidden := range []string{
		// 设计 ① 反约束 — Borgee 不带 runtime, 那是 plugin 内部事.
		"llm_provider",
		"model_name",
		"api_key",
		"prompt_template",
		// 设计 ③ 反约束 — runtime status ≠ presence, 不替代 AL-3.
		"is_online",
		"presence",
		// 蓝图 §4 留第 6 轮 — 不前置.
		"pid",
		"gpu_id",
		"priority",
		// RT-1 envelope cursor 拆死 (跟 al_3_1 / cv_1_1 / dm_2_1 同模式).
		"cursor",
	} {
		if _, has := cols[forbidden]; has {
			t.Errorf("agent_runtimes.%s exists — 反约束 broken (acceptance §1.5 + spec §0 设计 ①③)", forbidden)
		}
	}
}

// TestAL_RejectsInvalidProcessKind pins acceptance §1.2 — process_kind
// CHECK ('openclaw','hermes'). v1 仅 'openclaw' 蓝图 §2.2 v1 边界字面,
// 'hermes' 占号 v2+ (CHECK 已含 — schema 早就支持新值不需 v2 改 CHECK).
// 反约束: 'unknown' / '' / 'remote' 等枚举外值 reject.
func TestAL_RejectsInvalidProcessKind(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL41(t, db)

	insert := func(id, kind string) error {
		return db.Exec(`INSERT INTO agent_runtimes
			(id, agent_id, endpoint_url, process_kind, status, created_at, updated_at)
			VALUES (?, ?, 'ws://localhost:9000', ?, 'registered', 1700000000000, 1700000000000)`,
			id, "agent-"+id, kind).Error
	}
	// 白名单两值合法.
	for _, ok := range []string{"openclaw", "hermes"} {
		if err := insert("rt-"+ok, ok); err != nil {
			t.Errorf("process_kind=%q rejected — CHECK should accept: %v", ok, err)
		}
	}
	// 枚举外值 reject.
	for _, bad := range []string{"unknown", "", "remote", "local", "OpenClaw"} {
		if err := insert("rt-bad-"+bad, bad); err == nil {
			t.Errorf("process_kind=%q accepted — CHECK ('openclaw','hermes') missing or wrong", bad)
		}
	}
}

// TestAL_RejectsInvalidStatus pins acceptance §1.2 — status CHECK
// ('registered','running','stopped','error') 4 态. 反约束: 'busy' /
// 'idle' / 'starting' 等中间态 reject (设计 ③ 反约束 + 文案锁 §2 字面
// "v0 不允许 starting/stopping/restarting 中间态").
func TestAL_RejectsInvalidStatus(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL41(t, db)

	insert := func(id, status string) error {
		return db.Exec(`INSERT INTO agent_runtimes
			(id, agent_id, endpoint_url, process_kind, status, created_at, updated_at)
			VALUES (?, ?, 'ws://localhost:9000', 'openclaw', ?, 1700000000000, 1700000000000)`,
			id, "agent-"+id, status).Error
	}
	// 4 态全合法.
	for _, ok := range []string{"registered", "running", "stopped", "error"} {
		if err := insert("rt-"+ok, ok); err != nil {
			t.Errorf("status=%q rejected — CHECK 4 态 should accept: %v", ok, err)
		}
	}
	// 中间态 + 同义词 reject (跟 #321 文案锁 §3 同义词漂防御同源).
	for _, bad := range []string{"busy", "idle", "starting", "stopping", "restarting", ""} {
		if err := insert("rt-bad-"+bad, bad); err == nil {
			t.Errorf("status=%q accepted — CHECK 4 态 ('registered','running','stopped','error') missing or wrong", bad)
		}
	}
}

// TestAL_RejectsDuplicateRuntimePerAgent pins acceptance §1.3 —
// UNIQUE(agent_id). 设计 ① v1 不优化多 runtime 并行 (蓝图 §2.2 字面);
// 同 agent 二次 INSERT runtime reject (跟 AL-3.1 #310
// TestAL31_RejectsDuplicateSessionID 同模式 但语义反 — agent 单 runtime
// vs user 多 session).
func TestAL_RejectsDuplicateRuntimePerAgent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL41(t, db)

	insert := func(rtID, agentID string) error {
		return db.Exec(`INSERT INTO agent_runtimes
			(id, agent_id, endpoint_url, process_kind, status, created_at, updated_at)
			VALUES (?, ?, 'ws://localhost:9000', 'openclaw', 'registered', 1700000000000, 1700000000000)`,
			rtID, agentID).Error
	}
	if err := insert("rt-1", "agent-A"); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := insert("rt-2", "agent-A"); err == nil {
		t.Fatal("duplicate runtime per agent accepted — UNIQUE(agent_id) constraint missing")
	}
	// 不同 agent 合法.
	if err := insert("rt-3", "agent-B"); err != nil {
		t.Fatalf("different agent rejected: %v", err)
	}
}

// TestAgentRuntimes_HasAgentIDIndex pins acceptance §1.3 — INDEX
// idx_agent_runtimes_agent_id (lookup 热路径). UNIQUE 已经建了
// sqlite_autoindex, 此显式 idx 是 acceptance 字面要求 (跟 AL-3.1 / DM-2.1
// 同模式 — 显式命名让 EXPLAIN QUERY PLAN 可读 + 反查 grep 可断).
func TestAgentRuntimes_HasAgentIDIndex(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL41(t, db)

	const idx = "idx_agent_runtimes_agent_id"
	var name string
	err := db.Raw(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&name).Error
	if err != nil || name != idx {
		t.Errorf("missing index %s (got %q, err=%v)", idx, name, err)
	}
}

// TestAgentRuntimes_AcceptsAL1aReasonValues pins acceptance §1.1 + §2.5
// last_error_reason 复用 AL-1a #249 6 reason 枚举 (字面 byte-identical
// 跟 agent/state.go Reason* + AL-3 #305 ③ + lib/agent-state.ts
// REASON_LABELS 三处一致). schema 层无 CHECK enum (留 server 校验, 跟
// 11 项 language 白名单同思路 — schema CHECK 装不下产品级 enum), 此
// test 仅断言 INSERT 6 reason 全 OK.
func TestAgentRuntimes_AcceptsAL1aReasonValues(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL41(t, db)

	insert := func(rtID, agentID, reason string) error {
		return db.Exec(`INSERT INTO agent_runtimes
			(id, agent_id, endpoint_url, process_kind, status, last_error_reason, created_at, updated_at)
			VALUES (?, ?, 'ws://localhost:9000', 'openclaw', 'error', ?, 1700000000000, 1700000000000)`,
			rtID, agentID, reason).Error
	}
	for i, reason := range []string{
		"api_key_invalid",
		"quota_exceeded",
		"network_unreachable",
		"runtime_crashed",
		"runtime_timeout",
		"unknown",
	} {
		rtID := "rt-r-" + reason
		agentID := "agent-r-" + reason
		_ = i
		if err := insert(rtID, agentID, reason); err != nil {
			t.Errorf("AL-1a reason=%q rejected: %v", reason, err)
		}
	}
}

// TestAgentRuntimes_Idempotent pins acceptance §1.4 forward-only safety:
// re-running v=16 is no-op (CREATE TABLE IF NOT EXISTS + CREATE INDEX
// IF NOT EXISTS guards). Same as every migration body in the registry.
func TestAgentRuntimes_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL41(t, db)
	e := New(db)
	e.Register(agentRuntimes)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run al_4_1: %v", err)
	}
}

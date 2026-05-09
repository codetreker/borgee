package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runAL2A1 runs migration v=20 (AL-2a.1) on a memory DB. v=20 is a clean
// CREATE — agent_configs logical-FKs into users (agent rows), but SQLite FK
// enforcement is off, so we don't seed upstream tables. Tests that exercise
// real PATCH /api/v1/agents/:id/config behaviour live in AL-2a.2 (server
// path), not here (acceptance §数据契约 only).
func runAL2A1(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(al2a1AgentConfigs)
	if err := e.Run(0); err != nil {
		t.Fatalf("run al_2a_1: %v", err)
	}
}

// TestAL_CreatesAgentConfigsTable pins acceptance §数据契约 row 1: the
// table has the contract columns (agent_id PK / schema_version int / blob
// JSON / updated_at) with the right NOT NULL shape. 脱节 here breaks
// AL-2a.2 PATCH /api/v1/agents/:id/config or 4.1.a 并发 update schema_
// version 严格递增 implementation. 跟 CHN-3.1 #410
// TestCHN31_CreatesUserChannelLayoutTable 同模式.
func TestAL_CreatesAgentConfigsTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL2A1(t, db)

	cols := pragmaColumns(t, db, "agent_configs")
	if len(cols) == 0 {
		t.Fatal("agent_configs table not created")
	}

	for _, name := range []string{
		"agent_id",
		"schema_version",
		"blob",
		"created_at",
		"updated_at",
	} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("agent_configs missing %q (have %v)", name, keys(cols))
		}
		if !c.notNull {
			t.Errorf("agent_configs.%s must be NOT NULL", name)
		}
	}

	// PK (agent_id) — 单 agent 单 row, blob 整体替换 单一来源 设计.
	if agentIDCol := cols["agent_id"]; !agentIDCol.pk {
		t.Error("agent_configs.agent_id must be PRIMARY KEY")
	}
}

// TestAgentConfigs_NoDomainBleed pins acceptance §数据契约 row 2 反向约束 — 列名
// 反向断言: runtime-only 字段不在 schema 层 (blob TEXT JSON, runtime 校验
// 走 AL-2a.2 server REST API 层 + 4.1.c reflect scan); cursor 不挂
// (AL-2a 不含 BPP frame, 蓝图 §1.5); org_id 不重复持有 (走 users.org_id
// 单一来源).
func TestAgentConfigs_NoDomainBleed(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL2A1(t, db)

	cols := pragmaColumns(t, db, "agent_configs")
	for _, forbidden := range []string{
		// 蓝图 §1.4 单一来源 设计: runtime-only 字段不在 schema 层 (blob TEXT
		// JSON, 4.1.c reflect scan fail-closed). 反向断言 schema 不拆列.
		"api_key",
		"temperature",
		"token_limit",
		"retry_policy",
		// AL-2a 不含 BPP frame (蓝图 §1.5, 走轮询 reload 不挂 push frame),
		// schema 不挂 cursor (跟 al_3_1 / al_4_1 / cv_1_1 / cv_2_1 /
		// dm_2_1 / cv_4_1 / chn_3_1 同模式).
		"cursor",
		// org 隔离走 server-side ACL (users.org_id 单一来源 CM-1 #176), schema
		// 不重复持有 org_id 避免双源.
		"org_id",
		// 单一来源 blob 整体替换设计: 不拆 multi-row by config_key (PK 单
		// agent_id, 而非 composite (agent_id, config_key)).
		"config_key",
		"config_value",
	} {
		if _, has := cols[forbidden]; has {
			t.Errorf("agent_configs.%s exists — 反向约束 broken (acceptance §数据契约 row 2 + 蓝图 §1.4 单一来源 + §1.5 BPP frame 反向约束)", forbidden)
		}
	}
}

// TestAL_PKEnforcesSingleRowPerAgent pins acceptance §数据契约 row 1 +
// 单一来源 设计 — duplicate agent_id INSERT must reject (single row per agent,
// blob 整体替换 PATCH 语义).
func TestAL_PKEnforcesSingleRowPerAgent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL2A1(t, db)

	insert := func(agentID string, schemaVersion int, blob string) error {
		return db.Exec(`INSERT INTO agent_configs
			(agent_id, schema_version, blob, created_at, updated_at)
			VALUES (?, ?, ?, 1700000000000, 1700000000000)`,
			agentID, schemaVersion, blob).Error
	}

	if err := insert("agent-1", 1, `{"name":"Alpha"}`); err != nil {
		t.Fatalf("first insert should succeed: %v", err)
	}
	// Same agent_id → reject by PK.
	if err := insert("agent-1", 2, `{"name":"Beta"}`); err == nil {
		t.Fatal("duplicate agent_id should reject — PK violation (单一来源 blob 整体替换走 UPDATE 不走 INSERT)")
	}
	// Different agent_id → OK.
	if err := insert("agent-2", 1, `{"name":"Gamma"}`); err != nil {
		t.Errorf("different agent_id should succeed: %v", err)
	}
}

// TestAL_AcceptsMonotonicSchemaVersion pins acceptance §行为不变量
// 4.1.a — schema_version 单调递增. INSERT 不同 version 值; schema 不挂
// CHECK constraint (留 server 校验 server-stamp 递增, 跟 CHN-3.1 position
// REAL 同模式 — schema 受值, server 算).
func TestAL_AcceptsMonotonicSchemaVersion(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL2A1(t, db)

	// 不同 agent 行 schema_version 独立 (跨 agent 不全局递增, 仅本 agent
	// 行 server-side PATCH 语义递增).
	for i, v := range []int{1, 2, 5, 100, 999999} {
		agentID := []string{"a-1", "a-2", "a-3", "a-4", "a-5"}[i]
		if err := db.Exec(`INSERT INTO agent_configs
			(agent_id, schema_version, blob, created_at, updated_at)
			VALUES (?, ?, '{}', 1700000000000, 1700000000000)`,
			agentID, v).Error; err != nil {
			t.Errorf("schema_version=%d rejected: %v", v, err)
		}
	}
}

// TestAgentConfigs_HasAgentIDIndex pins acceptance §数据契约 row 1 — 显式命名
// idx_agent_configs_agent_id (PATCH/GET /api/v1/agents/:id/config 热路径,
// 单一来源 lookup). 跟 AL-4.1 #398 TestAL41_HasAgentIDIndex / CHN-3.1 #410
// TestCHN31_HasUserIDIndex 同模式.
func TestAgentConfigs_HasAgentIDIndex(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL2A1(t, db)

	var name string
	err := db.Raw(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`,
		"idx_agent_configs_agent_id").Scan(&name).Error
	if err != nil || name != "idx_agent_configs_agent_id" {
		t.Errorf("missing index idx_agent_configs_agent_id (got %q, err=%v)", name, err)
	}
}

// TestAgentConfigs_Idempotent pins forward-only safety: re-running v=20 is no-op
// (CREATE TABLE IF NOT EXISTS + CREATE INDEX IF NOT EXISTS guards). Same
// as every migration body in the registry (跟 chn_3_1 / cv_4_1 / al_4_1
// 同模式).
func TestAgentConfigs_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL2A1(t, db)
	e := New(db)
	e.Register(al2a1AgentConfigs)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run al_2a_1: %v", err)
	}
}

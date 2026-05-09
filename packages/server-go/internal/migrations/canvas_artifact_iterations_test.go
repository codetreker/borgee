package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runCV41 runs migration v=18 (CV-4.1) on a memory DB. v=18 is a clean
// CREATE — artifact_iterations logical-FKs into artifacts / users /
// agents / artifact_versions, but SQLite FK enforcement is off, so we
// don't seed upstream tables. Tests that exercise real iterate state-
// machine behaviour live in CV-4.2 (server path), not here
// (acceptance §1.* only).
func runCV41(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(artifactIterations)
	if err := e.Run(0); err != nil {
		t.Fatalf("run cv_4_1: %v", err)
	}
}

// TestCV_CreatesArtifactIterationsTable pins acceptance §1.1 (cv-4.md):
// artifact_iterations has the contract columns with the right NOT NULL /
// nullable shape. Drift here breaks CV-4.2 server iterate path or
// AL-1a #249 6 reason 复用. 跟 AL-4.1 #398
// TestAL41_CreatesAgentRuntimesTable 同模式.
func TestCV_CreatesArtifactIterationsTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV41(t, db)

	cols := pragmaColumns(t, db, "artifact_iterations")
	if len(cols) == 0 {
		t.Fatal("artifact_iterations table not created")
	}

	idCol, ok := cols["id"]
	if !ok {
		t.Fatalf("artifact_iterations missing id (have %v)", keys(cols))
	}
	if !idCol.pk {
		t.Error("artifact_iterations.id must be PRIMARY KEY")
	}

	for _, name := range []string{
		"artifact_id",
		"requested_by",
		"intent_text",
		"target_agent_id",
		"state",
		"created_at",
	} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("artifact_iterations missing %q (have %v)", name, keys(cols))
		}
		if !c.notNull {
			t.Errorf("artifact_iterations.%s must be NOT NULL", name)
		}
	}

	// Nullable: created_artifact_version_id (pending/running/failed 态时空,
	// completed 态时填); error_reason (success 态时空, failed 态时填);
	// completed_at (pending/running 态时空, completed/failed 态时填).
	for _, name := range []string{
		"created_artifact_version_id",
		"error_reason",
		"completed_at",
	} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("artifact_iterations missing %q (have %v)", name, keys(cols))
		}
		if c.notNull {
			t.Errorf("artifact_iterations.%s must be nullable (state-dependent填值)", name)
		}
	}
}

// TestCV_NoDomainBleed pins acceptance §1.5 — 设计 ① 域隔离 (跟 CHN-4
// #374/#378 设计 ② 同源). 反向断言列名: cursor / diff_blob / diff_lines /
// retry_count 全无 (反约束 #380 ⑦ failed 不复用 + 设计 ③ server 不算
// diff + RT-1 envelope cursor 拆死).
func TestCV_NoDomainBleed(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV41(t, db)

	cols := pragmaColumns(t, db, "artifact_iterations")
	for _, forbidden := range []string{
		// 设计 ③ server 不算 diff (jsdiff 仅 client, acceptance §2.6 + §4.4).
		"diff_blob",
		"diff_lines",
		"diff",
		// RT-1 envelope cursor 拆死 (跟 al_3_1 / al_4_1 / cv_1_1 / cv_2_1 /
		// dm_2_1 同模式).
		"cursor",
		// #380 ⑦ failed 不复用 — owner 重新触发 = 新 iteration_id.
		"retry_count",
		"retry_at",
		// 设计 ① 域隔离 — iteration 不抄送 message 路径.
		"message_id",
	} {
		if _, has := cols[forbidden]; has {
			t.Errorf("artifact_iterations.%s exists — 反约束 broken (acceptance §1.5 + spec §0 设计 ①③)", forbidden)
		}
	}
}

// TestCV_AcceptsAll4States pins acceptance §1.2 — state CHECK 4 态
// ('pending','running','completed','failed') byte-identical 跟 #380
// 文案锁 ③ 同源.
func TestCV_AcceptsAll4States(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV41(t, db)

	insert := func(id, state string) error {
		return db.Exec(`INSERT INTO artifact_iterations
			(id, artifact_id, requested_by, intent_text, target_agent_id, state, created_at)
			VALUES (?, ?, ?, 'do something', ?, ?, 1700000000000)`,
			id, "art-"+id, "user-"+id, "agent-"+id, state).Error
	}
	for _, ok := range []string{"pending", "running", "completed", "failed"} {
		if err := insert("it-"+ok, ok); err != nil {
			t.Errorf("state=%q rejected — CHECK 4 态 should accept: %v", ok, err)
		}
	}
}

// TestCV_RejectsUnknownState pins acceptance §1.2 — state CHECK 严格
// reject 中间态 / 同义词 / 大小写 (跟 #380 文案锁 ③ 同源 + AL-4.1 #398
// TestAL41_RejectsInvalidStatus 同模式 同义词漂防御).
func TestCV_RejectsUnknownState(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV41(t, db)

	insert := func(id, state string) error {
		return db.Exec(`INSERT INTO artifact_iterations
			(id, artifact_id, requested_by, intent_text, target_agent_id, state, created_at)
			VALUES (?, ?, ?, 'do something', ?, ?, 1700000000000)`,
			id, "art-"+id, "user-"+id, "agent-"+id, state).Error
	}
	for _, bad := range []string{
		// 文案锁 ③ 反约束字面禁中间态.
		"starting", "stopping", "restarting",
		// 同义词漂 (Pending/Running 英文形 / 处理中等汉化漂).
		"Pending", "Running", "busy", "idle",
		// 字典外值.
		"unknown", "queued", "cancelled", "",
	} {
		if err := insert("it-bad-"+bad, bad); err == nil {
			t.Errorf("state=%q accepted — CHECK 4 态 ('pending','running','completed','failed') missing or wrong", bad)
		}
	}
}

// TestCanvasArtifactIterations_HasIndexes pins acceptance §1.3 — 双索引字面:
// idx_iterations_artifact_id_state (per-artifact pending/running 热路径) +
// idx_iterations_target_agent (agent 工作队列查). 跟 AL-4.1 #398
// TestAL41_HasAgentIDIndex 同模式 — 显式命名让 EXPLAIN QUERY PLAN 可读 +
// 反查 grep 可断.
func TestCanvasArtifactIterations_HasIndexes(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV41(t, db)

	for _, idx := range []string{
		"idx_iterations_artifact_id_state",
		"idx_iterations_target_agent",
	} {
		var name string
		err := db.Raw(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&name).Error
		if err != nil || name != idx {
			t.Errorf("missing index %s (got %q, err=%v)", idx, name, err)
		}
	}
}

// TestCV_AcceptsAL1aReasonValues pins acceptance §1.1 + §2.5
// error_reason 复用 AL-1a #249 6 reason 枚举 + AL-4 stub fail-closed
// 'runtime_not_registered' 字面 byte-identical (跟 AL-4.1 #398
// TestAL41_AcceptsAL1aReasonValues 同模式). schema 层无 CHECK enum (留
// server 校验, 跟 AL-4.1 / 11 项 language 白名单同思路 — schema CHECK
// 装不下产品级 enum), 此 test 仅断言 INSERT 全 OK.
func TestCV_AcceptsAL1aReasonValues(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV41(t, db)

	insert := func(id, reason string) error {
		return db.Exec(`INSERT INTO artifact_iterations
			(id, artifact_id, requested_by, intent_text, target_agent_id, state, error_reason, created_at, completed_at)
			VALUES (?, ?, ?, 'do something', ?, 'failed', ?, 1700000000000, 1700000000001)`,
			id, "art-"+id, "user-"+id, "agent-"+id, reason).Error
	}
	for _, reason := range []string{
		// AL-1a #249 6 reason byte-identical 同源.
		"api_key_invalid",
		"quota_exceeded",
		"network_unreachable",
		"runtime_crashed",
		"runtime_timeout",
		"unknown",
		// AL-4 stub fail-closed (CV-4.2 server, AL-4 落地后切真路径).
		"runtime_not_registered",
	} {
		if err := insert("it-r-"+reason, reason); err != nil {
			t.Errorf("AL-1a / AL-4 stub reason=%q rejected: %v", reason, err)
		}
	}
}

// TestCanvasArtifactIterations_Idempotent pins acceptance §1.4 forward-only safety: re-running
// v=18 is no-op (CREATE TABLE IF NOT EXISTS + CREATE INDEX IF NOT EXISTS
// guards). Same as every migration body in the registry.
func TestCanvasArtifactIterations_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV41(t, db)
	e := New(db)
	e.Register(artifactIterations)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run cv_4_1: %v", err)
	}
}

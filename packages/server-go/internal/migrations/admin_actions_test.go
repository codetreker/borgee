package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runADM21 runs migration v=22 (ADM-2.1) on a memory DB. v=22 is a clean
// CREATE — admin_actions logical-FKs into admins / users, but SQLite FK
// enforcement is off, so we don't seed upstream tables. Tests that exercise
// real audit REST behaviour live in ADM-2.2 (server path), not here
// (acceptance §数据契约 + §行为不变量 4.1.a-d only at schema layer here).
func runADM21(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(adminActions)
	if err := e.Run(0); err != nil {
		t.Fatalf("run adm_2_1: %v", err)
	}
}

// TestADM_CreatesAdminActionsTable pins acceptance §数据契约 row 1: the
// table has the contract columns with the right NOT NULL shape. Drift here
// breaks ADM-2.2 GET /api/v1/me/admin-actions or 蓝图 §1.4 红线 1
// "受影响者必收 system message" implementation. 跟 CV-4.1 #399 +
// CHN-3.1 #410 同模式.
func TestADM_CreatesAdminActionsTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runADM21(t, db)

	cols := pragmaColumns(t, db, "admin_actions")
	if len(cols) == 0 {
		t.Fatal("admin_actions table not created")
	}

	for _, name := range []string{
		"id",
		"actor_id",
		"target_user_id",
		"action",
		"metadata",
		"created_at",
	} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("admin_actions missing %q (have %v)", name, keys(cols))
		}
		if !c.notNull {
			t.Errorf("admin_actions.%s must be NOT NULL", name)
		}
	}

	// PK on id (single-column, UUID).
	if idCol := cols["id"]; !idCol.pk {
		t.Error("admin_actions.id must be PRIMARY KEY")
	}
}

// TestADM_AcceptsAll5Actions pins acceptance §数据契约 row 2 — DB CHECK
// 约束 5 个 action 类型枚举字面 byte-identical. 跟 CV-4.1 #405
// TestCV41_AcceptsAll4States 同模式.
func TestADM_AcceptsAll5Actions(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runADM21(t, db)

	insert := func(id, action string) error {
		return db.Exec(`INSERT INTO admin_actions
			(id, actor_id, target_user_id, action, metadata, created_at)
			VALUES (?, 'admin-1', 'user-1', ?, '', 1700000000000)`,
			id, action).Error
	}

	for i, a := range []string{
		"delete_channel",
		"suspend_user",
		"change_role",
		"reset_password",
		"start_impersonation",
	} {
		id := "row-" + a
		if err := insert(id, a); err != nil {
			t.Errorf("row %d action=%q rejected by CHECK: %v", i, a, err)
		}
	}
}

// TestADM_RejectsUnknownAction pins acceptance §数据契约 row 2 反约束 —
// 同义词 / 大小写漂移 / 字典外值 / 空字符串 全 reject. 跟 CV-4.1 #405
// TestCV41_RejectsUnknownState 12 反约束值同模式.
func TestADM_RejectsUnknownAction(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runADM21(t, db)

	insert := func(id, action string) error {
		return db.Exec(`INSERT INTO admin_actions
			(id, actor_id, target_user_id, action, metadata, created_at)
			VALUES (?, 'admin-1', 'user-1', ?, '', 1700000000000)`,
			id, action).Error
	}

	rejected := []string{
		// 大小写漂移
		"Delete_Channel",
		"DELETE_CHANNEL",
		"SuspendUser",
		// 同义词 (蓝图字面只锁 5 个, 同义词必拒)
		"remove_channel",
		"ban_user",
		"update_role",
		"password_reset",
		"impersonate",
		"start_impersonate",
		// 字典外值 (v2+ 留账, 但 v1 schema 不开)
		"create_user",
		"export_audit",
		"force_logout",
		// 空 / null-ish
		"",
		"unknown",
	}
	for i, a := range rejected {
		id := "rej-row-" + a
		if a == "" {
			id = "rej-row-empty"
		}
		if err := insert(id, a); err == nil {
			t.Errorf("row %d action=%q should reject — CHECK 反约束 broken", i, a)
		}
	}
}

// TestAdminActions_NoDomainBleed pins admin-model.md §1.4 反约束 — 列名反向断言
// 'updated_at' (audit 不可改写) / 'org_id' (派生不冗余) / 'session_id'
// (impersonate 走单独表) 全无. 字面承袭 #366 黑名单同模式.
func TestAdminActions_NoDomainBleed(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runADM21(t, db)

	cols := pragmaColumns(t, db, "admin_actions")
	for _, forbidden := range []string{
		// audit 100% 留痕不可改写 (蓝图 §2 不变量) — 反向: updated_at
		// 引诱 UPDATE 路径, 此 schema 不开.
		"updated_at",
		"modified_at",
		// 受影响者 org 通过 users.org_id 派生, 不冗余存 (跟 CHN-3.1
		// 设计 ⑤ 不下沉派生字段 同精神).
		"org_id",
		"target_org_id",
		// impersonate 走单独 impersonation_grants 表 (蓝图 §3 数据模型片段),
		// 不混入此表 — 反向: session_id 引诱混合两类 audit.
		"session_id",
		"grant_id",
		"expires_at",
		// 反约束: 不挂 RT-1 envelope cursor (跟 al_3_1 / al_4_1 / cv_1_1 /
		// cv_2_1 / dm_2_1 / cv_4_1 / chn_3_1 同模式 — frame 路径不下沉
		// schema).
		"cursor",
	} {
		if _, has := cols[forbidden]; has {
			t.Errorf("admin_actions.%s exists — 反约束 broken (acceptance §数据契约 + admin-model.md §1.4 红线 + §2 不变量)", forbidden)
		}
	}
}

// TestADM_HasIndexes pins acceptance §数据契约 row 1 索引 + 蓝图 §1.4
// 双热路径 (受影响者 GET /me/admin-actions + admin GET /admin-api/v1/audit-log).
// 跟 CHN-3.1 #410 TestCHN31_HasUserIDIndex / CV-4.1 #405 TestCV41_HasIndexes
// 同模式 (双索引显式命名).
func TestADM_HasIndexes(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runADM21(t, db)

	for _, idx := range []string{
		"idx_admin_actions_target_user_id_created_at",
		"idx_admin_actions_actor_id_created_at",
	} {
		var name string
		err := db.Raw(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`,
			idx).Scan(&name).Error
		if err != nil || name != idx {
			t.Errorf("missing index %s (got %q, err=%v)", idx, name, err)
		}
	}
}

// TestAdminActions_PKEnforcesUniqueRowPerID pins 设计 — duplicate id INSERT must
// reject (UUID 由 server 端生成, 但 schema 层 PK 兜底防 collision).
func TestAdminActions_PKEnforcesUniqueRowPerID(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runADM21(t, db)

	insert := func(id string) error {
		return db.Exec(`INSERT INTO admin_actions
			(id, actor_id, target_user_id, action, metadata, created_at)
			VALUES (?, 'a1', 'u1', 'delete_channel', '', 1700000000000)`,
			id).Error
	}

	if err := insert("aa-1"); err != nil {
		t.Fatalf("first insert should succeed: %v", err)
	}
	if err := insert("aa-1"); err == nil {
		t.Fatal("duplicate id should reject — PK violation")
	}
	if err := insert("aa-2"); err != nil {
		t.Errorf("different id should succeed: %v", err)
	}
}

// TestAdminActions_Idempotent pins acceptance forward-only safety: re-running
// v=22 is no-op (CREATE TABLE IF NOT EXISTS + CREATE INDEX IF NOT EXISTS
// guards). Same as every migration body in the registry.
func TestAdminActions_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runADM21(t, db)
	e := New(db)
	e.Register(adminActions)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run adm_2_1: %v", err)
	}
}

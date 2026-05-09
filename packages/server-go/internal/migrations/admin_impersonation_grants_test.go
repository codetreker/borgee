package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runADM22 runs migration v=23 (ADM-2.2 impersonation_grants) on a memory DB.
// Logical FK to users; SQLite FK enforcement off, no upstream seed needed.
func runADM22(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(adminActions) // v=22 prerequisite
	e.Register(impersonationGrants)
	if err := e.Run(0); err != nil {
		t.Fatalf("run adm_2_2: %v", err)
	}
}

// TestADM_CreatesImpersonationGrantsTable pins acceptance §impersonate
// 红横幅 4.2.a — schema 5 列 (id PK / user_id NOT NULL / granted_at /
// expires_at / revoked_at NULL).
func TestADM_CreatesImpersonationGrantsTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runADM22(t, db)

	cols := pragmaColumns(t, db, "impersonation_grants")
	if len(cols) == 0 {
		t.Fatal("impersonation_grants table not created")
	}

	// 4 NOT NULL columns + 1 nullable revoked_at.
	for _, name := range []string{"id", "user_id", "granted_at", "expires_at"} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("impersonation_grants missing %q (have %v)", name, keys(cols))
		}
		if !c.notNull {
			t.Errorf("impersonation_grants.%s must be NOT NULL", name)
		}
	}
	if c, ok := cols["revoked_at"]; !ok || c.notNull {
		t.Error("impersonation_grants.revoked_at must exist and be nullable")
	}

	if idCol := cols["id"]; !idCol.pk {
		t.Error("impersonation_grants.id must be PRIMARY KEY")
	}
}

// TestAdminImpersonationGrants_NoDomainBleed pins admin-model.md §3 字面 "由 user 创建, admin
// 仅消费这条记录" — actor_id / cursor / token / etc 不在此表.
func TestAdminImpersonationGrants_NoDomainBleed(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runADM22(t, db)

	cols := pragmaColumns(t, db, "impersonation_grants")
	for _, forbidden := range []string{
		// admin-model.md §3 字面 "admin 仅消费这条记录" — admin_id 不入此表.
		"admin_id",
		"actor_id",
		"granted_by",
		// 跟 al_3_1 / al_4_1 / cv_*_1 / dm_2_1 / cv_4_1 / chn_3_1 / al_2a_1 /
		// adm_2_1 同模式 — RT-1 envelope cursor frame 路径不下沉 schema.
		"cursor",
		// session token / api_key 走 admin_sessions / users.api_key, 不混入此表
		"token",
		"session_token",
		"api_key",
		// 期限 server 固定 24h, 不允许 client 传 duration 字段
		"duration",
		"duration_hours",
	} {
		if _, has := cols[forbidden]; has {
			t.Errorf("impersonation_grants.%s exists — 反约束 broken (acceptance §4.2 + spec §2.5 + stance §1 设计 ⑦)", forbidden)
		}
	}
}

// TestADM_HasIndex pins acceptance §4.2 — idx_impersonation_grants_user_id_
// expires (ActiveGrant query 热路径).
func TestADM_HasIndex(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runADM22(t, db)

	var name string
	err := db.Raw(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`,
		"idx_impersonation_grants_user_id_expires").Scan(&name).Error
	if err != nil || name != "idx_impersonation_grants_user_id_expires" {
		t.Errorf("missing idx_impersonation_grants_user_id_expires (got %q, err=%v)", name, err)
	}
}

// TestAdminImpersonationGrants_PKEnforcesUniqueRowPerID pins UUID collision 兜底.
func TestAdminImpersonationGrants_PKEnforcesUniqueRowPerID(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runADM22(t, db)

	insert := func(id string) error {
		return db.Exec(`INSERT INTO impersonation_grants
			(id, user_id, granted_at, expires_at)
			VALUES (?, 'u1', 1700000000000, 1700086400000)`,
			id).Error
	}

	if err := insert("g-1"); err != nil {
		t.Fatalf("first insert should succeed: %v", err)
	}
	if err := insert("g-1"); err == nil {
		t.Fatal("duplicate id should reject — PK violation")
	}
}

// TestADM_AcceptsRevokedAtNullable pins revoked_at NULL semantic — 默认
// NULL 表示有效 grant; UPDATE 走 RevokeImpersonation 唯一允许的写路径
// (forward-only 设计 ⑤ 例外).
func TestADM_AcceptsRevokedAtNullable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runADM22(t, db)

	// INSERT 默认 revoked_at NULL.
	if err := db.Exec(`INSERT INTO impersonation_grants
		(id, user_id, granted_at, expires_at)
		VALUES ('g-active', 'u1', 1700000000000, 1700086400000)`).Error; err != nil {
		t.Fatalf("insert with NULL revoked_at: %v", err)
	}
	// UPDATE 业主撤销时 stamp.
	if err := db.Exec(`UPDATE impersonation_grants SET revoked_at = ? WHERE id = 'g-active'`,
		1700050000000).Error; err != nil {
		t.Errorf("UPDATE revoked_at: %v", err)
	}
}

// TestAdminImpersonationGrants_Idempotent pins forward-only safety: re-running v=23 is no-op.
func TestAdminImpersonationGrants_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runADM22(t, db)
	e := New(db)
	e.Register(adminActions)
	e.Register(impersonationGrants)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run adm_2_2: %v", err)
	}
}

package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runAP31 runs migration v=29 (AP-3.1) on a memory DB. Requires the
// user_permissions table; create it inline (matching 历史 store schema)
// so tests don't depend on store init.
func runAP31(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS user_permissions (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     TEXT NOT NULL,
  permission  TEXT NOT NULL,
  scope       TEXT NOT NULL DEFAULT '*',
  granted_by  TEXT,
  granted_at  INTEGER NOT NULL,
  UNIQUE(user_id, permission, scope)
)`).Error; err != nil {
		t.Fatalf("seed user_permissions: %v", err)
	}
	e := New(db)
	e.Register(userPermissionsOrg)
	if err := e.Run(0); err != nil {
		t.Fatalf("run ap_3_1: %v", err)
	}
}

// REG-AP3-001 (acceptance §1.1) — schema adds nullable org_id column
// (NULL = 历史 / inheritance, 跟 ap_1_1 expires_at 同模式).
func TestAP_AddsOrgIDColumn(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAP31(t, db)

	cols := pragmaColumns(t, db, "user_permissions")
	c, ok := cols["org_id"]
	if !ok {
		t.Fatalf("user_permissions missing org_id column (have %v)", keys(cols))
	}
	if c.notNull {
		t.Errorf("user_permissions.org_id must be nullable (NULL = 历史, 设计 ② + ⑥)")
	}
}

// REG-AP3-001b (acceptance §1.2) — sparse index covers org_id IS NOT NULL
// rows (跟 ap_1_1 expires_at sparse 同模式).
func TestAP_HasOrgIDIndex(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAP31(t, db)

	const idx = "idx_user_permissions_org_id"
	var sql string
	err := db.Raw(`SELECT sql FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&sql).Error
	if err != nil || sql == "" {
		t.Fatalf("missing index %s (err=%v)", idx, err)
	}
	if !containsCI(sql, "WHERE org_id IS NOT NULL") {
		t.Errorf("index %s must be partial (WHERE org_id IS NOT NULL) — sparse; got: %s", idx, sql)
	}
}

// REG-AP3-001c (acceptance §1.1 + 设计 ⑥) — 历史 rows preserve NULL
// org_id (AP-1 现网行为零变).
func TestAP_LegacyRowsNullPreserved(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAP31(t, db)

	if err := db.Exec(`INSERT INTO user_permissions
		(user_id, permission, scope, granted_at)
		VALUES ('u-legacy', 'message.read', '*', 1700000000000)`).Error; err != nil {
		t.Fatalf("insert without org_id: %v", err)
	}
	var n int64
	if err := db.Raw(`SELECT COUNT(*) FROM user_permissions WHERE user_id='u-legacy' AND org_id IS NULL`).Row().Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 NULL-org_id 历史 row, got %d", n)
	}
}

// REG-AP3-001d — schema accepts explicit org_id assignment (cross-org
// enforce 路径 grant 时显式写, 设计 ②).
func TestAP_AcceptsExplicitOrgID(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAP31(t, db)

	if err := db.Exec(`INSERT INTO user_permissions
		(user_id, permission, scope, granted_at, org_id)
		VALUES ('u-org-A', 'write_artifact', 'channel:ch-1', 1700000000000, 'org-A')`).Error; err != nil {
		t.Fatalf("insert with org_id: %v", err)
	}
	var got *string
	if err := db.Raw(`SELECT org_id FROM user_permissions WHERE user_id='u-org-A'`).Row().Scan(&got); err != nil {
		t.Fatalf("query: %v", err)
	}
	if got == nil || *got != "org-A" {
		t.Errorf("expected org_id='org-A', got %v", got)
	}
}

// REG-AP3-001e (spec §3 反向约束 + 设计 ②) — schema does NOT install a FK
// to organizations (跟 user.org_id 同精神, 业务校验 server 层做). 反向
// grep `user_permissions.*FOREIGN KEY.*organizations` count==0.
func TestAP_NoFKToOrganizations(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAP31(t, db)

	var sql string
	if err := db.Raw(`SELECT sql FROM sqlite_master WHERE type='table' AND name='user_permissions'`).Row().Scan(&sql); err != nil {
		t.Fatalf("query schema: %v", err)
	}
	if containsCI(sql, "FOREIGN KEY") && containsCI(sql, "organizations") {
		t.Errorf("user_permissions must NOT FK organizations(id) — 反向约束 设计 ②; got: %s", sql)
	}
}

// REG-AP3-001f — registry.go 字面锁定 v=29 sequencing.
func TestAP_RegistryHasV29(t *testing.T) {
	t.Parallel()
	for _, m := range All {
		if m.Version == 29 {
			if m.Name != "ap_3_1_user_permissions_org" {
				t.Errorf("v=29 name 脱节: got %q, want %q", m.Name, "ap_3_1_user_permissions_org")
			}
			return
		}
	}
	t.Fatal("v=29 (AP-3.1) not registered in migrations.All")
}

// TestPermissionUserPermissionsOrg_Idempotent — re-running v=29 on an already-applied DB is a
// no-op (schema_migrations gate).
func TestPermissionUserPermissionsOrg_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAP31(t, db)

	e := New(db)
	e.Register(userPermissionsOrg)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run ap_3_1: %v", err)
	}
}

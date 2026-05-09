package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runAP21 chains v=22 (adm_2_1 admin_actions) → v=30 (AP-2.1) on a memory
// DB. Requires user_permissions + admin_actions tables; create them inline
// (跟 ap_1_1 / ap_3_1 同模式).
func runAP21(t *testing.T, db *gorm.DB) {
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
	e.Register(adminActions)
	e.Register(userPermissionsRevoked)
	if err := e.Run(0); err != nil {
		t.Fatalf("run ap_2_1: %v", err)
	}
}

// REG-AP2-001 (acceptance §1.1) — schema adds nullable revoked_at column.
func TestAP_AddsRevokedAtColumn(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAP21(t, db)

	cols := pragmaColumns(t, db, "user_permissions")
	c, ok := cols["revoked_at"]
	if !ok {
		t.Fatalf("user_permissions missing revoked_at column (have %v)", keys(cols))
	}
	if c.notNull {
		t.Errorf("user_permissions.revoked_at must be nullable (NULL = active, 设计 ①)")
	}
}

// REG-AP2-001b (acceptance §1.1) — sparse index covers revoked_at IS NOT NULL.
func TestAP_HasRevokedAtIndex(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAP21(t, db)

	const idx = "idx_user_permissions_revoked"
	var sql string
	err := db.Raw(`SELECT sql FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&sql).Error
	if err != nil || sql == "" {
		t.Fatalf("missing index %s (err=%v)", idx, err)
	}
	if !containsCI(sql, "WHERE revoked_at IS NOT NULL") {
		t.Errorf("index %s must be partial (WHERE revoked_at IS NOT NULL); got: %s", idx, sql)
	}
}

// REG-AP2-001c (acceptance §1.2) — admin_actions CHECK accepts the new
// 'permission_expired' enum value (5 → 6 项扩).
func TestAP_AdminActionsCHECKAcceptsPermissionExpired(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAP21(t, db)

	if err := db.Exec(`INSERT INTO admin_actions
		(id, actor_id, target_user_id, action, metadata, created_at)
		VALUES ('act-1', 'system', 'u-1', 'permission_expired', '{}', 1700000000000)`).Error; err != nil {
		t.Fatalf("insert permission_expired: %v", err)
	}
	// Existing 5 actions still accepted.
	for _, a := range []string{"delete_channel", "suspend_user", "change_role", "reset_password", "start_impersonation"} {
		if err := db.Exec(`INSERT INTO admin_actions
			(id, actor_id, target_user_id, action, metadata, created_at)
			VALUES (?, 'a-x', 'u-x', ?, '{}', 1700000000000)`, "act-"+a, a).Error; err != nil {
			t.Errorf("legacy action %s rejected after rebuild: %v", a, err)
		}
	}
}

// REG-AP2-001d (acceptance §1.2 + 反约束) — admin_actions CHECK rejects
// values outside the 6-tuple (反 hardcode drift).
func TestAP_AdminActionsRejectsUnknownAction(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAP21(t, db)

	for _, bad := range []string{"permission_revoked", "expires", "ap2_revoke", "delete_user", ""} {
		if err := db.Exec(`INSERT INTO admin_actions
			(id, actor_id, target_user_id, action, metadata, created_at)
			VALUES (?, 'a-x', 'u-x', ?, '{}', 1700000000000)`, "bad-"+bad, bad).Error; err == nil {
			t.Errorf("admin_actions accepted bad action %q — CHECK 6-tuple drift", bad)
		}
	}
}

// REG-AP2-001e — registry.go 字面锁 v=30.
func TestAP_RegistryHasV30(t *testing.T) {
	t.Parallel()
	for _, m := range All {
		if m.Version == 30 {
			if m.Name != "ap_2_1_user_permissions_revoked" {
				t.Errorf("v=30 name drift: got %q, want %q", m.Name, "ap_2_1_user_permissions_revoked")
			}
			return
		}
	}
	t.Fatal("v=30 (AP-2.1) not registered in migrations.All")
}

// TestPermissionUserPermissionsRevoked_Idempotent — re-running v=30 against an already-applied DB is
// a no-op (schema_migrations gate).
func TestPermissionUserPermissionsRevoked_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAP21(t, db)

	e := New(db)
	e.Register(adminActions)
	e.Register(userPermissionsRevoked)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run ap_2_1: %v", err)
	}
}

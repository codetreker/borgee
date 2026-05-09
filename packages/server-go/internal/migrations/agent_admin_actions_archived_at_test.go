package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runAL71 chains adm_2_1 → ap_2_1 → bpp_8_1 → al_7_1 admin_actions
// migrations. Each migration extends the CHECK enum so chronological
// order matters.
func runAL71(t *testing.T, db *gorm.DB) {
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
	e.Register(adminActionsPluginActions)
	e.Register(adminActionsArchivedAt)
	if err := e.Run(0); err != nil {
		t.Fatalf("run al_7_1 chain: %v", err)
	}
}

// TestAL_AddsArchivedAtColumn — acceptance §1.1.
//
// admin_actions.archived_at must exist as nullable INTEGER (NULL = active
// row, sweeper UPDATE archived_at=now to archive).
func TestAL_AddsArchivedAtColumn(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL71(t, db)
	cols := pragmaColumns(t, db, "admin_actions")
	c, ok := cols["archived_at"]
	if !ok {
		t.Fatalf("admin_actions missing archived_at column (have %v)", keys(cols))
	}
	if c.notNull {
		t.Errorf("admin_actions.archived_at must be nullable (NULL = active 行)")
	}
}

// TestAL_HasSparseIdx — acceptance §1.1.
//
// idx_admin_actions_archived_at must be created with WHERE archived_at IS
// NOT NULL (sparse index 跟 ap_2_1 / ap_1_1 / ap_3_1 同模式).
func TestAL_HasSparseIdx(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL71(t, db)
	var sql string
	if err := db.Raw(`SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_admin_actions_archived_at'`).Scan(&sql).Error; err != nil {
		t.Fatalf("query idx: %v", err)
	}
	if sql == "" {
		t.Fatal("idx_admin_actions_archived_at not created")
	}
	// Sparse WHERE byte-identical with AP-2.1 same-mode partial index.
	if !contains(sql, "WHERE archived_at IS NOT NULL") {
		t.Errorf("expected sparse WHERE clause; got %q", sql)
	}
}

// TestAL_AcceptsAuditRetentionOverride — acceptance §1.2.
//
// All 12 actions (6 ADM-2.1 legacy + 1 AP-2 + 5 BPP-8 + 1 AL-7) must
// INSERT successfully.
func TestAL_AcceptsAuditRetentionOverride(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL71(t, db)
	actions := []string{
		// 6 legacy + 1 AP-2 + 5 BPP-8 + 1 AL-7 = 12
		"delete_channel", "suspend_user", "change_role", "reset_password",
		"start_impersonation", "permission_expired",
		"plugin_connect", "plugin_disconnect", "plugin_reconnect",
		"plugin_cold_start", "plugin_heartbeat_timeout",
		"audit_retention_override",
	}
	for _, a := range actions {
		if err := db.Exec(`INSERT INTO admin_actions
			(id, actor_id, target_user_id, action, metadata, created_at)
			VALUES (?, 'system', 'user-1', ?, '', 1700000000000)`,
			"act-al71-"+a, a).Error; err != nil {
			t.Errorf("INSERT %s: %v", a, err)
		}
	}
	var count int64
	db.Raw(`SELECT COUNT(*) FROM admin_actions`).Scan(&count)
	if count != int64(len(actions)) {
		t.Errorf("expected %d rows, got %d", len(actions), count)
	}
}

// TestAL_RejectsUnknownAction — acceptance §1.2.
//
// 5 spec-外 audit_retention_* names must be rejected by CHECK.
func TestAL_RejectsUnknownAction(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL71(t, db)
	bad := []string{
		"audit_retention_xxx",
		"audit_archive",
		"retention_override",
		"audit_purge",
		"al7_action",
	}
	for _, a := range bad {
		err := db.Exec(`INSERT INTO admin_actions
			(id, actor_id, target_user_id, action, metadata, created_at)
			VALUES (?, 'system', 'user-1', ?, '', 1700000000000)`,
			"act-al71-bad-"+a, a).Error
		if err == nil {
			t.Errorf("CHECK should reject unknown action %q", a)
		}
	}
}

// TestAL_VersionIs33 — registry literal lock.
func TestAL_VersionIs33(t *testing.T) {
	t.Parallel()
	if got, want := adminActionsArchivedAt.Version, 33; got != want {
		t.Errorf("AL-7.1 Version drift: got %d, want %d (post CV-6 v=32 sequencing)", got, want)
	}
	if got, want := adminActionsArchivedAt.Name, "al_7_1_admin_actions_archived_at"; got != want {
		t.Errorf("AL-7.1 Name drift: got %q, want %q", got, want)
	}
	found := false
	for _, m := range All {
		if m.Version == 33 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("AL-7.1 (v=33) not registered in migrations.All")
	}
}

// TestAgentAdminActionsArchivedAt_Idempotent — re-running chain against an already-applied DB
// is a no-op (schema_migrations gate).
func TestAgentAdminActionsArchivedAt_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL71(t, db)
	runAL71(t, db) // second run no-op
	if err := db.Exec(`INSERT INTO admin_actions
		(id, actor_id, target_user_id, action, metadata, created_at)
		VALUES ('act-x', 's', 'u', 'audit_retention_xxx', '', 1700000000000)`).Error; err == nil {
		t.Error("CHECK should still reject after idempotent re-run")
	}
}

// TestAL_NoSeparateArchiveTable — acceptance §1.3 设计 ① 反断.
//
// Verifies that no audit_archive_table / audit_history_log / al7_archive_log
// tables exist after migration chain runs (audit retention reuses
// admin_actions.archived_at, 不裂表).
func TestAL_NoSeparateArchiveTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runAL71(t, db)
	forbidden := []string{
		"audit_archive_table",
		"audit_history_log",
		"al7_archive_log",
	}
	for _, name := range forbidden {
		var n int64
		db.Raw(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?`, name).Scan(&n)
		if n > 0 {
			t.Errorf("AL-7 设计 ① broken: forbidden archive table %q exists (audit reuses admin_actions.archived_at)", name)
		}
	}
}

// contains is a small substring helper (testdouble for string libs that
// avoid pulling in `strings` here just for one call).
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

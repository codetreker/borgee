package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runBPP81 chains adm_2_1 → ap_2_1 → bpp_8_1 admin_actions migrations.
// Each migration extends the CHECK enum so chronological order matters.
func runBPP81(t *testing.T, db *gorm.DB) {
	t.Helper()
	// user_permissions table required by AP-2.1.
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
	if err := e.Run(0); err != nil {
		t.Fatalf("run bpp_8_1 chain: %v", err)
	}
}

// TestBPP_AcceptsAllNewActions — acceptance §1.1.
//
// All 11 actions (6 legacy + 5 new plugin_*) must INSERT successfully.
func TestBPP_AcceptsAllNewActions(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runBPP81(t, db)

	actions := []string{
		// 6 legacy
		"delete_channel", "suspend_user", "change_role",
		"reset_password", "start_impersonation", "permission_expired",
		// 5 new plugin_*
		"plugin_connect", "plugin_disconnect", "plugin_reconnect",
		"plugin_cold_start", "plugin_heartbeat_timeout",
	}
	for i, a := range actions {
		if err := db.Exec(`INSERT INTO admin_actions
			(id, actor_id, target_user_id, action, metadata, created_at)
			VALUES (?, 'system', 'user-1', ?, '', 1700000000000)`,
			"act-bpp81-"+a, a).Error; err != nil {
			t.Errorf("INSERT %s (i=%d): %v", a, i, err)
		}
	}
	var count int64
	db.Raw(`SELECT COUNT(*) FROM admin_actions`).Scan(&count)
	if count != int64(len(actions)) {
		t.Errorf("expected %d rows, got %d", len(actions), count)
	}
}

// TestBPP_RejectsUnknownAction — acceptance §1.1.
//
// 5 spec-外 plugin_* names must be rejected by CHECK constraint.
func TestBPP_RejectsUnknownAction(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runBPP81(t, db)

	bad := []string{
		"plugin_xxx", "plugin_unknown", "plugin_kill",
		"plugin_pause", "plugin_resume",
	}
	for _, a := range bad {
		err := db.Exec(`INSERT INTO admin_actions
			(id, actor_id, target_user_id, action, metadata, created_at)
			VALUES (?, 'system', 'user-1', ?, '', 1700000000000)`,
			"act-bpp81-bad-"+a, a).Error
		if err == nil {
			t.Errorf("CHECK should reject unknown plugin action %q", a)
		}
	}
}

// TestBPP_VersionIs31 — registry literal lock.
func TestBPP_VersionIs31(t *testing.T) {
	t.Parallel()
	if got, want := adminActionsPluginActions.Version, 31; got != want {
		t.Errorf("BPP-8.1 Version drift: got %d, want %d", got, want)
	}
	if got, want := adminActionsPluginActions.Name, "bpp_8_1_admin_actions_plugin_actions"; got != want {
		t.Errorf("BPP-8.1 Name drift: got %q, want %q", got, want)
	}
	// Confirm it's registered in All slice.
	found := false
	for _, m := range All {
		if m.Version == 31 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("BPP-8.1 (v=31) not registered in migrations.All")
	}
}

// TestBPP_Idempotent — re-running the chain against an already-applied
// DB is a no-op (schema_migrations gate prevents re-execution).
func TestBPP_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runBPP81(t, db)
	// Second run should be a no-op via Engine.Run schema_migrations gate.
	runBPP81(t, db)
	// Confirm CHECK still rejects unknown.
	if err := db.Exec(`INSERT INTO admin_actions
		(id, actor_id, target_user_id, action, metadata, created_at)
		VALUES ('act-x', 's', 'u', 'plugin_unknown', '', 1700000000000)`).Error; err == nil {
		t.Error("CHECK should still reject after idempotent re-run")
	}
}

// TestBPP_NoSeparateLifecycleTable — acceptance §1.2 设计 ① 反断.
//
// Verifies that no plugin_lifecycle_events / plugin_audit_log /
// bpp_event_log tables exist after migration chain runs (audit reuses
// admin_actions, 不裂表).
func TestBPP_NoSeparateLifecycleTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runBPP81(t, db)
	forbidden := []string{
		"plugin_lifecycle_events",
		"plugin_audit_log",
		"bpp_event_log",
	}
	for _, name := range forbidden {
		var n int64
		db.Raw(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?`, name).Scan(&n)
		if n > 0 {
			t.Errorf("BPP-8 设计 ① broken: forbidden lifecycle table %q exists (audit reuses admin_actions)", name)
		}
	}
}

package migrations

import "testing"

// TestHelperAndHostGrantsRailDrop_RemovesTables seeds the three helper
// rail / host_grants source migrations (v=27 host_grants + v=49
// helper_enrollments + v=50 helper_credential_rotation + v=51 helper_jobs
// + v=53 helper_updates_available), then runs the v=54 drop migration
// and asserts the three tables are gone from sqlite_master.
//
// Standalone registration (not Default/All) keeps the test self-contained
// without seeding the legacy users/channels/... baseline tables that
// store.createSchema normally builds.
func TestHelperAndHostGrantsRailDrop_RemovesTables(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	e := New(db)
	for _, m := range []Migration{
		hostGrants,
		helperEnrollments,
		helperCredentialRotation,
		helperJobs,
		helperUpdatesAvailable,
		helperAndHostGrantsRailDrop,
	} {
		e.Register(m)
	}
	if err := e.Run(0); err != nil {
		t.Fatalf("run helper + host_grants chain + v=54: %v", err)
	}

	for _, name := range []string{"helper_jobs", "helper_enrollments", "host_grants"} {
		var got string
		if err := db.Raw(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&got).Error; err != nil {
			t.Fatalf("query sqlite_master for %s: %v", name, err)
		}
		if got != "" {
			t.Fatalf("table %q must be dropped after v=54, sqlite_master returned %q", name, got)
		}
	}
}

// TestHelperAndHostGrantsRailDrop_IdempotentOnEmpty registers only the
// v=54 drop migration on an otherwise empty DB. DROP TABLE IF EXISTS
// must succeed even when none of the parent tables were ever created.
func TestHelperAndHostGrantsRailDrop_IdempotentOnEmpty(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	e := New(db)
	e.Register(helperAndHostGrantsRailDrop)
	if err := e.Run(0); err != nil {
		t.Fatalf("run v=54 on empty DB: %v", err)
	}
}

// TestMigrationRegistryV54Position pins v=54 as the tail of All with the
// expected name.
func TestMigrationRegistryV54Position(t *testing.T) {
	t.Parallel()
	if len(All) == 0 {
		t.Fatal("All is empty")
	}
	tail := All[len(All)-1]
	if tail.Version != 54 {
		t.Fatalf("tail migration Version = %d, want 54", tail.Version)
	}
	if tail.Name != "drop_helper_and_host_grants_rails" {
		t.Fatalf("tail migration Name = %q, want drop_helper_and_host_grants_rails", tail.Name)
	}
}

package migrations

import (
	"testing"

	"gorm.io/gorm"
)

func runHelperEnrollments(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(helperEnrollments)
	e.Register(helperCredentialRotation)
	if err := e.Run(0); err != nil {
		t.Fatalf("run helper enrollments: %v", err)
	}
}

func TestHelperEnrollmentsMigrationSchema(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runHelperEnrollments(t, db)

	rows, err := db.Raw(`PRAGMA table_info(helper_enrollments)`).Rows()
	if err != nil {
		t.Fatalf("PRAGMA: %v", err)
	}
	defer rows.Close()
	type col struct {
		name    string
		ctype   string
		notnull int
		pk      int
	}
	var cols []col
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt *string
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		cols = append(cols, col{name: name, ctype: ctype, notnull: notnull, pk: pk})
	}

	want := map[string]struct {
		typ     string
		notnull int
		pk      int
	}{
		"id":                           {"TEXT", 0, 1},
		"owner_user_id":                {"TEXT", 1, 0},
		"org_id":                       {"TEXT", 1, 0},
		"host_label":                   {"TEXT", 1, 0},
		"helper_device_id":             {"TEXT", 0, 0},
		"allowed_categories":           {"TEXT", 1, 0},
		"status":                       {"TEXT", 1, 0},
		"last_seen_at":                 {"INTEGER", 0, 0},
		"created_at":                   {"INTEGER", 1, 0},
		"updated_at":                   {"INTEGER", 1, 0},
		"claimed_at":                   {"INTEGER", 0, 0},
		"revoked_at":                   {"INTEGER", 0, 0},
		"uninstalled_at":               {"INTEGER", 0, 0},
		"enrollment_secret_digest":     {"TEXT", 0, 0},
		"enrollment_secret_expires_at": {"INTEGER", 0, 0},
		"persistent_credential_digest": {"TEXT", 0, 0},
		"credential_created_at":        {"INTEGER", 0, 0},
		"credential_rotated_at":        {"INTEGER", 0, 0},
		"credential_generation":        {"INTEGER", 1, 0},
	}
	if len(cols) != len(want) {
		t.Fatalf("column count mismatch: got %d want %d", len(cols), len(want))
	}
	for _, c := range cols {
		w, ok := want[c.name]
		if !ok {
			t.Errorf("unexpected column %q", c.name)
			continue
		}
		if c.ctype != w.typ || c.notnull != w.notnull || c.pk != w.pk {
			t.Errorf("column %s got (%s,%d,%d), want (%s,%d,%d)", c.name, c.ctype, c.notnull, c.pk, w.typ, w.notnull, w.pk)
		}
	}
}

func TestHelperEnrollmentsStatusEnumRejectsStale(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runHelperEnrollments(t, db)

	base := []any{"h-1", "u-1", "org-1", "host", `["status_collect"]`, int64(1), int64(1)}
	for _, bad := range []string{"stale", "queued", "running", "succeeded", "failed", ""} {
		args := append([]any{}, base...)
		args = append(args[:5], append([]any{bad}, args[5:]...)...)
		err := db.Exec(`INSERT INTO helper_enrollments
			(id, owner_user_id, org_id, host_label, allowed_categories, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, args...).Error
		if err == nil {
			t.Errorf("status %q should be rejected by CHECK", bad)
		}
	}
	for _, good := range []string{"pending", "connected", "offline", "revoked", "uninstalled"} {
		err := db.Exec(`INSERT INTO helper_enrollments
			(id, owner_user_id, org_id, host_label, allowed_categories, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, "id-"+good, "u-1", "org-1", "host", `["status_collect"]`, good, int64(1), int64(1)).Error
		if err != nil {
			t.Errorf("status %q should be accepted: %v", good, err)
		}
	}
}

func TestHelperEnrollmentsMigrationIndexesAndRegistry(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runHelperEnrollments(t, db)

	var idxs []struct{ Name string }
	if err := db.Raw(`SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='helper_enrollments'`).Scan(&idxs).Error; err != nil {
		t.Fatalf("query indexes: %v", err)
	}
	want := map[string]bool{
		"idx_helper_enrollments_owner_org":     false,
		"idx_helper_enrollments_device":        false,
		"idx_helper_enrollments_last_seen":     false,
		"idx_helper_enrollments_secret_expiry": false,
	}
	for _, idx := range idxs {
		if _, ok := want[idx.Name]; ok {
			want[idx.Name] = true
		}
	}
	for name, ok := range want {
		if !ok {
			t.Errorf("missing index %s", name)
		}
	}

	if helperEnrollments.Version != 49 || helperCredentialRotation.Version != 50 {
		t.Fatalf("helper migration versions=%d/%d, want 49/50", helperEnrollments.Version, helperCredentialRotation.Version)
	}
	found := false
	foundRotation := false
	for _, m := range All {
		if m.Version == 49 && m.Name == helperEnrollments.Name {
			found = true
		}
		if m.Version == 50 && m.Name == helperCredentialRotation.Name {
			foundRotation = true
		}
	}
	if !found {
		t.Fatal("helperEnrollments v49 missing from registry All")
	}
	if !foundRotation {
		t.Fatal("helperCredentialRotation v50 missing from registry All")
	}
}

func TestHelperCredentialRotationMigrationPropagatesDDLErrors(t *testing.T) {
	t.Parallel()
	t.Run("missing helper enrollments table", func(t *testing.T) {
		t.Parallel()
		db := openMem(t)
		if err := helperCredentialRotation.Up(db); err == nil {
			t.Fatal("helper credential rotation should fail when helper_enrollments is missing")
		}
	})

	t.Run("duplicate generation column", func(t *testing.T) {
		t.Parallel()
		db := openMem(t)
		if err := db.Exec(`CREATE TABLE helper_enrollments (id TEXT PRIMARY KEY, credential_generation INTEGER NOT NULL DEFAULT 1)`).Error; err != nil {
			t.Fatalf("create helper_enrollments: %v", err)
		}
		if err := helperCredentialRotation.Up(db); err == nil {
			t.Fatal("helper credential rotation should fail when credential_generation already exists")
		}
	})
}

package migrations

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestHelperJobsMigrationCreatesQueueEnvelope(t *testing.T) {
	t.Parallel()
	gdb := openMem(t)
	runHelperEnrollments(t, gdb)
	e := New(gdb)
	e.Register(helperJobs)
	if err := e.Run(0); err != nil {
		t.Fatalf("run helper jobs migration: %v", err)
	}

	columns := helperJobTableColumns(t, gdb)
	for _, name := range []string{
		"id", "owner_user_id", "org_id", "enrollment_id", "helper_device_id",
		"job_type", "category", "schema_version", "payload_json", "payload_hash",
		"manifest_digest", "manifest_binding_json", "idempotency_key", "idempotency_scope",
		"active_idempotency_scope", "status", "failure_code", "failure_message",
		"created_at", "updated_at", "expires_at", "leased_at", "lease_expires_at",
		"completed_at", "result_summary_json",
	} {
		if !columns[name] {
			t.Fatalf("helper_jobs missing column %q; columns=%v", name, columns)
		}
	}

	ddl := sqliteTableDDL(t, gdb, "helper_jobs")
	for _, status := range []string{"queued", "leased", "running", "succeeded", "failed", "cancelled", "expired"} {
		if !strings.Contains(ddl, "'"+status+"'") {
			t.Fatalf("helper_jobs status CHECK missing %q in DDL: %s", status, ddl)
		}
	}

	indexes := helperJobIndexes(t, gdb)
	for _, name := range []string{
		"idx_helper_jobs_owner_org",
		"idx_helper_jobs_enrollment_status",
		"idx_helper_jobs_status_expiry",
		"idx_helper_jobs_active_idempotency_scope",
	} {
		if !indexes[name] {
			t.Fatalf("helper_jobs missing index %q; indexes=%v", name, indexes)
		}
	}
	if indexes["idx_helper_jobs_idempotency_scope"] {
		t.Fatalf("helper_jobs must not have a permanent global unique idempotency_scope index")
	}

	idxDDL := sqliteIndexDDL(t, gdb, "idx_helper_jobs_active_idempotency_scope")
	if !strings.Contains(strings.ToLower(idxDDL), "unique") || !strings.Contains(strings.ToLower(idxDDL), "where active_idempotency_scope is not null") {
		t.Fatalf("active idempotency index must be partial unique on non-null active scope, got: %s", idxDDL)
	}
}

func TestMigrationRegistryIncludesHelperJobsAfterCredentialRotation(t *testing.T) {
	t.Parallel()
	last := All[len(All)-1]
	if last.Version != 51 || last.Name != "helper_job_enqueue_authority" {
		t.Fatalf("last migration = v%d %q, want v51 helper_job_enqueue_authority", last.Version, last.Name)
	}
	prev := -1
	for i, m := range All {
		if m.Version == 50 {
			prev = i
		}
		if m.Version == 51 && prev < 0 {
			t.Fatalf("helper jobs v51 appears before helper credential rotation v50")
		}
	}
}

func helperJobTableColumns(t *testing.T, gdb *gorm.DB) map[string]bool {
	t.Helper()
	rows, err := gdb.Raw(`PRAGMA table_info(helper_jobs)`).Rows()
	if err != nil {
		t.Fatalf("PRAGMA table_info(helper_jobs): %v", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan helper_jobs table_info: %v", err)
		}
		out[name] = true
	}
	return out
}

func helperJobIndexes(t *testing.T, gdb *gorm.DB) map[string]bool {
	t.Helper()
	rows, err := gdb.Raw(`PRAGMA index_list(helper_jobs)`).Rows()
	if err != nil {
		t.Fatalf("PRAGMA index_list(helper_jobs): %v", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan helper_jobs index_list: %v", err)
		}
		out[name] = true
	}
	return out
}

func sqliteTableDDL(t *testing.T, gdb *gorm.DB, name string) string {
	t.Helper()
	var ddl string
	if err := gdb.Raw(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&ddl).Error; err != nil {
		t.Fatalf("load table DDL %s: %v", name, err)
	}
	if ddl == "" {
		t.Fatalf("missing table DDL for %s", name)
	}
	return ddl
}

func sqliteIndexDDL(t *testing.T, gdb *gorm.DB, name string) string {
	t.Helper()
	var ddl string
	if err := gdb.Raw(`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?`, name).Scan(&ddl).Error; err != nil {
		t.Fatalf("load index DDL %s: %v", name, err)
	}
	if ddl == "" {
		t.Fatalf("missing index DDL for %s", name)
	}
	return ddl
}

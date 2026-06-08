package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openMem(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return db
}

// sqliteTableDDL returns the CREATE TABLE statement recorded in
// sqlite_master for the named table. Shared across migration tests that
// need to assert CHECK constraint literals (e.g. enum coverage). Defined
// here after the helper-rail tests were removed in t1; channel_member_*
// and others still depend on it.
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

func TestEnsureSchemaCreatesTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	e := New(db)
	if err := e.EnsureSchema(); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	// Calling twice must be safe.
	if err := e.EnsureSchema(); err != nil {
		t.Fatalf("ensure twice: %v", err)
	}
	var n int64
	if err := db.Raw("SELECT COUNT(*) FROM schema_migrations").Row().Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("fresh schema_migrations should be empty, got %d", n)
	}
}

func TestRunAppliesPendingInOrder(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	var calls []int
	e := New(db)
	e.RegisterAll([]Migration{
		{Version: 2, Name: "second", Up: func(tx *gorm.DB) error { calls = append(calls, 2); return nil }},
		{Version: 1, Name: "first", Up: func(tx *gorm.DB) error { calls = append(calls, 1); return nil }},
		{Version: 3, Name: "third", Up: func(tx *gorm.DB) error { calls = append(calls, 3); return nil }},
	})
	if err := e.Run(0); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(calls) != 3 || calls[0] != 1 || calls[1] != 2 || calls[2] != 3 {
		t.Fatalf("expected ascending order, got %v", calls)
	}
	// Re-running is a no-op.
	calls = nil
	if err := e.Run(0); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("expected idempotent rerun, got %v", calls)
	}
}

func TestRunRecordsVersionAndName(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	e := New(db)
	e.Register(Migration{
		Version: 42,
		Name:    "fake_dummy_table",
		Up: func(tx *gorm.DB) error {
			return tx.Exec("CREATE TABLE _dummy (id INTEGER PRIMARY KEY)").Error
		},
	})
	if err := e.Run(0); err != nil {
		t.Fatalf("run: %v", err)
	}

	// G0.1 acceptance — schema_migrations has the row.
	var (
		ver  int
		name string
		ts   int64
	)
	row := db.Raw("SELECT version, name, applied_at FROM schema_migrations WHERE version = 42").Row()
	if err := row.Scan(&ver, &name, &ts); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if ver != 42 || name != "fake_dummy_table" || ts == 0 {
		t.Fatalf("unexpected row: ver=%d name=%q ts=%d", ver, name, ts)
	}

	// And the dummy table really exists.
	if err := db.Exec("INSERT INTO _dummy (id) VALUES (1)").Error; err != nil {
		t.Fatalf("insert _dummy: %v", err)
	}
}

func TestRunTargetCaps(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	e := New(db)
	for _, v := range []int{1, 2, 3} {
		v := v
		e.Register(Migration{
			Version: v,
			Name:    "m",
			Up:      func(tx *gorm.DB) error { return nil },
		})
	}
	if err := e.Run(2); err != nil {
		t.Fatalf("run target=2: %v", err)
	}
	applied, _ := e.Applied()
	if _, ok := applied[1]; !ok {
		t.Fatal("expected v1 applied")
	}
	if _, ok := applied[2]; !ok {
		t.Fatal("expected v2 applied")
	}
	if _, ok := applied[3]; ok {
		t.Fatal("v3 should not be applied with target=2")
	}
}

func TestRunRollsBackOnFailure(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	e := New(db)
	e.Register(Migration{
		Version: 1,
		Name:    "boom",
		Up: func(tx *gorm.DB) error {
			if err := tx.Exec("CREATE TABLE _boom (id INTEGER)").Error; err != nil {
				return err
			}
			return errBoom
		},
	})
	if err := e.Run(0); err == nil {
		t.Fatal("expected error")
	}
	applied, _ := e.Applied()
	if _, ok := applied[1]; ok {
		t.Fatal("failed migration must not be recorded")
	}
	// Table created inside the failing tx should be rolled back.
	var count int64
	row := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='_boom'").Row()
	if err := row.Scan(&count); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if count != 0 {
		t.Fatal("expected _boom to be rolled back")
	}
}

func TestValidateRejectsDuplicates(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	e := New(db)
	e.Register(Migration{Version: 1, Name: "a", Up: func(tx *gorm.DB) error { return nil }})
	e.Register(Migration{Version: 1, Name: "b", Up: func(tx *gorm.DB) error { return nil }})
	if err := e.Run(0); err == nil {
		t.Fatal("expected duplicate version error")
	}
}

func TestValidateRejectsBadInput(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	cases := []Migration{
		{Version: 0, Name: "zero", Up: func(tx *gorm.DB) error { return nil }},
		{Version: 1, Name: "", Up: func(tx *gorm.DB) error { return nil }},
		{Version: 2, Name: "nilup", Up: nil},
	}
	for _, m := range cases {
		e := New(db)
		e.Register(m)
		if err := e.Run(0); err == nil {
			t.Fatalf("expected validation error for %+v", m)
		}
	}
}

func TestDefaultRegistryRunsClean(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	// CM-1.1 ALTERs five legacy tables that store.createSchema normally
	// builds. Recreate the minimum surface here so the migration package
	// stays self-contained at test time.
	for _, name := range []string{"users", "channels", "messages", "workspace_files", "remote_nodes"} {
		if err := db.Exec("CREATE TABLE " + name + " (id TEXT PRIMARY KEY)").Error; err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	// AP-0-bis (v=8) reads users.role/deleted_at and inserts into user_permissions.
	// Add the columns the migration touches so the registry test stays self-
	// contained without pulling in store.createSchema.
	if err := db.Exec(`ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'member'`).Error; err != nil {
		t.Fatalf("add users.role: %v", err)
	}
	if err := db.Exec(`ALTER TABLE users ADD COLUMN deleted_at INTEGER`).Error; err != nil {
		t.Fatalf("add users.deleted_at: %v", err)
	}
	if err := db.Exec(`CREATE TABLE user_permissions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		permission TEXT NOT NULL,
		scope TEXT NOT NULL DEFAULT '*',
		granted_by TEXT,
		granted_at INTEGER NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create user_permissions: %v", err)
	}
	// CM-3 (v=9) backfills resource org_id from creator/sender/uploader columns.
	// Add the foreign-key columns the UPDATE WHERE clauses reference so the
	// registry test stays self-contained.
	for _, alter := range []string{
		`ALTER TABLE channels        ADD COLUMN created_by TEXT`,
		`ALTER TABLE messages        ADD COLUMN sender_id  TEXT`,
		`ALTER TABLE workspace_files ADD COLUMN user_id    TEXT`,
		`ALTER TABLE remote_nodes    ADD COLUMN user_id    TEXT`,
	} {
		if err := db.Exec(alter).Error; err != nil {
			t.Fatalf("alter for cm-3 backfill: %v", err)
		}
	}
	if err := Default(db).Run(0); err != nil {
		t.Fatalf("default run: %v", err)
	}
	applied, err := Default(db).Applied()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range All {
		if _, ok := applied[m.Version]; !ok {
			t.Fatalf("default registry did not apply v%d (%s)", m.Version, m.Name)
		}
	}
}

var errBoom = boomErr("boom")

type boomErr string

func (b boomErr) Error() string { return string(b) }

// TestEnsureSchema_ExecError covers the err branch when the underlying
// DB rejects DDL (closed connection forces ErrInvalidDB / similar).
func TestEnsureSchema_ExecError(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	e := New(db)
	if err := e.EnsureSchema(); err == nil {
		t.Error("expected error from EnsureSchema after DB close")
	}
}

// TestApplied_QueryError covers the query-failure branch (drop the
// schema_migrations table after EnsureSchema records it the first time
// — no, instead use closed DB to force query err).
func TestApplied_QueryError(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	e := New(db)
	if err := e.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.Close()
	if _, err := e.Applied(); err == nil {
		t.Error("expected error from Applied after DB close")
	}
}

// TestRun_ValidationDuplicate covers the validate() duplicate-version branch.
func TestRun_ValidationDuplicate(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	e := New(db)
	noop := func(tx *gorm.DB) error { return nil }
	e.RegisterAll([]Migration{
		{Version: 5, Name: "a", Up: noop},
		{Version: 5, Name: "b", Up: noop},
	})
	if err := e.Run(0); err == nil {
		t.Error("expected duplicate-version error")
	}
}

// TestRun_ValidationZeroVersion covers the non-positive version branch.
func TestRun_ValidationZeroVersion(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	e := New(db)
	e.Register(Migration{Version: 0, Name: "x", Up: func(tx *gorm.DB) error { return nil }})
	if err := e.Run(0); err == nil {
		t.Error("expected zero-version error")
	}
}

// TestRun_ValidationEmptyName covers empty-name branch.
func TestRun_ValidationEmptyName(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	e := New(db)
	e.Register(Migration{Version: 1, Name: "", Up: func(tx *gorm.DB) error { return nil }})
	if err := e.Run(0); err == nil {
		t.Error("expected empty-name error")
	}
}

// TestRun_ValidationNilUp covers nil-Up branch.
func TestRun_ValidationNilUp(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	e := New(db)
	e.Register(Migration{Version: 1, Name: "x", Up: nil})
	if err := e.Run(0); err == nil {
		t.Error("expected nil-Up error")
	}
}

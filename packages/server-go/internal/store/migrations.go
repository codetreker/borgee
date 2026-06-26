package store

import (
	"fmt"
	"regexp"
	"strings"

	"borgee-server/internal/migrations"
)

func (s *Store) Migrate() error {
	// Disable FK constraints during migration to avoid issues with table recreation
	if err := s.execMigrationSQL("disable foreign keys", "PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}

	if err := s.createSchema(); err != nil {
		s.db.Exec("PRAGMA foreign_keys = ON")
		return err
	}

	if err := s.createSchemaIndexes(); err != nil {
		s.db.Exec("PRAGMA foreign_keys = ON")
		return err
	}

	// Re-enable FK constraints after migration
	if err := s.execMigrationSQL("enable foreign keys", "PRAGMA foreign_keys = ON"); err != nil {
		return err
	}

	// Run forward-only registry migrations after the baseline. The engine is
	// idempotent — already-applied versions are skipped via schema_migrations.
	// On an existing DB the single baseline (version 1) is already recorded, so
	// this is a no-op; on a fresh DB it records the baseline version. cmd/migrate
	// also runs this after Migrate(); having it here keeps in-process boot
	// (cmd/collab) and tests on the same path.
	if err := migrations.Default(s.db).Run(0); err != nil {
		return fmt.Errorf("forward-only migrations: %w", err)
	}

	// Seed the singleton 'system' user. This is essential runtime DATA (not
	// schema, so it is not part of the baseline DDL / golden): the app inserts
	// messages with sender_id='system' (welcome channel, capability grants,
	// artifact notices), all of which require this FK target to exist. It was
	// previously seeded by the cm_onboarding_welcome migration (v7), which the
	// baseline squash collapsed; the seed lives here so a fresh DB still has it.
	// INSERT OR IGNORE makes it a no-op on existing DBs that already carry the
	// row. This is NOT a dead backfill (it is required on every fresh DB) and so
	// is intentionally retained when the dead data-backfills were removed.
	if err := s.seedSystemUser(); err != nil {
		return fmt.Errorf("seed system user: %w", err)
	}

	return nil
}

// seedSystemUser inserts the singleton id='system' user if absent. Idempotent.
func (s *Store) seedSystemUser() error {
	return s.execMigrationSQL("seed system user", `
INSERT OR IGNORE INTO users (id, display_name, role, created_at, disabled, require_mention, org_id)
VALUES ('system', '系统', 'system', 0, 1, 0, '')`)
}

// ifNotExistsRE injects "IF NOT EXISTS" after the CREATE [UNIQUE] [VIRTUAL]
// TABLE|INDEX|TRIGGER|VIEW keyword so re-running the baseline on an existing DB
// is a no-op. The stored statements (schema_baseline_gen.go) are the verbatim
// golden `sql` text (no IF NOT EXISTS, matching sqlite_master) so the AC-1
// equivalence test compares byte-for-byte; the clause is added only at exec.
var ifNotExistsRE = regexp.MustCompile(`^(\s*CREATE\s+(?:UNIQUE\s+)?(?:VIRTUAL\s+)?(?:TABLE|INDEX|TRIGGER|VIEW))\s+`)

func withIfNotExists(stmt string) string {
	if ifNotExistsRE.MatchString(stmt) && !strings.Contains(strings.ToUpper(stmt), "IF NOT EXISTS") {
		return ifNotExistsRE.ReplaceAllString(stmt, "$1 IF NOT EXISTS ")
	}
	return stmt
}

// createSchema execs the re-baselined consolidated schema: every non-auto
// sqlite_master object (tables, the FTS5 virtual table, indexes, the view, and
// triggers) captured after the legacy baseline + all forward-only migrations.
// Auto objects (sqlite_sequence, FTS shadow tables, sqlite_autoindex_*)
// materialize automatically. FK enforcement is OFF here (Migrate toggles it),
// so inter-table reference order does not matter; the statements are emitted in
// dependency order (tables -> FTS5 -> indexes -> view -> triggers) for the
// objects that DO depend on each other at creation time (view/triggers on their
// base table, FTS triggers on the virtual table).
//
// Each statement is run with IF NOT EXISTS injected so this is a safe no-op on
// existing v48 DBs, which already carry the full schema.
func (s *Store) createSchema() error {
	for _, stmt := range schemaBaselineStatements {
		if stmt == "" || isIndexStmt(stmt) {
			continue
		}
		if err := s.execMigrationSQL("create schema", withIfNotExists(stmt)); err != nil {
			return err
		}
	}
	return nil
}

// createSchemaIndexes execs the CREATE INDEX statements from the baseline,
// after createSchema has created all tables. Split from createSchema only to
// preserve the historical Migrate() call shape; both run under the same FK-off
// window.
func (s *Store) createSchemaIndexes() error {
	for _, stmt := range schemaBaselineStatements {
		if !isIndexStmt(stmt) {
			continue
		}
		if err := s.execMigrationSQL("create schema indexes", withIfNotExists(stmt)); err != nil {
			return err
		}
	}
	return nil
}

var indexStmtRE = regexp.MustCompile(`^\s*CREATE\s+(?:UNIQUE\s+)?INDEX\s`)

func isIndexStmt(stmt string) bool { return indexStmtRE.MatchString(stmt) }

func (s *Store) execMigrationSQL(label, sql string) error {
	if err := s.db.Exec(sql).Error; err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	return nil
}

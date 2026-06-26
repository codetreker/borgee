package migrations

import "gorm.io/gorm"

// All is the canonical, ordered list of versioned migrations applied by the
// server on startup.
//
// Rules:
//   - Version is strictly increasing. Never reuse or renumber.
//   - Once a migration is on main, its body is immutable. To change schema,
//     append a new migration (v2+).
//
// One-time re-baseline (migrations-baseline-squash): the forward-only chain
// that previously ran from v1 (dummy marker) through v54 was collapsed into the
// store-layer baseline schema (internal/store: createSchema /
// schemaBaselineStatements), which now reproduces the exact post-v54 schema on
// a fresh DB. The registry therefore carries a single baseline entry. This was
// a deliberate, reviewed squash of already-shipped, immutable migrations — it
// does not change any existing database: every existing DB already recorded
// version 1 (the original always-applied dummy), so the baseline entry is a
// skip/no-op on them, and the store baseline uses IF NOT EXISTS so re-running
// schema creation on a populated DB changes nothing. The immutability /
// forward-only rule is unchanged for everything from here on: new schema work
// lands as v2+ here, never by editing this baseline. The baseline schema is
// regenerated only from the committed golden snapshot, gated by the AC-1
// equivalence test in internal/store.
var All = []Migration{
	{
		Version: 1,
		Name:    "baseline_schema",
		Up: func(tx *gorm.DB) error {
			// The full schema is created by the store baseline
			// (createSchema/schemaBaselineStatements) before this engine runs.
			// This entry only records the baseline version in schema_migrations
			// and ensures the inert marker table exists, so a fresh DB and an
			// existing (already-migrated) DB converge on the same bookkeeping
			// state. It is a no-op on existing DBs, which already have version 1
			// recorded.
			return tx.Exec(`CREATE TABLE IF NOT EXISTS _migrations_marker (
  version INTEGER PRIMARY KEY,
  note    TEXT
)`).Error
		},
	},
}

// Default returns an Engine wired to db with All registered.
func Default(db *gorm.DB) *Engine {
	e := New(db)
	e.RegisterAll(All)
	return e
}

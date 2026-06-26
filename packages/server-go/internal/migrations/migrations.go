// Package migrations is the forward-only versioned migration framework for
// Borgee server-go.
//
// Blueprint: data-layer §3.2 forward-only versioned migrations.
//
// The schema is established by a one-time baseline: the store layer
// (internal/store createSchema / schemaBaselineStatements) creates the full
// schema on a fresh DB, and this registry carries a single baseline entry
// (version 1). New schema changes are added here as numbered, immutable
// migrations (v2+) rather than mutating the baseline.
//
// Contract:
//   - Migrations are registered with a strictly increasing integer Version.
//   - Once a migration is applied (its Version recorded in schema_migrations),
//     its body MUST NOT be edited. Add a new migration instead.
//   - The single exception already taken: the v1..v54 chain was squashed once
//     into the baseline (migrations-baseline-squash). That squash is schema-
//     neutral on existing DBs (version 1 was always applied, so the baseline is
//     a no-op on them) and is not a license to edit applied migrations going
//     forward — v2+ remain forward-only and immutable.
//   - There is no Down(). Recovery relies on backups; disposable dev DBs are
//     rebuilt, not migrated.
package migrations

import (
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
)

// Migration is one forward-only schema change identified by a unique Version.
// Up runs inside the engine's transaction; the engine handles recording the
// version after Up returns nil.
type Migration struct {
	Version int
	Name    string
	Up      func(tx *gorm.DB) error
}

// Engine applies migrations and tracks state in schema_migrations.
type Engine struct {
	db   *gorm.DB
	regs []Migration
}

// New returns an Engine bound to db. The engine does not auto-discover
// migrations; callers register them explicitly via Register.
func New(db *gorm.DB) *Engine {
	return &Engine{db: db}
}

// Register adds a migration. Duplicate Version values cause Run to fail loudly.
func (e *Engine) Register(m Migration) {
	e.regs = append(e.regs, m)
}

// RegisterAll appends a slice of migrations.
func (e *Engine) RegisterAll(ms []Migration) {
	e.regs = append(e.regs, ms...)
}

// EnsureSchema creates the schema_migrations table if missing. Safe to call
// repeatedly. Run() invokes this; tests may call it directly.
func (e *Engine) EnsureSchema() error {
	const ddl = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version    INTEGER PRIMARY KEY,
  applied_at INTEGER NOT NULL,
  name       TEXT NOT NULL
)`
	if err := e.db.Exec(ddl).Error; err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	return nil
}

// Applied returns the set of versions already recorded in schema_migrations.
func (e *Engine) Applied() (map[int]struct{}, error) {
	if err := e.EnsureSchema(); err != nil {
		return nil, err
	}
	rows, err := e.db.Raw("SELECT version FROM schema_migrations").Rows()
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	out := map[int]struct{}{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = struct{}{}
	}
	return out, rows.Err()
}

// Pending returns registered migrations that have not been applied yet, sorted
// ascending by Version.
func (e *Engine) Pending() ([]Migration, error) {
	applied, err := e.Applied()
	if err != nil {
		return nil, err
	}
	pending := make([]Migration, 0, len(e.regs))
	for _, m := range e.regs {
		if _, ok := applied[m.Version]; ok {
			continue
		}
		pending = append(pending, m)
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].Version < pending[j].Version })
	return pending, nil
}

// Run applies all pending migrations in order. If target > 0, only migrations
// with Version <= target are applied. Each migration runs inside its own
// transaction; on failure the engine stops and returns the error.
func (e *Engine) Run(target int) error {
	if err := e.validate(); err != nil {
		return err
	}
	pending, err := e.Pending()
	if err != nil {
		return err
	}
	for _, m := range pending {
		if target > 0 && m.Version > target {
			break
		}
		if err := e.apply(m); err != nil {
			return fmt.Errorf("migration %d (%s): %w", m.Version, m.Name, err)
		}
	}
	return nil
}

// validate checks for duplicate or non-positive versions and missing names.
func (e *Engine) validate() error {
	seen := map[int]string{}
	for _, m := range e.regs {
		if m.Version <= 0 {
			return fmt.Errorf("migration version must be > 0 (got %d, name=%q)", m.Version, m.Name)
		}
		if m.Name == "" {
			return fmt.Errorf("migration version %d has empty name", m.Version)
		}
		if m.Up == nil {
			return fmt.Errorf("migration %d (%s) has nil Up", m.Version, m.Name)
		}
		if prev, ok := seen[m.Version]; ok {
			return fmt.Errorf("duplicate migration version %d (%s vs %s)", m.Version, prev, m.Name)
		}
		seen[m.Version] = m.Name
	}
	return nil
}

func (e *Engine) apply(m Migration) error {
	return e.db.Transaction(func(tx *gorm.DB) error {
		if err := m.Up(tx); err != nil {
			return err
		}
		return tx.Exec(
			"INSERT INTO schema_migrations (version, applied_at, name) VALUES (?, ?, ?)",
			m.Version, time.Now().UnixMilli(), m.Name,
		).Error
	})
}

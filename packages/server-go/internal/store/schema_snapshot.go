// Package store — schema_snapshot.go: PERF-SCHEMA-SHARED helpers for sqlite
// :memory: database serialization. Tests use this to skip per-test full
// Migrate (~13ms) by sharing a snapshot taken once after migrations.
//
// Background: testutil.NewTestServer calls store.Open(":memory:") +
// store.Migrate() per test (~20ms each, 256 api tests = ~5s). With race
// detector + parallel scheduling this multiplies. SerializeSchema captures
// the post-Migrate database as a byte slice once; DeserializeSchema rebuilds
// any future :memory: DB from those bytes — bypassing all Migration Up
// callbacks, just loading sqlite pages directly.
//
// Implementation: uses mattn/go-sqlite3's sqlite3_serialize / sqlite3_deserialize
// (sqlite ≥3.23, build tag sqlite_serialize is default for the driver). We
// reach the underlying *sqlite3.SQLiteConn via Conn.Raw().
//
// Invariants (covered by tests):
//   - Serialize is deterministic (same migrations → same bytes)
//   - Deserialize replaces the entire DB content (rows + schema)
//   - Per-test isolation preserved: each test deserializes into its own
//     :memory: DB, mutations don't bleed across tests
//   - Concurrent restore is safe (per-test :memory: DBs are independent)
//   - Fallback: if Serialize fails (older sqlite / build-tag mismatch),
//     callers fall back to full Migrate (no API change required)
//
// Single source of truth: this file owns the serialize/deserialize boundary;
// testutil/server.go is the single caller (NewTestServer two-phase init:
// snapshot once + restore N times).

package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mattn/go-sqlite3"
)

// SerializeSchema returns a byte snapshot of the underlying sqlite "main"
// database. The snapshot includes schema + all current rows; callers using
// it for per-test fast-restore should call this AFTER Migrate (and any
// shared fixture seeding that should be reproduced across tests).
//
// Returns an error if the underlying driver doesn't expose Serialize (e.g.
// non-sqlite3, or sqlite3 built without the serialize amalgamation).
func (s *Store) SerializeSchema() ([]byte, error) {
	sqlDB, err := s.db.DB()
	if err != nil {
		return nil, fmt.Errorf("get *sql.DB: %w", err)
	}
	return serializeFromSQL(sqlDB)
}

// DeserializeSchema replaces this store's underlying database content with
// the given snapshot. The Store must have been opened against ":memory:"
// (file-backed sqlite doesn't support deserialize without extra flags).
//
// After Deserialize the connection is fully replaced — schema + rows from
// the snapshot. The store's *gorm.DB handle is preserved (caller can
// continue using s.DB()).
func (s *Store) DeserializeSchema(snapshot []byte) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("get *sql.DB: %w", err)
	}
	return deserializeIntoSQL(sqlDB, snapshot)
}

// serializeFromSQL is the low-level helper — Conn.Raw() into *sqlite3.SQLiteConn,
// call Serialize("main"). Exposed at package level for testability without
// a *Store (used by tests that want to round-trip arbitrary DBs).
func serializeFromSQL(db *sql.DB) ([]byte, error) {
	conn, err := db.Conn(context.Background())
	if err != nil {
		return nil, fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Close()
	var snapshot []byte
	rawErr := conn.Raw(func(driverConn any) error {
		sc, ok := driverConn.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("driver conn is %T, want *sqlite3.SQLiteConn", driverConn)
		}
		b, err := sc.Serialize("main")
		if err != nil {
			return fmt.Errorf("sqlite3 Serialize: %w", err)
		}
		snapshot = b
		return nil
	})
	if rawErr != nil {
		return nil, rawErr
	}
	return snapshot, nil
}

// deserializeIntoSQL replaces the SQL DB's content with the snapshot via
// sqlite3_deserialize. The DB must be :memory: (file-backed needs extra flags).
//
// We use a raw conn to call Deserialize; after the call the conn is held
// open via sql.DB pool semantics — sqlite3_deserialize replaces the DB
// pages on this exact conn. With SetMaxOpenConns(1) + SetMaxIdleConns(1)
// the pool reuses this conn for all subsequent gorm operations, so they
// see the deserialized content.
func deserializeIntoSQL(db *sql.DB, snapshot []byte) error {
	conn, err := db.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	rawErr := conn.Raw(func(driverConn any) error {
		sc, ok := driverConn.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("driver conn is %T, want *sqlite3.SQLiteConn", driverConn)
		}
		if err := sc.Deserialize(snapshot, "main"); err != nil {
			return fmt.Errorf("sqlite3 Deserialize: %w", err)
		}
		return nil
	})
	// Release the conn back to the pool (it will be reused by gorm because
	// SetMaxOpenConns(1) + SetMaxIdleConns(1) keep it pinned).
	if closeErr := conn.Close(); closeErr != nil && rawErr == nil {
		return fmt.Errorf("release conn: %w", closeErr)
	}
	// PERF defensive: Ping the pool to ensure the conn is healthy before
	// gorm starts using it. This catches any post-deserialize state issues
	// up front rather than as flaky failures mid-test.
	if rawErr == nil {
		if err := db.PingContext(context.Background()); err != nil {
			return fmt.Errorf("post-deserialize ping: %w", err)
		}
	}
	return rawErr
}

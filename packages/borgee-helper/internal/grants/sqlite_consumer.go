// Package grants provides the SQLite-backed consumer for HB-3 #520 host_grants.
// It replaces the v0(C) MemoryConsumer mock for production reads.
//
// hb-2-v0d-spec.md §0.2 requires revoke <100ms. Each IPC call re-queries the
// read-only database with the HB-3 spec §1.4 `revoked_at IS NULL` predicate;
// grant state is not cached.
//
// 读: SELECT id, scope, expires_at, granted_at, revoked_at FROM host_grants
//      WHERE agent_id = ? AND scope = ? AND revoked_at IS NULL
//      ORDER BY granted_at DESC LIMIT 1
//
// HB-3 schema has 9 fields (id/user_id/agent_id/grant_type/scope/ttl_kind/
// granted_at/expires_at/revoked_at). HB-2 v0(D) consumes it read-only.

package grants

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3" // sqlite3 driver
)

// SQLiteConsumer is the read-only HB-3 host_grants consumer.
type SQLiteConsumer struct {
	db    *sql.DB
	nowFn func() int64
}

// NewSQLiteConsumer opens host_grants DB (read-only mode, mode=ro).
// dsn is a sqlite3 connection string, for example "file:/var/lib/borgee/server.db?mode=ro&_busy_timeout=5000".
func NewSQLiteConsumer(dsn string) (*SQLiteConsumer, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	// Read-only daemon — no writes; constrain conn pool.
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Minute)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite ping: %w", err)
	}
	return &SQLiteConsumer{
		db:    db,
		nowFn: func() int64 { return time.Now().UnixMilli() },
	}, nil
}

// SetNowFn injects a clock for TTL boundary tests.
func (c *SQLiteConsumer) SetNowFn(f func() int64) { c.nowFn = f }

// Close closes the DB.
func (c *SQLiteConsumer) Close() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

// Lookup implements Consumer.
//
// Each call executes a SELECT against the read-only DB, with no grant-state
// cache, so revoke visibility follows the per-call lookup path.
func (c *SQLiteConsumer) Lookup(ctx context.Context, agentID, scope string) (Grant, bool, error) {
	g, exists, expired, err := c.LookupRaw(ctx, agentID, scope)
	if err != nil || !exists || expired {
		return Grant{}, false, err
	}
	return g, true, nil
}

// LookupRaw distinguishes not_found from expired. Revoked rows are filtered by
// revoked_at IS NULL and therefore appear as not_found to helper lookup.
func (c *SQLiteConsumer) LookupRaw(ctx context.Context, agentID, scope string) (Grant, bool, bool, error) {
	const q = `SELECT id, scope, expires_at, granted_at, revoked_at
		FROM host_grants
		WHERE agent_id = ? AND scope = ? AND revoked_at IS NULL
		ORDER BY granted_at DESC LIMIT 1`
	row := c.db.QueryRowContext(ctx, q, agentID, scope)
	var (
		id         string
		dbScope    string
		expiresAt  sql.NullInt64
		grantedAt  int64
		revokedAt  sql.NullInt64
	)
	if err := row.Scan(&id, &dbScope, &expiresAt, &grantedAt, &revokedAt); err != nil {
		if err == sql.ErrNoRows {
			return Grant{}, false, false, nil
		}
		return Grant{}, false, false, fmt.Errorf("sqlite scan: %w", err)
	}
	g := Grant{
		AgentID:   agentID,
		Scope:     dbScope,
		GrantedAt: grantedAt,
	}
	if expiresAt.Valid {
		g.TTLUntil = expiresAt.Int64
	}
	// expired check: TTLUntil>0 且 ≤ now.
	if g.TTLUntil != 0 && g.TTLUntil <= c.nowFn() {
		return g, true, true, nil
	}
	return g, true, false, nil
}

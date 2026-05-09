// Package grants — SQLite consumer (v0(D) 真启). 替代 v0(C) MemoryConsumer
// mock. 走 read-only sqlite3 driver 真接 HB-3 #520 host_grants 表.
//
// hb-2-v0d-spec.md §0.2: 撤销 <100ms 真守 — 每次 IPC call 重查 (反 cache),
// HB-3 spec §1.4 "daemon 不缓存; revoked_at IS NULL 谓词单源". grep 检查
// `grantsCache|cachedGrants` 0 hit (反约束 §1.3).
//
// 读: SELECT id, scope, expires_at, revoked_at FROM host_grants
//      WHERE agent_id = ? AND scope = ? AND revoked_at IS NULL
//      ORDER BY granted_at DESC LIMIT 1
//
// HB-3 schema 9 字段 (id/user_id/agent_id/grant_type/scope/ttl_kind/
// granted_at/expires_at/revoked_at). HB-2 v0(D) 仅消费 read-only.

package grants

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3" // sqlite3 driver
)

// SQLiteConsumer 是 read-only HB-3 host_grants consumer (drop-in 替
// MemoryConsumer).
type SQLiteConsumer struct {
	db    *sql.DB
	nowFn func() int64
}

// NewSQLiteConsumer opens host_grants DB (read-only mode, mode=ro).
// dsn 是 sqlite3 connection string e.g. "file:/var/lib/borgee/server.db?mode=ro&_busy_timeout=5000".
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

// SetNowFn 注入时间源 (单测 TTL 边界用).
func (c *SQLiteConsumer) SetNowFn(f func() int64) { c.nowFn = f }

// Close 关闭 DB.
func (c *SQLiteConsumer) Close() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

// Lookup — Consumer interface 实现.
//
// 反约束 §1.3 不缓存: 每次 call SELECT 真走 SQL (撤销 <100ms 真守, 由
// internal/api/host_grants_test.go::TestHB_DELETE_RevokeStampsRevokedAt
// 行为 test 守门).
func (c *SQLiteConsumer) Lookup(ctx context.Context, agentID, scope string) (Grant, bool, error) {
	g, exists, expired, err := c.LookupRaw(ctx, agentID, scope)
	if err != nil || !exists || expired {
		return Grant{}, false, err
	}
	return g, true, nil
}

// LookupRaw 区分 not_found / expired / revoked (caller 决定 reason 字典).
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

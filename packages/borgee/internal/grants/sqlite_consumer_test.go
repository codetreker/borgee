// Package grants — sqlite_consumer_test.go: SQLite consumer real-DB integration test.
package grants

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// setupHostGrantsDB creates an in-memory sqlite DB seeded with the HB-3
// host_grants schema, byte-identical with packages/server-go/internal/
// migrations/host_grants.go, and returns the dsn plus raw db handle for
// seeding rows.
func setupHostGrantsDB(t *testing.T) (string, *sql.DB) {
	t.Helper()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	dsn := "file:" + dbPath + "?_busy_timeout=5000"
	rw, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("open rw: %v", err)
	}
	t.Cleanup(func() { rw.Close() })

	const schema = `CREATE TABLE host_grants (
  id          TEXT    PRIMARY KEY,
  user_id     TEXT    NOT NULL,
  agent_id    TEXT,
  grant_type  TEXT    NOT NULL,
  scope       TEXT    NOT NULL,
  ttl_kind    TEXT    NOT NULL,
  granted_at  INTEGER NOT NULL,
  expires_at  INTEGER,
  revoked_at  INTEGER
)`
	if _, err := rw.Exec(schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return dsn, rw
}

func TestHB2D_SQLiteConsumer_LookupHappyPath(t *testing.T) {
	t.Parallel()
	dsn, rw := setupHostGrantsDB(t)
	_, err := rw.Exec(
		`INSERT INTO host_grants(id, user_id, agent_id, grant_type, scope, ttl_kind, granted_at)
		 VALUES('g1','u1','a1','filesystem','/data',?, 100)`,
		"always",
	)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	c, err := NewSQLiteConsumer(dsn)
	if err != nil {
		t.Fatalf("consumer: %v", err)
	}
	defer c.Close()
	c.SetNowFn(func() int64 { return 200 })

	g, ok, err := c.Lookup(context.Background(), "a1", "/data")
	if err != nil || !ok {
		t.Fatalf("lookup: ok=%v err=%v", ok, err)
	}
	if g.Scope != "/data" {
		t.Errorf("scope 脱节: %q", g.Scope)
	}
}

func TestHB2D_SQLiteConsumer_NotFound(t *testing.T) {
	t.Parallel()
	dsn, _ := setupHostGrantsDB(t)
	c, _ := NewSQLiteConsumer(dsn)
	defer c.Close()
	_, ok, err := c.Lookup(context.Background(), "a1", "/missing")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Error("expected not-found")
	}
}

// TestHB2D_SQLiteConsumer_RevocationImmediate verifies revoke visibility under
// 100ms (HB-3 §1.5 behavior invariant). UPDATE revoked_at = now means the next
// Lookup immediately returns 0 rows. The server-go path
// internal/api/host_grants_test.go::TestHB_DELETE_RevokeStampsRevokedAt checks
// the same invariant, keeping the SQL behavior consistent end to end.
func TestHB2D_SQLiteConsumer_RevocationImmediate(t *testing.T) {
	t.Parallel()
	dsn, rw := setupHostGrantsDB(t)
	_, err := rw.Exec(
		`INSERT INTO host_grants(id, user_id, agent_id, grant_type, scope, ttl_kind, granted_at)
		 VALUES('g1','u1','a1','filesystem','/data','always',100)`,
	)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	c, _ := NewSQLiteConsumer(dsn)
	defer c.Close()

	t0 := time.Now()
	if _, ok, _ := c.Lookup(context.Background(), "a1", "/data"); !ok {
		t.Fatal("setup: grant missing")
	}

	// Revoke.
	_, err = rw.Exec(`UPDATE host_grants SET revoked_at = 999 WHERE id = 'g1'`)
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// Lookup immediately after revoke.
	_, ok, _ := c.Lookup(context.Background(), "a1", "/data")
	elapsed := time.Since(t0)
	if ok {
		t.Error("revocation 不立即生效 (反 grantsCache 反向约束 break)")
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("HB-3 §1.5 撤销→reject latency %v > 100ms (反向约束 §1.3 不缓存)", elapsed)
	}
}

func TestHB2D_SQLiteConsumer_Expired(t *testing.T) {
	t.Parallel()
	dsn, rw := setupHostGrantsDB(t)
	_, err := rw.Exec(
		`INSERT INTO host_grants(id, user_id, agent_id, grant_type, scope, ttl_kind, granted_at, expires_at)
		 VALUES('g1','u1','a1','filesystem','/data','one_shot',100, 500)`,
	)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	c, _ := NewSQLiteConsumer(dsn)
	defer c.Close()
	c.SetNowFn(func() int64 { return 600 }) // > expires_at=500

	_, ok, _ := c.Lookup(context.Background(), "a1", "/data")
	if ok {
		t.Error("expected expired (ok=false)")
	}
	_, exists, expired, _ := c.LookupRaw(context.Background(), "a1", "/data")
	if !exists || !expired {
		t.Errorf("LookupRaw expired: exists=%v expired=%v", exists, expired)
	}
}

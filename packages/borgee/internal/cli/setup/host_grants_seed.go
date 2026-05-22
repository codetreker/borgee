//go:build linux || darwin

package setup

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// hostGrantsSchema mirrors the canonical server-go migration v=27
// (packages/server-go/internal/migrations/host_grants.go). The daemon
// opens this file with mode=ro at runtime and refuses to start if the
// `host_grants` table is missing; `borgee setup` creates the empty
// database (with the schema + indexes) up front so a freshly-installed
// host has a working daemon without any server-side rsync of the
// authoritative SQLite store. Once the host is claimed and starts
// receiving grants via the host-grants REST channel, this local DB is
// the only thing the daemon ever reads — so the bytes are identical to
// what the server would emit.
const hostGrantsSchema = `CREATE TABLE IF NOT EXISTS host_grants (
  id          TEXT    PRIMARY KEY,
  user_id     TEXT    NOT NULL,
  agent_id    TEXT,
  grant_type  TEXT    NOT NULL CHECK (grant_type IN ('install','exec','filesystem','network')),
  scope       TEXT    NOT NULL,
  ttl_kind    TEXT    NOT NULL CHECK (ttl_kind IN ('one_shot','always')),
  granted_at  INTEGER NOT NULL,
  expires_at  INTEGER,
  revoked_at  INTEGER
);
CREATE INDEX IF NOT EXISTS idx_host_grants_user_id ON host_grants(user_id);
CREATE INDEX IF NOT EXISTS idx_host_grants_agent_id ON host_grants(agent_id) WHERE agent_id IS NOT NULL;`

// dsnFilePath extracts the on-disk file path from a sqlite3 DSN of the
// form `file:/abs/path?mode=ro&...`. Returns ("", false) for any other
// shape; the caller treats that as "skip seeding" so a future custom
// DSN (in-memory test variant, network DSN) won't be silently rewritten
// to a local file.
func dsnFilePath(dsn string) (string, bool) {
	if !strings.HasPrefix(dsn, "file:") {
		return "", false
	}
	trimmed := strings.TrimPrefix(dsn, "file:")
	if idx := strings.IndexByte(trimmed, '?'); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	if decoded, err := url.PathUnescape(trimmed); err == nil {
		trimmed = decoded
	}
	if !filepath.IsAbs(trimmed) {
		return "", false
	}
	return trimmed, true
}

// seedHostGrantsDB creates the on-disk SQLite database referenced by
// `dsn` if missing and ensures the `host_grants` schema is present. The
// file is then chowned to (username, groupname) so the daemon's
// mode=ro open succeeds under its unprivileged uid. Returns nil when
// the file already has the table — re-running `borgee setup` after a
// claim must NOT wipe the grants the server pushed in between.
//
// Sequence:
//  1. parse the DSN, extract the absolute file path
//  2. ensure parent directory exists
//  3. open writable (no mode=ro), run schema DDL, close
//  4. chown file to the helper user
func seedHostGrantsDB(dsn, username, groupname string) error {
	path, ok := dsnFilePath(dsn)
	if !ok {
		return fmt.Errorf("seed host_grants: unrecognized DSN %q (expected file:/abs/path?...)", dsn)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("seed host_grants: mkdir parent %s: %w", filepath.Dir(path), err)
	}
	// Open writable. The dsn passed to the daemon includes mode=ro; we
	// strip that here by reconstructing the bare file: URI.
	writableDSN := "file:" + path + "?_busy_timeout=5000"
	db, err := sql.Open("sqlite3", writableDSN)
	if err != nil {
		return fmt.Errorf("seed host_grants: open %s: %w", writableDSN, err)
	}
	defer db.Close()
	if _, err := db.Exec(hostGrantsSchema); err != nil {
		return fmt.Errorf("seed host_grants: schema: %w", err)
	}
	// Tighten file perms before chown so the brief world-readable window
	// is closed even on default-umask systems.
	if err := os.Chmod(path, 0o640); err != nil {
		return fmt.Errorf("seed host_grants: chmod %s: %w", path, err)
	}
	if username != "" {
		if err := chown(path, username, groupname); err != nil {
			return fmt.Errorf("seed host_grants: chown %s: %w", path, err)
		}
	}
	return nil
}

//go:build integration && (linux || darwin)

// Package e2e covers the HB-2 daemon startup integration behavior.
//
// hb-2-v0d-e2e-spec.md §1 case-1 daemon startup:
//   - go build daemon binary
//   - start it with --grants-db=<seeded-sqlite> and --read-paths=<tmp>
//   - wait for the UDS socket by polling stat as startup evidence
//   - send SIGTERM so ctx.Done closes the net.Listener and the process exits
//   - verify that the audit log file exists instead of silently aborting
//
// Design note (hb-2-v0d-e2e-spec.md §0 design ①+②):
//   - no production .go changes; this _test.go only uses the existing
//     cmd/borgee-helper main.go
//   - the `integration` build tag keeps this out of default CI, matching the
//     HB-2.0 #605 IPC coverage pattern
package e2e

import (
	"bytes"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// skipIfLandlockEPERM checks whether the daemon stderr indicates a
// landlock_restrict_self EPERM (CI runner lacks PR_SET_NO_NEW_PRIVS / CAP_SYS_ADMIN).
// Returns true if the test should skip with an explicit reason, as required by spec §0.1.
func skipIfLandlockEPERM(t *testing.T, stderr *bytes.Buffer) bool {
	t.Helper()
	if !strings.Contains(stderr.String(), "landlock_restrict_self") {
		return false
	}
	if !strings.Contains(stderr.String(), "operation not permitted") {
		return false
	}
	t.Skipf("landlock_restrict_self EPERM — runner lacks PR_SET_NO_NEW_PRIVS / CAP_SYS_ADMIN " +
		"(production daemon installed via systemd/launchd has these set; run full e2e coverage on a production-like runner)")
	return true
}

const hostGrantsSchema = `CREATE TABLE host_grants (
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

// seedHostGrantsDB creates a sqlite DB with HB-3 host_grants schema +
// one seed row for the daemon to consume on startup.
func seedHostGrantsDB(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "host_grants.db")
	dsn := "file:" + dbPath + "?_busy_timeout=5000"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(hostGrantsSchema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO host_grants(id,user_id,agent_id,grant_type,scope,ttl_kind,granted_at)
		 VALUES('g1','u1','a1','filesystem',?,'always',100)`,
		tmp,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return dsn
}

// buildDaemon builds the borgee-helper binary into a tempdir and returns its path.
func buildDaemon(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "borgee-helper")
	cmd := exec.Command("go", "build", "-o", binPath, "../cmd/borgee-helper")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build: %v", err)
	}
	return binPath
}

// TestHB2DE_DaemonStartup_BuildsAndListens covers case-1 daemon startup.
//
// It builds the daemon, starts it with --grants-db, --read-paths, and --socket,
// waits for UDS readiness, sends SIGTERM, and verifies that the process exits.
// The socket must be visible before the timeout so startup failures do not pass silently.
func TestHB2DE_DaemonStartup_BuildsAndListens(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires go build + fork+exec)")
	}
	binPath := buildDaemon(t)
	dsn := seedHostGrantsDB(t)

	tmp := t.TempDir()
	socketPath := filepath.Join(tmp, "borgee-helper.sock")
	auditPath := filepath.Join(tmp, "audit.log.jsonl")

	cmd := exec.Command(
		binPath,
		"--socket="+socketPath,
		"--audit-log="+auditPath,
		"--grants-db="+dsn,
		"--read-paths="+tmp,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("daemon start: %v", err)
	}
	t.Cleanup(func() {
		// Best-effort cleanup if SIGTERM handling fails partway through the test.
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	// Poll for UDS socket readiness. The 2s budget is for a startup readiness check,
	// not a performance threshold.
	deadline := time.Now().Add(2 * time.Second)
	var ready bool
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			ready = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		if skipIfLandlockEPERM(t, &stderr) {
			return
		}
		t.Fatalf("daemon did not create UDS socket within 2s (platform=%s) stderr=%q", runtime.GOOS, stderr.String())
	}

	// Audit log file should be created so startup failures do not pass silently.
	if _, err := os.Stat(auditPath); err != nil {
		t.Errorf("audit log not created: %v", err)
	}

	// SIGTERM → daemon should exit cleanly via signal.NotifyContext.
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM: %v", err)
	}
	doneCh := make(chan error, 1)
	go func() { doneCh <- cmd.Wait() }()
	select {
	case err := <-doneCh:
		// Linux/macOS: clean shutdown returns nil; signal-killed returns *exec.ExitError.
		// Either is acceptable; we only assert no hang.
		_ = err
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("daemon did not exit within 3s after SIGTERM")
	}
}

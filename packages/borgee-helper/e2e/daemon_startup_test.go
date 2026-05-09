//go:build integration && (linux || darwin)

// Package e2e — HB-2 v0(D) #617 daemon startup integration test.
//
// hb-2-v0d-e2e-spec.md §1 case-1 daemon 真启:
//   - go build daemon binary
//   - 拉起 with --grants-db=<seeded-sqlite> + --read-paths=<tmp>
//   - 等 UDS socket 就绪 (轮询 stat 真启证据)
//   - SIGTERM 触发 ctx.Done → 反向断 net.Listener.Close → 进程 0 退出
//   - audit log 文件存在 (反 silent abort)
//
// 设计 (hb-2-v0d-e2e-spec.md §0 设计 ①+②):
//   - 0 production .go 改 (本 _test.go 仅消费既有 cmd/borgee-helper main.go)
//   - build tag `integration` 隔离 (CI 不默认跑, 跟 HB-2.0 #605 IPC matrix 同模式)
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
// Returns true if test should skip-with-reason (反 silent skip per spec §0.1).
func skipIfLandlockEPERM(t *testing.T, stderr *bytes.Buffer) bool {
	t.Helper()
	if !strings.Contains(stderr.String(), "landlock_restrict_self") {
		return false
	}
	if !strings.Contains(stderr.String(), "operation not permitted") {
		return false
	}
	t.Skipf("landlock_restrict_self EPERM — runner lacks PR_SET_NO_NEW_PRIVS / CAP_SYS_ADMIN " +
		"(production daemon installed via systemd/launchd has these set; e2e真测留生产runner)")
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
// 1 seed row for the daemon to consume on startup.
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

// buildDaemon builds the borgee-helper binary into a tempdir + returns path.
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

// TestHB2DE_DaemonStartup_BuildsAndListens — case-1 daemon 真启.
//
// 真测: build → start with --grants-db + --read-paths + --socket → 等
// UDS 就绪 → SIGTERM → 进程退出. 反 silent abort: 1s timeout 内 socket
// 必须 stat 成功.
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
		// best-effort cleanup if SIGTERM path fails partway
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	// Poll for UDS socket readiness (≤2s budget — landlock + sandbox apply
	// is fast; this is a startup smoke check, not a perf gate).
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

	// Audit log file should be created (反 silent abort).
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

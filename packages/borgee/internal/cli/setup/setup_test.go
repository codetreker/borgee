//go:build linux || darwin

package setup

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestRenderLinuxUnit_Shape locks the rendered systemd unit shape.
// Originally enforced via outbound_prereq_assets_test.go against the static
// borgee-helper.service asset; that asset is now rendered by `borgee setup`
// so the same anti-regression net runs against the renderer.
func TestRenderLinuxUnit_Shape(t *testing.T) {
	layout := LinuxUserLayoutWithInstallPrefix("alice", 1000, 1000, "/home/alice", "/opt/borgee-test")
	unit := renderLinuxUserUnit("https://app.borgee.io", layout)
	required := []string{
		"[Install]",
		"WantedBy=default.target",
		"NoNewPrivileges=yes",
		"RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6",
		"--outbound-server-origin=https://app.borgee.io",
		"--outbound-allowed-origins=https://app.borgee.io",
		"--queue-state-dir=/home/alice/.local/state/borgee/queue",
		"--status-state-dir=/home/alice/.local/state/borgee/status",
		"--audit-handoff-dir=/home/alice/.local/state/borgee/audit-handoff",
		"--enrollment-id-file=/home/alice/.local/state/borgee/credential/enrollment-id",
		"--helper-device-id-file=/home/alice/.local/state/borgee/credential/device-id",
		"--helper-credential-file=/home/alice/.local/state/borgee/credential/credential",
		"--rootd-socket=/run/borgee/1000/borgee-rootd.sock",
		"RuntimeDirectory=borgee",
		"RuntimeDirectoryMode=0750",
		"ExecStart=/opt/borgee-test/bin/borgee daemon",
		"MemoryMax=256M",
		"CPUQuota=50%",
		"TasksMax=256",
		"Restart=on-failure",
		"RestartSec=10s",
		"StartLimitIntervalSec=5min",
		"StartLimitBurst=5",
		"After=network-online.target",
		"Wants=network-online.target",
		"After=network-online.target",
		"Wants=network-online.target",
		"Type=simple",
		// PR-4 amend (#1033) — ReadWritePaths must include the path
		// roots declared by the signed helper-policy manifest so the
		// four no-root executors' writes land within the systemd
		// hardening sandbox.
		"/home/alice/.local/state/borgee/openclaw",
		"/home/alice/.local/state/borgee/plugins",
		"/home/alice/.local/state/borgee/state",
		// Amend gap #3: landlock_create_ruleset / landlock_add_rule
		// are NOT in @system-service; the daemon's in-process landlock
		// layer SIGSYS-dies on @system-service alone. Additive group
		// syntax per systemd-syscall-filter(7).
		"SystemCallFilter=@system-service @sandbox",
	}
	for _, want := range required {
		if !strings.Contains(unit, want) {
			t.Fatalf("rendered linux unit missing %q\n%s", want, unit)
		}
	}
	forbidden := []string{
		"AF_PACKET",
		"AF_NETLINK",
		"AF_RAW",
		"sudo",
		"--remote-agent",
		"--reverse-ws",
		"--poll-loop",
		"--restart-service",
		// PR-3 #1041: paths come from signed manifest binding now,
		// no daemon-startup root flags.
		"--state-root",
		"--openclaw-config-root",
		"--plugin-config-root",
		"MemoryMax=infinity",
		"CPUQuota=0%",
		"TasksMax=infinity",
		"Restart=always",
		"WantedBy=graphical.target",
		"borgee-helper.service",
		"User=borgee",
		"Group=borgee",
		"/var/lib/borgee",
	}
	for _, bad := range forbidden {
		if strings.Contains(unit, bad) {
			t.Fatalf("rendered linux unit contains forbidden %q", bad)
		}
	}
}

func TestRenderDarwinPlist_Shape(t *testing.T) {
	layout := DarwinUserLayoutWithInstallPrefix("alice", 501, 20, "/Users/alice", "/opt/borgee-test")
	plist := renderDarwinUserPlist("https://app.borgee.io", layout)
	required := []string{
		"/usr/bin/sandbox-exec",
		"<string>/opt/borgee-test/bin/borgee</string>",
		"<string>daemon</string>",
		"--socket=/Users/alice/Library/Application Support/Borgee/borgee.sock",
		"--rootd-socket=/Users/Shared/Borgee/501/borgee-rootd.sock",
		"--outbound-server-origin=https://app.borgee.io",
		"--outbound-allowed-origins=https://app.borgee.io",
		"--queue-state-dir=/Users/alice/Library/Application Support/Borgee/Helper/QueueState",
		"--status-state-dir=/Users/alice/Library/Application Support/Borgee/Helper/StatusState",
		"--audit-handoff-dir=/Users/alice/Library/Application Support/Borgee/Helper/AuditHandoff",
		"--enrollment-id-file=/Users/alice/Library/Application Support/Borgee/Helper/credential/enrollment-id",
		"--helper-device-id-file=/Users/alice/Library/Application Support/Borgee/Helper/credential/device-id",
		"--helper-credential-file=/Users/alice/Library/Application Support/Borgee/Helper/credential/credential",
		"<key>RunAtLoad</key>",
		"<true/>",
		"<key>SuccessfulExit</key>",
		"<false/>",
		"<key>ThrottleInterval</key>",
		"<integer>10</integer>",
	}
	for _, want := range required {
		if !strings.Contains(plist, want) {
			t.Fatalf("rendered macOS plist missing %q", want)
		}
	}
	forbidden := []string{
		"<key>KeepAlive</key>\n    <true/>",
		"<integer>0</integer>",
		"--remote-agent",
		"sudo",
		// PR-3 #1041: no daemon-startup root flags on macOS plist either.
		"--state-root",
		"--openclaw-config-root",
		"--plugin-config-root",
		"<string>_borgee</string>",
		"<key>UserName</key>",
	}
	for _, bad := range forbidden {
		if strings.Contains(plist, bad) {
			t.Fatalf("rendered macOS plist contains forbidden %q", bad)
		}
	}
}

// TestRenderLinuxRootdUnit_Shape locks the rootd companion systemd unit
// shape. Different shape from borgee.service — rootd runs as User=root,
// has no network access (AF_UNIX-only), tighter resource caps, and
// ReadWritePaths covering what PR-4 root commands will need to write to.
func TestRenderLinuxRootdUnit_Shape(t *testing.T) {
	layout := LinuxUserLayoutWithInstallPrefix("alice", 1000, 1000, "/home/alice", "/opt/borgee-test")
	unit := renderLinuxRootdUnit(layout)
	required := []string{
		"Description=Borgee root-privileged companion daemon",
		"User=root",
		"ExecStart=/opt/borgee-test/bin/borgee rootd",
		"--socket=/run/borgee/1000/borgee-rootd.sock",
		"--allowed-peer-uid=1000",
		"--socket-owner-uid=1000",
		"--socket-owner-gid=1000",
		// Defense-in-depth: rootd is AF_UNIX-only (no network).
		"RestrictAddressFamilies=AF_UNIX",
		"NoNewPrivileges=yes",
		"ProtectSystem=strict",
		"ProtectHome=yes",
		"PrivateTmp=yes",
		"MemoryDenyWriteExecute=yes",
		"SystemCallFilter=@system-service",
		"LockPersonality=yes",
		// Tighter resource caps than main daemon.
		"MemoryMax=64M",
		"CPUQuota=10%",
		"TasksMax=32",
		// Issue #1053: rootd's UDS lives under /run/borgee. systemd must
		// create the dir before ExecStart; otherwise first boot fails
		// with "Failed to set up mount namespacing: /run/borgee: No such
		// file or directory" until borgee.service lazily creates it.
		"RuntimeDirectory=borgee/1000",
		"RuntimeDirectoryMode=0750",
		// ReadWritePaths covers PR-4 needs (install_plugin / service_lifecycle).
		"ReadWritePaths=/run/borgee/1000 /opt/borgee-test /home/alice/.local/state/borgee /etc/systemd/system",
		"Restart=on-failure",
		"RestartSec=10s",
		"WantedBy=multi-user.target",
	}
	for _, want := range required {
		if !strings.Contains(unit, want) {
			t.Fatalf("rendered rootd unit missing %q\n%s", want, unit)
		}
	}
	forbidden := []string{
		// rootd has NO network — these would defeat the threat model.
		"AF_INET",
		"AF_INET6",
		"AF_PACKET",
		"AF_NETLINK",
		// rootd must NOT run as borgee — the whole point is privilege split.
		"User=borgee",
		"PeerGroup=borgee",
		// Main daemon flags must not leak in.
		"--outbound-server-origin",
		"--outbound-allowed-origins",
		"--helper-credential-file",
		"--queue-state-dir",
		// Watchdog-spam style restart=always defeats StartLimit; on-failure only.
		"Restart=always",
	}
	for _, bad := range forbidden {
		if strings.Contains(unit, bad) {
			t.Fatalf("rendered rootd unit contains forbidden %q", bad)
		}
	}
}

// TestRenderDarwinRootdPlist_Shape locks the rootd launchd plist shape.
// User=root + GroupName=wheel, no sandbox-exec wrapper, separate log
// paths from the main plist so an operator can grep rootd-stdout
// independently.
func TestRenderDarwinRootdPlist_Shape(t *testing.T) {
	layout := DarwinUserLayoutWithInstallPrefix("alice", 501, 20, "/Users/alice", "/opt/borgee-test")
	plist := renderDarwinRootdPlist(layout)
	required := []string{
		"<key>Label</key>",
		"<string>cloud.borgee.host-bridge.rootd.501</string>",
		"<string>/opt/borgee-test/bin/borgee</string>",
		"<string>rootd</string>",
		"--socket=/Users/Shared/Borgee/501/borgee-rootd.sock",
		"--allowed-peer-uid=501",
		"--socket-owner-uid=501",
		"--socket-owner-gid=20",
		"<key>UserName</key>",
		"<string>root</string>",
		"<key>GroupName</key>",
		"<string>wheel</string>",
		"<key>RunAtLoad</key>",
		"<true/>",
		"<key>SuccessfulExit</key>",
		"<false/>",
		"<key>ThrottleInterval</key>",
		"<integer>10</integer>",
		"<string>/Library/Logs/Borgee/rootd-stdout.log</string>",
		"<string>/Library/Logs/Borgee/rootd-stderr.log</string>",
	}
	for _, want := range required {
		if !strings.Contains(plist, want) {
			t.Fatalf("rendered rootd plist missing %q\n%s", want, plist)
		}
	}
	forbidden := []string{
		// rootd is intentionally root — sandbox-exec wrapper is for the
		// main helper daemon only.
		"/usr/bin/sandbox-exec",
		// Must not run as the unprivileged user.
		"<string>_borgee</string>",
		// Main plist label must not collide.
		"<string>cloud.borgee.host-bridge</string>\n",
	}
	for _, bad := range forbidden {
		if strings.Contains(plist, bad) {
			t.Fatalf("rendered rootd plist contains forbidden %q", bad)
		}
	}
}

// TestRootdSeparateFromMainUnit guards privilege separation: the rootd
// unit MUST be distinct from the main borgee.service unit and the rootd
// plist MUST be distinct from the main launchd plist. A regression that
// folded both into one file would silently re-create the all-root-daemon
// design we are explicitly splitting away from.
func TestRootdSeparateFromMainUnit(t *testing.T) {
	if linuxRootdServiceDst == linuxServiceDst {
		t.Fatalf("rootd systemd unit path collides with main daemon: %s", linuxRootdServiceDst)
	}
	if darwinRootdPlistDst == darwinPlistDst {
		t.Fatalf("rootd launchd plist path collides with main daemon: %s", darwinRootdPlistDst)
	}
	if darwinRootdPlistLabel == darwinPlistLabel {
		t.Fatalf("rootd launchd label collides with main daemon: %s", darwinRootdPlistLabel)
	}
	if linuxRootdSocket == "/run/borgee/borgee.sock" {
		t.Fatalf("rootd UDS must not collide with main daemon UDS")
	}
}

// TestRenderLinuxUnit_WSSOrigin (PR-2 #1038) — the daemon's persistent
// transport is WebSocket so the systemd unit's --outbound-server-origin
// passes the wss:// URL through unchanged. Prior to PR-2 the install
// flow silently downgraded wss:// → https://; that silent downgrade is
// gone.
func TestRenderLinuxUnit_WSSOrigin(t *testing.T) {
	unit := renderLinuxUnit("wss://borgee.codetrek.cn")
	if !strings.Contains(unit, "--outbound-server-origin=wss://borgee.codetrek.cn") {
		t.Fatalf("rendered linux unit missing wss origin\n%s", unit)
	}
	if strings.Contains(unit, "https://borgee.codetrek.cn") {
		t.Fatalf("rendered linux unit must not silently downgrade wss to https\n%s", unit)
	}
}

// TestRenderDarwinPlist_WSSOrigin — same check for the launchd plist.
func TestRenderDarwinPlist_WSSOrigin(t *testing.T) {
	plist := renderDarwinPlist("wss://borgee.codetrek.cn")
	if !strings.Contains(plist, "--outbound-server-origin=wss://borgee.codetrek.cn") {
		t.Fatalf("rendered macOS plist missing wss origin")
	}
}

// TestSeedHostGrantsDB_CreatesSchemaAndChmodsTight (amend gap #2) — the
// daemon opens its grants DSN with mode=ro at startup; if `borgee setup`
// did not pre-create the file with the canonical `host_grants` schema the
// daemon dies with "no such table". This test seeds a fresh file into
// t.TempDir(), then asserts a separate mode=ro reader can SELECT against
// the table without error. Mirrors the daemon's exact runtime contract.
func TestSeedHostGrantsDB_CreatesSchemaAndChmodsTight(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "server.db")
	writableDSN := "file:" + dbPath + "?_busy_timeout=5000"
	// Pass empty username so chown is skipped — we cannot create the
	// `borgee` system user inside the test sandbox. The schema-creation
	// path is what matters; production chown is exercised at install
	// time when root runs setup.
	if err := seedHostGrantsDB(writableDSN, "", ""); err != nil {
		t.Fatalf("seedHostGrantsDB: %v", err)
	}
	// Reopen with mode=ro (the daemon's actual DSN shape) and query the
	// table. If schema didn't land, this errors with "no such table".
	roDSN := "file:" + dbPath + "?mode=ro&_busy_timeout=5000"
	db, err := sql.Open("sqlite3", roDSN)
	if err != nil {
		t.Fatalf("open mode=ro: %v", err)
	}
	defer db.Close()
	row := db.QueryRow(`SELECT COUNT(*) FROM host_grants`)
	var n int
	if err := row.Scan(&n); err != nil {
		t.Fatalf("query host_grants after seed: %v", err)
	}
	if n != 0 {
		t.Fatalf("freshly seeded host_grants should be empty, got %d rows", n)
	}
	// Re-seeding must be idempotent: running `borgee setup` after a
	// claim must NOT wipe any grants the server pushed in between.
	if _, err := sql.Open("sqlite3", writableDSN); err != nil {
		t.Fatalf("reopen writable for idempotence test: %v", err)
	}
	if err := seedHostGrantsDB(writableDSN, "", ""); err != nil {
		t.Fatalf("re-seed: %v", err)
	}
}

// TestDsnFilePath_RejectsNonFileScheme guards the DSN parser. setup's
// host-grants seed is the only writer; if a future code path passes a
// non-file: DSN (in-memory, network), it must NOT silently fall through
// to creating a path-named file in cwd.
func TestDsnFilePath_RejectsNonFileScheme(t *testing.T) {
	t.Parallel()
	if _, ok := dsnFilePath(":memory:"); ok {
		t.Fatal(":memory: DSN must not be treated as a file path")
	}
	if _, ok := dsnFilePath("relative/path.db"); ok {
		t.Fatal("bare relative path must be rejected")
	}
	if path, ok := dsnFilePath("file:/var/lib/borgee/server.db?mode=ro&_busy_timeout=5000"); !ok || path != "/var/lib/borgee/server.db" {
		t.Fatalf("canonical DSN extract: ok=%v path=%q", ok, path)
	}
}

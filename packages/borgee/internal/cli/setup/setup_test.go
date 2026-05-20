//go:build linux || darwin

package setup

import (
	"strings"
	"testing"
)

// TestRenderLinuxUnit_Shape locks the rendered systemd unit shape.
// Originally enforced via outbound_prereq_assets_test.go against the static
// borgee-helper.service asset; that asset is now rendered by `borgee setup`
// so the same anti-regression net runs against the renderer.
func TestRenderLinuxUnit_Shape(t *testing.T) {
	unit := renderLinuxUnit("https://app.borgee.io")
	required := []string{
		"User=borgee",
		"Group=borgee",
		"NoNewPrivileges=yes",
		"RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6",
		"--outbound-server-origin=https://app.borgee.io",
		"--outbound-allowed-origins=https://app.borgee.io",
		"--queue-state-dir=/var/lib/borgee/queue",
		"--status-state-dir=/var/lib/borgee/status",
		"--audit-handoff-dir=/var/lib/borgee/audit-handoff",
		"--enrollment-id-file=/var/lib/borgee/credential/enrollment-id",
		"--helper-device-id-file=/var/lib/borgee/credential/device-id",
		"--helper-credential-file=/var/lib/borgee/credential/credential",
		"StateDirectory=borgee",
		"ExecStart=/usr/local/lib/borgee/bin/borgee daemon",
		"MemoryMax=256M",
		"CPUQuota=50%",
		"TasksMax=256",
		"Restart=on-failure",
		"RestartSec=10s",
		"StartLimitIntervalSec=5min",
		"StartLimitBurst=5",
		"WantedBy=multi-user.target",
		"After=network-online.target",
		"Wants=network-online.target",
		"Type=simple",
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
		"MemoryMax=infinity",
		"CPUQuota=0%",
		"TasksMax=infinity",
		"Restart=always",
		"WantedBy=default.target",
		"WantedBy=graphical.target",
		"borgee-helper.service",
	}
	for _, bad := range forbidden {
		if strings.Contains(unit, bad) {
			t.Fatalf("rendered linux unit contains forbidden %q", bad)
		}
	}
}

func TestRenderDarwinPlist_Shape(t *testing.T) {
	plist := renderDarwinPlist("https://app.borgee.io")
	required := []string{
		"/usr/bin/sandbox-exec",
		"<string>/usr/local/libexec/borgee/borgee</string>",
		"<string>daemon</string>",
		"--socket=/Users/Shared/Borgee/borgee.sock",
		"--outbound-server-origin=https://app.borgee.io",
		"--outbound-allowed-origins=https://app.borgee.io",
		"--queue-state-dir=/Library/Application Support/Borgee/Helper/QueueState",
		"--status-state-dir=/Library/Application Support/Borgee/Helper/StatusState",
		"--audit-handoff-dir=/Library/Application Support/Borgee/Helper/AuditHandoff",
		"--enrollment-id-file=/Library/Application Support/Borgee/Helper/credential/enrollment-id",
		"--helper-device-id-file=/Library/Application Support/Borgee/Helper/credential/device-id",
		"--helper-credential-file=/Library/Application Support/Borgee/Helper/credential/credential",
		"<key>UserName</key>",
		"<string>_borgee</string>",
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
	unit := renderLinuxRootdUnit()
	required := []string{
		"Description=Borgee root-privileged companion daemon",
		"User=root",
		"ExecStart=/usr/local/lib/borgee/bin/borgee rootd",
		"--socket=/run/borgee/borgee-rootd.sock",
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
		// ReadWritePaths covers PR-4 needs (install_plugin / service_lifecycle).
		"ReadWritePaths=/run/borgee /usr/local/lib/borgee /var/lib/borgee /etc/systemd/system",
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
	plist := renderDarwinRootdPlist()
	required := []string{
		"<key>Label</key>",
		"<string>cloud.borgee.host-bridge.rootd</string>",
		"<string>/usr/local/libexec/borgee/borgee</string>",
		"<string>rootd</string>",
		"--socket=/Users/Shared/Borgee/borgee-rootd.sock",
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

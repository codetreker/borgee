package install

import (
	"os"
	"strings"
	"testing"
)

func readAsset(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

func TestLinuxServiceOutboundPrereqShape(t *testing.T) {
	service := readAsset(t, "borgee-helper.service")
	for _, want := range []string{
		"User=borgee-helper",
		"Group=borgee-helper",
		"NoNewPrivileges=yes",
		"RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6",
		"--outbound-server-origin=https://app.borgee.io",
		"--outbound-allowed-origins=https://app.borgee.io",
		"--queue-state-dir=/var/lib/borgee-helper/queue",
		"--status-state-dir=/var/lib/borgee-helper/status",
		"--audit-handoff-dir=/var/lib/borgee-helper/audit-handoff",
		// #968 R4 — heartbeat producer config files. File-based (not raw
		// strings) so secrets never appear in /proc/PID/cmdline.
		"--enrollment-id-file=/var/lib/borgee-helper/enrollment-id",
		"--helper-device-id-file=/var/lib/borgee-helper/device-id",
		"--helper-credential-file=/var/lib/borgee-helper/credential",
		"StateDirectory=borgee-helper",
		"ReadWritePaths=/var/log/borgee-helper /run/borgee-helper /var/lib/borgee-helper/queue /var/lib/borgee-helper/status /var/lib/borgee-helper/audit-handoff",
		"ReadOnlyPaths=/var/lib/borgee",
		// #1000 — cgroups resource caps (蓝图 host-bridge.md:57
		// "systemd + cgroups 限制"). Locks 3 numeric directives
		// at byte level so a future PR can't silently drop them.
		"MemoryMax=256M",
		"CPUQuota=50%",
		"TasksMax=256",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("linux service missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"RestrictAddressFamilies=AF_UNIX\n",
		"AF_PACKET",
		"AF_NETLINK",
		"AF_RAW",
		"sudo",
		"--remote-agent",
		"--reverse-ws",
		"--poll-loop",
		"--lease",
		"--result",
		"--restart-service",
		// #1000 — reverse-guard: an "infinity"/"0%" override would
		// re-introduce the unbounded daemon condition the cgroups
		// caps exist to prevent.
		"MemoryMax=infinity",
		"CPUQuota=0%",
		"TasksMax=infinity",
	} {
		if strings.Contains(service, forbidden) {
			t.Fatalf("linux service contains forbidden %q", forbidden)
		}
	}
}

func TestLinuxServiceBootCrashRestartIsBounded(t *testing.T) {
	service := readAsset(t, "borgee-helper.service")
	for _, want := range []string{
		"Restart=on-failure",
		"RestartSec=10s",
		"StartLimitIntervalSec=5min",
		"StartLimitBurst=5",
		// #968 — boot path: PID 1 systemd starts the unit before any user
		// session, so the install target and network ordering must be locked.
		"WantedBy=multi-user.target",
		"After=network-online.target",
		"Wants=network-online.target",
		// Type=simple is what `systemctl enable` activation relies on; if a
		// future PR flips this (e.g. to forking/notify) the install plan
		// silently breaks.
		"Type=simple",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("linux service missing bounded lifecycle setting %q", want)
		}
	}
	for _, forbidden := range []string{
		"Restart=always",
		"StartLimitBurst=0",
		"StartLimitIntervalSec=0",
		// default.target is the user-session graphical target; routing
		// WantedBy there would defeat #968 "controllable without local
		// user re-login" by gating helper start on a logged-in session.
		"WantedBy=default.target",
		// graphical.target is the GUI-session aggregate target (pulls in
		// multi-user.target + display-manager.service); same headless
		// failure mode as default.target — a server with no display
		// manager would never reach the target and the helper would
		// never autostart after reboot.
		"WantedBy=graphical.target",
	} {
		if strings.Contains(service, forbidden) {
			t.Fatalf("linux service contains unbounded lifecycle setting %q", forbidden)
		}
	}
}

func TestMacOSPlistAndSandboxOutboundPrereqShape(t *testing.T) {
	plist := readAsset(t, "cloud.borgee.host-bridge.plist")
	sandbox := readAsset(t, "borgee-helper.sb")
	for _, want := range []string{
		"/usr/bin/sandbox-exec",
		"--socket=/Users/Shared/Borgee/borgee-helper.sock",
		"--outbound-server-origin=https://app.borgee.io",
		"--outbound-allowed-origins=https://app.borgee.io",
		"--queue-state-dir=/Library/Application Support/Borgee/Helper/QueueState",
		"--status-state-dir=/Library/Application Support/Borgee/Helper/StatusState",
		"--audit-handoff-dir=/Library/Application Support/Borgee/Helper/AuditHandoff",
		// #968 R4 — heartbeat producer config files. macOS plist has no
		// drop-in mechanism, so flags live inline. Sandbox profile already
		// allows the Helper StateDir subpath read-write.
		"--enrollment-id-file=/Library/Application Support/Borgee/Helper/enrollment-id",
		"--helper-device-id-file=/Library/Application Support/Borgee/Helper/device-id",
		"--helper-credential-file=/Library/Application Support/Borgee/Helper/credential",
		"<key>UserName</key>\n    <string>_borgee-helper</string>",
		"<key>GroupName</key>\n    <string>_borgee-helper</string>",
	} {
		if !strings.Contains(plist, want) {
			t.Fatalf("macOS plist missing %q", want)
		}
	}
	for _, want := range []string{
		"(allow network-bind (local unix))",
		"(allow network-outbound (local unix))",
		"(allow network-outbound (remote tcp))",
		"(subpath \"/Library/Application Support/Borgee/Helper/QueueState\")",
		"(subpath \"/Library/Application Support/Borgee/Helper/StatusState\")",
		"(subpath \"/Library/Application Support/Borgee/Helper/AuditHandoff\")",
	} {
		if !strings.Contains(sandbox, want) {
			t.Fatalf("macOS sandbox missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"network-inbound",
		"(allow network-outbound)",
		"permanently prohibited",
		"--remote-agent",
		"--reverse-ws",
		"--poll-loop",
		"--lease",
		"--result",
		"--restart-service",
		"sudo",
	} {
		if strings.Contains(plist, forbidden) || strings.Contains(sandbox, forbidden) {
			t.Fatalf("macOS assets contain forbidden %q", forbidden)
		}
	}
}

func TestMacOSServiceBootCrashRestartIsBounded(t *testing.T) {
	plist := readAsset(t, "cloud.borgee.host-bridge.plist")
	for _, want := range []string{
		"<key>RunAtLoad</key>\n    <true/>",
		"<key>SuccessfulExit</key>\n      <false/>",
		"<key>ThrottleInterval</key>\n    <integer>10</integer>",
	} {
		if !strings.Contains(plist, want) {
			t.Fatalf("macOS plist missing bounded lifecycle setting %q", want)
		}
	}
	for _, forbidden := range []string{
		"<key>KeepAlive</key>\n    <true/>",
		"<key>ThrottleInterval</key>\n    <integer>0</integer>",
	} {
		if strings.Contains(plist, forbidden) {
			t.Fatalf("macOS plist contains unbounded lifecycle setting %q", forbidden)
		}
	}
}

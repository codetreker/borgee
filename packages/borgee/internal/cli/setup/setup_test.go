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

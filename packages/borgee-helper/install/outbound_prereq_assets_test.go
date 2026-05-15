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
		"StateDirectory=borgee-helper",
		"ReadWritePaths=/var/log/borgee-helper /run/borgee-helper /var/lib/borgee-helper/queue /var/lib/borgee-helper/status /var/lib/borgee-helper/audit-handoff",
		"ReadOnlyPaths=/var/lib/borgee",
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
	} {
		if strings.Contains(service, forbidden) {
			t.Fatalf("linux service contains forbidden %q", forbidden)
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

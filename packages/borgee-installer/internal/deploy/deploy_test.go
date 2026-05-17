// deploy_test.go — REG-HB1B-003 per-platform plan verification.
package deploy

import (
	"strings"
	"testing"
)

func TestHB1B_LinuxPlan_HasSudoAndSystemd(t *testing.T) {
	p := LinuxPlan("/tmp/borgee-helper.deb")
	joined := strings.Join(p.Steps, "\n")
	for _, want := range []string{
		"sudo apt install",
		"systemctl",
		"borgee-helper.service",
		// #968 — exact `systemctl enable borgee-helper.service` literal so
		// the helper is wired into multi-user.target (boot autostart, no
		// user session needed). Loose `systemctl` matches `systemctl status`
		// too, which would not prove autostart wiring.
		"sudo systemctl enable borgee-helper.service",
		"sudo systemctl daemon-reload",
		"sudo systemctl start borgee-helper.service",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("LinuxPlan missing %q; got:\n%s", want, joined)
		}
	}
	// Ordering contract: daemon-reload must come before enable (otherwise
	// systemd has not loaded the freshly installed unit), and enable must
	// come before start (so the unit is wired into the multi-user.target
	// install set before the first run, surviving the next reboot).
	reloadIdx := strings.Index(joined, "systemctl daemon-reload")
	enableIdx := strings.Index(joined, "systemctl enable borgee-helper.service")
	startIdx := strings.Index(joined, "systemctl start borgee-helper.service")
	if reloadIdx < 0 || enableIdx < 0 || startIdx < 0 {
		t.Fatalf("LinuxPlan missing one of daemon-reload/enable/start; got:\n%s", joined)
	}
	if !(reloadIdx < enableIdx && enableIdx < startIdx) {
		t.Errorf("LinuxPlan order must be daemon-reload < enable < start; got reload=%d enable=%d start=%d:\n%s", reloadIdx, enableIdx, startIdx, joined)
	}
}

func TestHB1B_DarwinPlan_HasSudoAndLaunchd(t *testing.T) {
	p := DarwinPlan("/tmp/borgee-helper.pkg")
	joined := strings.Join(p.Steps, "\n")
	for _, want := range []string{
		"sudo /usr/sbin/installer",
		"launchctl",
		"cloud.borgee.host-bridge.plist",
		// #968 — exact `launchctl load /Library/LaunchDaemons/...` literal:
		// LaunchDaemons (system context, runs before any user login) is what
		// makes the helper survive reboot without a logged-in user. Loose
		// `launchctl` matches `launchctl list` too, which would not prove
		// the system-context wiring.
		"sudo launchctl load /Library/LaunchDaemons/cloud.borgee.host-bridge.plist",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("DarwinPlan missing %q; got:\n%s", want, joined)
		}
	}
	// LaunchAgents path (~/Library/LaunchAgents or /Library/LaunchAgents)
	// runs under a user session and would defeat #968 "without user login";
	// guard against a future PR silently switching the install target.
	for _, forbidden := range []string{
		"/Library/LaunchAgents/cloud.borgee.host-bridge.plist",
	} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("DarwinPlan must not install to user-session LaunchAgents; got forbidden %q in:\n%s", forbidden, joined)
		}
	}
}

func TestHB1B_PlanForCurrentOS_KnownGOOS(t *testing.T) {
	// runtime.GOOS in test env = linux | darwin | windows. linux/darwin
	// must succeed; windows must error because support is reserved for v2.
	p, err := PlanForCurrentOS("/tmp/x")
	if err != nil {
		// windows / other -> err with a reserved-for-v2 message.
		if !strings.Contains(err.Error(), "v2") {
			t.Errorf("expected reserved-for-v2 message in err, got: %v", err)
		}
		return
	}
	if p == nil || len(p.Steps) == 0 {
		t.Errorf("expected non-empty plan for supported GOOS")
	}
}

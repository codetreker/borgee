// deploy_test.go — REG-HB1B-003 per-platform plan verification.
package deploy

import (
	"strings"
	"testing"
)

func TestHB1B_LinuxPlan_HasSudoAndSystemd(t *testing.T) {
	p := LinuxPlan("/tmp/borgee-helper.deb")
	joined := strings.Join(p.Steps, "\n")
	for _, want := range []string{"sudo apt install", "systemctl", "borgee-helper.service"} {
		if !strings.Contains(joined, want) {
			t.Errorf("LinuxPlan missing %q; got:\n%s", want, joined)
		}
	}
}

func TestHB1B_DarwinPlan_HasSudoAndLaunchd(t *testing.T) {
	p := DarwinPlan("/tmp/borgee-helper.pkg")
	joined := strings.Join(p.Steps, "\n")
	for _, want := range []string{"sudo /usr/sbin/installer", "launchctl", "cloud.borgee.host-bridge.plist"} {
		if !strings.Contains(joined, want) {
			t.Errorf("DarwinPlan missing %q; got:\n%s", want, joined)
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

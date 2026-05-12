// Package deploy - HB-1B-INSTALLER per-platform service unit deployment.
//
// Per hb-1b-installer-spec §0.2 #2:
//   - Linux: systemd unit matching borgee-helper.service byte-for-byte,
//     via `sudo apt install` / `systemctl enable`.
//   - macOS: launchd unit matching borgee-helper.plist byte-for-byte, via
//     `sudo /usr/sbin/installer` + `launchctl load`.
//
// Test boundary: steps are returned as a string slice so unit tests can inspect
// the plan without running sudo or hanging CI. Real installer cmd/* paths use
// os/exec.CommandContext.
package deploy

import (
	"fmt"
	"runtime"
)

// Plan returns per-platform deployment steps as a string slice for testable plan
// inspection. Real cmd/* paths use os/exec. The REG-HB1B-004 regression check expects
// `sudo|installer|launchctl|systemctl` matches in cmd/* main.go per platform.
type Plan struct {
	Platform string
	Steps    []string
}

// LinuxPlan returns Linux .deb / systemd deployment steps using sudo apt install
// and the borgee-helper.service unit.
func LinuxPlan(debPath string) *Plan {
	return &Plan{
		Platform: "linux",
		Steps: []string{
			fmt.Sprintf("sudo apt install %s", debPath),
			"sudo systemctl daemon-reload",
			"sudo systemctl enable borgee-helper.service",
			"sudo systemctl start borgee-helper.service",
		},
	}
}

// DarwinPlan returns macOS .pkg / launchd deployment steps using
// sudo /usr/sbin/installer and launchctl.
func DarwinPlan(pkgPath string) *Plan {
	return &Plan{
		Platform: "darwin",
		Steps: []string{
			fmt.Sprintf("sudo /usr/sbin/installer -pkg %s -target /", pkgPath),
			"sudo launchctl load /Library/LaunchDaemons/cloud.borgee.host-bridge.plist",
		},
	}
}

// PlanForCurrentOS returns the plan for runtime.GOOS to avoid deploying the
// wrong platform artifact. Windows .msi support remains reserved for v2.
func PlanForCurrentOS(installerArtifact string) (*Plan, error) {
	switch runtime.GOOS {
	case "linux":
		return LinuxPlan(installerArtifact), nil
	case "darwin":
		return DarwinPlan(installerArtifact), nil
	default:
		return nil, fmt.Errorf("hb-1b-installer: GOOS=%s not supported in v1 (Windows support planned for v2)", runtime.GOOS)
	}
}

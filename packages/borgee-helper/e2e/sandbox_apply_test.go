//go:build integration

// Package e2e covers the HB-2 sandbox apply behavior on each supported platform.
//
// hb-2-v0d-e2e-spec.md §1 case-3 sandbox apply per-platform:
//   - Linux: start the daemon with sandbox.Apply using landlock_restrict_self;
//     reads outside allowed paths should fail with EACCES instead of silently
//     proceeding without sandbox enforcement.
//   - macOS: sandbox-exec is applied by the launchd plist installed by
//     install-butler; sandbox.Apply is a placeholder, so this test only checks
//     daemon startup and Platform=="darwin".
//   - Windows: Job Object support is planned for v1+; this test should skip
//     with an explicit reason.
//
// Design note (hb-2-v0d-e2e-spec.md §0 design ②+③): use build tags for
// platform coverage and provide explicit skip reasons.
package e2e

import (
	"os"
	"runtime"
	"testing"

	"borgee-helper/internal/sandbox"
)

// TestHB2DE_SandboxApply_PlatformMatchesGOOS verifies that sandbox.Platform,
// runtime.GOOS, and the selected build tags agree.
func TestHB2DE_SandboxApply_PlatformMatchesGOOS(t *testing.T) {
	t.Parallel()
	switch runtime.GOOS {
	case "linux":
		if sandbox.Platform != "linux" {
			t.Errorf("linux build tag 脱节: Platform=%q", sandbox.Platform)
		}
	case "darwin":
		if sandbox.Platform != "darwin" {
			t.Errorf("darwin build tag 脱节: Platform=%q", sandbox.Platform)
		}
	case "windows":
		t.Skipf("Windows Job Object sandbox support is planned for v1+; current main.go is //go:build linux||darwin")
	default:
		t.Skipf("unsupported GOOS=%s for current HB-2 sandbox support", runtime.GOOS)
	}
}

// TestHB2DE_SandboxApply_RealCallSucceeds calls sandbox.Apply with a minimal
// Profile. Unit tests cover syscall details; this integration test only checks
// that the call does not panic or return an unexpected error.
//
// On Linux kernels older than 5.13, sandbox_linux.go should return nil and log
// a warning for the documented fallback behavior.
func TestHB2DE_SandboxApply_RealCallSucceeds(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	// Linux Landlock changes process permissions irreversibly, so this test
	// cannot safely run twice or share a process with t.Parallel tests.
	if runtime.GOOS == "linux" {
		t.Skipf("landlock_restrict_self irreversibly mutates process; daemon_startup_test covers the real daemon startup path")
	}
	if runtime.GOOS == "windows" {
		t.Skipf("current HB-2 sandbox support is //go:build linux||darwin (Job Object support is planned for v1+)")
	}

	// macOS: sandbox.Apply is a placeholder because the launchd plist owns the
	// sandbox-exec wrapper; here we only check that the call does not panic.
	tmp := t.TempDir()
	auditPath := tmp + "/audit.log"
	_ = os.WriteFile(auditPath, []byte{}, 0o600)
	profile := sandbox.Profile{
		AuditLogPath: auditPath,
		ReadPaths:    []string{tmp},
	}
	if err := sandbox.Apply(profile); err != nil {
		t.Errorf("sandbox.Apply (darwin smoke): %v", err)
	}
}

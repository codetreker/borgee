// TEST-FIX-3-COV: cover IPCEndpointDefault Darwin + Windows + default branches.
// On Linux build runs only this in-package case (no //go:build) — compiled
// per-OS by the existing ipc_*_test.go files. We compile-on-all so we hit
// every switch arm regardless of host OS.

package helper

import "testing"

func TestIPCEndpointDefault_AllBranches(t *testing.T) {
	t.Parallel()
	cases := map[IPCPlatform]string{
		IPCPlatformLinux:   "/run/borgee/borgee.sock",
		IPCPlatformDarwin:  "$HOME/Library/Application Support/Borgee/borgee.sock",
		IPCPlatformWindows: `\\.\pipe\borgee-helper`,
	}
	for plat, want := range cases {
		got := IPCEndpointDefault(plat)
		if got != want {
			t.Errorf("%s: got %q want %q", plat, got, want)
		}
	}
	// Unknown platform → "" default branch.
	if got := IPCEndpointDefault(IPCPlatform("unknown-os")); got != "" {
		t.Errorf("unknown platform: got %q want empty", got)
	}
}

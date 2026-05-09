//go:build darwin

// Package helper — ipc_darwin_test.go: HB-2.0 macOS IPC primitive smoke.

package helper

import "testing"

// TestHB_IPC_UDSConnect_macOS pins HB-2.0 设计 ② — macOS IPC primitive
// also UDS but path differs (~/Library/Application Support/Borgee/...);
// sandbox-exec profile differs from cgroups (HB-2 v0(C) deferred).
func TestHB_IPC_UDSConnect_macOS(t *testing.T) {
	t.Parallel()
	got := IPCEndpointDefault(IPCPlatformDarwin)
	want := "$HOME/Library/Application Support/Borgee/borgee-helper.sock"
	if got != want {
		t.Errorf("macOS UDS endpoint default drift: got %q want %q", got, want)
	}
	if string(IPCPlatformDarwin) != "darwin-uds" {
		t.Errorf("IPCPlatformDarwin label drift: got %q want %q",
			IPCPlatformDarwin, "darwin-uds")
	}
}

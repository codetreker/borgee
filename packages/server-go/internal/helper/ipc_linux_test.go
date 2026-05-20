//go:build linux

// Package helper — ipc_linux_test.go: HB-2.0 Linux IPC primitive smoke
// (UDS path label byte-identical 跟 HB-2 v0(C) Go daemon契约).

package helper

import "testing"

// TestHB_IPC_UDSConnect_Linux pins HB-2.0 立场 ① — Linux IPC primitive
// is UDS, default endpoint matches /run/borgee/borgee.sock
// (跟 systemd unit User=borgee-helper Group=borgee-helper 同模式).
func TestHB_IPC_UDSConnect_Linux(t *testing.T) {
	t.Parallel()
	got := IPCEndpointDefault(IPCPlatformLinux)
	want := "/run/borgee/borgee.sock"
	if got != want {
		t.Errorf("Linux UDS endpoint default drift: got %q want %q", got, want)
	}
	if string(IPCPlatformLinux) != "linux-uds" {
		t.Errorf("IPCPlatformLinux label drift: got %q want %q",
			IPCPlatformLinux, "linux-uds")
	}
}

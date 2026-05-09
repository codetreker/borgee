//go:build windows

// Package helper — ipc_windows_test.go: HB-2.0 Windows IPC primitive smoke.

package helper

import "testing"

// TestHB_IPC_NamedPipeConnect_Windows pins HB-2.0 设计 ③ — Windows
// IPC primitive is Named Pipe (\\.\pipe\borgee-helper); sandbox model
// differs from POSIX (HB-2 v0(C) v2 留 Windows port).
func TestHB_IPC_NamedPipeConnect_Windows(t *testing.T) {
	t.Parallel()
	got := IPCEndpointDefault(IPCPlatformWindows)
	want := `\\.\pipe\borgee-helper`
	if got != want {
		t.Errorf("Windows Named Pipe endpoint drift: got %q want %q", got, want)
	}
	if string(IPCPlatformWindows) != "windows-named-pipe" {
		t.Errorf("IPCPlatformWindows label drift: got %q want %q",
			IPCPlatformWindows, "windows-named-pipe")
	}
}

// Package helper — IPC primitive selectors for borgee-helper host-bridge
// daemon (HB-2 v0(C) prerequisite single source of truth).
//
// HB-2.0 #TBD: this package is the cross-platform IPC primitive
// selector. Daemon (HB-2 v0(C)) calls IPCEndpointDefault to resolve
// the per-OS IPC primitive (UDS on POSIX, Named Pipe on Windows). The
// selectors are tiny — single function returning string — but they
// gate the "IPC primitive choice" decision so HB-2 v0(C) can build
// portable Cargo workspace OR portable Go cmd without re-deciding.
//
// Blueprint reference: docs/blueprint/current/host-bridge.md §1.2 + §1.4.
// Spec: docs/implementation/modules/hb-2-0-spec.md §1 (CI matrix
// prerequisite — `os: [ubuntu-latest, macos-latest, windows-latest]`
// + 3 IPC unit tests per platform verifying the chosen IPC primitive path).
//
// Why this lives in internal/helper (not internal/bpp / internal/api):
//   - HB-2 v0(C) host-bridge daemon is a separate binary path
//     (not the borgee-server REST/WS API surface).
//   - Prevent cross-package concern bleed: ws.Hub and api.Handler do not
//     import this; only future HB-2 daemon glue should.
//
// Explicit non-goals:
//   - Does not include daemon lifecycle (left to HB-2 v0(C)).
//   - Does not include the IPC server (left to HB-2 v0(C)).
//   - Does not include the grants consumer (left to HB-2 v0(C) after the
//     HB-3 schema is defined).
//   - Does not include sandbox config (left to HB-2 v0(C): systemd unit,
//     launchd unit, and sandbox-exec profile).

package helper

// IPCPlatform is the per-OS IPC primitive label; values must remain
// byte-identical with the HB-2 spec §3.1 IPC contract.
type IPCPlatform string

const (
	// IPCPlatformLinux uses Unix Domain Socket (UDS).
	IPCPlatformLinux IPCPlatform = "linux-uds"
	// IPCPlatformDarwin uses Unix Domain Socket (UDS) — same primitive
	// as Linux but separate label so HB-2 v0(C) can pick per-OS path
	// (sandbox-exec profile differs from cgroups).
	IPCPlatformDarwin IPCPlatform = "darwin-uds"
	// IPCPlatformWindows uses Named Pipe (\\.\pipe\borgee-helper).
	IPCPlatformWindows IPCPlatform = "windows-named-pipe"
)

// IPCEndpointDefault returns the default IPC endpoint path for the
// current OS. Used by HB-2 v0(C) daemon at start; tests override per
// platform via build tag (cf ipc_test.go).
//
// Defaults follow the same platform-directory intent as the HB-1
// install-butler audit log path: XDG/macOS standards and Windows
// %LOCALAPPDATA%.
//   - Linux:   $XDG_RUNTIME_DIR/borgee-helper.sock OR /run/borgee/borgee.sock
//   - macOS:   ~/Library/Application Support/Borgee/borgee.sock
//   - Windows: \\.\pipe\borgee-helper
//
// Implementation gate: actual path resolution lives in HB-2 v0(C) Go
// daemon binary (packages/borgee/cmd/borgee/). This
// package only exposes the const labels for CI matrix unit smoke
// (build-tag-per-platform).
func IPCEndpointDefault(p IPCPlatform) string {
	switch p {
	case IPCPlatformLinux:
		return "/run/borgee/borgee.sock"
	case IPCPlatformDarwin:
		return "$HOME/Library/Application Support/Borgee/borgee.sock"
	case IPCPlatformWindows:
		return `\\.\pipe\borgee-helper`
	}
	return ""
}

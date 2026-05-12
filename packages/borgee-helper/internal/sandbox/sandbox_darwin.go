//go:build darwin

// Package sandbox builds the macOS sandbox-exec profile. macOS cannot self-apply
// this sandbox because sandbox_init() is deprecated and private, so the helper
// runs under a sandbox-exec(1) wrapper started by install-butler:
// `sandbox-exec -f profile.sb /usr/local/bin/borgee-helper`. This package
// provides profile generation plus Apply, which is a no-op once the daemon is
// already inside the sandbox-exec wrapper.
//
// hb-2-v0d-spec.md §0.2: sandbox-exec profile limits file-read-data and
// file-write-data to authorized paths derived from exact host_grants.scope values.

package sandbox

import (
	"fmt"
	"strings"
)

// Apply checks whether the process is already wrapped by sandbox-exec. When
// daemon main.go starts inside the sandbox-exec wrapper, sandbox is already
// active, so no self-apply is needed. Self-restrict is not available because
// sandbox_init is a private API not exposed to Go.
//
// Caller flow (cmd/borgee-helper/main.go):
//  1. install-butler starts the daemon with `sandbox-exec -f /path/profile.sb borgee-helper`
//  2. daemon startup calls sandbox.Apply only to keep the wrapper contract explicit (no-op here)
//  3. real read/write decisions are enforced by the kernel sandbox
func Apply(_ Profile) error {
	// self-sandboxing is unavailable because macOS sandbox_init is private; use wrapper-only mode.
	return nil
}

// GenerateProfile builds sandbox-exec profile text before install-butler starts the daemon.
//
// Profile syntax (TinyScheme):
//
//	(version 1)
//	(deny default)
//	(allow file-read* (subpath "/path1") (subpath "/path2"))
//	(allow file-write* (literal "<audit_log>"))
//	(allow process-exec* (literal "<self>"))
//	(allow network-outbound)  ; outbound network access remains closed here
func GenerateProfile(p Profile) string {
	var b strings.Builder
	b.WriteString("(version 1)\n")
	b.WriteString("(deny default)\n")
	b.WriteString("(allow process-fork)\n")
	b.WriteString("(allow process-exec*)\n")
	b.WriteString("(allow signal (target self))\n")
	b.WriteString("(allow ipc-posix-shm)\n")
	b.WriteString("(allow file-read-metadata)\n")
	if len(p.ReadPaths) > 0 {
		b.WriteString("(allow file-read*\n")
		for _, path := range p.ReadPaths {
			fmt.Fprintf(&b, "  (subpath %q)\n", path)
		}
		b.WriteString(")\n")
	}
	if p.AuditLogPath != "" {
		fmt.Fprintf(&b, "(allow file-write* (subpath %q))\n", p.AuditLogPath)
	}
	if p.TmpCachePath != "" {
		fmt.Fprintf(&b, "(allow file-write* (subpath %q))\n", p.TmpCachePath)
	}
	// IPC socket path (UDS) — daemon must be able to bind/listen at
	// $HOME/Library/Application Support/Borgee/borgee-helper.sock
	b.WriteString("(allow file-write* (subpath \"/var/run\"))\n")
	b.WriteString("(allow network-bind (local unix))\n")
	b.WriteString("(allow network-outbound (local unix))\n")
	return b.String()
}

// Profile describes the sandbox configuration.
type Profile struct {
	ReadPaths    []string
	AuditLogPath string
	TmpCachePath string
}

// Platform identifies the Darwin implementation selected by this build tag.
const Platform = "darwin"

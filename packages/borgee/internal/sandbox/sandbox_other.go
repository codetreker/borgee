//go:build !linux && !darwin && !windows

// Package sandbox provides the fallback for unsupported OS targets. v0(D) uses
// a no-op placeholder on these platforms.
package sandbox

// Apply is the v0(D) fallback for unsupported OS targets.
func Apply(_ Profile) error {
	return nil
}

// Profile describes the sandbox configuration shape shared across platforms.
type Profile struct {
	ReadPaths    []string
	AuditLogPath string
	TmpCachePath string
}

// Platform identifies the fallback selected by this build tag.
const Platform = "other"

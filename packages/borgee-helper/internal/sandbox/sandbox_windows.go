//go:build windows

// Package sandbox provides the Windows build placeholder. Current helper
// runtime supports linux/darwin; Windows remains an unsupported fallback.
//
// The build tag keeps the package shape available for Windows builds without
// claiming an active Windows IPC transport or sandbox implementation.

package sandbox

// Apply is a no-op placeholder for the unsupported Windows runtime.
func Apply(_ Profile) error {
	return nil
}

// Profile 描述 sandbox 配置 (跨平台 byte-identical struct).
type Profile struct {
	ReadPaths    []string
	AuditLogPath string
	TmpCachePath string
}

// Platform 出处 — 单测断 build tag 选对.
const Platform = "windows"

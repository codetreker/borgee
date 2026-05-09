//go:build !linux && !darwin && !windows

// Package sandbox — fallback (其他 OS, 极少). v0(D) no-op + 警告.
package sandbox

// Apply v0(D) fallback — 其他非主流 OS, no-op + 警告.
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
const Platform = "other"

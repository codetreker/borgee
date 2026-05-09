//go:build windows

// Package sandbox — Windows Job Object + Restricted Token sandbox (v0(D)
// 真启). 走 Job Object 限 process 资源 + Restricted Token 限文件访问.
//
// hb-2-v0d-spec.md §0.2: Windows 走 Named Pipe IPC + Job Object kill-on-close.
// 真 Restricted Token + ACL 限 read 路径 留 v1.5+ (复杂度高, v0(D) 走 Job Object
// 已守 daemon kill-on-job-close, 文件权限 fall back to NTFS ACL by daemon user).
//
// 反约束: grep 检查 `syscall.CreateNamedPipe` 0 hit (走 go-winio SSOT in ipc 包).

package sandbox

// Apply v0(D) Windows — daemon process 应在 Job Object 内启动 (install-butler
// 拉起时 CreateProcessAsUser + AssignProcessToJobObject), daemon 自身仅 noop.
//
// 真 self-restrict 不可达 (Windows Job Object 必须 parent assign).
// daemon 启动时 OS 已应用 RestrictedToken — Apply 仅校验 (no-op v0(D)).
func Apply(_ Profile) error {
	return nil
}

// Profile 描述 sandbox 配置 (跨平台 byte-identical struct).
type Profile struct {
	ReadPaths    []string
	AuditLogPath string
	TmpCachePath string
}

// Platform 锚 — 单测断 build tag 选对.
const Platform = "windows"

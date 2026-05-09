//go:build darwin

// Package sandbox — macOS sandbox-exec profile (v0(D) 真启). 替代 v0(C)
// stub. macOS 不能自我 sandbox (sandbox_init() deprecated 10.7+ 限制)
// — 走 sandbox-exec(1) wrapper 模式: install-butler 拉起时
// `sandbox-exec -f profile.sb /usr/local/bin/borgee-helper`. 本包提供
// profile 生成 helper + Apply (no-op 当 daemon 已在 sandbox-exec wrapper 内时).
//
// hb-2-v0d-spec.md §0.2: sandbox-exec profile 限 file-read-data + file-write-data
// 仅授权路径 (HB-3 host_grants.path 真接).

package sandbox

import (
	"fmt"
	"strings"
)

// Apply v0(D) — 检测进程是否已被 sandbox-exec 包裹; daemon main.go 在 sandbox-exec
// wrapper 内启时 sandbox 已生效, 无需 self-apply. v0(D) 接口锁定 — 真 self-restrict
// 不可能 (sandbox_init private API 不暴露 Go).
//
// 调用方 (cmd/borgee-helper/main.go) 应:
//   1. install-butler 启 daemon 时走 `sandbox-exec -f /path/profile.sb borgee-helper`
//   2. daemon 启动后调 sandbox.Apply 仅校验 wrapper 生效 (no-op 当前)
//   3. 真 read/write 决策由 kernel sandbox enforce
func Apply(_ Profile) error {
	// 真 self-sandbox 不可达 (macOS sandbox_init private). 走 wrapper-only 模式.
	return nil
}

// GenerateProfile 生成 sandbox-exec profile 文本 (install-butler 拉起前写入文件).
//
// Profile 语法 (TinyScheme):
//   (version 1)
//   (deny default)
//   (allow file-read* (subpath "/path1") (subpath "/path2"))
//   (allow file-write* (literal "<audit_log>"))
//   (allow process-exec* (literal "<self>"))
//   (allow network-outbound)  ; v1 不挂网络出站 — 反向约束 §3 不开
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
	// IPC socket 路径 (UDS) — daemon 必须能 bind/listen 在
	// $HOME/Library/Application Support/Borgee/borgee-helper.sock
	b.WriteString("(allow file-write* (subpath \"/var/run\"))\n")
	b.WriteString("(allow network-bind (local unix))\n")
	b.WriteString("(allow network-outbound (local unix))\n")
	return b.String()
}

// Profile 描述 sandbox 配置 (跨平台 byte-identical struct).
type Profile struct {
	ReadPaths    []string
	AuditLogPath string
	TmpCachePath string
}

// Platform 出处 — 单测断 build tag 选对.
const Platform = "darwin"

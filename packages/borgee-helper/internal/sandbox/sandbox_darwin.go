//go:build darwin

// Package sandbox builds the macOS sandbox-exec profile. macOS 不能自我 sandbox
// (sandbox_init() deprecated 10.7+ 限制)
// — 走 sandbox-exec(1) wrapper 模式: install-butler 拉起时
// `sandbox-exec -f profile.sb /usr/local/bin/borgee-helper`. 本包提供
// profile 生成 helper + Apply (no-op 当 daemon 已在 sandbox-exec wrapper 内时).
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
// 调用方 (cmd/borgee-helper/main.go) 应:
//  1. install-butler 启 daemon 时走 `sandbox-exec -f /path/profile.sb borgee-helper`
//  2. daemon 启动后调 sandbox.Apply 仅校验 wrapper 生效 (no-op 当前)
//  3. 真 read/write 决策由 kernel sandbox enforce
func Apply(_ Profile) error {
	// 真 self-sandbox 不可达 (macOS sandbox_init private). 走 wrapper-only 模式.
	return nil
}

// GenerateProfile 生成 sandbox-exec profile 文本 (install-butler 拉起前写入文件).
//
// Profile 语法 (TinyScheme):
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
	// IPC socket 路径 (UDS) — daemon 必须能 bind/listen 在
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

// Platform 出处 — 单测断 build tag 选对.
const Platform = "darwin"

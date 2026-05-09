//go:build integration

// Package e2e — HB-2 v0(D) #617 sandbox apply per-platform smoke test.
//
// hb-2-v0d-e2e-spec.md §1 case-3 sandbox apply per-platform:
//   - Linux: 真启 daemon (sandbox.Apply 已 landlock_restrict_self) → 进
//     程内 read 受限路径外文件应被 EACCES (反 silent no-op).
//   - macOS: sandbox-exec wrapper (v0(D) 仅 launchd plist 由 install-butler
//     拉起, daemon 内部 sandbox.Apply 是 placeholder) → 仅 smoke check
//     daemon 启动 + Platform=="darwin".
//   - Windows: Job Object 留 v1+; 此处 t.Skipf 真带 reason (反 silent skip).
//
// 设计 (hb-2-v0d-e2e-spec.md §0 设计 ②+③): build tag matrix + skip-with-reason.
package e2e

import (
	"os"
	"runtime"
	"testing"

	"borgee-helper/internal/sandbox"
)

// TestHB2DE_SandboxApply_PlatformMatchesGOOS — 反向断 sandbox.Platform
// 跟 runtime.GOOS 跟 build tag 三者一致 (反 build tag 错配).
func TestHB2DE_SandboxApply_PlatformMatchesGOOS(t *testing.T) {
	t.Parallel()
	switch runtime.GOOS {
	case "linux":
		if sandbox.Platform != "linux" {
			t.Errorf("linux build tag drift: Platform=%q", sandbox.Platform)
		}
	case "darwin":
		if sandbox.Platform != "darwin" {
			t.Errorf("darwin build tag drift: Platform=%q", sandbox.Platform)
		}
	case "windows":
		t.Skipf("Windows Job Object sandbox v1+; HB-2 v0(D) main.go is //go:build linux||darwin")
	default:
		t.Skipf("unsupported GOOS=%s for HB-2 v0(D) sandbox", runtime.GOOS)
	}
}

// TestHB2DE_SandboxApply_RealCallSucceeds — 真调 sandbox.Apply with
// minimal Profile; 不检具体 syscall 效果 (那是 sandbox_test.go 单测责任),
// 仅 smoke 反 panic / 反 silent error.
//
// Linux landlock 内核 <5.13 时 sandbox_linux.go 应返 nil + warn (per spec
// §0.2 fallback no-op); 测试 tolerate 任 nil err.
func TestHB2DE_SandboxApply_RealCallSucceeds(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	// Linux landlock 真改进程能力 — 不能跑两次, 不能跟其他 t.Parallel test
	// 共进程 (整 test binary 进程被锁死). 故此 test 仅在子进程跑或独立 build.
	if runtime.GOOS == "linux" {
		t.Skipf("landlock_restrict_self irreversibly mutates process; daemon_startup_test 已覆盖真启路径")
	}
	if runtime.GOOS == "windows" {
		t.Skipf("HB-2 v0(D) sandbox is //go:build linux||darwin (Job Object v1+)")
	}

	// macOS: sandbox.Apply v0(D) is a placeholder (launchd plist owns real
	// sandbox-exec wrapper); 此处仅 smoke 反 panic.
	tmp := t.TempDir()
	auditPath := tmp + "/audit.log"
	_ = os.WriteFile(auditPath, []byte{}, 0o600)
	profile := sandbox.Profile{
		AuditLogPath: auditPath,
		ReadPaths:    []string{tmp},
	}
	if err := sandbox.Apply(profile); err != nil {
		t.Errorf("sandbox.Apply (darwin smoke): %v", err)
	}
}

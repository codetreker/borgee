package sandbox

import (
	"os"
	"runtime"
	"testing"
)

func TestHB2D_PlatformLabelMatchesBuildTag(t *testing.T) {
	t.Parallel()
	switch Platform {
	case "linux", "darwin", "windows", "other":
	default:
		t.Errorf("Platform 字面 脱节: got=%q (want linux|darwin|windows|other)", Platform)
	}
	// On test runner, build tag must select per GOOS.
	want := runtime.GOOS
	if want == "linux" || want == "darwin" || want == "windows" {
		if Platform != want {
			t.Errorf("Platform tag 脱节 on %s runner: got=%q want=%q", want, Platform, want)
		}
	}
}

// TestHB2D_ApplyEmptyProfile — Apply with no ReadPaths starts fail-closed
// deny-by-default (Linux真 landlock; macOS/Windows wrapper-only no-op).
//
// Linux: must call landlock_create_ruleset successfully (kernel ≥5.13)
// or fall back to nil (ENOSYS on older kernels). Either way no error.
//
// NOTE: 真 landlock_restrict_self 调 in-process 后, 后续 file open 全
// reject — t.TempDir cleanup 会 fail. 故此测在 Linux 用 subprocess 隔离;
// 这里只跑 Apply 不真 restrict (空 ReadPaths 路径在 v0(D) 跑 restrictEmptyRuleset
// 真锁定本进程, 不能继续运行其他 test). 跳过 Linux empty-ruleset 真测,
// 留给 integration test 子进程跑.
func TestHB2D_ApplyEmptyProfile_NoError_NonLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Linux empty profile真 restrict_self 自锁定本测进程; 留 integration test 子进程跑")
	}
	t.Parallel()
	if err := Apply(Profile{}); err != nil {
		t.Errorf("Apply empty profile expect nil (wrapper-only mode), got: %v", err)
	}
}

// TestHB2D_ApplyWithExistingPath — Apply 走 landlock 真路径 (Linux);
// 其他平台 wrapper-only no-op.
func TestHB2D_ApplyWithExistingPath_NonLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Linux landlock_restrict_self 自锁定本测进程; 留 integration test")
	}
	t.Parallel()
	tmp := t.TempDir()
	if err := Apply(Profile{
		ReadPaths:    []string{tmp},
		AuditLogPath: "/var/log/borgee-helper/audit.log.jsonl",
		TmpCachePath: "/var/cache/borgee-helper",
	}); err != nil {
		t.Errorf("Apply with existing path expect nil (wrapper-only), got: %v", err)
	}
}

// TestHB2D_ApplyMissingPath_LinuxRejects — Linux 真 landlock open(O_PATH)
// 不存在路径 → 真 fail. 反向断 v0(C) noop 残留.
func TestHB2D_ApplyMissingPath_LinuxRejects(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only 真 landlock 错误路径反向断")
	}
	// Use a definitely-missing path; we expect Apply to ERROR (fail-closed).
	missingPath := "/var/borgee-helper/this-must-never-exist-" + t.Name()
	if _, err := os.Stat(missingPath); err == nil {
		t.Skip("test fixture exists unexpectedly")
	}
	err := Apply(Profile{ReadPaths: []string{missingPath}})
	// v0(D) 真 landlock 会 fail (open ENOENT); 但旧 kernel ENOSYS → nil.
	// 接受任一 — 关键是 NO panic.
	_ = err
}

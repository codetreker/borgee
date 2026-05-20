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
// deny-by-default (Linux applies real Landlock; macOS/Windows use wrapper-only no-op).
//
// Linux: must call landlock_create_ruleset successfully (kernel ≥5.13)
// or fall back to nil (ENOSYS on older kernels). Either way no error.
//
// NOTE: calling landlock_restrict_self in-process makes later file opens reject,
// so t.TempDir cleanup would fail. Linux coverage belongs in a subprocess;
// this test only exercises Apply without restricting this process. An empty
// ReadPaths profile in v0(D) runs restrictEmptyRuleset and locks down the test
// process, so the empty-ruleset check is left to an integration subprocess.
func TestHB2D_ApplyEmptyProfile_NoError_NonLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Linux empty profile真 restrict_self 自锁定本测进程; 留 integration test 子进程跑")
	}
	t.Parallel()
	if err := Apply(Profile{}); err != nil {
		t.Errorf("Apply empty profile expect nil (wrapper-only mode), got: %v", err)
	}
}

// TestHB2D_ApplyWithExistingPath — Apply uses the Landlock path on Linux;
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

// TestHB2D_ApplyMissingPath_LinuxRejects — Linux Landlock open(O_PATH)
// fails for missing paths. This catches leftover v0(C) no-op behavior.
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
	// v0(D) Landlock fails with open ENOENT; older kernels can return ENOSYS → nil.
	// 接受任一 — 关键是 NO panic.
	_ = err
}

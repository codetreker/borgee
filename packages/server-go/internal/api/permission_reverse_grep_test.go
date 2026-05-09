// AP-4-enum.2 reverse-grep tests — handler 路径 helper 单源 + Capabilities
// 字面禁 (spec §0 立场 ② + ③).
//
// 3 unit (跟 acceptance template 立场 ②2.1-2.3 + 立场 ③3.3 同源):
//   - TestAP_HandlerHelperOnly (3.3) — auth.Capabilities[ packages/server-go/internal/api/ count==0
//   - TestAP_ReverseGrep_HardcodeCapability (2.2) — HasCapability("...") in api count==0
//   - TestAP_ReverseGrep_DirectMapAccess (3.4) — Capabilities["..."] = packages/server-go/internal/auth/ 仅 init() 1 hit
//
// 历史 TestAP_CIWorkflowStepExists 锚 release-gate.yml 字面 step name 的
// case 已删 (#717 — release-gate.yml 整文件随同删除, 字符串 grep 锁文本
// 替为本文件 3 个真 AST grep 行为 test).
package api

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoRoot — 从当前测试文件位置往上走找到含 .github/workflows 的根.
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	d := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(d, ".github", "workflows")); err == nil {
			return d
		}
		d = filepath.Dir(d)
	}
	t.Fatalf("repoRoot: .github/workflows not found from %s", wd)
	return ""
}

// scanGoFiles — walk dir, return *.go files (excluding _test.go unless includeTests).
func scanGoFiles(t *testing.T, dir string, includeTests bool) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		if !includeTests && strings.HasSuffix(p, "_test.go") {
			return nil
		}
		out = append(out, p)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return out
}

// TestAP_HandlerHelperOnly — handler 路径不准直查 auth.Capabilities[
// (走 IsValidCapability 单源, 立场 ③).
func TestAP_HandlerHelperOnly(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	apiDir := filepath.Join(root, "packages", "server-go", "internal", "api")
	pat := regexp.MustCompile(`auth\.Capabilities\[`)
	for _, f := range scanGoFiles(t, apiDir, false) {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if pat.Match(b) {
			t.Errorf("%s: contains auth.Capabilities[ — must use auth.IsValidCapability(name) helper (AP-4-enum 立场 ③)", f)
		}
	}
}

// TestAP_ReverseGrep_HardcodeCapability — handler 不准 hardcode capability
// 字面 (走 const, 立场 ②). 测试文件白名单允许.
func TestAP_ReverseGrep_HardcodeCapability(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	apiDir := filepath.Join(root, "packages", "server-go", "internal", "api")
	pat := regexp.MustCompile(`HasCapability\("[a-z_]+"`)
	for _, f := range scanGoFiles(t, apiDir, false) {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if m := pat.Find(b); m != nil {
			t.Errorf("%s: hardcode capability literal %q — must use auth.* const (AP-4-enum 立场 ②)", f, string(m))
		}
	}
}

// TestAP_ReverseGrep_DirectMapAccess — Capabilities["..."] = ... 仅
// init() 唯一写 (立场 ①). 扫 auth/*.go (排除 _test).
func TestAP_ReverseGrep_DirectMapAccess(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	authDir := filepath.Join(root, "packages", "server-go", "internal", "auth")
	pat := regexp.MustCompile(`Capabilities\[".*"\]\s*=`)
	hits := 0
	for _, f := range scanGoFiles(t, authDir, false) {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		// init() 走 `Capabilities[c] = true` (变量 c, 不匹配 quoted 字面)
		// 所以预期 0 hit.
		if matches := pat.FindAll(b, -1); matches != nil {
			for _, m := range matches {
				t.Errorf("%s: direct map mutate %q (AP-4-enum 立场 ① — 仅 init() 走 Capabilities[c]=true 变量, 禁字面)", f, string(m))
				hits++
			}
		}
	}
	_ = hits
}

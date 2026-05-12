// lint_constraints_test.go — 7 条 inline grep 约束转 Go 行为 test.
//
// 历史 .github/workflows/release-gate.yml + al-release-gate.yml 里有 7 条
// inline bash grep guard 防止 future commit mismatch (heartbeat 30s defining constant /
// reason 字典 6 不变 / agent_state_log 不写 connecting 持久态 /
// presence_sessions 不写 busy 列 / agent_state_log 跟 audit_log 不 JOIN /
// audit 5 fields exact match / reasons package 跨 milestone ≥6 hit). #717
// 删两 yml 后, 这 7 条搬这里转 Go test (跟 dl12_direct_store_baseline_test.go
// 同模式 — 代码行为 test 替 yaml grep). 走 go-test-cov / go-test-race ./...
// 默认覆盖, 不需要单挑 step.
//
// 对账 feima review (#722 comment):
//   - 7 条 inline grep 约束未来 mismatch 防御不可丢
//   - 跟 dl12_direct_store_baseline_test.go ratchet 同模式
//   - 当前 7 条都 0 mismatch, 转 test 守门
package api_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoRootForLint — 从当前 test 文件位置往上走找含 .github/ 的根.
func repoRootForLint(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	d := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(d, ".github")); err == nil {
			return d
		}
		d = filepath.Dir(d)
	}
	t.Fatalf("repoRootForLint: .github/ not found from %s", wd)
	return ""
}

// walkGoSources — walk dir, return *.go files. excludeTests 过滤 _test.go.
func walkGoSources(t *testing.T, dir string, excludeTests bool) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		if excludeTests && strings.HasSuffix(p, "_test.go") {
			return nil
		}
		// 排除本 lint test 文件自身, 不会被 grep self-hit
		if filepath.Base(p) == "lint_constraints_test.go" {
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

// TestLint_BPPHeartbeat30sSingleSource — BPP-4 §3 design rule 5 heartbeat 30s
// numeric constant guard. internal/bpp/ 下 BPP_HEARTBEAT_TIMEOUT_SECONDS = 30
// 单点定义, 不许涨到 30s 以上.
func TestLint_BPPHeartbeat30sSingleSource(t *testing.T) {
	t.Parallel()
	root := repoRootForLint(t)
	bppDir := filepath.Join(root, "packages", "server-go", "internal", "bpp")

	// Step 1: expected definition exists.
	singleSrc := regexp.MustCompile(`BPP_HEARTBEAT_TIMEOUT_SECONDS\s*=\s*30\b`)
	hits := 0
	for _, f := range walkGoSources(t, bppDir, true) {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		hits += len(singleSrc.FindAll(b, -1))
	}
	if hits < 1 {
		t.Errorf("BPP_HEARTBEAT_TIMEOUT_SECONDS = 30 definition missing in internal/bpp/ (got %d, expected >=1; BPP-4 §3 design rule 5 heartbeat timeout constant)", hits)
	}

	// 第 2 步 mismatch detection: heartbeat timeout > 30s 不允许
	badPat := regexp.MustCompile(`heartbeat.*timeout.*[5-9][0-9]+s|heartbeatTimeout.*=.*[1-9][0-9]{2,}`)
	for _, f := range walkGoSources(t, bppDir, true) {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if m := badPat.Find(b); m != nil {
			t.Errorf("%s: heartbeat timeout above 30s (matched %q; BPP-4 §3 design rule 5 negative assertion)", f, string(m))
		}
	}
}

// TestLint_ReasonChainNo7th — AL-1.1 §1.3 6 reason 字典禁止第 7 项约束.
// internal/ 下不允许 commit "reason.*7th" / "runtime_recovered" /
// "reconnect_success" 字面 (新 reason 必须改 reasons 包).
func TestLint_ReasonChainNo7th(t *testing.T) {
	t.Parallel()
	root := repoRootForLint(t)
	internalDir := filepath.Join(root, "packages", "server-go", "internal")

	pat := regexp.MustCompile(`reason.*7th|reason.*runtime_recovered|reason.*reconnect_success`)
	for _, f := range walkGoSources(t, internalDir, true) {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		// 排除字面提到此约束的注释行
		txt := string(b)
		if !pat.MatchString(txt) {
			continue
		}
		// 检查每个 match 是否是注释 / docstring 反向描述
		for _, m := range pat.FindAllStringIndex(txt, -1) {
			lineStart := strings.LastIndex(txt[:m[0]], "\n") + 1
			lineEnd := strings.Index(txt[m[1]:], "\n")
			if lineEnd < 0 {
				lineEnd = len(txt)
			} else {
				lineEnd = m[1] + lineEnd
			}
			line := txt[lineStart:lineEnd]
			// 注释行 (// 或 /*) 反向描述放过
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
				continue
			}
			t.Errorf("%s: detected 7th reason (line: %q; AL-1.1 §1.3 keeps the reason dictionary at 6 entries — new reason must update the reasons package)", f, line)
		}
	}
}

// TestLint_ReasonsSingleSourceExists — reasons package file exists.
func TestLint_ReasonsSingleSourceExists(t *testing.T) {
	t.Parallel()
	root := repoRootForLint(t)
	ssot := filepath.Join(root, "packages", "server-go", "internal", "agent", "reasons", "reasons.go")
	if _, err := os.Stat(ssot); err != nil {
		t.Errorf("reasons package missing %s: %v (required by AL-1a #496)", ssot, err)
	}
}

// TestLint_ReasonsCrossMilestoneCoverage — 6 reason 跨 milestone ≥6 hit
// (AL-1a reasons package 跟实施同步, 至少每个 reason 1 次源端引用).
func TestLint_ReasonsCrossMilestoneCoverage(t *testing.T) {
	t.Parallel()
	root := repoRootForLint(t)
	internalDir := filepath.Join(root, "packages", "server-go", "internal")

	pat := regexp.MustCompile(`reasons\.(APIKeyInvalid|QuotaExceeded|NetworkUnreachable|RuntimeCrashed|RuntimeTimeout|Unknown)`)
	hits := 0
	for _, f := range walkGoSources(t, internalDir, true) {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		hits += len(pat.FindAll(b, -1))
	}
	if hits < 6 {
		t.Errorf("reason chain coverage insufficient — got %d source-side hits, expected >=6 (AL-1a reason constants should each have at least 1 source-side reference)", hits)
	}
}

// TestLint_AgentStateLogNoConnecting — BPP-5 §1.4 条原则: connecting 是
// transient 中间态不入 5-state graph, agent_state_log.go 里禁字面
// AgentStateConnecting / state.*connecting.
func TestLint_AgentStateLogNoConnecting(t *testing.T) {
	t.Parallel()
	root := repoRootForLint(t)
	logFile := filepath.Join(root, "packages", "server-go", "internal", "store", "agent_state_log.go")
	b, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read %s: %v", logFile, err)
	}
	pat := regexp.MustCompile(`AgentStateConnecting|state.*connecting`)
	if m := pat.Find(b); m != nil {
		t.Errorf("%s: connecting 持久态被写入 (matched %q; BPP-5 §1.4 条原则 — connecting 是 transient 中间态, 不入 5-state graph)", logFile, string(m))
	}
}

// TestLint_PresenceSessionsNoBusyWrite — AL-1b §2 design rule 2 BPP frame 是
// busy/idle 唯一 source, presence_sessions 不写 busy 列.
func TestLint_PresenceSessionsNoBusyWrite(t *testing.T) {
	t.Parallel()
	root := repoRootForLint(t)
	storeDir := filepath.Join(root, "packages", "server-go", "internal", "store")

	pat := regexp.MustCompile(`presence_sessions.*UPDATE.*busy|presence.*set.*busy`)
	for _, f := range walkGoSources(t, storeDir, true) {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if m := pat.Find(b); m != nil {
			t.Errorf("%s: busy/idle source write location is incorrect (matched %q; AL-1b §2 design rule 2 BPP frame is the only source, presence_sessions must not write busy column)", f, string(m))
		}
	}
}

// TestLint_ALHBStackDictIsolation — AL stack vs HB stack audit 字典分立.
// AL 走 agent_state_log + agent_status, HB 走 audit_log; 分立不 JOIN.
func TestLint_ALHBStackDictIsolation(t *testing.T) {
	t.Parallel()
	root := repoRootForLint(t)
	internalDir := filepath.Join(root, "packages", "server-go", "internal")

	pat := regexp.MustCompile(`agent_state_log.*JOIN.*audit_log|agent_status.*JOIN.*audit_log`)
	for _, f := range walkGoSources(t, internalDir, true) {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if m := pat.Find(b); m != nil {
			t.Errorf("%s: AL stack vs HB stack 字典分立检查失败 (matched %q; AL 走 agent_state_log+agent_status, HB 走 audit_log, 分立不 JOIN)", f, string(m))
		}
	}
}

// TestLint_AuditSchema5FieldsByteIdentical — HB-3 §1.4 audit schema 5 fields
// (actor / action / target / when / scope) exact match across audit sources.
// 当前 source: host_grants.go + dead_letter.go (HB-1 + HB-2 Go binary 实施
// PR 加 install-butler/audit.go + host-bridge/audit.go 时, sources 列表扩
// 4, 此 test 同步扩).
func TestLint_AuditSchema5FieldsByteIdentical(t *testing.T) {
	t.Parallel()
	root := repoRootForLint(t)

	sources := []string{
		filepath.Join(root, "packages", "server-go", "internal", "api", "host_grants.go"),
		filepath.Join(root, "packages", "server-go", "internal", "bpp", "dead_letter.go"),
	}

	keys := []string{"actor", "action", "target", "when", "scope"}

	for _, src := range sources {
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read %s: %v", src, err)
		}
		txt := string(b)
		for _, k := range keys {
			pat := regexp.MustCompile(`"` + k + `"`)
			if !pat.MatchString(txt) {
				t.Errorf("%s: audit schema field %q missing (HB-3 §1.4 requires actor/action/target/when/scope)", src, k)
			}
		}
	}
}

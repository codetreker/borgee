// Package api_test — chn_11_member_admin_test.go: CHN-11 0-server-prod
// grep 检查守门 (CHN-11 仅 client SPA — server-side POST/DELETE/GET
// /channels/:id/members CHN-1 #276 既有 path byte-identical 不变).
//
// Pins:
//   REG-CHN11-001 TestChannelAdmin_NoSchemaChange (filepath.Walk migrations/)
//   REG-CHN11-002 TestChannelAdmin_NoServerProductionCode (grep 检查 `chn_11`
//                  在 internal/api/*.go 非 _test.go 0 hit)
//   REG-CHN11-003 TestCHN_HandlersByteIdentical (handleAddMember +
//                  handleRemoveMember block grep 检查 `chn_11` 0 hit)
//   REG-CHN11-004 TestCHN_NoMemberAdminQueue (AST 锁链延伸第 19 处)
//   REG-CHN11-005 TestCHN_NoAdminMembersPath
package api_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// REG-CHN11-001 — 0 schema 改 (grep 检查 migrations/chn_11_*).
func TestChannelAdmin_NoSchemaChange(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "chn_11_") {
			t.Errorf("CHN-11 设计 ① broken — found schema migration %q (must be 0 schema)", e.Name())
		}
	}
}

// REG-CHN11-002 — 0 server production code (grep 检查 `chn_11` / `chn11`
// 在 internal/api/*.go 非 _test.go 0 hit).
func TestChannelAdmin_NoServerProductionCode(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "api")
	forbidden := []string{"chn_11", "chn11", "CHN11"}
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		// production only — exclude _test.go.
		if strings.HasSuffix(p, "_test.go") {
			return nil
		}
		body, _ := os.ReadFile(p)
		for _, tok := range forbidden {
			if strings.Contains(string(body), tok) {
				t.Errorf("CHN-11 设计 ② broken — token %q in production %s (must be 0 server prod)", tok, p)
			}
		}
		return nil
	})
}

// REG-CHN11-003 — 既有 handleAddMember + handleRemoveMember byte-identical.
// channels.go 内 2 个 handler block 不漂入 chn_11 字面 (CHN-1 #276 既有
// path byte-identical 不变).
func TestCHN_HandlersByteIdentical(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join("..", "api", "channels.go"))
	if err != nil {
		t.Fatalf("read channels.go: %v", err)
	}
	src := string(body)
	for _, fn := range []string{"handleAddMember", "handleRemoveMember"} {
		idx := strings.Index(src, fn)
		if idx < 0 {
			t.Errorf("既有 %s 不存在 — CHN-1 #276 path 漂走 (CHN-11 边界 ④ broken)", fn)
			continue
		}
		end := idx + 2500
		if end > len(src) {
			end = len(src)
		}
		block := src[idx:end]
		for _, tok := range []string{"chn_11", "chn11", "CHN11"} {
			if strings.Contains(block, tok) {
				t.Errorf("既有 %s block 漂入 CHN-11 — token %q (边界 ④ broken)", fn, tok)
			}
		}
	}
}

// REG-CHN11-004 — AST 锁链延伸第 19 处 forbidden 3 token.
func TestCHN_NoMemberAdminQueue(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingMemberAdmin",
		"memberAdminQueue",
		"deadLetterMemberAdmin",
	}
	dir := filepath.Join("..", "api")
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		body, _ := os.ReadFile(p)
		for _, tok := range forbidden {
			if strings.Contains(string(body), tok) {
				t.Errorf("AST 锁链延伸第 19 处 broken — token %q in %s", tok, p)
			}
		}
		return nil
	})
}

// REG-CHN11-005 — admin god-mode 不挂 PATCH/POST/PUT/DELETE 在 admin-api/
// v1/.../members (ADM-0 §1.3 红线; member admin 是 owner-only user-rail).
func TestCHN_NoAdminMembersPath(t *testing.T) {
	t.Parallel()
	dirs := []string{filepath.Join("..", "api"), filepath.Join("..", "server")}
	pat := regexp.MustCompile(`mux\.Handle\("(POST|DELETE|PATCH|PUT)[^"]*admin-api/v[0-9]+/[^"]*members`)
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			fb, _ := os.ReadFile(p)
			if loc := pat.FindIndex(fb); loc != nil {
				t.Errorf("CHN-11 admin god-mode broken — admin-rail members path in %s: %q",
					p, fb[loc[0]:loc[1]])
			}
			return nil
		})
	}
}

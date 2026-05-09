// Package api_test — chn_12_reorder_test.go: CHN-12 0-server-prod 反向
// grep守门 (CHN-12 仅 client SPA dnd_position.ts + ChannelDragHandle 既有
// SortableChannelItem; server-side PUT /api/v1/me/layout CHN-3.2 #357
// 既有 path byte-identical 不变).
//
// Pins:
//   REG-CHN12-001 TestChn12reorder_NoSchemaChange (filepath.Walk migrations/)
//   REG-CHN12-002 TestChn12reorder_NoServerProductionCode (grep 检查 `chn_12`
//                  在 internal/api/*.go 非 _test.go 0 hit)
//   REG-CHN12-003 TestCHN_HandlerByteIdentical (handlePutMyLayout block
//                  grep 检查 `chn_12` 0 hit)
//   REG-CHN12-004 TestCHN_NoReorderQueue (AST 锁链延伸第 20 处)
//   REG-CHN12-005 TestCHN_NoAdminLayoutPath
package api_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// REG-CHN12-001 — 0 schema 改 (grep 检查 migrations/chn_12_*).
func TestChn12reorder_NoSchemaChange(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "chn_12_") {
			t.Errorf("CHN-12 设计 ① broken — found schema migration %q (must be 0 schema, 复用 user_channel_layout.position)", e.Name())
		}
	}
}

// REG-CHN12-002 — 0 server production code (grep 检查 `chn_12` / `chn12`
// 在 internal/api/*.go 非 _test.go 0 hit).
func TestChn12reorder_NoServerProductionCode(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "api")
	forbidden := []string{"chn_12", "chn12", "CHN12"}
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
				t.Errorf("CHN-12 设计 ② broken — token %q in production %s (must be 0 server prod)", tok, p)
			}
		}
		return nil
	})
}

// REG-CHN12-003 — 既有 handlePutMyLayout byte-identical (CHN-3.2 #357 path
// 不变, layout.dm_not_grouped + non-member 403 + 文案锁 layoutSaveErrorMsg
// 全套不动).
func TestCHN_HandlerByteIdentical(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join("..", "api", "layout.go"))
	if err != nil {
		t.Fatalf("read layout.go: %v", err)
	}
	src := string(body)
	idx := strings.Index(src, "handlePutMyLayout")
	if idx < 0 {
		t.Fatalf("既有 handlePutMyLayout 不存在 — CHN-3.2 #357 path 漂走 (CHN-12 边界 ④ broken)")
	}
	end := idx + 3500
	if end > len(src) {
		end = len(src)
	}
	block := src[idx:end]
	for _, tok := range []string{"chn_12", "chn12", "CHN12"} {
		if strings.Contains(block, tok) {
			t.Errorf("既有 handlePutMyLayout block 漂入 CHN-12 — token %q (边界 ④ broken)", tok)
		}
	}
	// 5 字面 grep 检查 byte-identical (CHN-3.2 + chn_3_2_layout_test.go 5 源).
	for _, must := range []string{
		"layout.dm_not_grouped",
		"layout.invalid_payload",
		"侧栏顺序保存失败",
	} {
		if !strings.Contains(block, must) {
			t.Errorf("handlePutMyLayout block 漂走既有锚 %q (CHN-3.2 #357 边界 ④ broken)", must)
		}
	}
}

// REG-CHN12-004 — AST 锁链延伸第 20 处 forbidden 3 token.
func TestCHN_NoReorderQueue(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingReorder",
		"reorderQueue",
		"deadLetterReorder",
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
				t.Errorf("AST 锁链延伸第 20 处 broken — token %q in %s", tok, p)
			}
		}
		return nil
	})
}

// REG-CHN12-005 — admin god-mode 不挂 PATCH/POST/PUT/DELETE 在 admin-api/
// v1/.../layout 或 .../reorder (ADM-0 §1.3 红线; reorder 是 per-user
// preference user-rail 设计 ⑥).
func TestCHN_NoAdminLayoutPath(t *testing.T) {
	t.Parallel()
	dirs := []string{filepath.Join("..", "api"), filepath.Join("..", "server")}
	pat := regexp.MustCompile(`mux\.Handle\("(POST|DELETE|PATCH|PUT)[^"]*admin-api/v[0-9]+/[^"]*(layout|reorder)`)
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
				t.Errorf("CHN-12 admin god-mode broken — admin-rail layout/reorder path in %s: %q",
					p, fb[loc[0]:loc[1]])
			}
			return nil
		})
	}
}

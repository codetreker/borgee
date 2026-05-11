// Package api_test — CHN-4 wrapper grep audit.
//
// 设计第 1 条 — 7 源 字节级一致 反向断言不破:
//   #354 第 4 条 + #353 §3.1 + #357 第 2 条 + #364 + #371 + #374 + chn-4 设计约束
//   "DM 视图永不含 workspace tab" 7 源同根锁
//
// 设计第 4 条 — server production 0 行变更, 仅加 _test.go grep audit hook
//
// Note: 当 PERF-AST-LINT #506 (astscan helper) 合入 main 后, 此 test 应改
// 调 `astscan.AssertNoForbiddenIdentifiers` 替代 inline grep — REG-CHN4-004
// 后续.

package api_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCHN_DMViewHasNoWorkspaceTab pins 设计第 1 条 + 7 源 字节级一致 反向锁.
// production code (server-go internal/) 必不含 dm 视图渲染 workspace tab
// 的字面 (跟 ChannelView.tsx + chn-2-content-lock 同源).
//
// 此处扫 server side production *.go 反向断言 — 客户端 7 源 字节级一致
// 对齐链由 client vitest 守 (chn-2-content-lock.test.ts 等).
func TestCHN_DMViewHasNoWorkspaceTab(t *testing.T) {
	t.Parallel()
	// 反向 forbidden identifiers — 7 源同根锁的硬条件.
	forbidden := []string{
		`dmShowsWorkspace`,
		`enableDMWorkspace`,
		`allowDMWorkspaceTab`,
		`dm_view_workspace_enabled`,
	}
	dirs := []string{"../api", "../store", "../server", "../bpp"}
	hits := []string{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			if strings.HasSuffix(e.Name(), "_test.go") {
				continue // tests legally mention forbidden tokens
			}
			path := filepath.Join(dir, e.Name())
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			content := string(b)
			for _, bad := range forbidden {
				if strings.Contains(content, bad) {
					hits = append(hits, path+":"+bad)
				}
			}
		}
	}
	if len(hits) > 0 {
		t.Errorf("CHN-4 设计第 1 条 broken: dm-view-workspace-enabling identifier "+
			"found in production *.go (7 源 字节级一致 对齐链同根: #354 第 4 条 + "+
			"#353 §3.1 + #357 第 2 条 + #364 + #371 + #374 + chn-4 设计约束): %v", hits)
	}
}

// TestCHN_NoDMSyncBypassEndpoint pins 设计第 2 条 — DM 走普通 channel events
// path, 不开 dm-only sync endpoint (跟 DM-3 wrapper 同精神).
func TestCHN_NoDMSyncBypassEndpoint(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		`"/api/v1/dm/sync"`,
		`"/api/v1/dm/cursor"`,
		`"/api/v1/channels/dm-only"`,
	}
	dirs := []string{"../api", "../server"}
	hits := []string{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			if strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			content := string(b)
			for _, bad := range forbidden {
				if strings.Contains(content, bad) {
					hits = append(hits, path+":"+bad)
				}
			}
		}
	}
	if len(hits) > 0 {
		t.Errorf("CHN-4 设计第 2 条 broken: dm-only bypass endpoint paths found: %v", hits)
	}
}

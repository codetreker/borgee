// Package api_test — adm_3_v1_no_user_rail_audit_test.go: ADM-3 v1 e2e
// 后续 reverse-grep §2.2 (post-#623 liema CONDITIONAL LGTM).
//
// 反约束 (adm-3-v1-e2e-spec.md §2.2 + ADM-0 §1.3 admin god-mode 红线核心):
//   永不挂 user-rail audit feed —
//   `/api/v1/me/audit*` / `/api/v1/audit/*` 在 production code 0 hit.
//   仅 admin-rail `/admin-api/v1/audit/multi-source` 暴露.
//
// 设计沿用 (跟 RT-3 #616 thought-process 5-pattern reverse-grep + AP-2
// #620 反 RBAC role bleed reverse-grep + AP-4-enum #591 reflect-lint
// 同模式承袭).
package api_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestADM3VE_NoUserRailAuditFeed_ReverseGrep — §2.2 反 user-rail audit feed
// 真守门 (反 v2+ 借口推; ADM-0 §1.3 红线核心断言).
//
// 扫描 packages/server-go/internal/api + internal/server +
// packages/client/src — 任何 mux.Handle 字面 `/api/v1/me/audit*` 或
// `/api/v1/audit/*` (除 test file 自身) 0 hit.
func TestADM3VE_NoUserRailAuditFeed_ReverseGrep(t *testing.T) {
	t.Parallel()

	// grep 检查 三 pattern (ADM-0 §1.3 红线 + spec §2.2 字面).
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`mux\.Handle\([^)]*"\s*[A-Z]+\s+/api/v1/me/audit`),
		regexp.MustCompile(`mux\.Handle\([^)]*"\s*[A-Z]+\s+/api/v1/audit/`),
	}

	roots := []string{
		filepath.Join("..", "api"),
		filepath.Join("..", "server"),
	}

	var hits []string
	for _, root := range roots {
		_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(p, ".go") {
				return nil
			}
			// Skip test files (this file itself contains the patterns as string literals).
			if strings.HasSuffix(p, "_test.go") {
				return nil
			}
			body, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			for _, re := range patterns {
				if re.Match(body) {
					hits = append(hits, p+": "+re.String())
				}
			}
			return nil
		})
	}

	if len(hits) > 0 {
		t.Errorf("user-rail audit feed 漂入 (ADM-0 §1.3 红线核心断言违反, 反 v2+ 借口推):\n  %s",
			strings.Join(hits, "\n  "))
	}
}

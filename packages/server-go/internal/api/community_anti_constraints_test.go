// cm_5_1_anti_constraints_test.go — CM-5.1 negative source-scan guard.
//
// Spec: docs/implementation/modules/cm-5-spec.md §1.1 + §2 (5 条原则 + 4 行
// 禁止项列表).
// Acceptance: docs/qa/acceptance-templates/cm-5.md §1 schema (CM-5.1) +
// §4 negative source-scan forbidden list.
// Blueprint: concept-model.md §1.3 (§185 "未来你会看到 agent 互相协作") +
// agent-lifecycle.md §1 (Borgee 是协作平台, agent 之间走 Borgee 平台机制).
//
// CM-5 design rules (5 literal sources must stay aligned):
//   design rule 1 agent↔agent 协作走人协作 path — DM-2 mention router + CV-1 artifact
//      + AP-0/AP-2 permission, **不拆** "agent_only_message" / "ai_to_ai_
//      channel" 旁路.
//   design rule 2 owner-first responsibility — `artifact_versions.committed_by` 永远是 user
//      行 (agent 也是 user.role='agent', 走 user.id), 不拆
//      `triggered_by_agent_id` 列.
//   design rule 3 X2 conflict 复用 CV-1.2 既有 single-doc lock 30s + CV-4.1 iterations
//      state 机制 — 不引入新 schema (artifact_locks / iteration_priority
//      表).
//   design rule 4 agent A → B mention 走 DM-2 router 不旁路 — `MentionPushedFrame` 8
//      fields exactly match, 不开 `agent_to_agent_mention` 专属 frame.
//   design rule 5 owner-first visibility — 跟人协作产物 owner 可见同模式, 不拆
//      owner_visibility scope, 不引入 "ai_only" 隐藏字段.
//
// 此 test 等价于 acceptance §4 negative source scan over the 4 forbidden entries:
//   4.1 design rule 1 旁路表 — agent_messages\b / ai_to_ai_channel /
//       agent_only_message count==0
//   4.2 design rule 1 旁路 endpoint — POST /api/v1/agents/.*/notify-agent count==0
//   4.3 design rule 2 责任旁路 — triggered_by_agent_id / committed_by_agent count==0
//   4.4 design rule 3 新锁定表 — artifact_locks\s+TABLE / iteration_priority\s+TABLE
//       count==0
//
// 跟 hub_presence_grep_test.go (AL-3.2 §301) 同模式 — AST walk + go/parser
// 解析 string literal, 注释里说设计 (intentional doc) 不 trip; 仅守
// 代码字面 (string literal context).

package api_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoRoot returns absolute path of the borgee repo root by walking up from
// the test file location until a go.mod is found.
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 10; i++ {
		// Repo root has a top-level "packages" dir + "docs" dir.
		_, errPkg := os.Stat(filepath.Join(dir, "packages"))
		_, errDocs := os.Stat(filepath.Join(dir, "docs"))
		if errPkg == nil && errDocs == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("repo root not found from %s", wd)
	return ""
}

// walkGoFiles collects all non-test .go files under root recursively,
// excluding _test.go and excluding the api_test package itself.
func walkGoFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip vendor / .git / node_modules.
			if d.Name() == "vendor" || d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return out
}

// scanForbiddenLiterals walks AST of each Go file and reports string
// literals containing any of forbidden substrings. Comment-only mentions
// (intentional doc) don't trip — we use parser.SkipObjectResolution and
// only inspect *ast.BasicLit STRING nodes.
func scanForbiddenLiterals(t *testing.T, files []string, forbidden []string) []string {
	t.Helper()
	fset := token.NewFileSet()
	var hits []string
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		af, err := parser.ParseFile(fset, f, src, parser.SkipObjectResolution)
		if err != nil {
			// Skip unparseable files (rare; e.g. generated code with build
			// constraints); not fatal for source-scan guard.
			continue
		}
		ast.Inspect(af, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			for _, bad := range forbidden {
				if strings.Contains(lit.Value, bad) {
					hits = append(hits, f+":"+fset.Position(lit.Pos()).String()+" contains "+bad)
				}
			}
			return true
		})
	}
	return hits
}

// scanRegex walks all source files (.go + .ts + .tsx) under serverRoot +
// clientRoot and reports paths matching any regex. Comments not stripped —
// pattern should be specific enough to not false-positive in doc.
func scanRegex(t *testing.T, roots []string, exts []string, patterns []*regexp.Regexp, excludeTests bool) []string {
	t.Helper()
	var hits []string
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "vendor" || d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "testdata" || d.Name() == "__tests__" || d.Name() == "dist" {
					return filepath.SkipDir
				}
				return nil
			}
			matched := false
			for _, e := range exts {
				if strings.HasSuffix(path, e) {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
			if excludeTests && (strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".test.ts") || strings.HasSuffix(path, ".test.tsx") || strings.HasSuffix(path, ".spec.ts")) {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			for _, re := range patterns {
				if re.Match(b) {
					hits = append(hits, path+" matches "+re.String())
				}
			}
			return nil
		})
	}
	return hits
}

// TestCM_NoBypassTable 验证 acceptance §4.1 design rule 1 — agent↔agent 协作走人
// 协作 path, 不拆表/不开旁路 schema. 反向断言 server-go 全包不出现
// `agent_messages` / `ai_to_ai_channel` / `agent_only_message` /
// `agent_to_agent_mention` 字面 (string literal context, 注释里说设计不
// trip).
func TestCM_NoBypassTable(t *testing.T) {
	root := repoRoot(t)
	serverGo := filepath.Join(root, "packages", "server-go")
	files := walkGoFiles(t, serverGo)

	hits := scanForbiddenLiterals(t, files, []string{
		"agent_messages",         // design rule 1 bypass table name
		"ai_to_ai_channel",       // design rule 1 bypass channel kind
		"agent_only_message",     // design rule 1 bypass message kind
		"agent_to_agent_mention", // design rule 4 bypass frame name
	})
	if len(hits) > 0 {
		t.Fatalf("CM-5.1 design rule 1 negative check failed — server-go must not contain bypass table/channel/message/frame literals (走人协作 path); hits:\n  %s\n"+
			"acceptance #264 §4.1 + cm-5-spec.md §0 design rule 1 + §2 forbidden entries 1+2.",
			strings.Join(hits, "\n  "))
	}
}

// TestCM_NoBypassEndpoint 验证 acceptance §4.2 design rule 1 — server 不开
// `POST /api/v1/agents/:id/notify-agent` 旁路 endpoint. 反向断言 server-go
// 路由表不含此字面.
func TestCM_NoBypassEndpoint(t *testing.T) {
	root := repoRoot(t)
	serverGo := filepath.Join(root, "packages", "server-go")

	// Regex 字面较宽 — 'notify-agent' 子串足够 specific (蓝图无其它合法用途).
	hits := scanRegex(t, []string{serverGo},
		[]string{".go"},
		[]*regexp.Regexp{
			regexp.MustCompile(`/agents/[^/"]+/notify-agent`),
			regexp.MustCompile(`notify-agent`),
		},
		true)
	if len(hits) > 0 {
		t.Fatalf("CM-5.1 design rule 1 negative check failed — server-go must not expose POST /api/v1/agents/:id/notify-agent bypass endpoint (mention 走 DM-2 router); hits:\n  %s\n"+
			"acceptance §4.2 + cm-5-spec.md §0 design rule 1 + §2 forbidden entry 2.",
			strings.Join(hits, "\n  "))
	}
}

// TestCM_NoOwnerBypassColumn 验证 acceptance §4.3 design rule 2 — 责任归属
// owner-first, `artifact_versions.committed_by` 永远是 user 行, 不拆
// `triggered_by_agent_id` / `committed_by_agent` 列. 反向断言 server-go
// migrations + store 不出现此列名字面.
func TestCM_NoOwnerBypassColumn(t *testing.T) {
	root := repoRoot(t)
	serverGo := filepath.Join(root, "packages", "server-go")
	files := walkGoFiles(t, serverGo)

	hits := scanForbiddenLiterals(t, files, []string{
		"triggered_by_agent_id", // design rule 2 responsibility bypass column
		"committed_by_agent",    // design rule 2 commit bypass column
	})
	if len(hits) > 0 {
		t.Fatalf("CM-5.1 design rule 2 negative check failed — schema/store must not split owner-first responsibility into agent-specific columns; hits:\n  %s\n"+
			"acceptance §4.3 + cm-5-spec.md §0 design rule 2 + §2 forbidden entry 3.",
			strings.Join(hits, "\n  "))
	}
}

// TestCM_NoNewLockTable 验证 acceptance §4.4 design rule 3 — X2 冲突复用 CV-1.2
// single-doc lock + CV-4.1 iterations state, 不引入新锁定表. 反向断言
// migrations 不创建 `artifact_locks` / `iteration_priority` 表.
func TestCM_NoNewLockTable(t *testing.T) {
	root := repoRoot(t)
	migrationsDir := filepath.Join(root, "packages", "server-go", "internal", "migrations")

	// CREATE TABLE artifact_locks / iteration_priority — 跟 #366 yema 禁止项列表
	// regex 同模式 (`\s+TABLE` 严守, 避免误伤 col 名).
	hits := scanRegex(t, []string{migrationsDir},
		[]string{".go"},
		[]*regexp.Regexp{
			regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(IF\s+NOT\s+EXISTS\s+)?artifact_locks\b`),
			regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(IF\s+NOT\s+EXISTS\s+)?iteration_priority\b`),
			regexp.MustCompile(`artifact_locks\s+TABLE`),
			regexp.MustCompile(`iteration_priority\s+TABLE`),
		},
		true)
	if len(hits) > 0 {
		t.Fatalf("CM-5.1 design rule 3 negative check failed — X2 conflict must reuse CV-1.2 single-doc lock + CV-4.1 iterations state, no new artifact_locks/iteration_priority table; hits:\n  %s\n"+
			"acceptance §4.4 + cm-5-spec.md §0 design rule 3 + §2 forbidden entry 4.",
			strings.Join(hits, "\n  "))
	}
}

// TestCM_X2ConflictLiteralReuse 验证 acceptance §2.2 + cm-5-spec.md §1.2
// design rule 3 — X2 冲突 409 错码字面 must reuse the CV-4 #380 rule 7 literal.
// 验证: cv_4_2_iterations.go (or 同等 server file) 含
// `artifact.locked_by_another_iteration` 字面 (CM-5 复用此既有错码,
// 不另起 'cm5.x2_conflict' / 'agent_collision' 等同义词).
func TestCM_X2ConflictLiteralReuse(t *testing.T) {
	root := repoRoot(t)
	serverGo := filepath.Join(root, "packages", "server-go")
	files := walkGoFiles(t, serverGo)

	// negative assertion — 不出现 CM-5 自起的 X2 冲突错码 (复用 CV-4 #380 rule 7 字面).
	hits := scanForbiddenLiterals(t, files, []string{
		"cm5.x2_conflict",
		"agent_collision",
		"artifact.x2_conflict",
		"x2_lock_held",
	})
	if len(hits) > 0 {
		t.Fatalf("CM-5.1 design rule 3 negative check failed — must reuse CV-4 #380 rule 7 literal `artifact.locked_by_another_iteration`, not CM-5 specific synonym; hits:\n  %s\n"+
			"cm-5-spec.md §1.2 + §1.3 requires the existing CV-4 conflict literal.",
			strings.Join(hits, "\n  "))
	}
}

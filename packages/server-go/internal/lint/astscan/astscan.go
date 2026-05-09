// Package astscan — reusable AST identifier scan helper for test
// 反约束 (anti-constraint) assertions. PERF-AST-LINT spec brief by 飞马
// 2026-04-29 (`docs/implementation/modules/perf-ast-lint-spec.md`).
//
// 设计:
//   ① AST scan 单源 — 不再每 milestone 写 inline ast.Walk + parser.ParseFile;
//   ② 比 grep 狠 — 默认仅扫 *ast.Ident.Name (production identifier),
//      跳过 comment + string literal (避免 false positive);
//   ③ production-side 0 import — 只 _test.go import; production binary
//      不连此包 (反 `go tool nm` 验证).
//
// 跨 milestone 锁:
//   - BPP-4 #499 TestBPP4_NoRetryQueueInBPPPackage (4 forbidden id) — first
//     落, 重构后调 AssertNoForbiddenIdentifiers 替代 inline ast.Inspect;
//   - BPP-5 #503 (规划) TestBPP5_NoReconnectQueueInBPPPackage — 同模式 reuse;
//   - CM-5.1 #473 (留账) — 二阶段重构.
//
// 使用范例:
//
//	func TestBPP4_NoRetryQueueInBPPPackage(t *testing.T) {
//	    astscan.AssertNoForbiddenIdentifiers(t, ".", []astscan.ForbiddenIdentifier{
//	        {Name: "pendingAcks", Reason: "ack best-effort 不重发"},
//	        {Name: "retryQueue", Reason: "ack best-effort 不重发"},
//	    }, astscan.ScanOpts{})
//	}
package astscan

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ForbiddenIdentifier names a token that must not appear in production
// identifiers within the scanned package. Reason is surfaced in the failure
// message so reviewers know which 设计 the constraint protects.
type ForbiddenIdentifier struct {
	// Name is the substring to match against *ast.Ident.Name. The check is
	// strings.Contains (not equals) to catch derived names like
	// `pendingAcksMu` or `retryQueueLen` that share the forbidden root.
	Name string
	// Reason is the acceptance / 设计 anchor surfaced on hit. Required.
	Reason string
}

// ScanOpts configures the scan extent. Zero value is the safe default
// (production identifiers only, skip _test.go).
type ScanOpts struct {
	// IncludeStrings — also scan string literals (*ast.BasicLit Kind=STRING).
	// Default false (设计 ② — comments + strings 不算 production identifier).
	IncludeStrings bool
	// IncludeComments — also scan comment text (*ast.Comment.Text). Default
	// false; comments are typically where the forbidden token is **discussed**
	// (反约束 narrative requires them).
	IncludeComments bool
	// SkipFiles is a list of basename patterns (filepath.Match) to exclude.
	// `_test.go` suffix is **always** skipped (tests legally mention the
	// forbidden tokens via this very helper); SkipFiles is for additional
	// exclusions like generated code or tag-gated files.
	SkipFiles []string
}

// TestingT is the subset of *testing.T we need. Tests can pass *testing.T
// directly; self-tests can pass a fake to capture failures.
type TestingT interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// AssertNoForbiddenIdentifiers parses every non-test .go file in pkgDir and
// fails the test (t.Errorf, not Fatal — collect all hits before reporting)
// when any forbidden Name appears as a production identifier (or string /
// comment, per ScanOpts). Hits are sorted deterministically for stable
// failure messages.
//
// Errors during parsing fail the test fatally — a parse failure means the
// scan is unreliable, which is worse than missing a hit.
func AssertNoForbiddenIdentifiers(t TestingT, pkgDir string, forbidden []ForbiddenIdentifier, opts ScanOpts) {
	t.Helper()
	if len(forbidden) == 0 {
		t.Fatalf("astscan: forbidden list is empty — pass at least one ForbiddenIdentifier")
	}

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("astscan: ReadDir(%q): %v", pkgDir, err)
	}

	fset := token.NewFileSet()
	type hit struct {
		path   string
		name   string
		reason string
	}
	var hits []hit
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(e.Name(), "_test.go") {
			continue // 设计 ① — tests legally mention forbidden tokens.
		}
		if matchesAny(e.Name(), opts.SkipFiles) {
			continue
		}
		path := filepath.Join(pkgDir, e.Name())

		mode := parser.ParseComments // need comments for opt-in scan
		f, err := parser.ParseFile(fset, path, nil, mode)
		if err != nil {
			t.Fatalf("astscan: parse %s: %v", path, err)
		}

		// Identifier scan — always on (the primary 设计).
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.Ident:
				for _, bad := range forbidden {
					if strings.Contains(v.Name, bad.Name) {
						hits = append(hits, hit{path: path, name: v.Name, reason: bad.Reason})
					}
				}
			case *ast.BasicLit:
				if opts.IncludeStrings && v.Kind == token.STRING {
					for _, bad := range forbidden {
						if strings.Contains(v.Value, bad.Name) {
							hits = append(hits, hit{path: path, name: "string:" + v.Value, reason: bad.Reason})
						}
					}
				}
			}
			return true
		})

		if opts.IncludeComments {
			for _, cg := range f.Comments {
				for _, c := range cg.List {
					for _, bad := range forbidden {
						if strings.Contains(c.Text, bad.Name) {
							hits = append(hits, hit{path: path, name: "comment", reason: bad.Reason})
						}
					}
				}
			}
		}
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].path != hits[j].path {
			return hits[i].path < hits[j].path
		}
		return hits[i].name < hits[j].name
	})

	for _, h := range hits {
		t.Errorf("astscan: forbidden identifier %q in %s — reason: %s",
			h.name, h.path, h.reason)
	}
}

// matchesAny returns true if name matches any pattern via filepath.Match.
// Invalid patterns are reported via panic at test time (programming bug).
func matchesAny(name string, patterns []string) bool {
	for _, p := range patterns {
		ok, err := filepath.Match(p, name)
		if err != nil {
			panic("astscan: invalid SkipFiles pattern " + p + ": " + err.Error())
		}
		if ok {
			return true
		}
	}
	return false
}

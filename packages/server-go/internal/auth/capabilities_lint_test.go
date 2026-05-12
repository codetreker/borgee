// AP-4-enum.1 reflect-lint tests — capability ALL slice + init() rebuild
// Capabilities map + IsValidCapability helper single source (spec §0 design 1 + 3).
//
// 6 unit checks (matching acceptance template design 1.1-1.5 + design 3.1):
//   - TestAP_ALL_OrderedByteIdentical (1.1) — ALL order matches const declaration order
//   - TestAP_Capabilities_AutoBuildFromAll (1.2) — init() 派生 map ↔ ALL 双向
//   - TestAP_ALL_Length14 (1.3) — len(ALL) == 14
//   - TestAP_reflect_lint_NoOrphanConst (1.4a) — 14 const literals ⊂ ALL
//   - TestAP_reflect_lint_NoExtraInMap (1.4b) — Capabilities map ⊂ ALL
//   - TestAP_NoAdminGodModeInALL (1.5) — admin god-mode exclusion (ADM-0 §1.3)
//   - TestAP_IsValidCapability_TruthTable (3.1) — 14 true + 1 false
package auth

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestAP_ALL_OrderedByteIdentical — ALL slice order matches the const
// declaration order (channel scope → artifact scope → messaging → channel admin).
func TestAP_ALL_OrderedByteIdentical(t *testing.T) {
	t.Parallel()
	want := []string{
		"channel.read", "channel.write", "channel.delete",
		"artifact.read", "artifact.write", "artifact.commit", "artifact.iterate", "artifact.rollback",
		"user.mention", "dm.read", "dm.send",
		"channel.manage_members", "channel.invite", "channel.change_role",
	}
	if len(ALL) != len(want) {
		t.Fatalf("ALL len = %d, want %d", len(ALL), len(want))
	}
	for i, c := range ALL {
		if c != want[i] {
			t.Errorf("ALL[%d] = %q, want %q (顺序漂)", i, c, want[i])
		}
	}
}

// TestAP_Capabilities_AutoBuildFromAll — init() 派生 map 双向 ⊂ ALL.
func TestAP_Capabilities_AutoBuildFromAll(t *testing.T) {
	t.Parallel()
	if len(Capabilities) != len(ALL) {
		t.Fatalf("Capabilities len = %d, want %d", len(Capabilities), len(ALL))
	}
	for _, c := range ALL {
		if !Capabilities[c] {
			t.Errorf("Capabilities[%q] missing — init() 漏建", c)
		}
	}
	for k, v := range Capabilities {
		if !v {
			t.Errorf("Capabilities[%q] = false — init() 应全 true", k)
		}
		found := false
		for _, c := range ALL {
			if c == k {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Capabilities[%q] not in ALL — mismatch", k)
		}
	}
}

// TestAP_ALL_Length14 — AP-1 #493 expects exactly 14 capabilities.
func TestAP_ALL_Length14(t *testing.T) {
	t.Parallel()
	if len(ALL) != 14 {
		t.Fatalf("len(ALL) = %d, want 14 (AP-1 #493 字面锁)", len(ALL))
	}
}

// TestAP_reflect_lint_NoOrphanConst — capabilities.go const literals ⊂ ALL.
// Parse the capabilities.go const block with go/ast and verify every string literal ∈ ALL.
func TestAP_reflect_lint_NoOrphanConst(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "capabilities.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse capabilities.go: %v", err)
	}
	allSet := make(map[string]bool, len(ALL))
	for _, c := range ALL {
		allSet[c] = true
	}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, val := range vs.Values {
				bl, ok := val.(*ast.BasicLit)
				if !ok || bl.Kind != token.STRING {
					continue
				}
				lit := strings.Trim(bl.Value, `"`)
				if !allSet[lit] {
					t.Errorf("const literal %q not in ALL — orphan const mismatch", lit)
				}
			}
		}
	}
}

// TestAP_reflect_lint_NoExtraInMap — Capabilities map ⊂ ALL (无 extra).
func TestAP_reflect_lint_NoExtraInMap(t *testing.T) {
	t.Parallel()
	allSet := make(map[string]bool, len(ALL))
	for _, c := range ALL {
		allSet[c] = true
	}
	for k := range Capabilities {
		if !allSet[k] {
			t.Errorf("Capabilities[%q] not in ALL — extra mismatch", k)
		}
	}
}

// TestAP_NoAdminGodModeInALL — ADM-0 §1.3 excludes admin god-mode patterns.
func TestAP_NoAdminGodModeInALL(t *testing.T) {
	t.Parallel()
	banned := []string{"admin_", "godmode_", "impersonat"}
	for _, c := range ALL {
		for _, b := range banned {
			if strings.Contains(c, b) {
				t.Errorf("ALL contains banned god-mode pattern: %q ~ %q (ADM-0 §1.3 红线)", c, b)
			}
		}
	}
}

// TestAP_IsValidCapability_TruthTable — 14 true + 1 false.
func TestAP_IsValidCapability_TruthTable(t *testing.T) {
	t.Parallel()
	for _, c := range ALL {
		if !IsValidCapability(c) {
			t.Errorf("IsValidCapability(%q) = false, want true", c)
		}
	}
	bogus := []string{"", "admin_god", "channel.read ", "CHANNEL.READ", "no.such.perm"}
	for _, b := range bogus {
		if IsValidCapability(b) {
			t.Errorf("IsValidCapability(%q) = true, want false", b)
		}
	}
}

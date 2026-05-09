// AP-4-enum.1 reflect-lint tests — capability ALL slice + init() rebuild
// Capabilities map + IsValidCapability helper 单源 (spec §0 设计 ① + ③).
//
// 6 unit (跟 acceptance template 设计 ① 1.1-1.5 + 设计 ③ 3.1 同源):
//   - TestAP_ALL_OrderedByteIdentical (1.1) — ALL 顺序跟 const 声明顺序对齐
//   - TestAP_Capabilities_AutoBuildFromAll (1.2) — init() 派生 map ↔ ALL 双向
//   - TestAP_ALL_Length14 (1.3) — len(ALL) == 14 锁
//   - TestAP_reflect_lint_NoOrphanConst (1.4a) — 14 const 字面 ⊂ ALL
//   - TestAP_reflect_lint_NoExtraInMap (1.4b) — Capabilities map ⊂ ALL
//   - TestAP_NoAdminGodModeInALL (1.5) — admin god-mode 红线 (ADM-0 §1.3)
//   - TestAP_IsValidCapability_TruthTable (3.1) — 14 true + 1 false
package auth

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestAP_ALL_OrderedByteIdentical — ALL slice 顺序 byte-identical 跟
// const 声明顺序 (channel scope → artifact scope → messaging → channel admin).
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
			t.Errorf("Capabilities[%q] not in ALL — drift", k)
		}
	}
}

// TestAP_ALL_Length14 — 14 锁 (跟 AP-1 #493 同源).
func TestAP_ALL_Length14(t *testing.T) {
	t.Parallel()
	if len(ALL) != 14 {
		t.Fatalf("len(ALL) = %d, want 14 (AP-1 #493 字面锁)", len(ALL))
	}
}

// TestAP_reflect_lint_NoOrphanConst — capabilities.go const 字面 ⊂ ALL.
// 走 go/ast 解析 capabilities.go const block, 验每个 string literal ∈ ALL.
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
					t.Errorf("const literal %q not in ALL — orphan const drift", lit)
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
			t.Errorf("Capabilities[%q] not in ALL — extra drift", k)
		}
	}
}

// TestAP_NoAdminGodModeInALL — ADM-0 §1.3 红线 (admin 永久不挂).
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

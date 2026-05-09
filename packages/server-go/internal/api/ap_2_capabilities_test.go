// AP-2 server — capability 透明 UI response shape unit tests.
//
// 设计沿用 (ap-2-spec.md §0):
//   - 设计 ② response 加 `capabilities` 数组 (14 const SSOT 单源)
//   - 设计 ② 反向断言 response 不暴露 RBAC role 字面 admin/editor/viewer/owner
//     (反 role bleed); `role` 字段仅 legacy caller 兼容, UI 不显
//   - 设计 ① AP-1 14 const + AP-4-enum reflect-lint byte-identical 不破

package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"borgee-server/internal/auth"
)

func TestAP2_DeriveCapabilities_MemberFullGrant(t *testing.T) {
	got := deriveAP2Capabilities("member", []string{"*"})
	if len(got) != len(auth.ALL) {
		t.Fatalf("member 全权 want %d capabilities, got %d", len(auth.ALL), len(got))
	}
	// byte-identical 顺序 (跟 auth.ALL).
	for i, tok := range got {
		if tok != auth.ALL[i] {
			t.Errorf("capability order drift @%d: want %q got %q", i, auth.ALL[i], tok)
		}
	}
}

func TestAP2_DeriveCapabilities_AgentNarrowed(t *testing.T) {
	// agent permissions like ["channel.read:channel:abc", "dm.send:*"]
	got := deriveAP2Capabilities("agent", []string{
		"channel.read:channel:abc",
		"channel.read:channel:def", // dedupe
		"dm.send:*",
		"unknown_capability:*", // unknown forward-compat drop
	})
	want := []string{"channel.read", "dm.send"}
	if len(got) != len(want) {
		t.Fatalf("narrow want %d, got %d (%v)", len(want), len(got), got)
	}
	for i, tok := range want {
		if got[i] != tok {
			t.Errorf("@%d want %q got %q", i, tok, got[i])
		}
	}
}

func TestAP2_DeriveCapabilities_AgentNoGrant(t *testing.T) {
	got := deriveAP2Capabilities("agent", nil)
	if len(got) != 0 {
		t.Fatalf("empty grant want 0 caps, got %d", len(got))
	}
	got2 := deriveAP2Capabilities("agent", []string{})
	if len(got2) != 0 {
		t.Fatalf("empty slice want 0 caps, got %d", len(got2))
	}
}

func TestAP2_DeriveCapabilities_OnlyKnownTokens(t *testing.T) {
	// 反向断言: derive 输出全在 14 const 内 (反 role 字面 leak).
	all := deriveAP2Capabilities("member", []string{"*"})
	for _, tok := range all {
		if !auth.IsValidCapability(tok) {
			t.Errorf("derive leaked unknown token %q (反 14 const SSOT 闸)", tok)
		}
		// 反向断言 — 不含 RBAC role 字面.
		lower := strings.ToLower(tok)
		for _, role := range []string{"admin", "editor", "viewer", "owner"} {
			if lower == role {
				t.Errorf("设计 ② 反 role bleed — token %q 命中 RBAC role 字面", tok)
			}
		}
	}
}

func TestAP2_NoRoleNamesInResponseShape_MemberPath(t *testing.T) {
	// Build a fake response shape via the same shape as users.go writes,
	// then JSON-encode and reverse-grep for RBAC role JSON values.
	caps := deriveAP2Capabilities("member", []string{"*"})
	resp := map[string]any{
		"user_id":      "u-1",
		"role":         "member", // legacy caller field — value is the user's
		"permissions":  []string{"*"},
		"details":      []map[string]any{},
		"capabilities": caps,
	}
	rec := httptest.NewRecorder()
	writeJSONResponse(rec, 200, resp)
	body := rec.Body.String()

	// 反向断言: response JSON value 不含 RBAC role 字面 (admin/editor/
	// viewer/owner) — `role` field 值仅 'member' / 'agent' (legacy 字面),
	// `capabilities` 数组也不含 role 名.
	for _, bad := range []string{
		`"role":"admin"`,
		`"role":"editor"`,
		`"role":"viewer"`,
		`"role":"owner"`,
	} {
		if strings.Contains(body, bad) {
			t.Errorf("设计 ② 反 RBAC role bleed — response 含 %q (UI 不应显此值)", bad)
		}
	}

	// `capabilities` 字段必存在 (AP-2 SSOT 单源).
	if !strings.Contains(body, `"capabilities"`) {
		t.Error("AP-2 设计 ② — response 缺 `capabilities` 字段")
	}

	// JSON parse round-trip — capabilities 是数组.
	var parsed map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response JSON parse: %v", err)
	}
	got, ok := parsed["capabilities"].([]any)
	if !ok {
		t.Fatalf("`capabilities` 应为数组, got %T", parsed["capabilities"])
	}
	if len(got) != len(auth.ALL) {
		t.Errorf("member 全权 capabilities len want %d, got %d", len(auth.ALL), len(got))
	}
}

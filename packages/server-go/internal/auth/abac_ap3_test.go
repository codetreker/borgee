// Package auth — abac_ap3_test.go: AP-3 cross-org owner-only gate 单测.
//
// Pins:
//   REG-AP3-002a — cross-org user → false (即使有 wildcard 也 reject)
//   REG-AP3-002b — same-org user → true (跟 AP-1 既有路径完全兼容)
//   REG-AP3-002c — cross-org agent → false (BPP-1 #304 org sandbox 同源)
//   REG-AP3-002d — NULL org_id legacy 路径 (跟 AP-1 现网行为零变)
//   REG-AP3-002e — admin god-mode 不入此路径 (反向 grep)
//   REG-AP3-001 — ErrCodeCrossOrgDenied 字面单一来源
//   REG-AP3-003 — 反向 grep cross-org bypass count==0
package auth

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"borgee-server/internal/store"
)

// REG-AP3-001 (acceptance §1.3) — ErrCodeCrossOrgDenied byte-identical
// 字面单一来源 (跟 AP-1 const 同模式, 改 = 改 const 一处).
func TestAP_ErrCodeCrossOrgDeniedConst(t *testing.T) {
	t.Parallel()
	if ErrCodeCrossOrgDenied != "abac.cross_org_denied" {
		t.Errorf("ErrCodeCrossOrgDenied mismatch: got %q, want %q",
			ErrCodeCrossOrgDenied, "abac.cross_org_denied")
	}
}

// ap3TestStore builds a memory store with channels + user_permissions
// auto-migrated. Mirrors testStore() but adds Channel for org gate
// resolveScopeOrgID path.
func ap3TestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.DB().AutoMigrate(&store.User{}, &store.UserPermission{}, &store.Channel{}); err != nil {
		t.Fatal(err)
	}
	return s
}

// seed inserts a channel with a given org_id so resolveScopeOrgID
// returns the expected org. Channel created_by intentionally != grantee
// — owner-vs-acting-user is orthogonal to the org gate.
func seedChannelOrg(t *testing.T, s *store.Store, channelID, orgID string) {
	t.Helper()
	ch := &store.Channel{
		ID:        channelID,
		OrgID:     orgID,
		Name:      "ch-" + channelID,
		Type:      "public",
		CreatedBy: "owner-x",
		CreatedAt: 1700000000000,
	}
	if err := s.CreateChannel(ch); err != nil {
		t.Fatalf("seed channel %s: %v", channelID, err)
	}
}

// REG-AP3-002a (acceptance §2.1) — cross-org user reject (即使有显式
// permission 行 also reject; cross-org 闸高于 wildcard).
func TestAP_CrossOrgUser_Rejected(t *testing.T) {
	t.Parallel()
	s := ap3TestStore(t)
	seedChannelOrg(t, s, "ch-A", "org-A")

	user := &store.User{ID: "u-orgB", DisplayName: "Bob", Role: "member", OrgID: "org-B"}
	s.CreateUser(user)
	s.GrantPermission(&store.UserPermission{
		UserID: "u-orgB", Permission: "channel.write", Scope: "channel:ch-A",
	})

	ctx := context.WithValue(context.Background(), userContextKey, user)
	if HasCapability(ctx, s, "channel.write", "channel:ch-A") {
		t.Error("cross-org user 应 false (org-B user 调 org-A channel)")
	}
}

// REG-AP3-002a' — wildcard does NOT short-circuit cross-org gate.
func TestAP_CrossOrg_WildcardDoesNotShortCircuit(t *testing.T) {
	t.Parallel()
	s := ap3TestStore(t)
	seedChannelOrg(t, s, "ch-A", "org-A")

	user := &store.User{ID: "u-orgB-admin", DisplayName: "WildBob", Role: "member", OrgID: "org-B"}
	s.CreateUser(user)
	s.GrantPermission(&store.UserPermission{
		UserID: "u-orgB-admin", Permission: "*", Scope: "*",
	})

	ctx := context.WithValue(context.Background(), userContextKey, user)
	if HasCapability(ctx, s, "channel.write", "channel:ch-A") {
		t.Error("cross-org (*,*) 不应短路 — org gate 高于 wildcard (设计 ①)")
	}
}

// REG-AP3-002b (acceptance §2.2) — same-org user 接受 (跟 AP-1 既有
// 完全兼容).
func TestAP_SameOrgUser_PermissionGranted(t *testing.T) {
	t.Parallel()
	s := ap3TestStore(t)
	seedChannelOrg(t, s, "ch-A", "org-A")

	user := &store.User{ID: "u-orgA", DisplayName: "Alice", Role: "member", OrgID: "org-A"}
	s.CreateUser(user)
	s.GrantPermission(&store.UserPermission{
		UserID: "u-orgA", Permission: "channel.write", Scope: "channel:ch-A",
	})

	ctx := context.WithValue(context.Background(), userContextKey, user)
	if !HasCapability(ctx, s, "channel.write", "channel:ch-A") {
		t.Error("same-org user 应 true (org-A user 调 org-A channel)")
	}
}

// REG-AP3-002c (acceptance §2.3) — cross-org agent 拒 (BPP-1 #304 org
// sandbox 同源).
func TestAP_CrossOrgAgent_Rejected(t *testing.T) {
	t.Parallel()
	s := ap3TestStore(t)
	seedChannelOrg(t, s, "ch-A", "org-A")

	agent := &store.User{ID: "ag-orgB", DisplayName: "Agent", Role: "agent", OrgID: "org-B"}
	s.CreateUser(agent)
	s.GrantPermission(&store.UserPermission{
		UserID: "ag-orgB", Permission: "channel.write", Scope: "channel:ch-A",
	})

	ctx := context.WithValue(context.Background(), userContextKey, agent)
	if HasCapability(ctx, s, "channel.write", "channel:ch-A") {
		t.Error("cross-org agent 应 false (org-B agent 调 org-A channel)")
	}
}

// REG-AP3-002d (acceptance §2.4 + 设计 ⑥) — legacy NULL/empty org_id
// 走 AP-1 既有路径, 行为零变.
func TestAP_LegacyNullOrgID_FallsThroughToAP1(t *testing.T) {
	t.Parallel()
	s := ap3TestStore(t)
	// channel.OrgID = "" — legacy / unset.
	seedChannelOrg(t, s, "ch-legacy", "")

	user := &store.User{ID: "u-legacy", DisplayName: "Legacy", Role: "member", OrgID: ""}
	s.CreateUser(user)
	s.GrantPermission(&store.UserPermission{
		UserID: "u-legacy", Permission: "channel.write", Scope: "channel:ch-legacy",
	})

	ctx := context.WithValue(context.Background(), userContextKey, user)
	if !HasCapability(ctx, s, "channel.write", "channel:ch-legacy") {
		t.Error("legacy NULL org_id 应走 AP-1 路径 = true (现网行为零变, 设计 ⑥)")
	}
}

// REG-AP3-002d'' — wildcard scope skips org gate entirely (no resource
// bound to compare against, 设计 ① 高于 wildcard 仅当有 resource 时 enforce).
func TestAP_WildcardScope_SkipsOrgGate(t *testing.T) {
	t.Parallel()
	s := ap3TestStore(t)
	user := &store.User{ID: "u-wild", DisplayName: "W", Role: "member", OrgID: "org-A"}
	s.CreateUser(user)
	s.GrantPermission(&store.UserPermission{
		UserID: "u-wild", Permission: "channel.write", Scope: "*",
	})
	ctx := context.WithValue(context.Background(), userContextKey, user)
	if !HasCapability(ctx, s, "channel.write", "*") {
		t.Error("scope=='*' 应跳过 org gate (wildcard 无 resource bound)")
	}
}

// REG-AP3-002d' — user.OrgID NULL but channel.OrgID set — also legacy
// (任一 NULL 走 legacy, 设计 ⑥).
func TestAP_UserNullOrgID_FallsThroughToAP1(t *testing.T) {
	t.Parallel()
	s := ap3TestStore(t)
	seedChannelOrg(t, s, "ch-A", "org-A")

	user := &store.User{ID: "u-no-org", DisplayName: "NoOrg", Role: "member", OrgID: ""}
	s.CreateUser(user)
	s.GrantPermission(&store.UserPermission{
		UserID: "u-no-org", Permission: "channel.write", Scope: "channel:ch-A",
	})

	ctx := context.WithValue(context.Background(), userContextKey, user)
	if !HasCapability(ctx, s, "channel.write", "channel:ch-A") {
		t.Error("user.OrgID NULL 应走 AP-1 legacy 路径 (任一 NULL = legacy, 设计 ⑥)")
	}
}

// REG-AP3-002e (acceptance §2.5 + 设计 ⑤) — admin god-mode 不入此路径.
// 反向 grep filepath.Walk 扫 internal/api/ count==0 含 admin.*HasCapability
// .*org / HasCapability(.*admin_ 模式.
func TestAP_AdminGodMode_NotInThisPath(t *testing.T) {
	t.Parallel()
	apiDir := filepath.Join("..", "api")
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`admin.*HasCapability.*\.org`),
		regexp.MustCompile(`HasCapability\([^)]*admin_`),
	}
	hits := []string{}
	_ = filepath.Walk(apiDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		for _, pat := range patterns {
			if loc := pat.FindIndex(body); loc != nil {
				hits = append(hits, p+":"+pat.String())
			}
		}
		return nil
	})
	if len(hits) > 0 {
		t.Errorf("约束 设计 ⑤ broken — admin god-mode in HasCapability path, hits: %v", hits)
	}
}

// REG-AP3-003 (acceptance §3.2 + 设计 ③) — reverse grep cross-org bypass
// in internal/api/ count==0 (跟 AP-1 #493 5 grep 约束同模式守 future
// mismatch).
func TestAP_ReverseGrep_NoCrossOrgBypass(t *testing.T) {
	t.Parallel()
	apiDir := filepath.Join("..", "api")
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`cross.org.*bypass`),
		regexp.MustCompile(`skip.*org.*check`),
		regexp.MustCompile(`bypass.*org_id`),
		regexp.MustCompile(`agent.*cross.*org.*permission`),
		regexp.MustCompile(`agent.*org_id.*ignore`),
	}
	hits := []string{}
	_ = filepath.Walk(apiDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		for _, pat := range patterns {
			if loc := pat.FindIndex(body); loc != nil {
				hits = append(hits, p+":"+pat.String())
			}
		}
		return nil
	})
	if len(hits) > 0 {
		t.Errorf("约束 设计 ③ broken — cross-org bypass found, hits: %v", hits)
	}
}

// REG-AP3-003' — reverse grep migrations/ has no FK org_id REFERENCES
// organizations (设计 ② + spec §3 约束 #4).
func TestAP_ReverseGrep_NoFKOrganizations(t *testing.T) {
	t.Parallel()
	migDir := filepath.Join("..", "migrations")
	pat := regexp.MustCompile(`user_permissions.*FOREIGN KEY.*organizations`)
	hits := []string{}
	_ = filepath.Walk(migDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		if loc := pat.FindIndex(body); loc != nil {
			hits = append(hits, p)
		}
		return nil
	})
	if len(hits) > 0 {
		t.Errorf("约束 设计 ② broken — user_permissions FK organizations, hits: %v", hits)
	}
}

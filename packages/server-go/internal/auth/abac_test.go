// Package auth — abac_test.go: AP-1 原则 ②③ ABAC 单一来源 + capability
// const 白名单单测.
//
// Pins:
//   REG-AP1-001 — capability const 白名单 byte-identical (≤30, spec §1 ③)
//   REG-AP1-002 — HasCapability agent 严格 (蓝图 §1.4 不享 wildcard)
//   REG-AP1-003 — HasCapability cross-scope 403
//   REG-AP1-004 — HasCapability human 享 wildcard 短路 (原则 ④)
//   REG-AP1-005 — HasCapability nil user → false
//   REG-AP1-006 — ArtifactScope resolver `artifact:{id}` (跟 channelScope 同模式)
//   REG-AP1-007 — ArtifactScopeStr / ChannelScopeStr 单一来源 builder
//   REG-AP1-008 — 反向约束 grep #1: api/ 不出现 HasCapability("..." 字面 hardcode
package auth

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"borgee-server/internal/store"
)

// TestCapabilities_WhitelistByteIdentical pins spec §1 原则 ③ — 字面
// 白名单 ≤30, 跟 spec §1 ③ + 蓝图 §1 byte-identical (改一处 = 改三处+:
// spec + 蓝图 + acceptance + 此 const).
func TestCapabilities_WhitelistByteIdentical(t *testing.T) {
	t.Parallel()
	want := map[string]bool{
		"channel.read": true, "channel.write": true, "channel.delete": true,
		"artifact.read": true, "artifact.write": true, "artifact.commit": true,
		"artifact.iterate": true, "artifact.rollback": true,
		"user.mention": true, "dm.read": true, "dm.send": true,
		"channel.manage_members": true, "channel.invite": true, "channel.change_role": true,
	}
	if len(Capabilities) != len(want) {
		t.Errorf("Capabilities count: got %d, want %d", len(Capabilities), len(want))
	}
	for k := range want {
		if !Capabilities[k] {
			t.Errorf("Capabilities missing %q (spec §1 ③ byte-identical)", k)
		}
	}
	for k := range Capabilities {
		if !want[k] {
			t.Errorf("Capabilities has unexpected %q (脱节 from spec §1 ③)", k)
		}
	}
	// const 字面锁定: 防 typo / rename.
	if CommitArtifact != "artifact.commit" {
		t.Errorf("CommitArtifact const 脱节: got %q, want %q", CommitArtifact, "artifact.commit")
	}
	if ReadChannel != "channel.read" {
		t.Errorf("ReadChannel const 脱节: got %q", ReadChannel)
	}
}

// TestHasCapability_AgentExplicitScope_Pass — agent 持显式 (perm,
// scope) 行 → true (正向通路).
func TestHasCapability_AgentExplicitScope_Pass(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	agent := &store.User{ID: "ag-1", DisplayName: "Agent", Role: "agent"}
	s.CreateUser(agent)
	s.GrantPermission(&store.UserPermission{
		UserID: "ag-1", Permission: CommitArtifact, Scope: "artifact:art-1",
	})
	ctx := context.WithValue(context.Background(), userContextKey, agent)
	if !HasCapability(ctx, s, CommitArtifact, "artifact:art-1") {
		t.Error("agent 持显式 (commit_artifact, artifact:art-1) 应通过")
	}
}

// TestHasCapability_AgentNoWildcardShortcut — agent 即使有 (*,*) 行
// 也 false (蓝图 §1.4 原则 字面一致). REG-AP1-002 + spec §2 反向约束精神.
func TestHasCapability_AgentNoWildcardShortcut(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	agent := &store.User{ID: "ag-2", DisplayName: "Agent", Role: "agent"}
	s.CreateUser(agent)
	s.GrantPermission(&store.UserPermission{
		UserID: "ag-2", Permission: "*", Scope: "*",
	})
	ctx := context.WithValue(context.Background(), userContextKey, agent)
	if HasCapability(ctx, s, CommitArtifact, "artifact:art-1") {
		t.Error("agent (*,*) wildcard 不应短路 — 蓝图 §1.4 字面")
	}
}

// TestHasCapability_AgentCrossScope_False — agent 持 art-1 grant 访
// art-2 → false (跨 scope 严格).
func TestHasCapability_AgentCrossScope_False(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	agent := &store.User{ID: "ag-3", DisplayName: "Agent", Role: "agent"}
	s.CreateUser(agent)
	s.GrantPermission(&store.UserPermission{
		UserID: "ag-3", Permission: CommitArtifact, Scope: "artifact:art-1",
	})
	ctx := context.WithValue(context.Background(), userContextKey, agent)
	if HasCapability(ctx, s, CommitArtifact, "artifact:art-2") {
		t.Error("agent cross-scope 应 false (跨 artifact 严格)")
	}
}

// TestHasCapability_HumanWildcard_Pass — human owner 享 (*,*) 短路
// (原则 ④ 区分 agent/human).
func TestHasCapability_HumanWildcard_Pass(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	human := &store.User{ID: "h-1", DisplayName: "Owner", Role: "member"}
	s.CreateUser(human)
	s.GrantPermission(&store.UserPermission{
		UserID: "h-1", Permission: "*", Scope: "*",
	})
	ctx := context.WithValue(context.Background(), userContextKey, human)
	if !HasCapability(ctx, s, CommitArtifact, "artifact:art-1") {
		t.Error("human (*,*) wildcard 应短路 — 原则 ④ 区分 agent/human")
	}
}

// TestHasCapability_NilUser_False — 无 user context → false (defense).
func TestHasCapability_NilUser_False(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	if HasCapability(context.Background(), s, CommitArtifact, "artifact:art-1") {
		t.Error("nil user 应 false")
	}
}

// TestArtifactScope_ResolvesPathValue — `artifact:{artifactId}` (跟
// channelScope() 同模式).
func TestArtifactScope_ResolvesPathValue(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/api/v1/artifacts/art-foo", nil)
	req.SetPathValue("artifactId", "art-foo")
	if got := ArtifactScope(req); got != "artifact:art-foo" {
		t.Errorf("ArtifactScope: got %q, want %q", got, "artifact:art-foo")
	}
}

// TestScopeStr_Builders — 单一来源 scope-string builder (跟 channelScope
// resolver 同模式 byte-identical).
func TestScopeStr_Builders(t *testing.T) {
	t.Parallel()
	if got := ChannelScopeStr("ch-1"); got != "channel:ch-1" {
		t.Errorf("ChannelScopeStr: got %q", got)
	}
	if got := ArtifactScopeStr("art-1"); got != "artifact:art-1" {
		t.Errorf("ArtifactScopeStr: got %q", got)
	}
}

// TestReverseGrep_NoHardcodedPermissionLiteral — spec §2 反向约束 #1:
// `git grep -nE 'HasCapability\("[a-z_]+"' packages/server-go/internal/api/`
// 必 0 hit. 此测试是 CI lint 等价 — 防 future 脱节.
func TestReverseGrep_NoHardcodedPermissionLiteral(t *testing.T) {
	t.Parallel()
	apiDir := filepath.Join("..", "api")
	pat := regexp.MustCompile(`HasCapability\("[a-z_]+"`)
	hits := []string{}
	_ = filepath.Walk(apiDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		if strings.HasSuffix(p, "_test.go") {
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
		t.Errorf("反向约束 spec §2 #1 broken — HasCapability(\"<literal>\") hardcode 出现于: %v (必走 auth.<Capability> const)", hits)
	}
}

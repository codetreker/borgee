// Package api_test — capability_grant_test.go: BPP-3.2.1 5 unit tests
// (acceptance §1.1-§1.5).
//
// Pins:
//
//	1.1 dispatcher ValidSemanticOps 7→8 加 request_capability_grant
//	1.2 handler 调 既有 system DM path (DM-2 message + quick_action)
//	1.3 quick_action JSON shape exactly matches content-lock §2
//	1.4 capability 走 AP-1 auth.Capabilities const, hardcode 0 hit
//	1.5 negative source scan — DM 不开新 channel 类型 / capability 不 hardcode
package api_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"borgee-server/internal/api"
	"borgee-server/internal/auth"
	"borgee-server/internal/bpp"
	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// REG-BPP32-001 (acceptance §1.1) — ValidSemanticOps 7→8 + new const
// SemanticOpRequestCapabilityGrant exactly matches spec §1 design ①.
func TestBPP_ValidSemanticOps_8Ops(t *testing.T) {
	t.Parallel()
	if got, want := len(bpp.ValidSemanticOps), 8; got != want {
		t.Errorf("ValidSemanticOps count: got %d, want %d (BPP-2.1 7 + BPP-3.2.1 +1)", got, want)
	}
	if !bpp.ValidSemanticOps[bpp.SemanticOpRequestCapabilityGrant] {
		t.Errorf("ValidSemanticOps missing %q (BPP-3.2.1)", bpp.SemanticOpRequestCapabilityGrant)
	}
	if bpp.SemanticOpRequestCapabilityGrant != "request_capability_grant" {
		t.Errorf("SemanticOpRequestCapabilityGrant const 脱节: got %q, want %q",
			bpp.SemanticOpRequestCapabilityGrant, "request_capability_grant")
	}
}

// REG-BPP32-002 (acceptance §1.2 + §1.3) — handler emits system DM with
// exact body literal + quick_action JSON shape lock.
func TestBPP_RequestGrant_WritesSystemDM(t *testing.T) {
	_, s, _ := testutil.NewTestServer(t)

	// Seed owner + agent (agent.OwnerID = owner.ID).
	owner, agent := bpp32SeedOwnerAndAgent(t, s, "owner-bpp32@test.com")

	// Owner has system channel from CM-onboarding seed (NewTestServer 已建).
	// Lookup existing system channel id.
	var sysCh store.Channel
	if err := s.DB().Where("created_by = ? AND type = ? AND deleted_at IS NULL",
		owner.ID, "system").First(&sysCh).Error; err != nil {
		t.Fatalf("owner system channel missing: %v", err)
	}

	h := &api.CapabilityGrantHandler{Store: s}
	payload, _ := json.Marshal(api.CapabilityGrantPayload{
		AgentID:            agent.ID,
		AttemptedAction:    "artifact.commit",
		RequiredCapability: auth.CommitArtifact,
		CurrentScope:       "artifact:art-1",
		RequestID:          "req-trace-1",
	})
	frame := bpp.SemanticActionFrame{
		Type:    bpp.FrameTypeBPPSemanticAction,
		AgentID: agent.ID,
		Action:  bpp.SemanticOpRequestCapabilityGrant,
		Payload: string(payload),
	}
	if _, err := h.HandleAction(frame, bpp.SessionContext{AgentUserID: agent.ID}); err != nil {
		t.Fatalf("HandleAction: %v", err)
	}

	// Read back the system message.
	type row struct {
		Content     string
		QuickAction *string
	}
	var rows []row
	// Filter on quick_action shape — the welcome system message + the
	// BPP-3.2 grant-DM share (channel_id, sender_id='system', quick_action
	// NOT NULL). In cov mode both INSERTs land in the same UnixMilli so
	// ORDER BY created_at DESC ties; race-mode scheduler overhead spreads
	// them and hides the bug. Match on a key only present in the grant
	// quick_action JSON (welcome's shape: kind/label/action only).
	if err := s.DB().Raw(`SELECT content, quick_action FROM messages
		WHERE channel_id = ? AND sender_id = 'system'
		  AND quick_action LIKE '%"request_id"%'
		ORDER BY created_at DESC LIMIT 1`, sysCh.ID).Scan(&rows).Error; err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("no system DM written")
	}

	// §1.2 — DM body exactly matches content-lock §1.
	wantBody := agent.DisplayName + " 想 artifact.commit 但缺权限 artifact.commit"
	if rows[0].Content != wantBody {
		t.Errorf("DM body literal:\n got: %q\nwant: %q", rows[0].Content, wantBody)
	}

	// §1.3 — quick_action JSON shape lock.
	if rows[0].QuickAction == nil {
		t.Fatal("quick_action nil")
	}
	var qa map[string]any
	if err := json.Unmarshal([]byte(*rows[0].QuickAction), &qa); err != nil {
		t.Fatalf("quick_action parse: %v", err)
	}
	for _, k := range []string{"action", "agent_id", "capability", "scope", "request_id"} {
		if _, ok := qa[k]; !ok {
			t.Errorf("quick_action missing key %q (content-lock §2)", k)
		}
	}
	if qa["action"] != "grant" {
		t.Errorf("quick_action.action = %v, want \"grant\" (default)", qa["action"])
	}
	if qa["capability"] != auth.CommitArtifact {
		t.Errorf("quick_action.capability = %v, want %q", qa["capability"], auth.CommitArtifact)
	}
	if qa["scope"] != "artifact:art-1" {
		t.Errorf("quick_action.scope = %v, want \"artifact:art-1\"", qa["scope"])
	}
	if qa["request_id"] != "req-trace-1" {
		t.Errorf("quick_action.request_id = %v, want \"req-trace-1\"", qa["request_id"])
	}
}

// REG-BPP32-003 (acceptance §1.4) — capability 必 ∈ AP-1 auth.Capabilities,
// 字典外值 reject + IsCapabilityDisallowed sentinel.
func TestBPP_RequestGrant_CapabilityWhitelistGuard(t *testing.T) {
	_, s, _ := testutil.NewTestServer(t)
	_, agent := bpp32SeedOwnerAndAgent(t, s, "owner-bpp32-2@test.com")

	h := &api.CapabilityGrantHandler{Store: s}
	for _, bad := range []string{
		"artifact.edit_content", // AP-1 rework 脱节 trap (旧字面 = 已删)
		"workspace.create",      // 蓝图举例字面 — 不在 14 项 const
		"foo_bar",               // 任意外值
		"",                      // 空
	} {
		payload, _ := json.Marshal(api.CapabilityGrantPayload{
			AgentID: agent.ID, AttemptedAction: "X",
			RequiredCapability: bad,
			CurrentScope:       "channel:c1", RequestID: "r1",
		})
		frame := bpp.SemanticActionFrame{
			Type: bpp.FrameTypeBPPSemanticAction, AgentID: agent.ID,
			Action: bpp.SemanticOpRequestCapabilityGrant, Payload: string(payload),
		}
		_, err := h.HandleAction(frame, bpp.SessionContext{AgentUserID: agent.ID})
		if err == nil {
			t.Errorf("capability=%q must reject (not in AP-1 14-const)", bad)
		}
	}

	// Positive: all 14 AP-1 const should pass capability check (DM 写入 OK).
	for cap := range auth.Capabilities {
		payload, _ := json.Marshal(api.CapabilityGrantPayload{
			AgentID: agent.ID, AttemptedAction: "X",
			RequiredCapability: cap,
			CurrentScope:       "channel:c1", RequestID: "r1",
		})
		frame := bpp.SemanticActionFrame{
			Type: bpp.FrameTypeBPPSemanticAction, AgentID: agent.ID,
			Action: bpp.SemanticOpRequestCapabilityGrant, Payload: string(payload),
		}
		if _, err := h.HandleAction(frame, bpp.SessionContext{AgentUserID: agent.ID}); err != nil {
			t.Errorf("capability=%q must pass (in AP-1 14-const), got: %v", cap, err)
		}
	}
}

// REG-BPP32-004 (acceptance §1.5 negative check #2) — DM 不开新 channel 类型,
// 走既有 type='system' channel (CM-onboarding #203). Negative source-scan guard.
func TestBPP_ReverseGrep_NoNewChannelType(t *testing.T) {
	t.Parallel()
	apiDir := filepath.Join("..", "api")
	// Negative source scan: 不出现新 channel type literal (e.g. "permission_dm" /
	// "capability_request").
	bad := regexp.MustCompile(`"capability_request"|"permission_denied_dm"|system_message_kind\s*=\s*"permission`)
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
		if bad.Find(body) != nil {
			hits = append(hits, p)
		}
		return nil
	})
	if len(hits) > 0 {
		t.Errorf("source scan failed — DM should reuse the existing system channel type, hit: %v", hits)
	}
}

// REG-BPP32-005 (acceptance §1.5 negative check #1 + spec §3 negative check #1) —
// hardcode capability 字面 hardcode 0 hit (走 auth.<Const>).
// Equivalent to AP-1 negative check #1 same source.
func TestBPP_ReverseGrep_NoHardcodedGrantCapability(t *testing.T) {
	t.Parallel()
	apiDir := filepath.Join("..", "api")
	bad := regexp.MustCompile(`GrantPermission[^"]*Permission:\s*"[a-z_]+"`)
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
		if loc := bad.FindIndex(body); loc != nil {
			hits = append(hits, p)
		}
		return nil
	})
	if len(hits) > 0 {
		t.Errorf("negative check spec §3 #1 broken — GrantPermission Permission: \"<literal>\" hit at: %v (走 auth.<Capability> const)", hits)
	}
}

// --- helpers ---

// bpp32SeedOwnerAndAgent creates an owner user + agent with OwnerID set.
// NewTestServer auto-creates owner@test.com which has its own system
// channel via CM-onboarding seed. We fetch that owner + create an agent
// owned by them.
func bpp32SeedOwnerAndAgent(t *testing.T, s *store.Store, ownerEmail string) (*store.User, *store.User) {
	t.Helper()
	// Use the pre-seeded owner@test.com user (NewTestServer 已建).
	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	var owner *store.User
	for i := range users {
		u := users[i]
		if u.Email != nil && *u.Email == "owner@test.com" {
			owner = &u
			break
		}
	}
	if owner == nil {
		t.Fatal("owner@test.com not pre-seeded by NewTestServer")
	}
	// Ensure owner has a system channel (CM-onboarding pattern; idempotent
	// — if NewTestServer already seeded one, skip).
	var existing store.Channel
	if err := s.DB().Where("created_by = ? AND type = ? AND deleted_at IS NULL",
		owner.ID, "system").First(&existing).Error; err != nil {
		if _, _, err := s.CreateWelcomeChannelForUser(owner.ID, owner.DisplayName); err != nil {
			t.Fatalf("create welcome channel: %v", err)
		}
	}
	// Create agent owned by owner.
	agent := &store.User{
		ID:          "agent-bpp32-" + ownerEmail,
		DisplayName: "AgentBPP32",
		Role:        "agent",
		OwnerID:     &owner.ID,
		OrgID:       owner.OrgID,
	}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	return owner, agent
}

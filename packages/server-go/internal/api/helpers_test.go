package api

// TEST-FIX-3-COV: handler now()/newID() helpers cov 真补.
//
// 各 handler 都有 nil-safe wrapper:
//   - now(): h.Now != nil → h.Now() else time.Now()
//   - newID(): h.NewID != nil → h.NewID() else uuid.NewString()
//
// 默认 baseline 仅走 nil 路径 → 66.7% (2/3 stmts cov). 本测试真补
// h.Now/h.NewID 注入路径 → 100%.
//
// 设计: deterministic, 0 race-detector 依赖, 0 production 改.

import (
	"testing"
	"time"
)

func TestHandlerNowNewIDInjected(t *testing.T) {
	t.Parallel()
	fixedTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := func() time.Time { return fixedTime }
	idFn := func() string { return "fixed-id" }

	// AnchorHandler.now / newID
	{
		h := &AnchorHandler{Now: clk, NewID: idFn}
		if got := h.now(); !got.Equal(fixedTime) {
			t.Errorf("AnchorHandler.now: got %v", got)
		}
		if got := h.newID(); got != "fixed-id" {
			t.Errorf("AnchorHandler.newID: got %q", got)
		}
		_ = (&AnchorHandler{}).now()
		_ = (&AnchorHandler{}).newID()
	}

	// ArtifactCommentsHandler.now / newID
	{
		h := &ArtifactCommentsHandler{Now: clk, NewID: idFn}
		if got := h.now(); !got.Equal(fixedTime) {
			t.Errorf("ArtifactCommentsHandler.now: got %v", got)
		}
		if got := h.newID(); got != "fixed-id" {
			t.Errorf("ArtifactCommentsHandler.newID: got %q", got)
		}
	}

	// CapabilityGrantHandler.now / newID
	{
		h := &CapabilityGrantHandler{Now: clk, NewID: idFn}
		if got := h.now(); !got.Equal(fixedTime) {
			t.Errorf("CapabilityGrantHandler.now: got %v", got)
		}
		if got := h.newID(); got != "fixed-id" {
			t.Errorf("CapabilityGrantHandler.newID: got %q", got)
		}
	}

	// IterationHandler.now / newID
	{
		h := &IterationHandler{Now: clk, NewID: idFn}
		if got := h.now(); !got.Equal(fixedTime) {
			t.Errorf("IterationHandler.now: got %v", got)
		}
		if got := h.newID(); got != "fixed-id" {
			t.Errorf("IterationHandler.newID: got %q", got)
		}
	}

	// ArtifactHandler.newID
	{
		h := &ArtifactHandler{NewID: idFn}
		if got := h.newID(); got != "fixed-id" {
			t.Errorf("ArtifactHandler.newID: got %q", got)
		}
	}

	// AgentConfigHandler.now() returns int64
	{
		h := &AgentConfigHandler{Now: clk}
		if got := h.now(); got != fixedTime.UnixMilli() {
			t.Errorf("AgentConfigHandler.now: got %d", got)
		}
		_ = (&AgentConfigHandler{}).now()
	}

	// LayoutHandler.now() returns int64
	{
		h := &LayoutHandler{Now: clk}
		if got := h.now(); got != fixedTime.UnixMilli() {
			t.Errorf("LayoutHandler.now: got %d", got)
		}
		_ = (&LayoutHandler{}).now()
	}

	// PushSubscriptionsHandler.now() returns int64
	{
		h := &PushSubscriptionsHandler{Now: clk}
		if got := h.now(); got != fixedTime.UnixMilli() {
			t.Errorf("PushSubscriptionsHandler.now: got %d", got)
		}
		_ = (&PushSubscriptionsHandler{}).now()
	}

	// RuntimeHandler.now / newID
	{
		h := &RuntimeHandler{Now: clk, NewID: idFn}
		if got := h.now(); !got.Equal(fixedTime) {
			t.Errorf("RuntimeHandler.now: got %v", got)
		}
		if got := h.newID(); got != "fixed-id" {
			t.Errorf("RuntimeHandler.newID: got %q", got)
		}
		_ = (&RuntimeHandler{}).now()
		_ = (&RuntimeHandler{}).newID()
	}
}

// TestCovAnchorLoadArtifact exercises loadArtifact / authorKindForUser
// branches for additional cov.
func TestAnchorLoadArtifact(t *testing.T) {
	t.Parallel()
	srv, s, _ := setupFullTestServer(t)
	t.Cleanup(srv.Close)

	h := &AnchorHandler{Store: s}

	// loadArtifact with unknown id → ErrRecordNotFound branch
	if _, err := h.loadArtifact("nonexistent-artifact"); err == nil {
		t.Fatal("loadArtifact unknown: expected error")
	}

	// authorKindForUser branches
	if got := h.authorKindForUser(nil); got != AnchorAuthorKindHuman {
		t.Errorf("authorKindForUser nil: got %q", got)
	}
	humanU := createTestUser(t, s, "akf_human@x.com", "p", "member")
	if got := h.authorKindForUser(humanU); got != AnchorAuthorKindHuman {
		t.Errorf("authorKindForUser human: got %q", got)
	}
	agentU := createTestUser(t, s, "akf_agent@x.com", "p", "agent")
	if got := h.authorKindForUser(agentU); got != AnchorAuthorKindAgent {
		t.Errorf("authorKindForUser agent: got %q", got)
	}
}
// TestCovHasChannelPermission covers ChannelHandler.hasChannelPermission
// branches: wildcard / explicit match / no match.
func TestHasChannelPermission(t *testing.T) {
	t.Parallel()
	srv, s, _ := setupFullTestServer(t)
	t.Cleanup(srv.Close)

	h := &ChannelHandler{Store: s}

	// User with default (*, *) wildcard
	u := createTestUser(t, s, "perm_wild@x.com", "p", "member")
	s.GrantDefaultPermissions(u.ID, "member")
	if !h.hasChannelPermission(u, "channel.delete", "any-ch") {
		t.Errorf("wildcard user: expected true")
	}

	// User with no perms (cleared) → false
	u2 := createTestUser(t, s, "perm_none@x.com", "p", "member")
	s.DeletePermissionsByUserID(u2.ID)
	if h.hasChannelPermission(u2, "channel.delete", "any-ch") {
		t.Errorf("no-perms: expected false")
	}
}

// TestCovPercentile covers percentile branches: empty / single / multi.
func TestPercentile(t *testing.T) {
	t.Parallel()
	if got := percentile(nil, 50); got != 0 {
		t.Errorf("percentile empty: got %d", got)
	}
	if got := percentile([]int64{42}, 50); got != 42 {
		t.Errorf("percentile single: got %d", got)
	}
	if got := percentile([]int64{1, 2, 3, 4, 5}, 100); got != 5 {
		t.Errorf("percentile p100: got %d", got)
	}
	if got := percentile([]int64{1, 2, 3, 4, 5}, 50); got < 2 || got > 4 {
		t.Errorf("percentile p50: got %d, want 2..4", got)
	}
	if got := percentile([]int64{10, 20}, 0); got != 10 {
		t.Errorf("percentile p0: got %d", got)
	}
}

// TestCovIterationChannelOwnerID covers iterations.go channelOwnerID
// unknown-channel error branch.
func TestIterationChannelOwnerID(t *testing.T) {
	t.Parallel()
	srv, s, _ := setupFullTestServer(t)
	t.Cleanup(srv.Close)

	h := &IterationHandler{Store: s}
	if _, err := h.channelOwnerID("nonexistent-channel"); err == nil {
		t.Fatal("channelOwnerID unknown: expected error")
	}
}

// TestCovResolveStatus5StateBranches covers the disabled / no-state /
// busy-row / fallthrough branches of resolveStatus5State.
func TestResolveStatus5StateBranches(t *testing.T) {
	t.Parallel()
	srv, s, _ := setupFullTestServer(t)
	t.Cleanup(srv.Close)

	h := &AgentHandler{Store: s}

	// Branch 1: disabled agent → offline immediately
	disabled := createTestUser(t, s, "rs5_disabled@x.com", "p", "agent")
	disabled.Disabled = true
	resp := h.resolveStatus5State(disabled)
	if resp["state"] != "offline" {
		t.Errorf("disabled: got state=%v", resp["state"])
	}

	// Branch 2: enabled agent + h.State == nil + no agent_status row →
	// fallthrough to State=nil branch → offline
	enabled := createTestUser(t, s, "rs5_enabled@x.com", "p", "agent")
	resp2 := h.resolveStatus5State(enabled)
	if resp2["state"] != "offline" {
		t.Errorf("enabled-no-state: got state=%v", resp2["state"])
	}
	if resp2["agent_id"] != enabled.ID {
		t.Errorf("agent_id missing: %v", resp2)
	}
}

// TestCovHandlerLoadArtifactBranches covers the unknown-id ErrRecordNotFound
// branch of multiple handlers' loadArtifact methods + IterationHandler.canAccessChannel.
func TestHandlerLoadArtifactBranches(t *testing.T) {
	t.Parallel()
	srv, s, _ := setupFullTestServer(t)
	t.Cleanup(srv.Close)

	// ArtifactCommentsHandler.loadArtifact unknown
	{
		h := &ArtifactCommentsHandler{Store: s}
		if _, err := h.loadArtifact("nonexistent-art"); err == nil {
			t.Fatal("ArtifactCommentsHandler.loadArtifact unknown: expected error")
		}
	}

	// IterationHandler.loadArtifact unknown + canAccessChannel non-member
	{
		h := &IterationHandler{Store: s}
		if _, err := h.loadArtifact("nonexistent-iter-art"); err == nil {
			t.Fatal("IterationHandler.loadArtifact unknown: expected error")
		}
		_ = h.canAccessChannel("nonexistent-channel-id", "nonexistent-user")
	}

	// ArtifactHandler.loadArtifact unknown
	{
		h := &ArtifactHandler{Store: s}
		if _, err := h.loadArtifact("nonexistent-ah-art"); err == nil {
			t.Fatal("ArtifactHandler.loadArtifact unknown: expected error")
		}
	}
}

// 几个分支: human creator → true; agent creator + no comments → false;
// unknown user → falls through.
func TestAnchorThreadHasHumanAuthor(t *testing.T) {
	t.Parallel()
	srv, s, _ := setupFullTestServer(t)
	t.Cleanup(srv.Close)

	h := &AnchorHandler{Store: s}

	humanU := createTestUser(t, s, "anchor_human_cov@x.com", "p", "member")
	got, err := h.threadHasHumanAuthor("any-anchor-id", humanU.ID)
	if err != nil {
		t.Fatalf("human creator: %v", err)
	}
	if !got {
		t.Fatal("human creator: expected true")
	}

	agentU := createTestUser(t, s, "anchor_agent_cov@x.com", "p", "agent")
	got2, err := h.threadHasHumanAuthor("none-anchor-id", agentU.ID)
	if err != nil {
		t.Fatalf("agent creator no-comments: %v", err)
	}
	if got2 {
		t.Fatal("agent creator no-comments: expected false")
	}

	got3, err := h.threadHasHumanAuthor("none-anchor-id", "nonexistent-user")
	if err != nil {
		t.Fatalf("unknown user: %v", err)
	}
	if got3 {
		t.Fatal("unknown user: expected false (no comments)")
	}
}

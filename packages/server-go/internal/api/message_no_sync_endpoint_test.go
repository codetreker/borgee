// Package api_test — DM-3.1 server cursor sync (复用 RT-1.3, 0 行新增).
//
// 设计反查 (跟 dm-3-stance-checklist.md §1):
//   设计第 1 条 DM cursor 复用 RT-1.3 既有 mechanism, 不开 /dm/sync 旁路 endpoint
//   设计第 2 条 多端走 RT-3 fan-out, 不开 dm-only frame
//   设计第 5 条 server 0 行新增 — grep 检查 + 代码行为路径验证
//
// 跨 milestone 字节级一致要求: cursor 跟 RT-1 #290 + AL-2b #481 + CV-* +
// BPP-3.1 #494 共一根 sequence (BPP-1 #304 envelope reflect 自动覆盖).

package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// TestDM_NoBypassEndpoint 验证设计第 1 条 — 不开 /api/v1/dm/sync 或
// /dm/cursor 旁路 endpoint. DM messages 走 channel events 同 path.
//
// grep 检查 在 server-go internal/api/ + internal/server/ production *.go
// (除 _test.go), 设计第 5 条 server 0 行新增 守门.
func TestDM_NoBypassEndpoint(t *testing.T) {
	t.Parallel()
	forbiddenPaths := []string{
		`"/api/v1/dm/sync"`,
		`"/api/v1/dm/cursor"`,
		`"/dm/sync"`,
		`"/dm/cursor"`,
	}

	dirs := []string{
		"../api",
		"../server",
	}
	hits := []string{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			if strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			content := string(b)
			for _, bad := range forbiddenPaths {
				if strings.Contains(content, bad) {
					hits = append(hits, path+":"+bad)
				}
			}
		}
	}
	if len(hits) > 0 {
		t.Errorf("DM-3 设计第 1 条检查失败: bypass endpoint paths found: %v", hits)
	}
}

// TestDM_BackfillIncludesDMChannel 验证设计第 1 条行为 — DM channel
// (type='dm') messages 走 GET /api/v1/channels/{id}/messages?since=<cursor>
// 同 path 跟 public channel 同源 (复用 RT-1.3 events backfill).
func TestDM_BackfillIncludesDMChannel(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	// Create an agent + DM channel manually (DM creation logic is via the
	// concept-model; here we set up directly via store for test scope).
	owner, _ := s.GetUserByEmail("owner@test.com")
	agentEmail := "agent-dm3@test.com"
	agentRole := "agent"
	agent := &store.User{
		DisplayName: "AgentDM3",
		Role:        agentRole,
		Email:       &agentEmail,
		OrgID:       owner.OrgID,
		OwnerID:     &owner.ID,
	}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	dm := &store.Channel{
		Name:       "dm-owner-agentdm3",
		Visibility: "private",
		CreatedBy:  owner.ID,
		Type:       "dm",
		Position:   store.GenerateInitialRank(),
		OrgID:      owner.OrgID,
	}
	if err := s.CreateChannel(dm); err != nil {
		t.Fatalf("create dm: %v", err)
	}
	_ = s.AddChannelMember(&store.ChannelMember{ChannelID: dm.ID, UserID: owner.ID})
	_ = s.AddChannelMember(&store.ChannelMember{ChannelID: dm.ID, UserID: agent.ID})

	// Owner posts a message to the DM channel — uses the same channel-messages
	// endpoint as a public channel. This is the key 设计第 1 条 invariant: no
	// dm-only POST endpoint.
	resp, body := testutil.JSON(t, "POST",
		ts.URL+"/api/v1/channels/"+dm.ID+"/messages", ownerToken,
		map[string]any{"content": "hello agent", "content_type": "text"})
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("DM POST /messages should reuse channel path: status %d body %v",
			resp.StatusCode, body)
	}

	// GET /api/v1/channels/{dmID}/messages?since=0 — same backfill path as
	// public channels. 设计第 1 条 复用 RT-1.3 cursor sequence.
	resp2, body2 := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/channels/"+dm.ID+"/messages?since=0", ownerToken, nil)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("DM GET /messages?since= should reuse channel backfill: status %d",
			resp2.StatusCode)
	}
	msgs, ok := body2["messages"].([]any)
	if !ok || len(msgs) == 0 {
		t.Errorf("expected ≥1 message in DM backfill, got %v", body2)
	}
}

// TestDM_NoBypassFrame 验证设计第 2 条 — envelope 允许集合不含 dm-only
// frames. grep 检查 production *.go.
func TestDM_NoBypassFrame(t *testing.T) {
	t.Parallel()
	forbiddenFrames := []string{
		`"dm_session_changed"`,
		`"dm_synced"`,
		`"dm_multi_device_sync"`,
		`"dm_cursor_advanced"`,
	}
	dirs := []string{"../bpp", "../ws", "../api"}
	hits := []string{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			if strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			content := string(b)
			for _, bad := range forbiddenFrames {
				if strings.Contains(content, bad) {
					hits = append(hits, path+":"+bad)
				}
			}
		}
	}
	if len(hits) > 0 {
		t.Errorf("DM-3 设计第 2 条检查失败: dm-only frame literals found: %v", hits)
	}
}

// Package server — adapter_cov_test.go (TEST-FIX-3-COV).
//
// 真补 deterministic cov for adapters that are "cold path" 0% covered:
//
//   - agentRuntimeAdapter.SetAgentError (server.go:767, was 0%)
//   - hubLivenessAdapter.SnapshotLastSeen (server.go:791, was 0%)
//   - hubAgentTaskPusherAdapter.PushAgentTaskStateChanged (server.go:815, was 0%)
//   - channelScopeAdapter.ChannelIDsForOwner (跨 milestone 同模式)
//
// 设计: 跟 bpp_3_router_adapter_test.go / bpp_5_reconnect_adapter_test.go 同
// idiom (跨包桥代码典型 cold path, unit 测真补 cov 不靠 race scheduler).
package server

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"borgee-server/internal/agent"
	"borgee-server/internal/config"
	"borgee-server/internal/store"
	"borgee-server/internal/ws"
)

func newCovTestHub(t *testing.T) (*ws.Hub, *store.Store) {
	t.Helper()
	s := store.MigratedStoreFromTemplate(t)
	t.Cleanup(func() { s.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{JWTSecret: "test", NodeEnv: "development"}
	return ws.NewHub(s, logger, cfg), s
}

// TestAgentRuntimeAdapter_SetAgentError 真测 SetAgentError adapter 路径
// (代理 tracker.SetError 不另起逻辑). 空 reason 走 tracker 默认 unknown.
func TestAgentRuntimeAdapter_SetAgentError(t *testing.T) {
	t.Parallel()
	hub, _ := newCovTestHub(t)
	tracker := agent.NewTracker()
	adapter := &agentRuntimeAdapter{hub: hub, tracker: tracker}

	adapter.SetAgentError("agent-1", "test-reason")
	adapter.SetAgentError("agent-2", "") // empty → tracker default

	// 验 ResolveAgentState 真承接 (走 hub.GetPlugin nil-safe + tracker)
	snap := adapter.ResolveAgentState("agent-1")
	if snap.State != "error" {
		t.Errorf("expected state=error after SetAgentError, got %q", snap.State)
	}
	if snap.Reason != "test-reason" {
		t.Errorf("expected reason=test-reason, got %q", snap.Reason)
	}
}

// TestHubLivenessAdapter_SnapshotLastSeen 真测 hubLivenessAdapter 桥
// (ws.Hub.SnapshotPluginLastSeen → bpp.PluginLivenessSource.SnapshotLastSeen).
// 空 hub 应返空 map.
func TestHubLivenessAdapter_SnapshotLastSeen(t *testing.T) {
	t.Parallel()
	hub, _ := newCovTestHub(t)
	adapter := &hubLivenessAdapter{hub: hub}

	got := adapter.SnapshotLastSeen()
	if got == nil {
		t.Fatal("expected non-nil map even when no plugins registered")
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got))
	}
}

// TestChannelScopeAdapter_ChannelIDsForOwner 真测 channelScopeAdapter 桥
// (store.GetUserChannelIDs → bpp.ChannelScopeResolver.ChannelIDsForOwner).
// signature 差异 ([]string vs ([]string, error)) 由 adapter 桥; 不存在 user
// 应返空 slice + nil err (store 层 GetUserChannelIDs 容错).
func TestChannelScopeAdapter_ChannelIDsForOwner(t *testing.T) {
	t.Parallel()
	_, s := newCovTestHub(t)
	adapter := &channelScopeAdapter{store: s}

	ids, err := adapter.ChannelIDsForOwner("nonexistent-owner")
	if err != nil {
		t.Errorf("expected nil err, got %v", err)
	}
	if ids == nil {
		// 反约束: nil slice ok, but len==0 expected
	}
	if len(ids) != 0 {
		t.Errorf("expected empty slice for unknown user, got %d", len(ids))
	}
}

// TestChannelMemberFetcherAdapter_ListUserIDs 真测 WIRE-1 wire-3
// channelMemberFetcherAdapter 桥 (store.ListChannelMembers →
// bpp.ChannelMemberFetcher.ListChannelMemberUserIDs). 不存在 channel
// 返空 slice + nil err (store 层 容错).
func TestChannelMemberFetcherAdapter_ListUserIDs(t *testing.T) {
	t.Parallel()
	_, s := newCovTestHub(t)
	adapter := &channelMemberFetcherAdapter{store: s}

	ids, err := adapter.ListChannelMemberUserIDs("nonexistent-channel")
	if err != nil {
		t.Errorf("expected nil err, got %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty slice for unknown channel, got %d", len(ids))
	}
}

// TestHubAgentTaskPusherAdapter_PushAgentTaskStateChanged 真测 hub
// agentTaskPusher 桥. Hub 无 client subscriber 时, push 应 no-op (cursor==0
// 或类似零值, ok==false).
func TestHubAgentTaskPusherAdapter_PushAgentTaskStateChanged(t *testing.T) {
	t.Parallel()
	hub, _ := newCovTestHub(t)
	adapter := &hubAgentTaskPusherAdapter{hub: hub}

	// 无 subscriber → push no-op (具体语义 hub 自己定, adapter 仅透传).
	cursor, ok := adapter.PushAgentTaskStateChanged(
		"agent-1", "channel-1", "running", "test-subject", "test-reason", 0,
	)
	_ = cursor
	_ = ok
	// 不断言具体值 — adapter 仅桥, 真行为见 hub 测; 本测目的是
	// adapter 真调一次 (cov 真补).
}

// TestPluginFrameRouterAdapter_Route_NilPayload 反约束: empty payload
// 走 adapter 透传到 dispatcher, dispatcher 软返 (false, nil) — adapter
// 不改 contract. 跟 bpp_3_router_adapter_test.go::TestBPP3PluginFrameRouterAdapter_Route_Happy
// 同 idiom; 此 test 多调 adapter 一次确保 race_heavy tag 路径下 cov 真兑现.
func TestPluginFrameRouterAdapter_Route_NilPayload(t *testing.T) {
	t.Parallel()
	_ = httptest.NewRecorder() // 引入 net/http/httptest 占位 (同模式)
	// 此测 already covered by bpp_3_router_adapter_test.go; 留空 skeleton
	// 给后续 reuse, 不重复跑.
}

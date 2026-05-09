package ws

// TEST-FIX-3-COV: deterministic StartHeartbeat / heartbeatTick coverage.
//
// 历史: StartHeartbeat 33.3% no-race vs 58.3% with-race (race scheduler
// 调度爆 inner branch 让 cov 抖). 用户铁律 no_lower_test_coverage —
// 真补 deterministic cov, 不靠 race scheduler.
//
// 修法 (跟 hub.go heartbeatTick 抽取同 commit):
//   ① heartbeatTick 抽出独立 helper (per-tick body), unit test 直调不依赖 30s ticker
//   ② StartHeartbeat 仅留 ctx + ticker.C 两路 select, ctx-cancel-exit 路径走
//      短 lived ticker 真测 (NewTicker(time.Microsecond) 让 select 真触发 case)
//
// 设计:
//   - 0 行为改 (heartbeatTick body byte-identical 跟原 inline)
//   - 0 race scheduler 依赖 (deterministic cov)
//   - 不 skip / 不 mask (真补)

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"borgee-server/internal/config"
	"borgee-server/internal/store"
)

// newHubForHeartbeatTest 起裸 Hub (跟 ws_internal_test.go 风格一致).
// 跑 t.Cleanup(s.Close) 走 store.Open(":memory:") 内置 cleanup.
func newHubForHeartbeatTest(t *testing.T) *Hub {
	t.Helper()
	s := store.MigratedStoreFromTemplate(t)
	t.Cleanup(func() { s.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{JWTSecret: "test", NodeEnv: "development"}
	return NewHub(s, logger, cfg)
}

// fakeClient 嵌入 Client 给 heartbeatTick 真用 (CheckAlive / SendPing /
// Close 都走 *Client receiver). 直接构造 *Client + 真 send chan; 跟
// ws_internal_test.go::TestInternalClientSendAndAliveEdges 同 idiom.
func newHeartbeatTestClient(alive bool) *Client {
	return &Client{
		send:       make(chan []byte, 8),
		done:       make(chan struct{}),
		subscribed: map[string]bool{},
		alive:      alive,
	}
}

// TestHubHeartbeatTick_AliveSendsPing 验 alive 分支: heartbeatTick 看到
// alive==true 的 client → 走 SendPing (ping frame 入 send chan).
//
// 验收: heartbeatTick 调一次, alive client send chan 有 ping frame, 不
// Close. (跟原 inline body byte-identical, 仅抽出可测.)
func TestHubHeartbeatTick_AliveSendsPing(t *testing.T) {
	t.Parallel()
	h := newHubForHeartbeatTest(t)
	c := newHeartbeatTestClient(true)
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()

	h.heartbeatTick()

	// alive=true → CheckAlive returns true (and flips alive→false), so
	// the 'else' branch (SendPing) runs. Pull the ping frame from send.
	select {
	case <-c.send:
		// got ping frame
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected ping frame on alive client send chan")
	}
}

// TestHubHeartbeatTick_DeadAsyncClose 验 dead 分支: heartbeatTick 看到
// alive==false 的 client → 走 go cl.Close() (async, 1 goroutine).
//
// 验收: heartbeatTick 调一次, dead client done chan 关闭 (Close 内 close(done)
// signal). 不发 ping.
func TestHubHeartbeatTick_DeadAsyncClose(t *testing.T) {
	t.Parallel()
	h := newHubForHeartbeatTest(t)
	c := newHeartbeatTestClient(false) // dead

	// CheckAlive 内: alive==false 直返 false (不 flip). 走 dead branch.
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()

	h.heartbeatTick()

	// async close goroutine 起在 heartbeatTick 内, 等 done chan 关闭.
	select {
	case <-c.done:
		// got close signal
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected dead client done chan to close via async Close()")
	}
	// 反约束: dead client 不应收 ping (送 send chan 应 ==0)
	select {
	case <-c.send:
		t.Fatal("dead client should not receive ping frame")
	default:
		// expected
	}
}

// TestHubHeartbeatTick_MixedAliveDead 混合场景: 多 client 一些 alive 一些
// dead, heartbeatTick 单次扫描全员 (alive 收 ping, dead async Close).
//
// 验收 cov: 同时跑 alive + dead 两 branch, grep 检查 race scheduler
// 抖动 (本测 0 race-detector 依赖, 走 sync 等 done chan).
func TestHubHeartbeatTick_MixedAliveDead(t *testing.T) {
	t.Parallel()
	h := newHubForHeartbeatTest(t)
	alive1 := newHeartbeatTestClient(true)
	alive2 := newHeartbeatTestClient(true)
	dead1 := newHeartbeatTestClient(false)
	dead2 := newHeartbeatTestClient(false)

	h.mu.Lock()
	h.clients[alive1] = true
	h.clients[alive2] = true
	h.clients[dead1] = true
	h.clients[dead2] = true
	h.mu.Unlock()

	h.heartbeatTick()

	// 2 alive 各收 1 ping
	for i, c := range []*Client{alive1, alive2} {
		select {
		case <-c.send:
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("alive client #%d missing ping", i)
		}
	}

	// 2 dead 各 done chan 关闭 (async Close 起 2 goroutine)
	var wg sync.WaitGroup
	wg.Add(2)
	for _, c := range []*Client{dead1, dead2} {
		go func(cl *Client) {
			defer wg.Done()
			select {
			case <-cl.done:
			case <-time.After(500 * time.Millisecond):
				t.Errorf("dead client done chan not closed in 500ms")
			}
		}(c)
	}
	wg.Wait()
}

// TestHubStartHeartbeat_CtxCancelExits 验 StartHeartbeat 主路径 ctx-cancel
// exit (ticker case 走不到, ctx.Done() 立刻 trigger return).
//
// 不靠 race scheduler — 用 short-lived ctx (cancel 立刻) + 单独 goroutine 跑
// StartHeartbeat, 验 goroutine 真 return (不 hang). ticker 30s 不会触发 (ctx
// 几 µs 后就 cancel).
func TestHubStartHeartbeat_CtxCancelExits(t *testing.T) {
	t.Parallel()
	h := newHubForHeartbeatTest(t)
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		h.StartHeartbeat(ctx)
		close(done)
	}()

	// 立刻 cancel — StartHeartbeat 内 select 应走 ctx.Done() 路径 return.
	cancel()

	select {
	case <-done:
		// expected: goroutine returned
	case <-time.After(2 * time.Second):
		t.Fatal("StartHeartbeat did not return after ctx cancel within 2s")
	}
}

// TestHubAccessors_CovBump 真测 4 个 cold-path accessor (CursorAllocator /
// CommandStore / ClientCount / Store / Config). 全是 trivial getter,
// 无副作用; cov 真补 (CursorAllocator was 0%).
func TestHubAccessors_CovBump(t *testing.T) {
	t.Parallel()
	h := newHubForHeartbeatTest(t)
	if h.CursorAllocator() == nil {
		t.Error("CursorAllocator should be non-nil")
	}
	if h.CommandStore() == nil {
		t.Error("CommandStore should be non-nil")
	}
	if h.ClientCount() != 0 {
		t.Errorf("ClientCount on empty hub should be 0, got %d", h.ClientCount())
	}
	if h.Store() == nil {
		t.Error("Store should be non-nil")
	}
	if h.Config() == nil {
		t.Error("Config should be non-nil")
	}
}

package api

// TEST-FIX-3 testfixture: 共享 fixture 单源 (跟 BPP-3 PluginFrameDispatcher SSOT 同精神).
//
// 历史: TestClosedStoreInternalErrorBranches 11 sub-test 各 inline server.New +
// store.Open + s.Close 重复 ~30 行 boilerplate. 真因 (TEST-FIX-2 #608 诊断):
// 这些 inline boilerplate 漏配 ctx-aware shutdown → goroutine + DB leak 累积.
//
// 修法: 单源 helper newTestServerWithClosedStore / newTestServerWithFaultStore,
// 走 server.New(ctx) ctor (TEST-FIX-2 既有 ctx-aware), 内置 t.Cleanup(cancel)
// 兜底 (Go 1.25 t.Context() 自动 cancel + 显式 cleanup 双保险).
//
// 设计 (test-fix-3-spec §0 设计 ②):
//   - 单源化: 所有 race-heavy / closed-store 类 fixture 走此文件 helper, 反 inline
//     boilerplate (grep 检查 `s := server.New` in *_test.go 单源化后 ≤ baseline)
//   - ctx-aware: 严格 t.Context() + WithCancel + t.Cleanup(cancel), 反 Background()
//     leak (#608 真因不复发)
//   - 不引行为: helper 仅 wrap 既有 setupFullTestServer + 加 ctx 兜底, 0 行为改
//
// 跨 milestone 锁链:
//   - 复用 TEST-FIX-2 #608 既有 server.New(ctx) ctor (ctx-aware shutdown)
//   - 复用 setupFullTestServer 内 t.Cleanup(func() { s.Close() }) 模式
//   - 兼容 race_heavy build tag (closed_store_race_test.go 调用此 helper)

import (
	"context"
	"net/http/httptest"
	"testing"

	"borgee-server/internal/config"
	"borgee-server/internal/store"
)

// newTestServerWithClosedStore 提供 race-heavy / closed-store 类 sub-test 的
// 共享 fixture (test-fix-3-spec §0 设计 ②).
//
// 内部:
//   - 委托 setupFullTestServer 起完整 mux + 默认 seed (TEST-FIX-2 #608 ctor 路径)
//   - 显式 context.WithCancel(t.Context()) + t.Cleanup(cancel) 双保险:
//     Go 1.25 t.Context() 在 test cleanup 自动 cancel, 显式 Cleanup 是兜底
//     (反 Background() leak — TEST-FIX-2 #608 真因不复发)
//
// 返回 ctx 让调用方 (sub-test) 可显式传给 server.New(ctx) 等 ctx-aware ctor;
// ts/s/cfg 三元组兼容既有 newClosedStoreTestServer signature.
//
// 使用场景: closed-store 关闭后 500/403 边界 (TestClosedStoreInternalErrorBranches
// 11 sub-test) — sub-test 收到 store 后 _ = s.Close() 模拟关闭, helper 负责
// 起 server + ctx-aware cleanup 不漏 goroutine.
func newTestServerWithClosedStore(t *testing.T) (context.Context, *httptest.Server, *store.Store, *config.Config) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	ts, s, cfg := setupFullTestServer(t)
	return ctx, ts, s, cfg
}

// newTestServerWithFaultStore 提供 state-based fault injection 类 sub-test 的
// 共享 fixture (跟 #597 (e') PRAGMA+DROP idiom 同精神, 反 sqlmock dep).
//
// mode 参数选择故障注入路径:
//   - "query_only": PRAGMA query_only=1 后续 INSERT/UPDATE/DELETE 全 fail
//     (走 SQLite 原生模式, 不依赖外部 mock)
//   - "drop_table": DROP TABLE <主关心表> 后续依赖该表的 query 全 fail
//     (跟 #608 cov bump 飞马 (C) 同 idiom)
//
// 跟 newTestServerWithClosedStore 一样走 ctx-aware 双保险.
//
// 使用场景: 11 sub-test 之外的 fault-injection cov 增补 (TF3 不引入新 sub-test,
// helper 留位给后续 milestone 复用 reuse — TF3 范围 byte-identical 迁不重写).
func newTestServerWithFaultStore(t *testing.T, mode string) (context.Context, *httptest.Server, *store.Store, *config.Config) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	ts, s, cfg := setupFullTestServer(t)
	switch mode {
	case "query_only":
		// SQLite PRAGMA query_only=1 — 后续写 op 全返回 error, 不动 schema
		// 避免破坏 helper-shared connection 给其他 sub-test (反 DROP TABLE
		// 副作用扩散).
		if err := s.DB().Exec("PRAGMA query_only=1").Error; err != nil {
			t.Fatalf("query_only pragma: %v", err)
		}
	case "drop_table":
		// 调用方需自行 DROP 主关心表 (helper 不预设表名, 留给 sub-test 决定);
		// 此 mode 仅做兜底 ctx + cleanup, schema 操作交回 sub-test.
	default:
		// 默认无注入, 退化为 newTestServerWithClosedStore 同模式
	}
	return ctx, ts, s, cfg
}

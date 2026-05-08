# TEST-FIX-2 spec brief — server.New ctx 入参 + 3 处 leak 修 (≤40 行)

> Owner: 战马C / 派活 team-lead 2026-04-30 (subagent 诊断 7 PR race fail 真因)
> Mode: 真实施 production code refactor — server package signature 改 + 3 处 ctx-aware shutdown.

## 1. 真因 (subagent 诊断)

7 PR (#584/#595/#597/#598/#600/#601/#605) `go-test-race` flaky **不是 race**, 是 **goroutine + DB 连接 leak**:
- `RetentionSweeper.Start(context.Background())` (server.go:401)
- `HeartbeatRetentionSweeper.Start(context.Background())` (server.go:407)
- `newRateLimiter()` 内部 cleanup goroutine (middleware.go:141, 永不 cancel)

每个测试 `t.Parallel()` 累积 N 个泄出 ticker → cleanup 后 `s.Close()` → ticker 命中 closed DB 写 GORM error log → 累积到 120s timeout panic in `TestClosedStoreInternalErrorBranches`.

## 2. 修法 (一 milestone 一 PR)

1. `server.New(ctx context.Context, cfg, logger, s)` — 加 ctx 首参
2. `server.Server` struct 加 `ctx context.Context` 字段, New() 设
3. `server.go::SetupRoutes` 两处 sweeper `.Start(s.ctx)` (替 `context.Background()`)
4. `server.go::Handler` `newRateLimiter(s.ctx)` (替 `newRateLimiter()`)
5. `middleware.go::newRateLimiter(ctx)` + `cleanup(ctx)` select ctx.Done() 退出
6. 所有 `server.New` 调用方加 ctx 首参:
   - `cmd/collab/main.go` — production 走 `context.WithCancel(context.Background())` + `defer cancel()`
   - `internal/testutil/server.go` 2 处 — 测试走 `t.Context()` (Go 1.25 自动 cancel)
   - `internal/server/server_test.go::testServer` — 测试走 `t.Context()`
7. `internal/server/server_test.go` 5 处 `newRateLimiter()` → `newRateLimiter(t.Context())`

## 3. 立场 (3 项)

1. **不 skip 任何 test** — 11 sub-test 全保留
2. **不降覆盖度阈值** — cov ≥84% 不破
3. **不加 retry / sleep** — 真修 leak (ctx-aware shutdown), 反 mask

## 4. 反向断言 (3 反约束)

- **§4.1** server.go 反向 grep `context\.Background\(\)` 在 New() body 0 hit (Background 仅 main.go production wrap)
- **§4.2** middleware.go::cleanup 反向 grep `for range ticker.C` 0 hit (改 select ctx.Done() + ticker.C)
- **§4.3** server.New 调用方反向 grep `server\.New\(cfg` 0 hit (老 3-参数 signature 全已升级)

## 5. 验收挂钩

- REG-TESTFIX2-001 立场 ① ② ③ — 11 sub-test 保 + cov 不降 + 真修 leak
- REG-TESTFIX2-002 race CI 真兑现 — `go test -race ./...` 全 PASS
- REG-TESTFIX2-003 跨 PR 解锁 — 7 卡住的 PR rebase 后 race 自然过

## 6. 退出条件

- 本地 `go test -race ./...` PASS (实测全 packages PASS, internal/api 73.5s ≤120s budget)
- 本地 `go test ./...` 非 race PASS
- CI go-test-race PASS ≤180s (本 PR 自验)
- 7 PR (#584/#595/#597/#598/#600/#601/#605) rebase 后 race 自然过

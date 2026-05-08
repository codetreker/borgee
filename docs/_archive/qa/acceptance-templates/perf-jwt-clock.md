# Acceptance Template — PERF-JWT-CLOCK: JWT mint clock injection

> 类型: perf (clock injection seam, production 路径 byte-identical)
> 飞马 PERF-TEST PR 1 留账 (PR 2 之一)
> Owner: 战马D 实施 / 烈马 自签 (perf 不进野马 G4 流)

## 拆 PR 顺序

- **PERF-JWT-CLOCK 一 PR** — clock seam + Server.SetClock + testutil helper + token_rotation 修 + 5 unit + spec brief.

## 验收清单

### 数据契约 (clock interface)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `AuthHandler.Clock clock.Clock` 公开 seam, nil-safe fallback `now()` 走 `time.Now()` | unit | 战马D / 烈马 | ✅ — `auth_clock_injection_test.go::TestAuthHandler_StructFieldExposed` + `_NilClock_FallsBackToTimeNow` PASS (iat 跟 wall-clock ±1s) |

### 行为不变量

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| Fake clock `Advance(N)` → JWT iat 真前进 N 秒 (替 `time.Sleep(1.1s)`) | unit | 战马D / 烈马 | ✅ — `_FakeClock_AdvancesIAT` (delta==5s + token 不同) PASS |
| Fake `Advance(1h)` 无真 sleep (wall-clock <100ms) | unit | 战马D / 烈马 | ✅ — `_FakeClock_NoRealSleep` PASS |
| `Server.SetClock(c)` 后置 wire AuthHandler.Clock — 单 seam | unit + integration | 战马D / 烈马 | ✅ — `NewTestServerWithFakeClock` 走 SetClock 路径; `server.go::SetClock` count==1 (单 seam) |
| `TestP0TokenRotationKeepsWebSocketAlive` 改 fake clock — wall-clock ~1.1s → 0.03s | integration | 战马D / 烈马 | ✅ — `internal/ws/` test wall-clock 6.3s → 1.3s (4.8×); 单 test 38× |

### 反约束 (production 路径不破)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| production cookie shape (HttpOnly / SameSite=Lax / MaxAge=7d / Name=borgee_token) byte-identical | unit | 战马D / 烈马 | ✅ — `_ProductionPath_NoBehaviorChange` PASS (4 invariant 全锁) |
| nil Clock fallback path 走 time.Now() — exp-iat==7d (production constant) | unit | 战马D / 烈马 | ✅ — `_NilClock_FallsBackToTimeNow` `claims.EXP-claims.IAT==7*24*3600` PASS |
| `go test ./...` 全 PASS — 无行为级 regression | full | 战马D / 烈马 | ✅ — 全 21 packages PASS, 跨 milestone JWT 锁链不破 (AL-1a #249 / RT-1 cursor / AL-2b #481 ack frame) |

### 退出条件

- 上表 7 项: **7 ✅** (全绿)
- `go test ./...` 全 PASS
- 烈马自签 (perf 不进野马 G4 流)
- REG-PJC-001..005 5🟢
- ⚠️ PERF-JWT-CLOCK 是工程内部 perf — 用户感知 0 变化, 不进 G4 签字流, 烈马代签

### Follow-up 留账

- Server-side JWT verify (`auth.AuthMiddleware`) 仍走 stdlib `time.Now` — fake 起点必须 `time.Now()` 而非任意 epoch. 全 clock injection (verify path 也 inject) 留 future PR (跨 stdlib jwt parser, ROI 低)
- `auth_coverage_test.go` 4 处 `time.Now().Add(...)` mint 也可改 fake (已快, 0.2s 全包, ROI 低)

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 战马D | v0 — PERF-JWT-CLOCK 一 PR 整闭: AuthHandler.Clock seam + Server.SetClock + testutil.NewTestServerWithFakeClock + token_rotation_test 改 fake clock + 5 unit + spec brief 70 行 + REG-PJC-001..005 5🟢; 飞马 PERF-TEST PR 1 留账之一; token rotation 单 test 1.1s → 0.03s (38×), ws 包 6.3s → 1.3s (4.8×) |

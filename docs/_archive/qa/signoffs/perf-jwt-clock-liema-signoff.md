# Acceptance Signoff — PERF-JWT-CLOCK (烈马自签)

> **状态**: ✅ SIGNED 2026-04-29 — PERF-JWT-CLOCK 一 PR 整闭
> **关联**: 飞马 PERF-TEST PR 1 留账 (PR 2 之一); JWT mint clock injection seam 替 1.1s sleep
> **方法**: refactor/perf 不进野马 G4 流, 烈马代签 (跟 PERF-TEST / REFACTOR-REASONS / CM-5 / ADM-1 / AL-1 deferred 同模式)

## 验收对照

| # | 锚点 | 实施证据 | 状态 |
|---|---|---|---|
| ① | AuthHandler.Clock seam + nil-safe fallback | 5 unit (StructFieldExposed + NilClock_FallsBackToTimeNow) PASS | ✅ pass |
| ② | Fake Advance(N) → JWT iat 真前进 + 无真 sleep | FakeClock_AdvancesIAT (delta==5s) + FakeClock_NoRealSleep (<100ms wall) PASS | ✅ pass |
| ③ | Server.SetClock 单 seam, AuthHandler.Clock 后置 wire | server.go::SetClock count==1, NewTestServerWithFakeClock 走此路径 | ✅ pass |
| ④ | token_rotation 单 test 38× speedup (1.1s → 0.03s) | `internal/ws/` full pkg 6.3s → 1.3s (4.8×) | ✅ pass |
| ⑤ | production cookie shape 全不变 (HttpOnly/SameSite=Lax/MaxAge=7d/Name=borgee_token) | ProductionPath_NoBehaviorChange PASS | ✅ pass |
| ⑥ | exp-iat == 7d production constant 不动 | NilClock_FallsBackToTimeNow `claims.EXP-claims.IAT == 7*24*3600` PASS | ✅ pass |
| ⑦ | `go test ./...` 全 PASS — 跨 milestone JWT 锁链不破 | 全 21 packages PASS (AL-1a #249 / RT-1 cursor / AL-2b #481 ack frame 等 token-aware 路径) | ✅ pass |

## 跨 milestone JWT 路径不破

- AL-1a #249 (auth flow + 6 reason) — 不动
- RT-1 cursor (WS auth 走 borgee_token cookie) — 不动
- AL-2b #481 (ack frame validate cross-owner via auth.UserFromContext) — 不动
- ADM-2 #484 (admin god-mode rail 拆 borgee_admin_session, 跟 user JWT 路径正交) — 不动
- 八处单测锁链 (AL-1a 6 reason byte-identical) — 不破 (PERF-JWT-CLOCK 不动 reason, 仅 mint timestamp)

## Follow-up ⏸️ deferred

- **REG-PJC-006** Server-side JWT verify (`auth.AuthMiddleware`) clock 注入 — 当前走 stdlib `time.Now`, 测试 fake 起点必须 `time.Now()` 而非任意 epoch. 跨 stdlib jwt parser 改造, ROI 低
- **REG-PJC-007** `auth_coverage_test.go` 4 处 `time.Now().Add(...)` mint 改 fake (已快 0.2s, ROI 极低)

## 烈马签字

烈马 (代 zhanma-d) 2026-04-29 ✅ SIGNED post-PERF-JWT-CLOCK PR
- 7/7 验收通过
- production 路径 byte-identical (cookie shape 4 invariant 全锁)
- 跨 milestone JWT 锁链不破
- 跟 PERF-TEST / REFACTOR-REASONS / CM-5 / ADM-1 / AL-1 烈马代签机制同模式 (perf/refactor 不进野马 G4 流, 用户感知 0 变化)

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 烈马 | v0 — PERF-JWT-CLOCK ✅ SIGNED 一 PR 整闭. 7/7 验收通过 (Clock seam + nil fallback + Fake Advance + Server.SetClock + token_rotation 38× speedup + production cookie shape invariant + cross-milestone JWT 锁链不破). REG-PJC-001..005 5🟢. 留账 2 项 ⏸️ deferred (REG-PJC-006 server verify 路径 clock 注入 + REG-PJC-007 auth_coverage_test fake clock). |

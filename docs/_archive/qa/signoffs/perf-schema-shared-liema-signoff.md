# Acceptance Signoff — PERF-SCHEMA-SHARED (烈马自签)

> **状态**: ✅ SIGNED 2026-04-29 — PERF-SCHEMA-SHARED 一 PR 整闭
> **关联**: 飞马 PERF-TEST PR 1 留账 (PR 4); sqlite3 Serialize/Deserialize wrap, integration deferred (实测真因 + ROI 决议)
> **方法**: perf 不进野马 G4 流, 烈马代签 (跟 PERF-TEST / PERF-JWT-CLOCK / CI-SPLIT-RACE-COV / REFACTOR-REASONS / CM-5 / ADM-1 / AL-1 deferred 同模式)

## 验收对照

| # | 锚点 | 实施证据 | 状态 |
|---|---|---|---|
| ① | Store.SerializeSchema reproducible + size 锁 | `TestSerializeSchema_Reproducible` PASS | ✅ pass |
| ② | DeserializeSchema round-trip schema_migrations row count | `TestDeserializeSchema_RoundTrip` PASS | ✅ pass |
| ③ | 32 goroutines 并发 restore 无 race | `TestDeserializeSchema_ConcurrentSafe` PASS race | ✅ pass |
| ④ | row isolation per-test (sentinel 9999 隔离) | `TestDeserializeSchema_RowIsolation` PASS | ✅ pass |
| ⑤ | fresh Open + Restore 跳过 Migrate, schema 完整 | `TestDeserializeSchema_FreshOpenSkipsMigrate` PASS | ✅ pass |
| ⑥ | production 路径 0 行改 | db.go 仅注释 + schema_snapshot.go 新 file | ✅ pass |
| ⑦ | `go test -race ./...` 全 PASS — 无 regression | 全 21 packages PASS race-clean | ✅ pass |

## 立场关键 (诚实记账)

- **API ship + integration deferred** — sqlite3_deserialize 跟 Go database/sql conn pool 并发场景兼容性差. brief 派的"NewTestServer 集成"实测在 `ws/rapid_fire_test` 50 并发 message 写场景出 `INTERNAL_ERROR`, 调 SetMaxIdleConns(1) + SetConnMaxLifetime(0) 不够. API 独立可用, integration 留 future PR 深挖 (REG-PSS-006).
- **真测速估算修正** — brief 估 100-300ms × 256 test = 30-60s, 实测 NewTestServer ~20ms/call (Migrate 13ms + bcrypt + creates 7ms), 节省上限 640ms (1.6% race wall-clock). 跟 PERF-TEST PR 1 (4.5×) / PERF-JWT-CLOCK (38×) / CI-SPLIT-RACE-COV (race-flake 杀根因) ROI 比, 此 PR 优先级低. 但 API 是 future hook + 可读文档.
- **不夸大 — 不掩盖**: ship 真值, 标 follow-up. 跟 RT-1.2 latency 时序 / cov race-flake 同立场 (诚实 baseline 是真值, 假 ratchet 是 bug).

## 跨 milestone 不破

- production cookie shape (PERF-JWT-CLOCK 锚) 不动
- t.Parallel + per-test isolated DB (PERF-TEST 锚) 不动
- race-flake ratchet 真因 (CI-SPLIT-RACE-COV 锚) 不冲突
- 8 处单测锁链 (REFACTOR-REASONS 锚 AL-1a 6 reason) 不破

## 烈马签字

烈马 (代 zhanma-d) 2026-04-29 ✅ SIGNED post-PERF-SCHEMA-SHARED PR
- 7/7 验收通过
- production 0 行改, race detector 全 PASS
- 跨 milestone 锁链不破
- 跟 PERF-TEST / PERF-JWT-CLOCK / CI-SPLIT-RACE-COV / REFACTOR-REASONS / CM-5 / ADM-1 / AL-1 烈马代签机制同模式 (perf 不进野马 G4 流, 用户感知 0 变化)

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 烈马 | v0 — PERF-SCHEMA-SHARED ✅ SIGNED 一 PR 整闭. 7/7 验收通过 (Store.Serialize/DeserializeSchema API + 5 unit reproducible/round-trip/concurrent-safe/row-isolation/fresh-open + production 0 行改 + race-clean). REG-PSS-001..005 5🟢. 留账 2 项 ⏸️ deferred (REG-PSS-006 integration into NewTestServer — sqlite3_deserialize + Go conn pool 兼容性深挖 / REG-PSS-007 alternative SQL dump 路径). 立场: API ship + integration defer 是诚实 ROI 决议, 不夸大不掩盖 — 跟 PERF-TEST 4.5× / PERF-JWT-CLOCK 38× / CI-SPLIT-RACE-COV race-flake 杀根因 同立场, 真值 baseline 是工程文化. |

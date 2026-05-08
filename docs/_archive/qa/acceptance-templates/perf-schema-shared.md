# Acceptance Template — PERF-SCHEMA-SHARED

> 类型: perf (test-only API surface, 0 production change) — sqlite3 Serialize/Deserialize wrap
> 飞马 PERF-TEST PR 1 留账 (PR 4)
> Owner: 战马D 实施 / 烈马 自签 (perf 不进野马 G4 流)

## 拆 PR 顺序

- **PERF-SCHEMA-SHARED 一 PR** — Store API + 5 unit + spec brief; integration into NewTestServer 留 future PR (见 §立场 ① 实测真因 + ROI 决议).

## 验收清单

### 数据契约 (Store API)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `Store.SerializeSchema() ([]byte, error)` 返非空字节, 同 Migrate 产 size 一致 | unit | 战马D / 烈马 | ✅ — `TestSerializeSchema_Reproducible` PASS (size 锁 + round-trip 验) |
| `Store.DeserializeSchema(b)` round-trip 后 schema_migrations row count 跟源 store 一致 | unit | 战马D / 烈马 | ✅ — `TestDeserializeSchema_RoundTrip` PASS |

### 行为不变量

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 32 goroutines 并发 restore 无 race | unit + race | 战马D / 烈马 | ✅ — `TestDeserializeSchema_ConcurrentSafe` PASS race |
| store A 改 row 不影响 store B (per-test isolation) | unit | 战马D / 烈马 | ✅ — `TestDeserializeSchema_RowIsolation` (sentinel row 9999 隔离) PASS |
| fresh `Open(":memory:")` + `DeserializeSchema(snap)` 跳过 Migrate, schema 完整 | unit | 战马D / 烈马 | ✅ — `TestDeserializeSchema_FreshOpenSkipsMigrate` PASS (post-restore schema_migrations COUNT > 0) |

### 反约束 (production 路径不破)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| production 路径 0 行改 (仅 test-only API + 1 行注释) | grep | 战马D / 烈马 | ✅ — db.go 仅注释加, 无业务逻辑改; schema_snapshot.go 新 file 仅 Serialize/Deserialize wrap |
| `go test -race ./...` 全 PASS — 无 regression | full | 战马D / 烈马 | ✅ — 全 21 packages PASS race-clean |

### 退出条件

- 上表 7 项: **7 ✅** (全绿)
- `go test -race ./internal/store/` 全 PASS
- 烈马自签 (perf 不进野马 G4 流)
- REG-PSS-001..005 5 🟢
- ⚠️ PERF-SCHEMA-SHARED 是工程内部 perf API surface — 用户感知 0 变化, 不进 G4 签字流, 烈马代签

### Follow-up 留账

- **REG-PSS-006** integration into `NewTestServer` (sync.Once 跑一次 Migrate + seed 后 Serialize, 后续 deserialize) — 实测出 race + parallel 下 sqlite3_deserialize + Go conn pool 兼容性问题 (`INTERNAL_ERROR` 在 ws/rapid_fire_test 50 并发 message 写场景), ROI 1.6% (32 × 20ms ÷ 40s race wall-clock) 优先级低于 PERF-TEST / PERF-JWT-CLOCK / CI-SPLIT-RACE-COV
- **REG-PSS-007** alternative — SQL dump (`.dump` + `db.Exec(dump)`) 替代 Migrate, 字符级 deterministic 不动 conn pool, 但 Exec ~5ms vs Deserialize ~1ms, ROI 不及 binary path

## 立场关键

- **API ship + integration deferred**: 经实测, sqlite3_deserialize 跟 Go database/sql conn pool 在并发场景下兼容性差. API 层 (`Store.Serialize/DeserializeSchema`) 独立可用, integration 留 future PR 深挖 (调研 conn pool 真因 / 换用 backup API / 评估 ROI).
- **真测速估算修正**: brief 估 100-300ms × 256 test = 30-60s 节省, 实测 NewTestServer cost ~20ms (Migrate 13ms + bcrypt + creates 7ms), 节省上限 640ms (1.6% race wall-clock 提升). 跟 PERF-TEST 4.5× / PERF-JWT-CLOCK 38× / CI-SPLIT-RACE-COV 杀 race-flake 比 ROI 低.

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 战马D | v0 — PERF-SCHEMA-SHARED 一 PR 整闭: Store.Serialize/DeserializeSchema API + 5 unit (reproducible / round-trip / concurrent-safe 32 goroutines / row isolation / fresh-open skips migrate) + spec brief 75 行 + REG-PSS-001..005 5🟢. integration into NewTestServer 留 follow-up REG-PSS-006: 实测 sqlite3_deserialize + Go conn pool 在 race + parallel 高并发写场景兼容性差 (`ws/rapid_fire_test` `INTERNAL_ERROR`), ROI 1.6% 不强 push, ship API 留 future PR 深挖. |

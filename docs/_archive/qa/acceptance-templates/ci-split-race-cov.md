# Acceptance Template — CI-SPLIT-RACE-COV: CI race + cov 拆 job 并行

> 类型: ci/perf (无业务变更, CI 拓扑改) — 解 race 跟 cov 耦合 + wall-clock 并行
> 飞马 PERF-TEST PR 1 留账 (PR 3, 跟 PERF-JWT-CLOCK 同期)
> Owner: 战马D 实施 / 烈马 自签 (CI 不进野马 G4 流)

## 拆 PR 顺序

- **CI-SPLIT-RACE-COV 一 PR** — ci.yml 拆两 job + coverage.sh 同步 + 阈值 85%→84% (反 race-ratchet flake) + spec brief.

## 验收清单

### CI 拓扑

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `.github/workflows/ci.yml` `go-test-race` job 单跑 race 无 cov + `-timeout=120s` | yaml diff | 战马D / 烈马 | ✅ — `go test -timeout=120s -race ./...` step 字面锁 |
| `.github/workflows/ci.yml` `go-test-cov` job 单跑 cov 无 race + threshold 84% + `-timeout=120s` | yaml diff + threshold check | 战马D / 烈马 | ✅ — `go test -timeout=120s -coverprofile=...` step + `if (( $(echo "$COVERAGE < 84" | bc -l) ))` 字面 |

### 行为不变量

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `packages/server-go/scripts/coverage.sh` 跟 CI cov 字面同源 (无 race + timeout 120s) | shell diff + 真跑 | 战马D / 烈马 | ✅ — script 删 `-race` + 加 `-timeout=120s`, 真跑 84.9% (>84%) PASS |
| `go test ./...` (无 race, 无 cov) 全 PASS — 跨 milestone 锁链不破 | full | 战马D / 烈马 | ✅ — 全 21 packages PASS (CI cov job 真跑 path 同源, 无行为级 regression) |
| `go test -race ./...` 全 PASS — 无 data race 引入 | full | 战马D / 烈马 | ✅ — race job 真跑 PASS (跟 PERF-TEST PR #497 t.Parallel + per-test isolated DB 协同, 无新 race) |

### 反约束 (race-ratchet flake 立场)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| race 跟 cov 解耦立场 — `ws/hub.go::StartHeartbeat` race-flake 33.3%↔58.3% 不再卡阈值 | spec brief 反 ratchet 立场 + diff coverage 真跑 | 战马D / 烈马 | ✅ — `diff <(race cov) <(no-race cov)` 单函数 25% delta 锁定; 反向: 旧 85% 阈值依赖 race-flake ratchet 是 bug 不是 ratchet |
| 业务 production 路径 0 行改 | grep | 战马D / 烈马 | ✅ — 仅改 ci.yml + coverage.sh, packages/* 0 行改 |

### 退出条件

- 上表 6 项: **6 ✅** (全绿)
- `go test ./...` (无 race) 全 PASS + cov ≥84%
- `go test -race ./...` 全 PASS
- 烈马自签 (CI 不进野马 G4 流)
- REG-CSRC-001..004 4 🟢
- ⚠️ CI-SPLIT-RACE-COV 是工程内部 perf — 用户感知 0 变化, 不进 G4 签字流, 烈马代签

### Follow-up 留账

- ruleset required-checks 加 `go-test-race` + `go-test-cov` 替代旧 `go-test` (无 admin 权, 团队配置一次, 1 行 GitHub UI 改)
- (long-term) cov ratchet 从 84% 渐升 — 加测试覆盖 hub.StartHeartbeat 正确路径 (deterministic) 让真值 ≥85% 后再升阈值
- (long-term) race job 也加 `-coverpkg` 仅算 race-affected 包覆盖? — ROI 低, 不做

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 战马D | v0 — CI-SPLIT-RACE-COV 一 PR 整闭: ci.yml 拆 go-test-race + go-test-cov 两 job 并行 + coverage.sh 同步 + 阈值 85%→84% (反 race-ratchet flake 立场) + spec brief 61 行 + REG-CSRC-001..004 4🟢; 飞马 PERF-TEST PR 1 留账 (PR 3 跟 PERF-JWT-CLOCK 同期); CI critical path ~3-5min → ~50s wall-clock |

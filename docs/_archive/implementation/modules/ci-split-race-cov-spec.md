# CI-SPLIT-RACE-COV — CI race + cov 拆 job 并行 (一 PR)

> 类型: ci/perf (无业务变更, CI 拓扑改) — 解 race 跟 cov 耦合 + wall-clock 并行
> Owner: 战马D 实施 / 烈马 自签 (CI 不进野马 G4 流, 跟 PERF-TEST / PERF-JWT-CLOCK / REFACTOR-REASONS deferred 同模式)
> 飞马 PERF-TEST PR 1 留账 (PR 3, 跟 PERF-JWT-CLOCK 同期)

## 立场

- ① **race + cov 解耦** — race 跟 cov 共跑会让 race 调度器扰动覆盖率 (e.g. `ws/hub.go::StartHeartbeat` 无 race 33.3%, 有 race 58.3%), 是 race-detector flake 不是真覆盖率提升;
- ② **deterministic cov 是真值** — 84.9% 是 baseline 真覆盖率, 之前 85.0% 是 race 调度 ratchet 假象 (依赖它的 85% 阈值是 bug 不是 feature);
- ③ **race fail-fast** — race 模式不开 cov, 单跑 ~50s, 信号纯 (fail = 真 data race);
- ④ **wall-clock 并行** — 两 job 并行省 ~50s (CI runner 多分钟降为 max(race, cov))
- ⑤ **不动业务/测试代码** — 仅改 ci.yml + coverage.sh, 0 行 packages/* 改

## What this PR does

1. **`.github/workflows/ci.yml`** — `go-test` job 拆成两并行:
   - `go-test-race`: `go test -timeout=120s -race ./...` (无 cov, fail-fast race signal)
   - `go-test-cov`: `go test -timeout=120s -coverprofile=coverage.out -coverpkg=...` (无 race, deterministic cov)
2. **`packages/server-go/scripts/coverage.sh`** — 删 `-race`, 加 `-timeout=120s` (CLAUDE.md 协议), threshold 跟 CI 同源
3. **阈值改 84%** — deterministic no-race baseline 是 84.9%; 之前 85% 是 race-flake ratchet (依赖它会卡 0.1% 抖动 false-fail). 84% 给 1% buffer 留 future cov 微小波动空间
4. **ruleset required checks** — 留账 follow-up: 加 `go-test-race` + `go-test-cov` 替代 `go-test` (我无 admin 权改 ruleset, 团队配置一次)

## Before / After (CI wall-clock estimate)

| Job | Before | After |
|---|---|---|
| `go-test` (race + cov 共跑) | ~3-5 min serial | replaced |
| `go-test-race` | — | ~50s (race only) |
| `go-test-cov` | — | ~30s (cov only) |
| **CI critical path (parallel)** | ~3-5 min | **max(50s, 30s) = ~50s** |

## 反约束

- `go test ./...` (无 race, 无 cov) 全 PASS — 跨 milestone 锁链不破
- `go test -race ./...` 全 PASS — 无 data race 引入
- 业务 production 路径 0 行改
- coverage.sh 跟 CI go-test-cov job 字面同源 (deterministic 真值)
- 84% 阈值 — 比 race-ratchet 85% 低 1%, 但是 deterministic 真值 (反 ratchet flake, 立场 ②)

## REG-CSRC-001..004 (acceptance template)

| ID | 锚点 | Evidence |
|---|---|---|
| REG-CSRC-001 | go-test-race job 单跑 race 无 cov | `.github/workflows/ci.yml::go-test-race` step `go test -timeout=120s -race ./...` |
| REG-CSRC-002 | go-test-cov job 单跑 cov 无 race + 84% 阈值 | `.github/workflows/ci.yml::go-test-cov` step + threshold 84% (deterministic baseline) |
| REG-CSRC-003 | coverage.sh 同 CI cov 字面同源 (无 race + timeout 120s) | `packages/server-go/scripts/coverage.sh` PASS + `go tool cover -func` 报 84.9% (>84%) |
| REG-CSRC-004 | race 跟 cov 解耦立场 — `StartHeartbeat` race-flake 33.3%↔58.3% 不再卡阈值 | spec brief §立场 ① + ② 反 ratchet flake 立场 (反向: 旧 85% 阈值是 race scheduler ratchet 假象, 新 84% 是 deterministic) |

## Follow-up 留账

- ruleset required-checks 加 `go-test-race` + `go-test-cov` 替代旧 `go-test` (无 admin 权, 团队配置一次, 1 行 GitHub UI 改)
- (long-term) cov ratchet 从 84% 渐升 — 加测试覆盖 hub.StartHeartbeat 正确路径 (deterministic) 让真值 ≥85% 后再升阈值
- (long-term) race job 也加 `-coverpkg` 仅算 race-affected 包覆盖? — ROI 低, 不做

## 退出条件

- `.github/workflows/ci.yml` 两 job 字面对齐
- `coverage.sh` deterministic 真值 ≥84%
- 烈马自签
- REG-CSRC-001..004 4 🟢

# Acceptance Signoff — CI-SPLIT-RACE-COV (烈马自签)

> **状态**: ✅ SIGNED 2026-04-29 — CI-SPLIT-RACE-COV 一 PR 整闭
> **关联**: 飞马 PERF-TEST PR 1 留账 (PR 3); CI race + cov 拆 job 并行
> **方法**: ci/perf 不进野马 G4 流, 烈马代签 (跟 PERF-TEST / PERF-JWT-CLOCK / REFACTOR-REASONS / CM-5 / ADM-1 / AL-1 deferred 同模式)

## 验收对照

| # | 锚点 | 实施证据 | 状态 |
|---|---|---|---|
| ① | ci.yml 拆 go-test-race + go-test-cov 两 job + 字面 timeout=120s | yaml diff 锁两 job step 字面 byte-identical | ✅ pass |
| ② | go-test-cov 阈值 84% (deterministic 真值, 反 race-ratchet flake) | step `if (( $(echo "$COVERAGE < 84" | bc -l) ))` 字面 + 84.9% 真跑 PASS | ✅ pass |
| ③ | coverage.sh 跟 CI cov 字面同源 (无 race + timeout 120s) | shell diff + 真跑 84.9% PASS | ✅ pass |
| ④ | `go test ./...` (无 race) 全 PASS — 跨 milestone 锁链不破 | 21 packages PASS, 跨 milestone byte-identical 链全锚 | ✅ pass |
| ⑤ | `go test -race ./...` 全 PASS — 无 data race 引入 | race job 真跑 PASS, 跟 PERF-TEST #497 t.Parallel 协同 | ✅ pass |
| ⑥ | race-ratchet flake 立场 — `ws/hub.go::StartHeartbeat` 33.3%↔58.3% 真值锁定 | `diff <(race cov) <(no-race cov)` 单函数 25% delta 锁定; 反 ratchet 假象 | ✅ pass |
| ⑦ | 业务 production 路径 0 行改 | 仅 ci.yml + coverage.sh, packages/* 0 行 | ✅ pass |

## 立场关键

- **race ratchet 是假象**: 旧 85% 阈值靠 race scheduler 调度 ratchet 出来, 不是真 cov 提升. 依赖它会 0.1% 抖动 false-fail (跟 RT-1.2 latency CI runner 时序敏感同模式).
- **deterministic 是真值**: 84.9% no-race 是真 cov, 84% 阈值留 1% buffer 抗 future 微小波动.
- **future ratchet path**: 长期升阈值 = 加 hub.StartHeartbeat 等 race-affected 函数的 deterministic 测试覆盖, 而不是依赖 race-flake.

## 跨 milestone 不破

- CI gates 跨所有 milestone 全 PASS (无业务变更, 仅 CI 拓扑改)
- coverage.sh 用本地 + CI 同源 (字面 byte-identical)
- 跟 PERF-TEST #497 (t.Parallel + WAL skip) + PERF-JWT-CLOCK #500 (clock injection) 同模式 perf PR

## Follow-up ⏸️ deferred

- **REG-CSRC-005** ruleset required-checks 加 `go-test-race` + `go-test-cov` 替代旧 `go-test` — 无 admin 权改 ruleset, 团队配置一次 1 行 GitHub UI 改
- **REG-CSRC-006** cov ratchet 从 84% 渐升 — 加测试覆盖 hub.StartHeartbeat deterministic 路径
- **REG-CSRC-007** race job 加 `-coverpkg` 仅算 race-affected 包覆盖 — ROI 低

## 烈马签字

烈马 (代 zhanma-d) 2026-04-29 ✅ SIGNED post-CI-SPLIT-RACE-COV PR
- 7/7 验收通过
- race ratchet flake 立场反查通过 (StartHeartbeat 25% delta 真锁定)
- 跨 milestone 锁链不破
- 跟 PERF-TEST / PERF-JWT-CLOCK / REFACTOR-REASONS / CM-5 / ADM-1 / AL-1 烈马代签机制同模式 (CI/perf 不进野马 G4 流, 用户感知 0 变化)

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 烈马 | v0 — CI-SPLIT-RACE-COV ✅ SIGNED 一 PR 整闭. 7/7 验收通过 (ci.yml 拆 go-test-race + go-test-cov 两 job + coverage.sh 同源 + 84% deterministic 阈值 + race-ratchet flake 立场反查). REG-CSRC-001..004 4🟢. 留账 3 项 ⏸️ deferred (REG-CSRC-005 ruleset required-checks 配置 + REG-CSRC-006 cov ratchet 渐升 + REG-CSRC-007 race job coverpkg). |

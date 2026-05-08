# AL-2 wrapper 立场反查清单 (战马A v0)

> 战马A · 2026-04-29 · 立场 review checklist (跟 HB-4 #509 + BPP-4/5 + HB-3 #507 stance 同模式)
> **目的**: AL-2 wrapper 三段实施 (AL-2.1 release-gate doc / 2.2 CI workflow / 2.3 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言 release-time 收口守住每条立场.
> **关联**: spec `docs/implementation/modules/al-2-wrapper-spec.md` (战马A v0 61d6554) + acceptance `docs/qa/acceptance-templates/al-2-wrapper.md` (战马A v0)
> **不需 content-lock** — release gate docs + CI workflow, 无 DOM 文案 (跟 HB-4 #509 + BPP-3/4/5 server-only 同模式).

## §0 立场总表 (3 立场 + 5 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | AL stack 整收口闸 ≥10 硬条件, 任意一行不过 → release block; **不允许人工 sign-off 跳过** (跟 HB-4 #509 立场 ① byte-identical 同源) | agent-lifecycle.md §2.3 + 烈马 R2 立场 | 反向 grep `al.release.gate.*skip\|al.release.*manual.*sign\|allow.*bypass` 在 `.github/workflows/al-release-gate.yml` + `docs/release/agent-lifecycle-release-gate.md` count==0 |
| ② | reason 字典锁链 #496 SSOT 跨 ≥10 处 byte-identical 已落 (AL-1a→AL-3→CV-4→AL-2a→AL-1b→AL-4→BPP-2.2→AL-2b→BPP-4→BPP-5 = 10 处). **AL-2 wrapper = release-time 收口验证**, 不另起第 7 reason | agent-lifecycle.md §2.3 字面 + reasons SSOT #496 | grep `reasons\.(APIKeyInvalid\|QuotaExceeded\|NetworkUnreachable\|RuntimeCrashed\|RuntimeTimeout\|Unknown)` 跨 packages/server-go/internal/ ≥10 hit (#496 SSOT 单源验证) |
| ③ | 5-state graph 锁 #492 byte-identical (Initial/Online/Busy/Idle/Error/Offline 5 态 + valid edges); **不另起 connecting 持久态** (BPP-5 立场承袭) | agent-lifecycle.md §2.3 5-state graph 字面 + #492 + BPP-5 #503 立场 | 反向 grep `AgentStateConnecting\|state.*connecting` 在 `internal/store/agent_state_log.go` count==0; validTransitions reflect lint 自动覆盖 |
| ④ (边界) | busy/idle BPP source 锁 (AL-1b 立场 ②) — BPP frame 唯一 source, presence_sessions 不写 busy 列 | agent-lifecycle.md §2.3 + AL-1b #482 立场 ② | 反向 grep `presence_sessions.*UPDATE.*busy\|presence.*set.*busy` 在 `internal/store/` count==0 |
| ⑤ (边界) | AL stack 跟 HB stack audit 字典分立 — AL 走 agent_state_log + agent_status 两表 SSOT, HB 走 audit log; 拆死 (跟 HB-3 host vs runtime 字典分立同立场承袭) | concept-model.md §1.4 字段划界 + HB-3 stance §2 立场 ② | 反向 grep `agent_state_log.*JOIN.*audit_log\|agent_status.*audit` count==0 |
| ⑥ (边界) | admin god-mode 不入 AL release gate — agent lifecycle 是 owner-only 路径 (anchor #360 同模式) | admin-model.md ADM-0 §1.3 红线 | 反向 grep `admin.*al.release.gate\|admin.*AL2W` 在 `internal/api/admin*.go` count==0 |
| ⑦ (边界) | AL release gate yml 跟 HB-4 release-gate.yml 拆独立 — host 层 vs runtime 层守门拆死 | HB-3 字典分立立场承袭 + AL stack vs HB stack 概念分立 | `.github/workflows/al-release-gate.yml` 跟 `release-gate.yml` 是两 yml; AL yml 反向 grep `host_grants\|host_bridge\|install-butler` 0 hit (host 层 grep 不入 AL gate) |
| ⑧ (边界) | release gate yml step 跑真 go test (不是 grep stub), 跟 HB-4 #509 同模式 — 复用既有 AL-1a/AL-1.4/AL-1b/AL-2a/AL-2b unit test | AL-1a #249 + AL-1.4 #492 + AL-1b #482 + AL-2a #480 + AL-2b #481 unit test 已落 | yml 跑 `go test ./internal/agent/ ./internal/store/ ./internal/api/al_*` 真测 (非 grep stub) |

## §1 立场 ① ≥10 硬条件 不允许跳过 (AL-2.1+2.2 守)

**反约束清单**:

- [ ] AL release gate clean 清单 ≥10 项 (state graph reflect / reason chain ≥10 hit / busy/idle source / no-connecting / al-1.4 state log coverage / al-2a config blob validation / al-3 presence write end / al-4 agent_runtimes FK / no-bypass + no-admin-godmode-al)
- [ ] 任一 step fail → workflow red → release block
- [ ] 反向 grep `al.release.gate.*skip` 在 `.github/workflows/` + `docs/release/` count==0 (CI lint 守门, 防隐式跳过)
- [ ] 反向 grep `--admin.*merge.*al.release\|al.release.gate.*human.review` count==0 (跟 #486 cron skill 反 admin merge bypass + HB-4 立场 ④ 4.1 vs 4.2 拆死同源)

## §2 立场 ② reason 字典 ≥10 处单源 (AL-2.2 yml 守)

**反约束清单**:

- [ ] yml step `reason-chain-cross-milestone` 跑 grep `reasons\.(APIKeyInvalid\|QuotaExceeded\|NetworkUnreachable\|RuntimeCrashed\|RuntimeTimeout\|Unknown)` 跨 packages/server-go/internal/ ≥10 hit (release-time 单源验证)
- [ ] 反向 grep `reason.*7th\|reason.*runtime_recovered\|reason.*reconnect_success\|new.*reason\.` count==0 (不另起第 7 reason)
- [ ] reasons.go SSOT 文件 reflect lint (struct 6 const 字面 byte-identical 跟 AL-1a #249 字面)

## §3 立场 ③ 5-state graph 锁 (AL-2.2 yml 守)

**反约束清单**:

- [ ] yml step `state-graph-reflect` 跑 reflect lint (validTransitions map 5 states + valid edges 字面 byte-identical 跟 #492)
- [ ] 反向 grep `AgentStateConnecting\|state.*connecting` 在 `internal/store/agent_state_log.go` count==0 (BPP-5 立场承袭确认)
- [ ] yml step `al-1-4-state-log-coverage` 跑既有 al_1_4_state_log_test.go 真测 (5 transition coverage)

## §4 蓝图边界 ④⑤⑥⑦⑧ — busy/idle source / 字典分立 / admin / yml 拆死 / 真测不 stub

**反约束清单**:

- [ ] yml step `busy-idle-bpp-source` 跑反向 grep `presence_sessions.*UPDATE.*busy\|presence.*set.*busy` count==0 (AL-1b 立场 ②)
- [ ] yml step `dict-isolation-al-vs-hb` 跑反向 grep `agent_state_log.*JOIN.*audit_log\|agent_status.*audit` count==0 (AL vs HB 字典分立)
- [ ] yml step `no-admin-godmode-al` 跑反向 grep `admin.*al.release.gate\|admin.*AL2W` 在 `internal/api/admin*.go` count==0
- [ ] al-release-gate.yml 跟 release-gate.yml 是两独立 yml (反向 grep `host_grants\|host_bridge` 在 al-release-gate.yml count==0)
- [ ] yml 跑真 go test step (不是 grep stub) — `go test ./internal/agent/ ./internal/store/ ./internal/api/al_*` 全绿

## §5 退出条件

- §1 (4) + §2 (3) + §3 (3) + §4 (5) 全 ✅
- AL release gate ≥10 项硬条件 + CI workflow 跑全闭, 任一 step fail → release block
- reason 字典锁链 ≥10 处 byte-identical 不漂 (改 = 改十处单测锁)
- 5-state graph + busy/idle source + 字典分立 + admin red line 全 0 hit
- al-release-gate.yml 跟 HB-4 release-gate.yml 拆独立 (host 层 vs runtime 层守门拆死)

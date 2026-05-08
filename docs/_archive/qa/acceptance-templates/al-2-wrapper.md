# Acceptance Template — AL-2 wrapper ⭐ agent lifecycle release gate

> 蓝图: `agent-lifecycle.md` §1.6 (失联与故障状态) + §2.3 (5-state graph + reason 字典) + 蓝图 R3 4 人 review #5 决议
> Spec: `docs/implementation/modules/al-2-wrapper-spec.md` (战马A v0 61d6554)
> Stance: `docs/qa/al-2-wrapper-stance-checklist.md` (战马A v0)
> 不需 content-lock — release gate docs + CI, 无 DOM 文案 (跟 HB-4 + BPP-3/4/5 server-only 同模式)
> 拆 PR: **AL-2 wrapper 整 milestone 一 PR** `feat/al-2-wrapper` 三段一次合
> Owner: 战马A (实施) / 飞马 review / 烈马 验收 (4.1) / 野马签字 (4.2 demo)

## 验收清单

### §1 AL-2.1 — release-gate.md doc ≥10 硬条件清单

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 release-gate.md ≥10 硬条件清单, 每项三元组 (蓝图 § 锚 + CI workflow path + assertion) | doc lint | 战马A / 烈马 | `docs/release/agent-lifecycle-release-gate.md` ≥10 numbered rows |
| 1.2 蓝图 §2.3 5-state graph 字面 byte-identical 入清单 (Initial/Online/Busy/Idle/Error/Offline + valid edges) | doc lint + table reflect | 战马A / 烈马 | doc §1 表 5 状态 byte-identical 跟 store/agent_state_log.go |
| 1.3 reason 字典 #496 SSOT ≥10 处锁链入清单 (跨 milestone grep 真守) | doc + CI cross-ref | 战马A / 飞马 | doc §2 锁链跟 spec §0.2 byte-identical |
| 1.4 跨 milestone 反约束扩 4 项 (busy/idle BPP source + 字典分立 AL vs HB + no-7th-reason + no-connecting-persisted) | doc + CI cross-ref | 战马A / 飞马 | doc §3 反约束清单 |

### §2 AL-2.2 — CI workflow al-release-gate.yml

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 al-release-gate.yml step 数 ≥ 10, 任一 fail → workflow red | yml + dry-run | 战马A / 烈马 | `.github/workflows/al-release-gate.yml` step count ≥10 + 每 step `run:` 含 fail path |
| 2.2 reason chain ≥10 hit (跨 packages/server-go/internal/ 跑 grep #496 SSOT 6 const) | CI grep | 战马A / 飞马 | step `reason-chain-cross-milestone` ≥10 hit assertion |
| 2.3 5-state graph reflect lint (validTransitions byte-identical 跟 #492) | CI go test | 战马A / 烈马 | step `state-graph-reflect` 跑 al_1_4_state_log_test.go 真测 |
| 2.4 busy/idle BPP source 锁 (反向 `presence_sessions.*UPDATE.*busy` 0 hit) | CI grep | 战马A / 烈马 | step `busy-idle-bpp-source` 反向断言 |
| 2.5 字典分立 AL vs HB (反向 `agent_state_log.*JOIN.*audit_log` 0 hit) | CI grep | 战马A / 飞马 | step `dict-isolation-al-vs-hb` |
| 2.6 反约束 admin god-mode + no-bypass + no-7th-reason + no-connecting 4 项 0 hit | CI grep | 飞马 / 烈马 | step `no-bypass` + `no-admin-godmode-al` + `no-7th-reason` + `no-connecting-persisted` |

### §3 AL-2.3 — closure (REG + PROGRESS + 野马签字 placeholder)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 REG-AL2W-001..010 全 🟢 入 docs/qa/regression-registry.md | docs flip | 飞马 / 烈马 | regression-registry.md REG-AL2W-* 10 行 |
| 3.2 PROGRESS AL-2 wrapper [x] 翻 (4.1 ✅ + 4.2 ⏸️ deferred 留账) | docs flip | 飞马 | PROGRESS.md AL-2 wrapper 行 |
| 3.3 野马 4.2 demo 签字 placeholder (3 张截屏锚: 5-state UI / error→online 反向链 / busy/idle BPP frame 触发) | doc placeholder | 野马 (主) | `docs/qa/signoffs/al-2-wrapper-yema-signoff.md` placeholder |

### §4 反向 grep / e2e 兜底 (跨 AL-2 wrapper 反约束)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 立场 ① 不允许跳过 — `al.release.gate.*skip\|al.release.*manual.*sign\|allow.*bypass` count==0 | CI grep | 飞马 / 烈马 | yml step `no-bypass` |
| 4.2 立场 ② 不另起第 7 reason — `reason.*7th\|reason.*runtime_recovered\|reason.*reconnect_success` count==0 | CI grep | 飞马 / 烈马 | yml step `no-7th-reason` |
| 4.3 立场 ③ 不挂 connecting 持久态 — `AgentStateConnecting\|state.*connecting` count==0 | CI grep | 飞马 / 烈马 | yml step `no-connecting-persisted` (BPP-5 立场承袭 verify) |
| 4.4 立场 ⑥ admin god-mode 不入 — `admin.*al.release.gate\|admin.*AL2W` 在 `internal/api/admin*.go` count==0 | CI grep | 飞马 / 烈马 | yml step `no-admin-godmode-al` |
| 4.5 立场 ⑦ AL yml 跟 HB-4 yml 拆独立 — al-release-gate.yml 反向 grep `host_grants\|host_bridge\|install-butler` 0 hit | CI grep | 飞马 / 烈马 | yml step `al-vs-hb-yml-isolation` |

## 边界 (跟其他 milestone 关系)

| Milestone | 关系 | 字面承袭 |
|---|---|---|
| AL-1a #249 | reason 字典 SSOT 第 1 处 (字面单源) | `internal/agent/state.go::Reason*` 6 const |
| AL-1.4 #492 | 5-state graph + agent_state_log validTransitions | reflect lint 跟 #492 byte-identical |
| AL-1b #482 | busy/idle BPP source 立场 ② | presence_sessions 不写 busy 列 |
| AL-2a #480 | config blob validation + allowedConfigKeys 7 字段 | release gate 跑既有 al_2a_2_agent_config_test 真测 |
| AL-2b #481 | BPP frame fanout + idempotency | 跑既有 al_2b_pusher_test 真测 |
| AL-3 #310 | presence write end (SessionsTracker) | TrackOnline / TrackOffline 路径 |
| AL-4 #427 | agent_runtimes FK 完整 | agent_runtimes_test 真测 |
| BPP-2.2 #485 | reason 字典锁链第 7 处 (task_finished) | 字面承袭 |
| BPP-4 #499 | reason 锁链第 9 处 + best-effort 立场承袭 | watchdog 触发 reason=network_unreachable |
| BPP-5 #503 | reason 锁链第 10 处 + 不挂 connecting 持久态 | error→online 反向 valid edge |
| reasons SSOT #496 | 6-dict 字面单源 | 改 = 改十处单测锁链 |
| HB-4 #509 | release gate 拆独立 yml — host 层 vs runtime 层守门拆死 | al-release-gate.yml ≠ release-gate.yml |
| ADM-0 §1.3 | admin god-mode 不入 AL release gate | 字面立场反断 |

## 退出条件

- §1 doc (4) + §2 CI workflow (6) + §3 closure (3) + §4 反约束 (5) **全 🟢**
- reason 字典锁链 ≥10 处 byte-identical 不漂 (改 = 改十处单测锁)
- 5-state graph + busy/idle BPP source + 字典分立 + admin red line 全 0 hit
- al-release-gate.yml 跟 HB-4 release-gate.yml 拆独立 (host 层 vs runtime 层守门拆死)
- 4.2 野马 demo 签字 ⏸️ deferred 留账 (release 前真补 3 张截屏)

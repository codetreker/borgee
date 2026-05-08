# AL-2 wrapper spec brief — agent lifecycle release gate ⭐ (≤80 行)

> 战马A · Phase 5 agent-lifecycle 收口闸 · ≤80 行 · 蓝图 [`agent-lifecycle.md`](../../blueprint/current/agent-lifecycle.md) §2.3 (5-state graph + reason 字典) + §1.6 (失联与故障状态). 模块锚 [`agent-lifecycle.md`](agent-lifecycle.md). 跟 HB-4 #509 release gate 同模式 (≥10 硬条件 + CI workflow + 不允许人工 sign-off 跳过). 依赖 AL-1a #249 reason 6-dict + AL-1.4 #492 5-state graph + AL-1b #482 busy/idle + AL-2a #480 config + AL-2b #481 BPP frame + AL-3 #310 presence + AL-4 #427 agent_runtimes + reasons SSOT #496 + BPP-2.2 #485 + BPP-4 #499 + BPP-5 #503.

## 0. 关键约束 (3 条立场, 蓝图 §2.3 字面承袭)

1. **AL stack 整收口闸 ≥10 硬条件** (跟 HB-4 #509 同模式) — release gate 是硬条件清单, 任意一行不过 → release block. **不允许人工 sign-off 跳过任一项**. 反约束: 反向 grep `al.release.gate.*skip\|al.release.*manual.*sign\|allow.*bypass` 0 hit (跟 HB-4 立场 ① byte-identical 同源).

2. **reason 字典锁链 #496 SSOT 跨 ≥10 处** — `internal/agent/reasons/reasons.go` 6-dict (api_key_invalid/quota_exceeded/network_unreachable/runtime_crashed/runtime_timeout/unknown) 是 source-of-truth. 锁链已落: AL-1a #249 第 1 处 + AL-3 #305 第 2 处 + CV-4 #380 第 3 处 + AL-2a #454 第 4 处 + AL-1b #458 第 5 处 + AL-4 #387/#461 第 6 处 + BPP-2.2 #485 第 7 处 + AL-2b #481 第 8 处 + BPP-4 #499 第 9 处 + BPP-5 #503 第 10 处. **AL-2 wrapper = release-time 收口验证**, 不另起第 7th 字面 reason. 反向 grep `reason.*7th\|reason.*runtime_recovered\|reason.*new` 0 hit.

3. **5-state graph 锁 #492 byte-identical** — agent_state_log validTransitions (Initial→Online/Offline / Online→Busy/Idle/Error/Offline / Busy↔Idle / Error→Online/Offline / Offline→Online) 单源, BPP-5 reconnect 已复用 error→online valid edge **不另起 connecting 持久态**. 反约束: 反向 grep `AgentStateConnecting\|state.*connecting` 在 `internal/store/agent_state_log.go` count==0 (BPP-5 立场承袭确认); busy/idle source 锁 — `presence_sessions.*UPDATE.*busy` count==0 (AL-1b 立场 ② BPP frame 唯一 source).

## 1. 拆段 (一 milestone 一 PR, 整段一次合 — 跟 HB-4 #509 协议同源)

| 段 | 文件 | 范围 |
|---|---|---|
| AL-2.1 release-gate doc | `docs/release/agent-lifecycle-release-gate.md` (新, ≤200 行) — ≥10 硬条件清单, 每项三元组 (蓝图 § 锚 + CI path + assertion); §1 蓝图 §2.3 5-state graph 锁 (validTransitions reflect 自动覆盖) + §2 reason 字典锁链 ≥10 处 byte-identical (跨 milestone 跑 grep `reasons\.(APIKeyInvalid\|QuotaExceeded\|NetworkUnreachable\|RuntimeCrashed\|RuntimeTimeout\|Unknown)` ≥10 hit) + §3 busy/idle BPP source 锁 (反向 `presence_sessions.*UPDATE.*busy` 0 hit) + §4 反约束守门 4 项 (no-bypass + admin god-mode + 不挂第 7 reason + connecting 持久态 0 hit) |
| AL-2.2 CI workflow | `.github/workflows/al-release-gate.yml` (新) — 跑 ≥10 硬条件 step (state-graph reflect / reason-chain ≥10 hit / busy-idle-source / no-7th-reason / no-connecting-persisted / al-1-state-log-coverage / al-2a-config-blob-validation / al-3-presence-write-end / al-4-agent-runtimes-fk / no-bypass + no-admin-godmode-al). 任一 fail → workflow red → release block. 跟 HB-4 release-gate.yml 同模式; 共 runs-on 标签 (烈马 R2 基准 ubuntu-latest 4vCPU 16GB) |
| AL-2.3 closure | `docs/qa/acceptance-templates/al-2-wrapper.md` 新 + REG-AL2W-001..010 入 regression-registry + PROGRESS AL-2 wrapper [x] + `docs/qa/signoffs/al-2-wrapper-yema-signoff.md` placeholder (野马 release 前补 3 张截屏: 5-state UI 渲染 / error→online 反向链 / busy/idle BPP frame 触发) |

## 2. 留账边界

- **野马 4.2 demo 签字截屏** (留 release 前真补) — 3 张, 本 PR 仅 placeholder
- **AL-1b BPP frame 真接入** (跟 BPP-2.2 #485 已落, AL-2 wrapper 仅 verify 不重做) — release-gate.yml 跑 `TestBPP2_TaskStarted_*` + `TestBPP2_TaskFinished_*` 真触发 busy/idle
- **AL-5 (蓝图未落)** 留账 (蓝图 §AL-5 v2 路径 — 跨 host agent 迁移, 本 wrapper 不涵)
- **HB-4 release gate 复用** — AL-2 wrapper 跟 HB-4 拆独立 yml (host 层 vs runtime 层守门拆死), 跨 milestone audit schema 锁链 HB-4 已收第 5 处, AL-2 wrapper 不重复 audit 守门 (AL stack 不写 audit log; 走 agent_state_log + agent_status 两表 SSOT, 跟 HB audit log 字典分立)

## 3. 反查 grep 锚 (Phase 5 收尾验收 + AL-2 wrapper 实施 PR 必跑)

```
git grep -nE 'al.release.gate.*skip|al.release.*manual.*sign|allow.*bypass' .   # 0 hit (立场 ①)
git grep -nE 'reasons\.(APIKeyInvalid|QuotaExceeded|NetworkUnreachable|RuntimeCrashed|RuntimeTimeout|Unknown)' packages/server-go/internal/   # ≥10 hit (跨 10+ 处 #496 SSOT 单源)
git grep -nE 'AgentStateConnecting|state.*connecting' packages/server-go/internal/store/   # 0 hit (BPP-5 立场承袭)
git grep -nE 'presence_sessions.*UPDATE.*busy|presence.*set.*busy' packages/server-go/internal/   # 0 hit (busy/idle BPP source 锁)
git grep -nE 'reason.*7th|reason.*runtime_recovered|reason.*reconnect_success' packages/server-go/internal/   # 0 hit (不扩第 7 reason)
git grep -nE 'admin.*al.release.gate|admin.*AL2W' packages/server-go/internal/api/admin   # 0 hit (admin 不入)
```

## 4. 不在本轮范围 (反约束 deferred)

- ❌ AL-5 跨 host agent 迁移 (蓝图 v2 路径, AL-2 wrapper 不涵)
- ❌ HB-4 audit schema 锁链复用 (字典分立 — AL stack 走 agent_state_log + agent_status, HB stack 走 audit log; 拆死)
- ❌ 野马 4.2 demo 截屏 (留 release 前真补)
- ❌ admin god-mode 走 AL release gate (用户主权, ADM-0 §1.3 红线)
- ❌ BPP-1 envelope CI lint 复用 (BPP 层是 wire 协议, AL 层是 state machine; 拆死, BPP-1 #304 reflect lint 不重复 verify in AL release gate)
- ❌ reason 字典扩第 7 (AL-1a 6-dict 单源锁链 release 前 verify, 不允许 release 前 last-minute 加 reason)

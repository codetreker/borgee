# Borgee Agent Lifecycle (AL stack) v1 release gate ⭐

> **Source-of-truth.** This doc enumerates the ≥10 hard release-block
> conditions for the AL stack (AL-1a reasons / AL-1.4 5-state graph /
> AL-1b busy/idle / AL-2a config / AL-2b BPP frame / AL-3 presence /
> AL-4 agent_runtimes). Each row is a three-tuple: blueprint anchor
> + CI workflow path + assertion.
>
> **关闭条件 (跟 HB-4 #509 同模式)**: 任一行 fail → ⭐ AL-2 wrapper
> milestone 不能关. **不允许人工 sign-off 跳过任一行**.
> 4.2 demo 签字走野马 [`docs/qa/signoffs/al-2-wrapper-yema-signoff.md`](../qa/signoffs/al-2-wrapper-yema-signoff.md)
> 独立路径, **不混入** 本 doc 的 4.1 行为不变量数字化清单.
>
> CI workflow: [`.github/workflows/al-release-gate.yml`](../../.github/workflows/al-release-gate.yml).
> 跟 HB-4 [`release-gate.yml`](../../.github/workflows/release-gate.yml) **拆独立 yml**:
> host 层 vs runtime 层守门拆死 (AL stack 走 agent_state_log + agent_status,
> HB stack 走 audit log; 字典分立).

## §1 蓝图 §2.3 5-state graph + AL-1a reason 字典锁链

| # | 守门 | 阈值 / assertion | 蓝图 § / spec 锚 | CI step |
|---|------|------------------|-------------------|---------|
| 1 | 5-state graph reflect lint | validTransitions byte-identical 跟 #492 (Initial→Online/Offline / Online→Busy/Idle/Error/Offline / Busy↔Idle / Error→Online/Offline / Offline→Online) | agent-lifecycle.md §2.3 + #492 | `al-release-gate.yml::state-graph-reflect` (跑 al_1_4_state_log_test) |
| 2 | reason 字典锁链 ≥10 处 | grep `reasons\.(APIKeyInvalid\|QuotaExceeded\|NetworkUnreachable\|RuntimeCrashed\|RuntimeTimeout\|Unknown)` 跨 packages/server-go/internal/ ≥10 hit | reasons SSOT #496 + AL-1a→BPP-5 锁链 10 处 | `al-release-gate.yml::reason-chain-cross-milestone` |
| 3 | 不另起第 7 reason | 反向 grep `reason.*7th\|reason.*runtime_recovered\|reason.*reconnect_success\|new.*reason\.` count==0 | BPP-4/5 + AL-2b 立场承袭 | `al-release-gate.yml::no-7th-reason` |
| 4 | 不挂 connecting 持久态 | 反向 grep `AgentStateConnecting\|state.*connecting` 在 `internal/store/agent_state_log.go` count==0 | BPP-5 #503 立场承袭 (reconnect 走 error→online valid edge, 不另起 connecting) | `al-release-gate.yml::no-connecting-persisted` |

## §2 跨 AL stack 行为不变量 (复用既有 unit test)

| # | 守门 | assertion | spec 锚 | CI step |
|---|------|-----------|---------|---------|
| 5 | AL-1.4 state log coverage (5 transition 真测) | `al_1_4_state_log_test.go` 真跑全过 | #492 | `al-release-gate.yml::al-1-4-state-log-coverage` |
| 6 | AL-2a config blob validation 真测 | `al_2a_2_agent_config_test.go` 全过 (allowedConfigKeys 7 字段 + 并发递增 + cross-owner 403) | #480 | `al-release-gate.yml::al-2a-config-blob-validation` |
| 7 | AL-2b BPP frame fanout 真测 | `al_2b_pusher_test.go` 全过 (cursor 共序 + idempotency + plugin offline drop) | #481 | `al-release-gate.yml::al-2b-bpp-fanout` |
| 8 | busy/idle BPP source 锁 | 反向 grep `presence_sessions.*UPDATE.*busy\|presence.*set.*busy` count==0 (AL-1b 立场 ② BPP frame 唯一 source) | AL-1b #482 | `al-release-gate.yml::busy-idle-bpp-source` |

## §3 跨 stack 字典分立 + 反约束守门

| # | 反约束 | assertion | 立场 | CI step |
|---|--------|-----------|------|---------|
| 9 | AL stack vs HB stack audit 字典分立 (AL 走 agent_state_log + agent_status, HB 走 audit log; 拆死) | 反向 grep `agent_state_log.*JOIN.*audit_log\|agent_status.*audit` count==0 | concept-model.md §1.4 + HB-3 字典分立立场承袭 | `al-release-gate.yml::dict-isolation-al-vs-hb` |
| 10 | 不允许人工 sign-off 跳过 AL release gate | 反向 grep `al.release.gate.*skip\|al.release.*manual.*sign\|allow.*bypass\|--admin.*merge.*al.release` 在 `.github/workflows/` + `docs/release/agent-lifecycle-release-gate.md` count==0 | 烈马 R2 立场 ① + HB-4 立场 ① 字面承袭 | `al-release-gate.yml::no-bypass` |
| 11 | admin god-mode 不入 AL release gate (agent lifecycle 是 owner-only 路径) | 反向 grep `admin.*al.release.gate\|admin.*AL2W` 在 `internal/api/admin*.go` count==0 | ADM-0 §1.3 红线 + anchor #360 owner-only | `al-release-gate.yml::no-admin-godmode-al` |
| 12 | AL yml 跟 HB-4 yml 拆独立 (host vs runtime 守门拆死) | al-release-gate.yml 反向 grep `host_grants\|host_bridge\|install-butler` count==0 | HB-4 + HB-3 字典分立同立场承袭 | `al-release-gate.yml::al-vs-hb-yml-isolation` |

## §4 退出条件 (AL-2 wrapper ⭐ 关闭条件)

- §1 蓝图 §2.3 5-state graph + reason 字典锁链 4 项 (state-graph reflect + reason chain ≥10 + no-7th + no-connecting) 全 ✅
- §2 跨 AL stack 行为不变量 4 项 (AL-1.4 + AL-2a + AL-2b + busy/idle source) 全 ✅ (复用既有 unit test 真跑)
- §3 跨 stack 字典分立 + 反约束守门 4 项 (AL/HB 字典分立 + no-bypass + no-admin-godmode + AL/HB yml 拆独立) 全 0 hit
- 4.2 demo 签字 ⏸️ deferred (野马 release 前补 3 张截屏: 5-state UI / error→online 反向链 / busy/idle BPP frame 触发)

**任一行 fail → ⭐ AL-2 wrapper 不能关 → release block.**

# Acceptance Template — HB-3 v2: heartbeat decay 三档 derive + cross-bucket audit + owner-only 视图

> 蓝图 `plugin-protocol.md` §1.6 + BPP-4 #499 watchdog + BPP-8 #532 lifecycle audit. Spec `hb-3-v2-spec.md` (战马D v0 f5a50bb) + Stance `hb-3-v2-stance-checklist.md` (战马D v0). 不需 content-lock — owner-only API 无新 DOM. 命名拆死: HB-3 #507 host_grants 不交. Owner: 战马D 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 HB-3 v2.1 — DecayState enum + DeriveDecayState 纯 fn

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 DecayState enum 3 字面 byte-identical (`fresh / stale / dead`) + StaleThreshold=30s + DeadThreshold=60s const 字面 byte-identical 跟 BPP-4 watchdog 30s 同源 | unit (5 边界 case) | 战马D / 烈马 | `internal/bpp/heartbeat_decay_test.go::TestHB3V2_DeriveDecayState_Boundaries` (t=0/29/30/59/60 → fresh/fresh/stale/stale/dead) + `_ConstThresholdsByteIdentical` |
| 1.2 立场 ① 0 schema 改 + 不裂表 — 反向 grep `heartbeat_decay_table\|hb3_decay_log\|stale_ratio_history` + `CREATE TABLE.*heartbeat_decay` 0 hit | grep | 战马D / 飞马 / 烈马 | `TestHB3V2_NoSchemaChange` (反向 grep 5 forbidden literal 0 hit) |
| 1.3 nil-safe / 极值 — DeriveDecayState(now, 0) 返 dead (永久不活); 负 lastHeartbeatAt 当 0 处理 | unit | 战马D / 烈马 | `TestHB3V2_DeriveDecayState_NilSafe` (lastHeartbeatAt = 0/-1 行为) |

### §2 HB-3 v2.2 — watchdog cross-bucket wire + GET endpoint

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 立场 ② cross-bucket transition wire 调 BPP-8 RecordHeartbeatTimeout — 不另开 RecordHeartbeatStale 旁路 (反向 grep 0 hit); 同档 (fresh→fresh / stale→stale) 不重复触 audit | unit (mock auditor + 多 bucket 真验) | 战马D / 烈马 | `internal/bpp/heartbeat_decay_test.go::TestHB3V2_CrossBucket_TriggersAudit` (fresh→stale 调 1 次) + `_SameBucket_NoAuditCall` (fresh→fresh 调 0 次) + `_NoStaleSidePath` (反向 grep RecordHeartbeatStale 0 hit) |
| 2.2 立场 ③ GET /api/v1/agents/{agentId}/heartbeat-decay owner-only ACL — happy 200 + cross-owner 403 + 401 + 404 | unit (4 sub-case) | 战马D / 烈马 | `internal/api/hb_3_v2_decay_list_test.go::TestHB3V2_DecayList_HappyPath` + `_CrossOwnerReject` + `_Unauthorized401` + `_AgentNotFound404` |
| 2.3 立场 ④ DecayState 字面单源 + 立场 ⑥ threshold byte-identical — 反向 grep hardcode `"fresh"\|"stale"\|"dead"` 在 hb_3_v2*.go 外 0 hit + StaleThreshold const literal 跟 BPP-4 watchdog 30s + BPP-7 SDK HeartbeatInterval 同源 | grep + unit | 战马D / 飞马 / 烈马 | `TestHB3V2_DecayState_LiteralSingleSource` + `TestHB3V2_StaleThreshold_ByteIdentical` |

### §3 HB-3 v2.3 — closure + AST 锁链延伸第 6 处 + admin god-mode 不挂

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 立场 ⑤ AST 锁链延伸第 6 处 — forbidden 3 token (`pendingDecayQueue / decayRetryQueue / deadLetterDecay`) 在 internal/ 0 hit (跟 BPP-4 + BPP-5 + BPP-6 + BPP-7 + BPP-8 best-effort 锁链延伸第 6 处) | AST scan | 飞马 / 烈马 | `TestHB3V2_NoDecayQueueOrSchema` (AST ident scan internal/bpp + internal/api production 0 hit) |
| 3.2 立场 ③ admin god-mode 不挂 — 反向 grep `admin.*heartbeat.*decay\|admin.*HB3` 在 admin*.go 0 hit (ADM-0 §1.3 红线) | grep | 飞马 / 烈马 | `TestHB3V2_AdminGodModeNotMounted` |

## 边界

- BPP-4 #499 watchdog (30s threshold const 复用 — StaleThreshold byte-identical) / BPP-7 SDK (HeartbeatInterval const 同源 30s) / BPP-8 #532 lifecycle audit (RecordHeartbeatTimeout 复用, 不另开旁路) / AL-1 #492 + REFACTOR-REASONS #496 6-dict (reasons.NetworkUnreachable 复用 — AL-1a 锁链第 14 处) / ADM-2.1 #484 admin_actions audit (HB-3 v2 不增 schema, 复用既有 plugin_heartbeat_timeout action) / ADM-0 §1.3 红线 / AST 锁链延伸第 6 处 (BPP-4+5+6+7+8+HB-3 v2)

## 退出条件

- §1 (3) + §2 (3) + §3 (2) 全绿 — 一票否决
- AL-1a reason 锁链 HB-3 v2 = 第 14 处 (BPP-8 第 13 链承袭不漂)
- audit forward-only 锁链不增 (复用 BPP-8 既有 plugin_heartbeat_timeout action, ADM-2.1+AP-2+BPP-4+BPP-7+BPP-8 跨五 milestone 同精神)
- AST 锁链延伸第 6 处 (BPP-4 + BPP-5 + BPP-6 + BPP-7 + BPP-8 + HB-3 v2 forbidden tokens 全 0 hit)
- owner-only 锁链第 9 处 (AL-2a/BPP-3.2/AL-1/AL-5/DM-4/CV-4 v2/BPP-7/BPP-8/HB-3 v2)
- 0 schema 改 + 反向 grep 7 项全 0 hit
- 登记 REG-HB3V2-001..005

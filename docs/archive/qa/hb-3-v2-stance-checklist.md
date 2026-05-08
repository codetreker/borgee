# HB-3 v2 立场反查清单 (战马D v0)

> 战马D · 2026-04-30 · 立场 review checklist (跟 BPP-7/BPP-8 stance 同模式)
> **目的**: HB-3 v2 三段实施 (v2.1 derive helper / v2.2 watchdog wire + GET endpoint / v2.3 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/hb-3-v2-spec.md` (战马D v0 f5a50bb) + acceptance `docs/qa/acceptance-templates/hb-3-v2.md` (战马D v0)
> **不需 content-lock** — endpoint 是 owner-only API 无新 DOM/UI; decay 状态枚举 3 字面 ('fresh'/'stale'/'dead') 跟 spec §1 byte-identical 即可锁.
> **命名拆死**: HB-3 #507 host_grants 在 host-bridge 域; 本 milestone HB-3 v2 = heartbeat decay 主题, branch feat/hb-3-v2 + 文件名 hb-3-v2-spec.md / hb_3_v2_*.go 字面拆死.

## §0 立场总表 (3 立场 + 4 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | heartbeat decay 三档 (fresh / stale / dead) — decay 状态从 `last_heartbeat_at` 反向 derive, **0 schema 改** + 复用 BPP-4 30s threshold const + 不另起 sequence | plugin-protocol.md §1.6 "失联非 binary" + BPP-4 #499 watchdog 30s threshold | 反向 grep `heartbeat_decay_table\|hb3_decay_log\|stale_ratio_history` 在 internal/ 0 hit; 反向 grep `CREATE TABLE.*heartbeat_decay\|ALTER TABLE.*heartbeat_decay` 在 migrations/ 0 hit |
| ② | stale 跨档 transition 复用 BPP-8 RecordHeartbeatTimeout — 不另开 RecordHeartbeatStale 旁路; reason 复用 reasons.NetworkUnreachable byte-identical (跟 BPP-4 + BPP-8 同源, AL-1a reason 锁链第 14 处) | reasons-spec.md (#496 SSOT) + BPP-8 #532 lifecycle audit 复用 | 反向 grep `RecordHeartbeatStale\|LifecycleAuditor.*Stale` 在 internal/ 0 hit; 反向 grep `runtime_recovered\|hb3_specific_reason\|stale_reason\|7th.*reason` 0 hit |
| ③ | owner-only 视图 GET /api/v1/agents/{agent_id}/heartbeat-decay — 跟 AL-2a/BPP-3.2/AL-1/AL-5/DM-4/CV-4 v2/BPP-7/BPP-8 owner-only 8 处同模式; admin god-mode 不挂 (ADM-0 §1.3 红线) | admin-model.md ADM-0 §1.3 + auth-permissions.md owner-only 锚 | 反向 grep `admin.*heartbeat.*decay\|admin.*HB3` 在 internal/api/admin*.go 0 hit |
| ④ (边界) | DecayState enum 3 态 byte-identical — `fresh / stale / dead` 字面单源 (const 字面锁, 跟 BPP-1 envelope reflect lint 同精神) | spec §1 字面 | const DecayStateFresh/Stale/Dead 字面单源, 反向 grep hardcode `"fresh"\|"stale"\|"dead"` 在 hb_3_v2*.go 外 0 hit |
| ⑤ (边界) | best-effort 立场承袭 BPP-4/5/6/7/8 — 不挂 retry queue / 不持久化 deferred audit; cross-bucket transition 调 auditor 是 fire-and-forget (跟 BPP-8 best-effort 同精神) | bpp-4.md §0.3 best-effort 立场 | AST scan forbidden tokens `pendingDecayQueue\|decayRetryQueue\|deadLetterDecay` 在 internal/ 0 hit (锁链延伸第 6 处) |
| ⑥ (边界) | StaleThreshold = 30s / DeadThreshold = 60s 字面 byte-identical 跟 BPP-4 watchdog 30s 同源 | BPP-4 #499 watchdog 30s threshold | const literal 单源, 反向 grep 非 30s/60s 字面 0 hit; StaleThreshold 跟 srvbpp.HeartbeatInterval (BPP-7 SDK + BPP-4 server) byte-identical |
| ⑦ (边界) | watchdog wire 触发条件: 仅 cross-bucket transition (fresh→stale / stale→dead) 触 audit, 同档不重发 — 防止 audit log 被高频 noise 淹没 | best-effort 立场 + audit forward-only 同精神 | wire 处 unit test 真验: 同档 (fresh→fresh) 不调 auditor, cross-bucket 调 1 次 |

## §1 立场 ① decay 三档 0 schema (HB-3 v2.1 守)

**反约束清单**:

- [ ] DeriveDecayState(now, lastHeartbeatAt) 纯 fn — 输入 (now, last_ms) 输出 DecayState enum, 无 IO / 无 store 依赖
- [ ] StaleThreshold = 30 * time.Second / DeadThreshold = 60 * time.Second const 字面单源 (跟 BPP-4 watchdog 30s 同源)
- [ ] 0 schema 改 — 反向 grep `CREATE TABLE.*heartbeat_decay\|ALTER TABLE.*heartbeat_decay` 0 hit + git diff packages/server-go/internal/migrations/ 仅 _test.go 或为空
- [ ] 不裂表 — 反向 grep `heartbeat_decay_table\|hb3_decay_log\|stale_ratio_history` 0 hit
- [ ] 边界 unit test 真覆盖 — t=0/29/30/59/60 → fresh/fresh/stale/stale/dead

## §2 立场 ② 复用 BPP-8 audit (HB-3 v2.2 守)

**反约束清单**:

- [ ] watchdog cross-bucket transition wire 调 `auditor.RecordHeartbeatTimeout` (BPP-8 既有 method) — 不另开 `RecordHeartbeatStale` 旁路
- [ ] 反向 grep `RecordHeartbeatStale\|LifecycleAuditor.*Stale` 0 hit
- [ ] reason 复用 reasons.NetworkUnreachable byte-identical (auditor 内部已写, watchdog wire 不重复)
- [ ] AL-1a reason 锁链 HB-3 v2 = 第 14 处 (改 = 改十四处)
- [ ] 反向 grep `runtime_recovered\|hb3_specific_reason\|stale_reason` 0 hit

## §3 立场 ③ owner-only + AST 锁链延伸第 6 处 (HB-3 v2.2 守)

**反约束清单**:

- [ ] GET endpoint owner-only ACL 真测 — happy + non-owner 403 + 401 + 404
- [ ] admin god-mode 不挂 — 反向 grep `admin.*heartbeat.*decay\|admin.*HB3` 在 admin*.go 0 hit
- [ ] response shape 含 decay state enum 字面 + 反向 grep raw `last_heartbeat_at` 0 hit (sanitizer 反向)
- [ ] AST scan forbidden tokens 0 hit (锁链延伸第 6 处)

## §4 蓝图边界 ④⑤⑥⑦ — 跟 enum 单源 / best-effort / threshold 同源 / cross-bucket trigger 不漂

**反约束清单**:

- [ ] DecayState const 字面单源 — 反向 grep hardcode `"fresh"\|"stale"\|"dead"` 在 hb_3_v2*.go 外的 production *.go 0 hit
- [ ] best-effort 锁链延伸第 6 处 — AST scan `pendingDecayQueue\|decayRetryQueue\|deadLetterDecay` 0 hit
- [ ] StaleThreshold 跟 BPP-4 watchdog 30s + BPP-7 SDK HeartbeatInterval byte-identical (const 引用同 const, 不再次声明)
- [ ] watchdog wire 同档不触 audit — TestHB3V2_SameBucket_NoAuditCall (fresh→fresh / stale→stale 调用 0 次)

## §5 退出条件

- §1 (5) + §2 (5) + §3 (4) + §4 (4) 全 ✅
- 反向 grep 7 项全 0 hit (decay 表 / Stale 旁路 method / Stale reason / decay queue / admin / decay schema / threshold drift)
- AL-1a reason 锁链 HB-3 v2 = 第 14 处
- audit forward-only 锁链不增 (复用 BPP-8 既有 plugin_heartbeat_timeout action, 跟 ADM-2.1+AP-2+BPP-4+BPP-7+BPP-8 跨五 milestone 同精神 — HB-3 v2 不增 admin_actions schema)
- AST 锁链延伸第 6 处 (BPP-4 + BPP-5 + BPP-6 + BPP-7 + BPP-8 + HB-3 v2 forbidden tokens 全 0 hit)
- owner-only 锁链第 9 处 (AL-2a/BPP-3.2/AL-1/AL-5/DM-4/CV-4 v2/BPP-7/BPP-8/HB-3 v2)

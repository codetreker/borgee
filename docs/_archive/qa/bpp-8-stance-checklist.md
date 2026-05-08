# BPP-8 立场反查清单 (战马D v0)

> 战马D · 2026-04-30 · 立场 review checklist (跟 BPP-7 stance + AP-2 stance 同模式)
> **目的**: BPP-8 三段实施 (8.1 schema migration / 8.2 server hooks + handler / 8.3 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/bpp-8-spec.md` (战马D v0) + acceptance `docs/qa/acceptance-templates/bpp-8.md` (战马D v0)
> **不需 content-lock** — 内部 audit log 无 DOM/UI; admin_actions enum 字面已含 5 条 plugin_* 字面锁 (跟 BPP-3/4/5/6/7 同模式).

## §0 立场总表 (3 立场 + 4 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | plugin lifecycle 5 事件复用 admin_actions 表 — 不另开 plugin_lifecycle_events 表; admin_actions CHECK enum 12-step rebuild +5 条字面 (`plugin_connect / plugin_disconnect / plugin_reconnect / plugin_cold_start / plugin_heartbeat_timeout`); audit forward-only 跟 ADM-2.1 + AP-2 + BPP-4 跨四 milestone 同精神 (锁链第 5 处) | plugin-protocol.md §1.6 + ADM-2.1 admin_actions audit 立场承袭 | 反向 grep `plugin_lifecycle_events\|plugin_audit_log\|bpp_event_log` 在 internal/ 0 hit; 反向 grep `CREATE TABLE.*plugin_lifecycle\|ALTER TABLE plugin_lifecycle` 在 migrations/ 0 hit |
| ② | reason 字典复用 AL-1a 6-dict — heartbeat_timeout reason=`reasons.NetworkUnreachable`; cold_start reason=`reasons.RuntimeCrashed` (跟 BPP-6 #522 + BPP-7 SDK ColdStart byte-identical, AL-1a reason 锁链第 13 处) | reasons-spec.md (#496 SSOT) + AL-1 #492 single-gate | 反向 grep `runtime_recovered\|plugin_event_reason\|sdk_reason\|7th.*reason` 在 internal/ + sdk/ 0 hit; auditor metadata 必复用 reasons.* const 字面, 不 hardcode |
| ③ | lifecycle audit 仅 owner-only 视图 — `GET /api/v1/plugins/{plugin_id}/lifecycle` owner-only (跟 AL-2a/BPP-3.2/AL-1/AL-5/DM-4/CV-4 v2/BPP-7 owner-only 7 处同模式); admin god-mode 不挂此路径 (ADM-0 §1.3 红线) | admin-model.md ADM-0 §1.3 + auth-permissions.md owner-only 锚 | 反向 grep `admin.*plugin.*lifecycle\|admin.*BPP8` 在 internal/api/admin*.go 0 hit |
| ④ (边界) | LifecycleAuditor interface 单门 — 5 事件全走 `LifecycleAuditor.Record*` 5 method, 不开 raw `Store.InsertAdminAction("plugin_*", ...)` 旁路 | bpp-3.md §1 dispatcher single-gate 同精神 + AL-1 #492 single-gate | 反向 grep `InsertAdminAction.*"plugin_` 在 internal/bpp + internal/api 非 lifecycle_audit.go 路径 0 hit |
| ⑤ (边界) | 5 wire 处复用 BPP-3/4/5/6 既有 handler — connect 走 BPP-3 dispatcher / disconnect 走 hub Cleanup / reconnect 走 BPP-5 reconnect handler / cold_start 走 BPP-6 cold_start handler / heartbeat_timeout 走 BPP-4 watchdog | bpp-3/4/5/6 既有 handler 路径 | 5 处真挂 auditor.Record*, AST scan handler 文件含 `auditor.Record` 调用 |
| ⑥ (边界) | best-effort 立场承袭 — auditor 调用 fire-and-forget (log.Warn on InsertAdminAction error, 不 fail handler); 无 retry queue / 无持久化 deferred audit (跟 BPP-4/5/6/7 best-effort 立场承袭, 锁链延伸第 5 处) | bpp-4.md §0.3 best-effort 立场 | AST scan forbidden tokens `pendingLifecycleAudit\|lifecycleQueue\|deadLetterLifecycle` 在 internal/ 0 hit (锁链延伸第 5 处) |
| ⑦ (边界) | actor='system' byte-identical 跟 BPP-4 watchdog + AP-2 sweeper 同源 — auditor.Record* 写 `actor_id="system"` 字面; 跟 admin_actions actor 字段 audit 跨五 milestone 一致 | ADM-2.1 + BPP-4 + AP-2 cross-milestone audit 同精神 | const `LifecycleSystemActor = "system"` 字面单源, 反向 grep hardcode `"system"` 在 lifecycle_audit.go 1 处 (const 定义) |

## §1 立场 ① admin_actions 表复用 (BPP-8.1 守)

**反约束清单**:

- [ ] migration v=31 12-step rebuild — admin_actions CHECK enum 6 项 → 11 项加 5 条 plugin_* 字面 (跟 CV-3.1 / CV-2 v2 / AP-2 同模式)
- [ ] 不裂表 — 反向 grep `plugin_lifecycle_events\|plugin_audit_log\|bpp_event_log` 0 hit
- [ ] AcceptsAllNewActions 真测 — 6 legacy + 5 new INSERT 全通过
- [ ] RejectsUnknownAction — 5 反约束 reject (`plugin_xxx` 自造名字 reject)
- [ ] migration idempotent + Version=31 字面锁

## §2 立场 ② reason 6-dict 复用 (BPP-8.2 守)

**反约束清单**:

- [ ] LifecycleAuditor.RecordHeartbeatTimeout reason 字面=`reasons.NetworkUnreachable` (不 hardcode "network_unreachable" 字符串)
- [ ] LifecycleAuditor.RecordColdStart reason 字面=`reasons.RuntimeCrashed` (跟 BPP-6/BPP-7 byte-identical)
- [ ] AL-1a reason 锁链 BPP-8 = 第 13 处 (改 = 改十三处)
- [ ] 反向 grep `"network_unreachable"\|"runtime_crashed"` 在 lifecycle_audit.go 0 hit (强制走 reasons.* 引用)

## §3 立场 ③ owner-only 视图 (BPP-8.2 守)

**反约束清单**:

- [ ] GET endpoint 真测: owner returns 200 + cross-owner 403 + 401 unauth + 404 plugin not found
- [ ] admin god-mode 不挂 — 反向 grep `admin.*plugin.*lifecycle\|admin.*BPP8` 在 admin*.go 0 hit
- [ ] response sanitize — actor_id="system" 不展开 raw 用户 ID (跟 ADM-2 sanitizer 同模式)

## §4 蓝图边界 ④⑤⑥⑦ — 跟 single-gate / 复用 handler / best-effort / actor='system' 不漂

**反约束清单**:

- [ ] LifecycleAuditor 单门 — 反向 grep `InsertAdminAction.*"plugin_` 在 lifecycle_audit.go 外 0 hit
- [ ] 5 处 wire 真挂: BPP-3 dispatcher Connect / hub Cleanup Disconnect / BPP-5 reconnect / BPP-6 cold_start / BPP-4 watchdog timeout
- [ ] AST scan forbidden tokens `pendingLifecycleAudit\|lifecycleQueue\|deadLetterLifecycle` 0 hit (锁链延伸第 5 处)
- [ ] LifecycleSystemActor const 字面 byte-identical 跟 BPP-4 watchdog + AP-2 sweeper actor='system' 同源

## §5 退出条件

- §1 (5) + §2 (4) + §3 (3) + §4 (4) 全 ✅
- 反向 grep 5 项全 0 hit (不裂表 / 不扩 reason / 不开 dispatcher 旁路 / best-effort 锁链延伸第 5 处 / admin)
- AL-1a reason 锁链 BPP-8 = 第 13 处
- audit forward-only 锁链第 5 处 (ADM-2.1 + AP-2 + BPP-4 + BPP-8 跨五 milestone)
- AST 锁链延伸第 5 处 (BPP-4 + BPP-5 + BPP-6 + BPP-7 + BPP-8 forbidden tokens 全 0 hit)

# BPP-8 spec brief — plugin lifecycle audit log (≤80 行)

> 战马D · Phase 6 · ≤80 行 · 蓝图 [`plugin-protocol.md`](../../blueprint/current/plugin-protocol.md) §1.6 (失联与故障状态) + §3 (plugin lifecycle audit) + ADM-2.1 admin_actions forward-only 五 milestone 跨链同精神. 模块锚 [`plugin-protocol.md`](plugin-protocol.md) §BPP-8. 依赖 BPP-3 #489 dispatcher + BPP-4 #499 watchdog + BPP-5 #503 reconnect + BPP-6 #522 cold-start + BPP-7 SDK + AL-1 #492 5-state + REFACTOR-REASONS #496 6-dict + ADM-2.1 #484 admin_actions table.

## 0. 关键约束 (3 条立场, 蓝图 §1.6 + ADM-2.1 字面承袭)

1. **plugin lifecycle 5 事件复用 admin_actions 表 audit** — 不另开 `plugin_lifecycle_events` 表; 5 事件 (connect / disconnect / reconnect / cold_start / heartbeat_timeout) 走 `Store.InsertAdminAction(actor='system', action='plugin_<event>', target=<agent_id>, metadata=JSON{plugin_id, reason, ...})` (跟 ADM-2.1 #484 + AP-2 sweeper + BPP-4 watchdog 跨四 milestone audit forward-only 同精神 — 锁链第 5 处). **反约束**: 反向 grep `plugin_lifecycle_events\|plugin_audit_log\|bpp_event_log` 在 internal/ 0 hit (不裂表); admin_actions CHECK enum 加 5 条 'plugin_*' 字面 (12-step rebuild 跟 CV-3.1 / CV-2 v2 / AP-2 同模式).

2. **reason 字典复用 AL-1a 6-dict** — heartbeat_timeout reason 走 `reasons.NetworkUnreachable`; cold_start reason 走 `reasons.RuntimeCrashed` (跟 BPP-6 #522 + BPP-7 SDK ColdStart byte-identical, AL-1a reason 锁链 BPP-8 = **第 13 处** BPP-2.2/AL-2b/BPP-4/BPP-5/BPP-6/BPP-7/BPP-8). **反约束**: 反向 grep `runtime_recovered\|plugin_event_reason\|sdk_reason\|7th.*reason` 在 internal/ + sdk/ 0 hit; reasons SSOT 6-dict 不动.

3. **lifecycle audit 仅 owner-only 视图** — `GET /api/v1/plugins/{plugin_id}/lifecycle` owner-only (跟 AL-2a/BPP-3.2/AL-1/AL-5/DM-4/CV-4 v2 owner-only 7 处同模式); admin god-mode 不挂此路径 (ADM-0 §1.3 红线 — admin /admin-api/* rail 隔离, 跟 BPP-7 stance §0.7 同精神). **反约束**: 反向 grep `admin.*plugin.*lifecycle\|admin.*BPP8` 在 admin*.go 0 hit.

## 1. 拆段 (一 milestone 一 PR, 整段一次合 — 跟 BPP-2/3/4/5/6/7 协议同源)

| 段 | 文件 | 范围 |
|---|---|---|
| BPP-8.1 schema migration v=31 + admin_actions CHECK 扩 | `internal/migrations/bpp_8_1_admin_actions_plugin_actions.go` 新 (admin_actions CHECK enum 12-step rebuild — 6 项 → 11 项加 5 条 `'plugin_connect'`/`'plugin_disconnect'`/`'plugin_reconnect'`/`'plugin_cold_start'`/`'plugin_heartbeat_timeout'`) + `bpp_8_1_admin_actions_plugin_actions_test.go` 新 (4 unit: AcceptsAllNewActions / RejectsUnknownAction / VersionIs31 / Idempotent) | admin_actions CHECK 扩 5 条字面, 复用 ADM-2.1 表无新表 |
| BPP-8.2 server lifecycle hooks + handler | `internal/bpp/lifecycle_audit.go` 新 (LifecycleAuditor interface + AdminActionsLifecycleAuditor impl 走 Store.InsertAdminAction; 5 method RecordConnect / Disconnect / Reconnect / ColdStart / HeartbeatTimeout) + 既有 BPP-3 dispatcher + BPP-4 watchdog + BPP-5/6 handler 加 auditor 调用 (5 处 wire) + `internal/api/bpp_8_lifecycle_list.go` 新 (GET /api/v1/plugins/{plugin_id}/lifecycle owner-only ACL DESC limit 100) + 7 unit (5 RecordX / cross-owner reject / nil-safe + AST scan + reason 复用反断) | server-side wire 5 事件路径; admin_actions 行 audit forward-only |
| BPP-8.3 closure REG-BPP8 + acceptance + PROGRESS [x] | REG-BPP8-001..006 + acceptance/bpp-8.md + PROGRESS update + AST scan forbidden tokens 锁链延伸第 5 处 (`pendingLifecycleAudit\|lifecycleQueue\|deadLetterLifecycle\|plugin_lifecycle_events\|plugin_audit_log`) 0 hit | best-effort 立场承袭 BPP-4/5/6/7 锁链延伸第 5 处 |

## 2. 留账边界

- **lifecycle metric exporter** (留 v3) — Prometheus counter per event type, 跟 BPP-7 §2 留账同源
- **lifecycle event WS push** (留 v3) — owner UI 实时推 lifecycle event 留 RT-3.2 follow-up
- **per-org lifecycle aggregation** (留 v3) — 跨 owner / cross-org 看 organization-wide plugin health
- **lifecycle event retention policy** (留 v3) — admin_actions 表无 TTL, audit forward-only; v3 加 retention sweep

## 3. 反查 grep 锚 (Phase 6 验收 + BPP-8 实施 PR 必跑)

```
git grep -nE 'plugin_connect|plugin_disconnect|plugin_reconnect|plugin_cold_start|plugin_heartbeat_timeout' packages/server-go/internal/   # ≥ 5 hit (5 字面真挂)
git grep -nE 'AdminActionsLifecycleAuditor|LifecycleAuditor' packages/server-go/internal/bpp/   # ≥ 1 hit (auditor interface seam)
# 反约束 (5 条 0 hit)
git grep -nE 'plugin_lifecycle_events|plugin_audit_log|bpp_event_log' packages/server-go/internal/   # 0 hit (复用 admin_actions, §0.1)
git grep -nE 'runtime_recovered|plugin_event_reason|sdk_reason|7th.*reason' packages/server-go/internal/   # 0 hit (复用 6-dict, §0.2)
git grep -nE 'pendingLifecycleAudit|lifecycleQueue|deadLetterLifecycle' packages/server-go/internal/   # 0 hit (best-effort 锁链延伸第 5 处, §3)
git grep -nE 'admin.*plugin.*lifecycle|admin.*BPP8' packages/server-go/internal/api/admin*.go   # 0 hit (ADM-0 §1.3 红线, §0.3)
git grep -nE 'CREATE TABLE.*plugin_lifecycle|ALTER TABLE plugin_lifecycle' packages/server-go/internal/migrations/   # 0 hit (不裂表, §0.1)
```

## 4. 不在本轮范围 (反约束 deferred)

- ❌ Prometheus metrics exporter (留 v3 跟 BPP-7 §2 同源)
- ❌ lifecycle event WS push (留 v3 RT-3.2)
- ❌ cross-org lifecycle aggregation (留 v3 跟 AP-3 联动)
- ❌ lifecycle retention policy (留 v3 sweep)
- ❌ admin god-mode 看 lifecycle history (ADM-0 §1.3 红线)
- ❌ AL-1 6-dict 扩第 7 reason (字典分立, 跟 BPP-7 §4 同源)
- ❌ 另开 plugin_lifecycle_events 表 (§0.1 立场, 复用 admin_actions)
- ❌ lifecycle event 实时 retry queue (best-effort 立场承袭 BPP-4/5/6/7 锁链延伸第 5 处)

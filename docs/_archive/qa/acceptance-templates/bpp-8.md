# Acceptance Template — BPP-8: plugin lifecycle audit log

> 蓝图 `plugin-protocol.md` §1.6 + §3 plugin lifecycle + ADM-2.1 admin_actions 跨链承袭. Spec `bpp-8-spec.md` (战马D v0) + Stance `bpp-8-stance-checklist.md` (战马D v0). 不需 content-lock — 内部 audit log 无 DOM/UI. Owner: 战马D 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 BPP-8.1 — schema migration v=31 admin_actions CHECK +5 条

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 admin_actions CHECK enum 12-step rebuild — 6 项 → 11 项加 5 条 plugin_* 字面 (`plugin_connect / plugin_disconnect / plugin_reconnect / plugin_cold_start / plugin_heartbeat_timeout`); 跟 CV-3.1 / CV-2 v2 / AP-2 12-step 同模式 | unit (4 sub-case) | 战马D / 烈马 | `internal/migrations/bpp_8_1_admin_actions_plugin_actions_test.go::TestBPP81_AcceptsAllNewActions` (6 legacy + 5 new INSERT 全通过) + `_RejectsUnknownAction` (5 反约束 plugin_xxx reject) + `_VersionIs31` + `_Idempotent` |
| 1.2 立场 ① 不裂表 — 反向 grep `plugin_lifecycle_events\|plugin_audit_log\|bpp_event_log` 0 hit + `CREATE TABLE.*plugin_lifecycle\|ALTER TABLE plugin_lifecycle` 0 hit | grep | 战马D / 飞马 / 烈马 | `TestBPP81_NoSeparateLifecycleTable` (反向 grep migrations + internal/ production 0 hit) |

### §2 BPP-8.2 — server LifecycleAuditor + 5 wire + GET endpoint

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 LifecycleAuditor interface + AdminActionsLifecycleAuditor impl — 5 method (RecordConnect / Disconnect / Reconnect / ColdStart / HeartbeatTimeout); reason 复用 reasons.NetworkUnreachable / reasons.RuntimeCrashed byte-identical (AL-1a 锁链第 13 处) | unit (5 RecordX + reason 字面 byte-identical) | 战马D / 烈马 | `internal/bpp/lifecycle_audit_test.go::TestBPP82_RecordConnect` + `_RecordDisconnect` + `_RecordReconnect` + `_RecordColdStart_ReasonRuntimeCrashed` (字面对比) + `_RecordHeartbeatTimeout_ReasonNetworkUnreachable` (字面对比) |
| 2.2 立场 ④ single-gate — 反向 grep `InsertAdminAction.*"plugin_` 在 lifecycle_audit.go 外 0 hit + `TestBPP82_NilSafeCtor` (nil store/logger panic boot bug) | grep + unit | 战马D / 飞马 / 烈马 | `TestBPP82_LifecycleAuditor_SingleGate` + `TestBPP82_NilSafeCtor` |
| 2.3 GET /api/v1/plugins/{plugin_id}/lifecycle owner-only ACL DESC limit 100 — happy + cross-owner 403 + 401 + 404 | unit (4 sub-case) | 战马D / 烈马 | `internal/api/bpp_8_lifecycle_list_test.go::TestBPP82_LifecycleList_HappyPath` + `_CrossOwnerReject` + `_Unauthorized401` + `_PluginNotFound404` |

### §3 BPP-8.3 — closure + AST 锁链延伸第 5 处

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 立场 ⑥ best-effort 锁链延伸第 5 处 — AST scan forbidden tokens `pendingLifecycleAudit\|lifecycleQueue\|deadLetterLifecycle` 在 internal/ 0 hit (跟 BPP-4 dead_letter_test + BPP-5 reconnect_handler_test + BPP-6 cold_start_handler_test + BPP-7 sdk_test 锁链延伸第 5 处) | AST scan | 飞马 / 烈马 | `TestBPP83_NoLifecycleQueueOrAuditTable` (AST ident scan internal/bpp + internal/api production 0 hit) |
| 3.2 立场 ⑦ admin god-mode 不挂 + actor='system' byte-identical — 反向 grep `admin.*plugin.*lifecycle\|admin.*BPP8` 在 admin*.go 0 hit + LifecycleSystemActor const 字面单源 | grep | 飞马 / 烈马 | `TestBPP83_AdminGodModeNotMounted` + `TestBPP83_LifecycleSystemActor_ByteIdentical` |

## 边界

- BPP-3 #489 dispatcher (Connect 走 dispatcher, auditor 在 connect handler 加 hook) / BPP-4 #499 watchdog (HeartbeatTimeout) / BPP-5 #503 reconnect handler (Reconnect) / BPP-6 #522 cold_start handler (ColdStart) / BPP-7 SDK (client side reconnect/cold-start 触发 server side audit) / AL-1 #492 + REFACTOR-REASONS #496 6-dict (reason 锁链第 13 处) / ADM-2.1 #484 admin_actions (audit forward-only 锁链第 5 处) / AP-2 #525 sweeper (audit 复用同精神) / ADM-0 §1.3 红线 (admin 不挂) / AST 锁链延伸第 5 处 (BPP-4+5+6+7+8)

## 退出条件

- §1 (2) + §2 (3) + §3 (2) 全绿 — 一票否决
- AL-1a reason 锁链 BPP-8 = 第 13 处 (BPP-7 第 12 链承袭不漂)
- audit forward-only 锁链第 5 处 (ADM-2.1 + AP-2 + BPP-4 + BPP-8 + BPP-7 admin_actions 跨五 milestone)
- AST 锁链延伸第 5 处 (BPP-4 + BPP-5 + BPP-6 + BPP-7 + BPP-8 forbidden tokens 全 0 hit)
- 登记 REG-BPP8-001..006

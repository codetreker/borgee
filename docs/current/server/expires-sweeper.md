# AP-2 expires_at sweeper — implementation note

> AP-2 (#525) · Phase 5+ · 蓝图 [`auth-permissions.md`](../../blueprint/current/auth-permissions.md) §5 (周期性 sweep, 不要求实时) + AP-1.1 #493 schema (`user_permissions.expires_at` reserved) + ADM-2.1 #484 admin_actions audit reuse + AL-1b #458 sweeper goroutine pattern.

## 1. 设计

闭 AP-1.1 reserved column 的运行时回路 — 周期性 goroutine 扫 `user_permissions WHERE expires_at < now AND revoked_at IS NULL`, 写 `revoked_at = expires_at` (forward-only soft-delete, 不 DELETE row), 每行落一条 `admin_actions` audit (复用 ADM-2.1 既有 path, actor='system').

设计原则 (跟 ap-2-spec.md §0):
- ① forward-only soft-delete (UPDATE revoked_at, 不 DELETE row, 跟 AL-1 state_log + ADM-2.1 admin_actions 同精神)
- ② 复用 admin_actions, 不另起 expires_audit 表 (audit 单表, 改 = 改 ADM-2.1 一处)
- ③ time.Ticker, 不引 cron 框架 (跟 AL-1b agent_status sweeper #458 同模式)
- ④ actor='system' 字面跨 milestone 锁 (BPP-4 watchdog / AL-1 system writer / DL-4 push GC / 未来自动 audit 写者同源)
- 反约束: admin god-mode 不挂此 path (ADM-0 §1.3 红线; admin 主动 revoke 走 ADM-3+ 单独 path)
- 反约束: 不 time.Sleep, 不真删 row, 不另起 reason 字典

## 2. Public surface (`internal/auth/expires_sweeper.go`)

| Symbol | Purpose |
|---|---|
| `ExpiresSweeper{Store, Logger, Interval, Now}` | config struct (全字段 nil-safe, 跟 AL-1b 同模式) |
| `(*ExpiresSweeper).Start(ctx)` | goroutine 启动 (ticker, ctx-aware shutdown, returns immediately) |
| `(*ExpiresSweeper).RunOnce(ctx) (count int, err error)` | 单次扫描入口 (testable 同步 path, Start 内部循环走此) |
| `ReasonPermissionExpired = "permission_expired"` | byte-identical 跟 admin_actions CHECK 6-tuple 同源 (改 = 改 const + ap_2_1 migration) |
| `SystemActorID = "system"` | 跨 milestone 锁字面 |
| `DefaultSweeperInterval = 1 * time.Hour` | 蓝图 §5 字面 (业务 SLA + 运维成本平衡, v2+ 可调) |

## 3. Sweeper flow (RunOnce)

1. `nowMs := now().UnixMilli()`
2. SELECT `id, user_id, permission, scope, expires_at` FROM `user_permissions` WHERE `expires_at IS NOT NULL AND expires_at < ? AND revoked_at IS NULL`
3. 对每行:
   - UPDATE `user_permissions SET revoked_at = expires_at WHERE id = ? AND revoked_at IS NULL` (idempotent — 二次 WHERE 守)
   - `Store.InsertAdminAction(actor='system', target=user_id, action='permission_expired', meta={permission, scope, original_expires_at})`
4. Return `(revoked_count, nil)`

幂等性: 同一瞬间二次 RunOnce 返回 count==0 (revoked rows 被 WHERE 排除).

## 4. Schema 联动 (ap_2_1 migration v=30)

`packages/server-go/internal/migrations/ap_2_1_user_permissions_revoked.go` 加 `revoked_at INTEGER NULL` 字段 + 扩 admin_actions CHECK 加入 `'permission_expired'` 6-tuple. Migration registry 顺序锁定, `revoked_at` 跟 `expires_at` 同 INTEGER ms 时间戳.

## 5. 反约束 grep (PR lint 守)

- `DELETE FROM user_permissions` 在 `internal/auth/`+`internal/api/` 除本文件 count==0
- `cron|gocron` 在 `internal/auth/expires_sweeper.go` count==0
- `time.Sleep` 在本文件 count==0
- `expires_audit` 字面在 `internal/` count==0 (复用 admin_actions 守)
- admin-api router 不挂 sweeper trigger endpoint (ADM-0 §1.3 红线)

## 6. 测试覆盖

`internal/auth/expires_sweeper_test.go` (353 lines, 11 unit):
- `_RunOnce_NoExpired_Returns0` — empty / future expires_at 不动
- `_RunOnce_ExpiredRow_Revoked` — happy path: revoked_at = expires_at + audit row
- `_RunOnce_NullExpiresAt_Skipped` — 永久权限 (NULL) 不扫
- `_RunOnce_AlreadyRevoked_Skipped` — 二次 sweep 幂等
- `_RunOnce_MultipleRows_AllRevoked` — batch path
- `_RunOnce_AuditRow_ActorSystem` — actor='system' 字面守
- `_RunOnce_AuditRow_ActionLiteral` — action='permission_expired' 字面守
- `_RunOnce_AuditMeta_PermissionScope` — meta JSON 含 permission + scope + original_expires_at
- `_Start_TickerFires_RunOnceCalled` — goroutine + ticker 联动
- `_Start_CtxCancel_GoroutineExits` — ctx-aware shutdown
- `_NilSafe_StoreNil_NoOp` — defensive

## 7. 跨 milestone byte-identical 锁

- AP-1.1 #493 `user_permissions.expires_at` schema (改 = 改 AP-1.1 migration + 本 sweeper WHERE)
- ADM-2.1 #484 admin_actions audit path (改 = 改 ADM-2.1 InsertAdminAction + 本 sweeper Step 3)
- AL-1b #458 sweeper goroutine pattern (Start nil-safe + ctx-aware + RunOnce sync entry)
- BPP-4 watchdog actor='system' 跨五 milestone 字面锁
- ADM-0 §1.3 红线 (admin god-mode 不入业务态变更)

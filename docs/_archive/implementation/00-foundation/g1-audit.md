# G1 Audit — Phase 1 退出闸 (audit 集成)

> 作者: 烈马 (QA) · 2026-04-28 · team-lead Phase 1 退出 gate 收尾派活
> 目的: 把 Phase 1 五道闸 (G1.1–G1.5) + audit row 一次性集成审完, 留下 single-source-of-truth, 后续 milestone 引用即可不重抄。
> 形式: 此文件 = audit 报告; 签字单独走 `docs/qa/signoffs/g1-exit-gate.md`。

---

## 1. 闸概览

| 闸 | 主旨 | Trigger PR | Status |
|---|---|---|---|
| G1.1 | CM-1 organizations 表 + users.org_id 列存在 + NOT NULL + 默认 `''` | #184 (CM-1) | ✅ |
| G1.2 | CM-1 idx_*_org_id 索引齐 (users/channels/messages/workspace_files/remote_nodes 5 个) | #184 | ✅ |
| G1.3 | CM-1 v=2 schema_migrations 行落地 + 幂等 rerun | #184 | ✅ |
| G1.4 | 读路径 EXPLAIN 走 idx_*_org_id + JOIN owner_id 黑名单 grep + 跨 org 反向 403 | #208 (CM-3) + audit 2026-04-28 | ✅ closed (本文 §2) |
| G1.5 | AP-0 注册回填默认权限 `[message.send, message.read]` | #184 | ✅ |
| G1.audit | Phase 1 跨 milestone audit row (CM-1 / AP-0 / CM-4 / CM-3) | 本文 §3 | ✅ |

G1.4 在 PR #184 merge 时被标 ⏸️ deferred (因为读路径 + 跨 org 反向需要 CM-3 owner_id→org_id 替换之后才有意义), 本次 audit 闭合。

---

## 2. G1.4 audit 集成 (2026-04-28)

### 2.1 §2 黑名单 grep — JOIN.*owner_id count==0

cm-3 §2 要求: 主资源表 (messages / channels / workspace_files / agents / remote_nodes) JOIN owner_id 必须全删。

```
$ grep -rEn "JOIN.*(messages|channels|workspace_files|agents|remote_nodes).*owner_id" \
    packages/server-go/internal/store/ \
    | grep -v _test.go | grep -v queries_cm3.go
(no matches)
```

`queries_cm3.go` 含此 regex 的注释串 (用于 self-document 黑名单), 不算违规, 故 grep 排除。

### 2.2 §3 反向 403 — 跨 org 三类资源

```
=== RUN   TestCrossOrgRead403/PUT_cross-org_→_403         --- PASS
=== RUN   TestCrossOrgRead403/DELETE_cross-org_→_403      --- PASS
=== RUN   TestCrossOrgChannel403                          --- PASS
=== RUN   TestCrossOrgFile403                             --- PASS
```

3 个 test 函数 (TestCrossOrgRead403 / TestCrossOrgChannel403 / TestCrossOrgFile403)
落 `internal/api/cross_org_test.go`。 PUT/DELETE/GET 三动词 × messages/channels/workspace_files
三资源全部返 403, 不是 200/404/500 — 即 authz 在归属层失败-关闭, 不是 sneaky 404 或路径泄漏。

### 2.3 §3 EXPLAIN QUERY PLAN — 6 主查询 idx_*_org_id 命中

跑 `EXPLAIN QUERY PLAN` 对 6 条主读路径 (build-time 验证, sqlite in-mem):

```
== users by org ==              SEARCH users          USING INDEX idx_users_org_id           (org_id=?)
== users role=agent ==          SEARCH users          USING INDEX idx_users_org_id           (org_id=?)
== channels by org ==           SEARCH channels       USING INDEX idx_channels_org_id        (org_id=?)
== messages by org+channel ==   SEARCH messages       USING INDEX idx_messages_org_id        (org_id=?)
== workspace_files by org ==    SEARCH workspace_files USING INDEX idx_workspace_files_org_id (org_id=?)
== remote_nodes by org ==       SEARCH remote_nodes   USING INDEX idx_remote_nodes_org_id    (org_id=?)
```

注: 项目无独立 `agents` 表 — agent = `users WHERE role='agent'`, 故 agent 路径走
`idx_users_org_id` (与 users 同索引)。 6 条全部 `SEARCH ... USING INDEX`, 无 `SCAN TABLE` —
即 org 隔离不会触发全表扫描, 多 org 规模线性扩展不退化。

> Audit harness 是临时的 `cmd/explain-audit/main.go`, 跑完即删 (不入产物); EXPLAIN
> 输出留此文件即可作 single source of truth。 后续 G2.x 若要常态化, 提到 RT-0 时再补。

### 2.4 全量 go test ./...

```
$ cd packages/server-go && go test ./...
ok      ... 16 packages green
```

`internal/migrations` (含 cm_3_org_id_backfill_test.go) + `internal/api`
(cross_org_test.go) + `internal/store` 全绿。

---

## 3. G1.audit row — Phase 1 跨 milestone codedebt 登记

| Audit ID | Source milestone | 内容 (单行) | 接收 milestone | Status |
|---|---|---|---|---|
| AUD-G1-CM1-a | CM-1 (#184) | `organizations.deleted_at` 软删未实现, 现版 organizations 不可删 | Phase 2 (TBD) | 📝 logged |
| AUD-G1-CM1-b | CM-1 (#184) | `users.org_id` 默认 `''` (空串) — 注册时必须显式 set, 历史回填依 CM-3 backfill | CM-3 #208 ✅ closed | ✅ closed |
| AUD-G1-AP0 | AP-0 (#184) | 默认权限 source-of-truth 锁 `internal/store/queries.go::GrantDefaultPermissions` 行 375 (R3 Decision #1) | (持续) | ✅ stable |
| AUD-G1-CM4 | CM-4.1 (#185) | `c.JSON.*\b(User\|AgentInvitation)\b` sanitizer fail-closed (REG-INV-001) — 防新 endpoint 漏过 | (持续) | ✅ stable |
| AUD-G1-CM3-a | CM-3 (#208) | `cm_3_org_id_backfill` v=9 迁移历史 NULL/空串 org_id 行 — idempotent | (forward only) | ✅ closed |
| AUD-G1-CM3-b | CM-3 (#208) | owner_id 列保留 (没删) — read 路径不读, write 时双写; 后续 G2 evaluate retire | Phase 2 (TBD) | 📝 logged |

`📝 logged` = 留作 Phase 2 输入, 不阻塞 Phase 1 退出; `✅ closed` / `✅ stable` 已闭合。

---

## 3.5. G2.AP-0-bis audit (2026-04-28, 实测)

PR #206 acceptance template 11 项是 owner 自勾, 此节是 QA 实测 + 落 audit row。

### 3.5.1 Backfill migration 跑两遍 idempotent

```
$ go test ./internal/migrations/ -run AP0Bis -v
=== RUN   TestAP0Bis_BackfillsMessageReadForLegacyAgents       --- PASS
=== RUN   TestAP0Bis_Idempotent                                --- PASS
=== RUN   TestAP0Bis_SkipsNonAgentRoles                        --- PASS
=== RUN   TestAP0Bis_SkipsSoftDeletedAgents                    --- PASS
ok      borgee-server/internal/migrations  0.007s
```

`TestAP0Bis_Idempotent` 跑 v=8 两次, 验 `(agent_id, 'message.read', '*')` 不重复插。
`SkipsNonAgentRoles` + `SkipsSoftDeletedAgents` 是 SQL `WHERE` 守门反向断言 (role!='agent' 或 deleted_at IS NOT NULL → 不回填), 即 backfill SQL 范围正确, 不污染 member / admin / 软删 agent 行。

### 3.5.2 SeedLegacyAgent + GET messages → 403 反向断言

```
$ go test ./internal/api/ -run 'TestGetMessages.*Legacy|TestGetMessages.*Agent' -v
=== RUN   TestGetMessages_LegacyAgentNoReadPerm_403            --- PASS
=== RUN   TestGetMessages_AgentWithReadPerm_200                --- PASS
ok      borgee-server/internal/api  0.048s
```

`TestGetMessages_LegacyAgentNoReadPerm_403`: `testutil.SeedLegacyAgent` (只授 `message.send`, 不授 `message.read`) → GET /api/v1/channels/{id}/messages 返 403 (不是 200/404/500), 即 RequirePermission gate 真生效。配套正向: `TestGetMessages_AgentWithReadPerm_200` 默认 agent (含 `message.read`) → 200。

### 3.5.3 EXPLAIN QUERY PLAN — user_permissions 索引命中

`auth.RequirePermission` 调 `Store.ListUserPermissions(userID)` = `WHERE user_id = ?`。 EXPLAIN 输出 (sqlite in-mem, 真 schema):

```
== SELECT * FROM user_permissions WHERE user_id = 'u1'
   SEARCH user_permissions USING INDEX idx_user_permissions_lookup (user_id=?)

== SELECT * FROM user_permissions WHERE user_id = 'u1' AND permission = 'message.read' AND scope = '*'
   SEARCH user_permissions USING INDEX sqlite_autoindex_user_permissions_1 (user_id=? AND permission=? AND scope=?)
```

两条全部 `SEARCH ... USING INDEX`, 无 `SCAN TABLE`。 第一条用 `idx_user_permissions_lookup` 复合索引 (user_id 前缀); 第二条用 sqlite 自动索引 (UNIQUE 约束自带, store/queries.go:257 `FirstOrCreate` 依此去重)。 `idx_user_permissions_user` 单列索引存在但未被规划器优先选 — 因 `_lookup` 复合索引覆盖更多列, sqlite 选了它。 (即: 中间件 hot path 不会全表扫描。)

### 3.5.4 Default capability set 锁

```
$ grep -n '"message.read"' packages/server-go/internal/store/queries.go
375:        perms = []string{"message.send", "message.read"}
$ go test ./internal/store/ -run TestDefaultPermissionsAgent -v
=== RUN   TestDefaultPermissionsAgent                          --- PASS
ok      borgee-server/internal/store  0.014s
```

R3 Decision #1 锁 `[message.send, message.read]` 在 `GrantDefaultPermissions` 单一 source-of-truth (REG-AP0B-006)。

### 3.5.5 G2.AP-0-bis audit row

| Audit ID | Source milestone | 内容 (单行) | 接收 milestone | Status |
|---|---|---|---|---|
| AUD-G2-APB-a | AP-0-bis (#206) | v=8 backfill idempotent + role/deleted_at 守门, 实测 4 test PASS | (forward only) | ✅ closed |
| AUD-G2-APB-b | AP-0-bis (#206) | RequirePermission gate 反向 403 实测 (TestGetMessages_LegacyAgentNoReadPerm_403) | (持续) | ✅ stable |
| AUD-G2-APB-c | AP-0-bis (#206) | user_permissions hot-path EXPLAIN 走 idx_user_permissions_lookup, 无 SCAN | (持续) | ✅ stable |
| AUD-G2-APB-d | AP-0-bis (#206) | Default cap set `[message.send, message.read]` 锁 queries.go:375 (R3 Decision #1) | (持续) | ✅ stable |
| AUD-G2-APB-e | AP-0-bis (#206) | `idx_user_permissions_user` 单列索引存在但 sqlite 规划器优先选 `_lookup` 复合 — 是否退役? Phase 2 evaluate | Phase 2 (TBD) | 📝 logged |

---

## 4. Phase 1 退出闸结论

5 道闸 + audit row 全签 ✅ — 见 `docs/qa/signoffs/g1-exit-gate.md`。
Registry 引用: 见 `docs/qa/regression-registry.md` §4 + 第 6 节 (CM-3 + G1.4 4 行 🟢)。

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-28 | 烈马 | v1 — Phase 1 退出 gate audit 集成完成, G1.4 闭合, audit row 落地 |
| 2026-04-28 | 烈马 | + §3.5 G2.AP-0-bis 实测 audit (4 mig test + 2 api test PASS, EXPLAIN idx_*_lookup 命中, 5 audit row 落地) |

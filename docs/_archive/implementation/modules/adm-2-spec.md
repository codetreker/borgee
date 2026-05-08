# ADM-2 实施 Spec v0 — 分层透明 audit (战马接力直接吃)

> **状态**: v0 (战马D PM 客串, 2026-04-29) — ADM-1 ✅ 落 (#455+#459+#464), ADM-2 实施前置就位
> **配套**: `docs/qa/adm-2-content-lock.md` (文案锁) + `docs/qa/adm-2-stance-checklist.md` (立场反查) + `docs/qa/acceptance-templates/adm-2.md` v1 (验收 7 项)
> **派活**: ADM-2.1 schema ✅ #470 (admin_actions v=22) / ADM-2.2 server (待派) / ADM-2.3 e2e (待派) / ADM-2.x closure
> **关联**: `admin-model.md §1.4` (谁能看到什么 + 三红线) + `admin-model.md §2` (Audit 100% 留痕 不变量) + `adm-1-implementation-spec.md §4 第 4 项` (admin 写动作 system DM, ADM-1 deferred 在此真实施)
> **R2 注**: 野马 R2 取消 ⭐ 标志性 — 普通用户零感知, 不进野马签字闸 4 (内部 milestone, 烈马代签 ADM-1 §4.1 文案兑现)

---

## 0. 立场速读 (跟 stance-checklist.md 同源)

ADM-2 = **admin 一切写动作必留痕 + 受影响者必感知 + audit 分层可见**。

5 立场 byte-identical (反查表 §1):

1. **每写必留痕** — admin SPA 任意写路径自动 INSERT admin_actions, 无白名单旁路 (蓝图 §1.4 红线 1)
2. **受影响者必感知** — system DM 强制下发, 不依赖前端订阅, 字面含 `admin_name` (admins.username) 非 raw UUID (蓝图 §1.4 红线 2)
3. **admin 之间互可见** — `/admin-api/v1/audit-log` 返回**全部** admin_actions; user 只见自己 target_user_id 行 (蓝图 §1.4 红线 3 + 分层)
4. **forward-only** — audit 不可改写, 表无 updated_at, 不开 DELETE/UPDATE 路径 (蓝图 §2 不变量)
5. **impersonate 显眼** — 业主 grant 时 admin 端 24h 倒计时显示 + 业主端顶部红横幅 (蓝图 §1.4 红线 2 第二档)

---

## 1. ADM-2.1 schema (已 #470 落地)

| Task | 文件 | 说明 |
|---|---|---|
| ① migration v=22 | `packages/server-go/internal/migrations/adm_2_1_admin_actions.go` | admin_actions 6 列 + CHECK 5 action + 双索引 |
| ② 7 单测 | `adm_2_1_admin_actions_test.go` | 表结构 / 5 action accept / 15 反约束 reject / 反向列名 / 索引 / PK / idempotent |

详见 PR #470 — schema-only 单 PR (跟 #361 DM-2.1 / #410 CHN-3.1 / #405 CV-4.1 占号同模式)。

---

## 2. ADM-2.2 server — 5 个写动作 + 双 GET endpoint + system DM emit

### 2.1 admin 写动作 audit hook (5 个枚举对应 5 个 admin SPA 路径)

| action | admin endpoint | target_user_id 来源 | metadata JSON |
|---|---|---|---|
| `delete_channel` | `DELETE /admin-api/v1/channels/:id` | channels.created_by | `{channel_id, channel_name, org_id}` |
| `suspend_user` | `PATCH /admin-api/v1/users/:id/suspend` | URL :id | `{reason}` |
| `change_role` | `PATCH /admin-api/v1/users/:id/role` | URL :id | `{old_role, new_role}` |
| `reset_password` | `POST /admin-api/v1/users/:id/reset-password` | URL :id | `{}` |
| `start_impersonation` | `POST /admin-api/v1/impersonation` | body.user_id | `{grant_id, expires_at}` |

实施: handler 内部 wrap — 写动作成功 commit 后 `INSERT INTO admin_actions(...)` + emit `system DM` 给 target_user (跟 CM-onboarding welcome system message 同模式, 走 `internal/store/queries.go::CreateSystemMessage`).

### 2.2 用户侧 GET (受影响者只见自己的)

`GET /api/v1/me/admin-actions` — 走 user cookie:

- 返回 `WHERE target_user_id = current_user_id ORDER BY created_at DESC LIMIT 50`
- 反向: 不接受 ?target_user_id 参数 (跨业主 inject 防线)
- 索引锚: `idx_admin_actions_target_user_id_created_at`

### 2.3 admin 侧 GET (admin 之间全可见)

`GET /admin-api/v1/audit-log` — 走 admin cookie:

- 返回 `ORDER BY created_at DESC LIMIT 100` (无 WHERE — admin 之间全可见, 蓝图 §1.4 红线 3)
- ?actor_id / ?action / ?target_user_id 三参数过滤 (admin SPA filter UI)
- 反向: user cookie 调此 endpoint → 401/403 (REG-ADM0-001/002 共享底线)
- 索引锚: `idx_admin_actions_actor_id_created_at`

### 2.4 system DM emit hook (蓝图 §1.4 红线 1)

每个写动作 success 后 emit system DM 给 target_user, body 字面 byte-identical (见 `adm-2-content-lock.md §1`):

```
你的 {resource_label} 被 admin {admin_username} 于 {ts} {action_verb}。详情见设置页"隐私 → 影响记录"。
```

5 个 action 对应 5 个 `{resource_label}` × `{action_verb}` 组合 (content lock 字面锁)。

### 2.5 impersonate 24h cooldown 机制

新表 `impersonation_grants` (本 spec §3 锚, ADM-2.2 第二阶段):

```
impersonation_grants:
  id, user_id (业主), granted_at, expires_at (granted_at + 24h),
  revoked_at (nullable; 业主可主动撤销)
```

业主走设置页 grant → 写一行 + emit DM 给业主自己 ("你已授权 admin 24h 影响你账号, 剩 23h59m, [立即撤销]")。

admin 写动作时若需 impersonate (例如重置密码影响业主活跃数据), server 校验 `impersonation_grants WHERE user_id = target_user_id AND expires_at > now AND revoked_at IS NULL`; 无 grant → 403 `impersonate.no_grant`。

> v1 ADM-2.2 范围: 仅落 5 个 audit action + 双 GET + system DM emit; impersonate grant 表落 v1, **业主授权 UI 留 ADM-2.3 (client) 一起做**。

---

## 3. ADM-2.3 e2e + G4.2 demo (野马代签 R2 取消, 烈马代签字面验)

| Task | 文件 | 说明 |
|---|---|---|
| ① e2e admin 重置密码 | `packages/e2e/tests/adm-2-audit.spec.ts` | admin login → POST /admin-api/v1/users/:id/reset-password → user cookie GET /api/v1/me/admin-actions 返 1 行 + body 含 `admin_username` 非 UUID |
| ② e2e admin 互可见 | 同 spec | 两个 admin: A 重置, B 调 /admin-api/v1/audit-log 见 A 的 1 行 |
| ③ e2e 反向 (跨业主 inject) | 同 spec | user-B cookie 调 GET /me/admin-actions?target_user_id=user-A → 参数被忽略 (只返 user-B 自己的) |
| ④ e2e 红横幅 24h | 同 spec | 业主 grant impersonate → DOM `[data-banner="impersonate-active"]` toBeVisible + 倒计时字面 `剩 23h` 字面锁 |
| ⑤ 截屏 G4.2 双张 | `docs/qa/screenshots/g4.2-adm2-{audit-list,red-banner}.png` | (1) 用户设置页 audit 列表显示 admin 操作记录 (2) 业主端顶部红横幅 24h 倒计时 |

---

## 4. 反向断言 7 项 (跟 acceptance §行为不变量 4.1.a-d 共测试)

| 反向断言 | 测试位置 | 锁点 |
|---|---|---|
| 4.1.a 每个 action 路径必写 audit | `internal/api/admin_audit_test.go::TestEachActionWritesAdminActions` (table-driven 5 action) | 反向: action 路径不写 audit → 该 endpoint 必红测 |
| 4.1.b 受影响者必收 system DM | `TestAdminWriteEmitsNamedDM` (跟 ADM-1 spec §4 第 4 项**共测试**, deferred 在此真实施) | grep DM body 不含 raw UUID + 含 `admin_username` 字面 |
| 4.1.c user 只见自己的 | `TestGetMeAdminActionsScopedToTargetUser` | 反向: ?target_user_id 参数被忽略 + 跨 user 调 → 空数组 (不泄漏跨 org) |
| 4.1.d admin 互可见 | `TestAdminAuditLogFullVisibility` | admin-X 调 /admin-api/v1/audit-log 见 admin-Y 的行 |
| §1.4 红线 1 user cookie 拒 admin endpoint | `TestUserCookieRejectsAdminAuditLog` (跟 REG-ADM0-002 共享底线) | 401/403 |
| §2 forward-only audit 不可改写 | `grep -rE "UPDATE admin_actions\|DELETE FROM admin_actions" packages/server-go/internal/` count==0 (除 migration 自身, 反向锁) | CI grep |
| 反向: action 路径有白名单旁路 | `grep -rE "skip_audit\|noAudit\|bypassAudit" packages/server-go/internal/admin/` count==0 | CI grep |

---

## 5. 不在 ADM-2 范围

- ❌ admin 之间互相 promote / demote — multi-admin v2+ (蓝图 §5 字面)
- ❌ Audit log 导出 (GDPR delete request 等) — v2+ (蓝图 §5)
- ❌ Impersonate 期间 BPP 行为 (agent 端是否知道) — 蓝图 §5 已确定 BPP 不暴露此概念
- ❌ 跨 org admin 多业主分桶 — multi-org admin v1+
- ❌ Audit log push 通知 (email / SMS) — system DM 即可
- ❌ admin SPA 端 audit log filter UI 复杂查询 (时间范围 / 全文搜索) — v1 仅 actor_id / action / target_user_id 三参数

---

## 6. 验收挂钩

- ADM-2.1 PR (#470) 合并条件: §1 schema 7 单测全绿 + registry §3 ADM-2 段 6+ 行 ⚪ 占号
- ADM-2.2 server PR 合并条件: §2 5 action audit hook + 双 GET + system DM emit + §4 反向断言 7 项绿 + REG-ADM2-001..007 ⚪→🟢
- ADM-2.3 client + e2e PR 合并条件: §3 5 e2e case + 双截屏 + 红横幅 24h DOM 锁
- ADM-2 closure (野马 R2 取消 ⭐ 不进 G4 签字流): 烈马代签 acceptance + ADM-1 #464 deferred 2 行翻 🟢

---

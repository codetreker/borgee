# ADM-2 立场反查 v0 — 7 立场 byte-identical

> **状态**: v0 (战马D, 2026-04-29) — 跟 `chn-3-stance-checklist.md` (野马 #366) / `cv-4-stance-checklist.md` (#378) 同模式
> **配套**: `docs/implementation/modules/adm-2-spec.md` + `docs/qa/adm-2-content-lock.md` + `docs/qa/acceptance-templates/adm-2.md` v1
> **锚**: 蓝图 `admin-model.md §1.4` (谁能看到什么 + 三红线) + §2 (Audit 100% 留痕 不变量) + §0 (强权但不窥视)

---

## §1. 7 立场 (一句话锁, 配反向)

### 立场 ① 每写必留痕 (蓝图 §1.4 红线 1)

**正面**: admin SPA 任意写动作 → 自动 INSERT admin_actions, 无白名单旁路。

**反面**: 不允许 `skip_audit` / `noAudit` / `bypassAudit` flag 走 fast-path; 反向 grep 在 `internal/admin/` count==0。

### 立场 ② 受影响者必感知 (蓝图 §1.4 红线 1 第二档)

**正面**: system DM 强制下发, 不依赖前端订阅; body 字面含 `admin_username` 非 raw UUID。

**反面**: 不允许业主端 mute / 关闭 system DM; 反向 grep `silent_admin_dm` / `mute_admin_actions` count==0。

### 立场 ③ admin 之间互可见 (蓝图 §1.4 红线 3)

**正面**: `/admin-api/v1/audit-log` 返回**全部** admin_actions, 不按 actor_id 过滤 (默认全可见, ?actor_id 仅 filter UI 用)。

**反面**: 不允许 super-admin / org-admin 权限分桶 (v2+ 留账, 蓝图 §5 字面); 反向 grep `super_admin\|org_admin` 在 admin 路径 count==0。

### 立场 ④ user 只见自己 (蓝图 §1.4 第四档)

**正面**: `/api/v1/me/admin-actions` 走 user cookie + `WHERE target_user_id = current_user_id`, 不接受 ?target_user_id 参数 (跨业主 inject 防线)。

**反面**: 不开全站 audit log 公开端点 (避免跨 org 隐私泄漏, 蓝图 §1.4 字面 "不对全体 user 公开"); 反向 grep `GET /api/v1/audit-log` (无 /me/) count==0。

### 立场 ⑤ forward-only audit (蓝图 §2 不变量)

**正面**: schema 不挂 `updated_at` 列, server 不开 UPDATE / DELETE 路径; 错误录入只能再写新行 (action='correction', metadata 引旧 id)。

**反面**: 不允许 admin 删除自己的 audit 行 (避免互相包庇); 反向 grep `DELETE FROM admin_actions\|UPDATE admin_actions` 在 internal/ (除 migration) count==0。

### 立场 ⑥ admin ∉ 业务路径 (ADM-0 红线承袭)

**正面**: actor_id FK admins.id (独立表, 蓝图 §2 不变量 "Admin ∉ users 表" 派生); admin 不在 channels / DMs / org members 中。

**反面**: 不允许 actor_id 引用 users.id; 反向 grep `actor_id.*users\b` 在 schema / migration 不存在 (跟 REG-ADM0-001/002 共享底线)。

### 立场 ⑦ impersonate 显眼 (蓝图 §1.4 红线 2 第二档 + §4.1 R3 ADM-1 兑现)

**正面**: 业主授权 → 顶部红横幅常驻 + 24h 倒计时 + `[立即撤销]` 入口; admin 写动作需 impersonate 时 server 校验 grant 存在 + 未过期 + 未撤销, 否则 403 `impersonate.no_grant`。

**反面**: 不允许 admin 自助 impersonate (没有 grant 直接进); 反向 grep `force_impersonate\|admin_impersonate_self` count==0。横幅不开 dismiss 按钮 (蓝图 R3 "常驻直到结束")。

---

## §2. 反约束黑名单 (cross-stance, CI grep 锁)

| ID | grep 模式 | 路径 | 期望 | 立场锚 |
|---|---|---|---|---|
| ADM2-NEG-001 | `\{admin_id\}\|\{actor_id\}\|\$\{adminId\}` | `internal/api/admin_actions/`, `packages/client/src/components/Settings/` | count==0 | 立场 ② |
| ADM2-NEG-002 | `skip_audit\|noAudit\|bypassAudit` | `packages/server-go/internal/admin/` | count==0 | 立场 ① |
| ADM2-NEG-003 | `silent_admin_dm\|mute_admin_actions` | `packages/server-go/internal/`, `packages/client/src/` | count==0 | 立场 ② |
| ADM2-NEG-004 | `super_admin\|org_admin` | `packages/server-go/internal/admin/` | count==0 (v2+ 留账) | 立场 ③ |
| ADM2-NEG-005 | `GET /api/v1/audit-log[^/]\|GET .*audit-log[^_]` | `packages/server-go/internal/api/` | count==0 (仅 /me/admin-actions + /admin-api/v1/audit-log 双 endpoint, 无全站公开) | 立场 ④ |
| ADM2-NEG-006 | `DELETE FROM admin_actions\|UPDATE admin_actions SET` | `internal/` (除 `internal/migrations/`) | count==0 | 立场 ⑤ |
| ADM2-NEG-007 | `actor_id.*users\b\|FROM users.*actor_id` | `internal/migrations/adm_2_*` | count==0 (FK admins.id 锁) | 立场 ⑥ |
| ADM2-NEG-008 | `force_impersonate\|admin_impersonate_self\|skip_grant` | `packages/server-go/internal/admin/` | count==0 | 立场 ⑦ |
| ADM2-NEG-009 | epoch ms 字面 (`1700000000000` 等具体值) 在 system DM body | `internal/api/admin_actions_dm.go` (待新建) | count==0, 走 `time.Format` | content-lock §1 |
| ADM2-NEG-010 | "重置了你的" / "暂停了你的" 中文动词字面 | `packages/client/src/admin/pages/` | count==0 (admin SPA 字面英文 enum, 跨端拆死) | content-lock §5 |

---

## §3. 跟 ADM-0 / ADM-1 共享底线 (cross-milestone 锚)

| 锚 | 来源 | ADM-2 兑现方式 |
|---|---|---|
| REG-ADM0-001 admin cookie 拒 user-api | adm-0.md §4.1.a | ADM-2 反向断言 §4 第 5 项: user cookie 调 `/admin-api/v1/audit-log` → 401/403 同款轨道 |
| REG-ADM0-002 user token 拒 admin-api | adm-0.md §4.1.b | 同上, 双向 fail-closed |
| REG-ADM0-003 admin god-mode 仅元数据 | adm-0.md §4.1.c | ADM-2 audit metadata JSON 不含 channel content / DM body / artifact 内容 (反向: metadata.body / metadata.content 字面 0 hit) |
| ADM-1 §4.1 R3 第 2 条 "24h 红横幅可撤销" | admin-model.md §4.1 (野马 R3 锁) | 立场 ⑦ + content-lock §2 + acceptance §3 e2e ④ |
| ADM-1 deferred §4 第 4 项 "admin 写动作 system DM admin_name 非 UUID" | adm-1.md acceptance §4 (deferred 给 ADM-2) | 立场 ② + content-lock §1 + acceptance §行为不变量 4.1.b |

---

## §4. R2 取消 ⭐ 注 (野马备注 + 烈马代签机制)

蓝图 R2 决议: ADM-2 取消 ⭐ 标志性 — 普通用户零感知 (受影响者才感知, 占总用户 < 1%), 不进野马 G4 签字流。

**烈马代签机制** (跟 cm-4 / adm-0 同格式 deferred):
- ADM-2 closure follow-up 由烈马签 acceptance + 野马仅审 ADM-1 §4.1 文案兑现 (字面 byte-identical 跟 admin-model §4.1 同源)
- G4.2 demo 双截屏 (audit 列表 + 红横幅) 入 git, 烈马签 `docs/qa/signoffs/adm-2-liema-signoff.md`
- 跟 ADM-1 G4.1 demo 同模式 (G4.1 ⏸️ pending 野马, G4.2 烈马代签)

---

## §5. 不在立场范围 (跟 spec §5 同源)

- ❌ admin 之间互相 promote / demote — multi-admin v2+
- ❌ Audit log 导出 (GDPR delete request) — v2+
- ❌ Impersonate 期间 BPP 行为 — 蓝图 §5 BPP 不暴露
- ❌ 跨 org admin 多业主分桶 — multi-org admin v1+
- ❌ Audit log push (email / SMS) — system DM 即可
- ❌ admin SPA filter UI 复杂查询 (时间范围 / 全文搜索) — v1 三参数

---

## §6. 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 战马D | v0 — 7 立场 byte-identical + 10 反约束黑名单 + 5 cross-milestone 共享底线 + R2 烈马代签机制 |

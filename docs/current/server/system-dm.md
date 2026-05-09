# System DM — Admin Action 受影响者通知 (ADM-2.2)

> ADM-2.2 (#484) · Phase 4 · 蓝图 [`admin-model.md`](../../blueprint/current/admin-model.md) §1.4 三红线 1 ("受影响者必感知") + §2 不变量 ("Audit 100% 留痕"). Content lock: [`docs/_archive/qa/adm-2-content-lock.md`](../../_archive/qa/adm-2-content-lock.md) §1 (5 模板字面). 原则反查: [`docs/_archive/qa/adm-2-stance-checklist.md`](../../_archive/qa/adm-2-stance-checklist.md) §2 ADM2-NEG-001..010.

## 1. 设计

5 个 admin write-action **每写必发** system DM 给受影响者 (delete_channel 给频道 created_by, 其余给 target_user_id). 模板字面**只用 admin login 名 (具体名)**, 不渲染 raw UUID 或模板占位符.

DM emit failure 不 rollback audit (蓝图 §2 优先 — audit 必留痕, DM 是体验补丁).

## 2. 5 模板字面 (byte-identical 跟 `internal/store/admin_actions.go::RenderAdminActionDMBody`)

| action | 模板 |
|---|---|
| `delete_channel` | `你的 channel #{channel_name} 被 admin {admin_username} 于 {ts} 删除。详情见设置页"隐私 → 影响记录"。` |
| `suspend_user` | `你的账号被 admin {admin_username} 于 {ts} 暂停: {reason}。详情见设置页"隐私 → 影响记录"。` (空 reason → `(未提供原因)`) |
| `change_role` | `你的账号角色被 admin {admin_username} 于 {ts} 从 {old_role} 调整为 {new_role}。详情见设置页"隐私 → 影响记录"。` |
| `reset_password` | `你的登录密码被 admin {admin_username} 于 {ts} 重置, 请重新生成。详情见设置页"隐私 → 影响记录"。` |
| `start_impersonation` | `admin {admin_username} 已对你的账号开启 24h impersonate, 起于 {ts}, 至 {expires_at}。可在设置页随时撤销。` |

## 3. 字段渲染规则

- **`{admin_username}`** = `admins.Login` (具体名). 反向约束 ADM2-NEG-001/009: body literal **不**含 `{admin_id}` / `{actor_id}` / `${adminId}` 占位符; **不**渲染 raw UUID.
- **`{ts}`** = `time.Format("2006-01-02 15:04")` (本地化人类可读). 反向约束: 不渲染 epoch ms 字面.
- **`{channel_name}`** / **`{old_role}`** / **`{new_role}`** / **`{reason}`** = `AdminActionDMContext` 调用方填. 空字符串走默认 ("(未提供原因)" 仅 suspend_user reason).

## 4. Emit 路径

`store.EmitAdminActionAudit(actorAdminID, action, targetUserID, ctx)` composite:
1. INSERT `admin_actions` (audit 行落库) — 设计 ① 每写必留痕
2. RenderAdminActionDMBody → 写入 system DM channel (受影响者 inbox)

DM emit 失败 (channel 缺失 / 网络) → 仅 log warn, audit 行已落不 rollback. 跟 DM-2.2 #372 mention dispatch 失败不阻 message 落库 同模式.

## 5. 反向约束 (原则 §2 ADM2-NEG)

- ADM2-NEG-001: body 不含模板占位符字面 — `git grep -nE '\{admin_id\}|\{actor_id\}|\$\{adminId\}'` count==0
- ADM2-NEG-009: actor 必走 admins.Login 不走 UUID — `RenderAdminActionDMBody` 不接受 admin id 入参
- ADM2-NEG-007: ts 不渲染 epoch — body literal **不**含 `\d{13}` 13 位整数字面 (单测正则锁定)
- ADM2-NEG-005: `admin_actions.metadata` JSON 不挂 `body` / `content` / `text` / `artifact` 字段 (god-mode 仅元数据, 蓝图 §1.4 隐私边界)

## 6. 相关参考

- 实施: `internal/store/admin_actions.go::RenderAdminActionDMBody` (字面单一来源)
- 单测: `internal/store/admin_actions_test.go` (5 模板 + 反向约束); `internal/api/adm_2_2_audit_hook_test.go` (full path E2E)
- spec brief: [`docs/implementation/modules/adm-2-spec.md`](../../implementation/modules/adm-2-spec.md) §2 ADM-2.2
- acceptance: [`docs/_archive/qa/acceptance-templates/adm-2.md`](../../_archive/qa/acceptance-templates/adm-2.md)

---

## 4. BPP-3.2.1 — `request_capability_grant` system DM (Phase 5)

> BPP-3.2.1 (#494 后续) · Phase 5 · 蓝图 [`auth-permissions.md`](../../blueprint/current/auth-permissions.md) §1.3 主入口字面 + spec [`bpp-3.2-spec.md`](../../implementation/modules/bpp-3.2-spec.md) §1 设计 ① + content-lock [`bpp-3.2-content-lock.md`](../../qa/bpp-3.2-content-lock.md) §1+§2.

### 4.1 触发路径

Plugin SDK 收 BPP-3.1 `permission_denied` frame (after AP-1 `auth.HasCapability` false 路径) → plugin 上行 `SemanticActionFrame{Action: "request_capability_grant"}` → BPP-2.1 dispatcher 路由到 `internal/api/capability_grant.go::CapabilityGrantHandler` → handler 写 system DM 到 owner's `type='system'` channel (CM-onboarding #203 既有).

### 4.2 DM body 字面 byte-identical (content-lock §1)

模板 const: `internal/api/capability_grant.go::CapabilityGrantDMTemplate`
```
{agent_name} 想 {attempted_action} 但缺权限 {required_capability}
```

字段 byte-identical 跟 BPP-3.1 PermissionDeniedFrame body + AP-1 `auth.abac.go` 403 body (跨 PR 防脱节, content-lock §5 五处). grep 检查 `agent.*尝试.*权限\|agent.*请求.*授权` 在 internal/api/ count==0 (近义词脱节禁).

### 4.3 quick_action JSON shape (content-lock §2)

```json
{"action":"grant","agent_id":"<uuid>","capability":"<14-AP-1-const>","scope":"<v1-three-layer>","request_id":"<uuid>"}
```

`action` 默认 `"grant"` (server-emitted); client UI 渲染三按钮 grant/reject/snooze 时按用户点击替换 action 字面 POST `/api/v1/me/grants`.

### 4.4 Capability + scope 校验

`required_capability` 必走 AP-1 `auth.Capabilities` 14 项 const; 字典外值 reject + log warn `bpp.grant_capability_disallowed`. grep 检查 `GrantPermission.*Permission:.*"<literal>"` 在 internal/api/ count==0 (跟 AP-1 反向约束 #1 同源).

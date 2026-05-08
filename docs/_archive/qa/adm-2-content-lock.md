# ADM-2 Content Lock v0 — 字面 byte-identical 锁

> **状态**: v0 (战马D, 2026-04-29) — 跟 `chn-2-content-lock.md` / `dm-2-content-lock.md` 同模式
> **配套**: `docs/implementation/modules/adm-2-spec.md` §2 + `docs/qa/acceptance-templates/adm-2.md` v1
> **锚**: 蓝图 `admin-model.md §1.4` (受影响者必感知 + 红横幅) + §4.1 (ADM-1 文案在此兑现)

---

## 1. system DM 字面锁 (蓝图 §1.4 红线 1 — 受影响者必感知)

每个 admin action 写动作成功后, server emit system DM 给 target_user, body 字面 byte-identical:

| action | resource_label | action_verb | DM body 模板 |
|---|---|---|---|
| `delete_channel` | `channel #{channel_name}` | `删除` | `你的 channel #{channel_name} 被 admin {admin_username} 于 {ts} 删除。详情见设置页"隐私 → 影响记录"。` |
| `suspend_user` | `账号` | `暂停` | `你的账号被 admin {admin_username} 于 {ts} 暂停: {reason}。详情见设置页"隐私 → 影响记录"。` |
| `change_role` | `账号角色` | `调整` | `你的账号角色被 admin {admin_username} 于 {ts} 从 {old_role} 调整为 {new_role}。详情见设置页"隐私 → 影响记录"。` |
| `reset_password` | `登录密码` | `重置` | `你的登录密码被 admin {admin_username} 于 {ts} 重置, 请重新生成。详情见设置页"隐私 → 影响记录"。` |
| `start_impersonation` | `账号` | `开启 24h impersonate` | `admin {admin_username} 已对你的账号开启 24h impersonate, 起于 {ts}, 至 {expires_at}。可在设置页随时撤销。` |

> 反约束: `{admin_username}` 必须是 `admins.username` (具体名), **绝不**渲染 raw UUID. 反向 grep `r'\{admin_id\}|\{actor_id\}|\$\{adminId\}'` 在 `internal/api/` count==0.

> 反约束: `{ts}` 走 RFC3339 本地化 (服务端 `time.Now().Format("2006-01-02 15:04")`), 不渲染 epoch ms. 反向 grep `r'created_at\s*\}|epoch'` 在 system DM body 路径 count==0.

---

## 2. 业主端红横幅字面锁 (蓝图 §1.4 红线 2 第二档 — impersonate 显眼)

业主端顶部红横幅 (跟 ADM-1 §4.1 R3 第 2 条 "顶部红色横幅常驻, 可随时撤销" 同源):

```
support {admin_username} 正在协助你, 剩 {remaining_h}h{remaining_m}m。  [立即撤销]
```

DOM 锚: `[data-banner="impersonate-active"]` (e2e 反查锚, 跟 `data-row-kind` / `data-tab` 同模式)。

CSS 锁: 红色 `#d33` 加粗 (跟 ADM-1 PrivacyPromise 三色锁 deny 同源 token, 不开第 4 色)。

倒计时刷新: client 端 `setInterval(1000)` 重算 `expires_at - now`, server 不 push (避免 RT-1 第 5 frame, 字面跟 CHN-4 立场 ⑥ 同精神)。

> 反约束: 横幅文案不渲染 raw UUID; `{admin_username}` 来自 `/api/v1/me/impersonate-status` GET 响应 (server JOIN admins 表派生)。

---

## 3. 业主授权页字面锁 (设置页 → 隐私 tab → impersonate 子段)

跟 ADM-1 PrivacyPromise 同页扩展 v2 (本 spec ADM-2.3 client 范围):

```
### 临时授权 admin 影响

授权后 24h 内, admin 可对你的账号执行 password 重置 / suspend / role 调整等写动作; 24h 后自动失效。

[ ] 授权 (24h, 顶部会显示红色横幅常驻)

当前状态: {未授权 | 已授权剩 23h59m, 由 admin {admin_username} 于 {granted_at} 起算}

[立即撤销] (仅在已授权时显示)
```

> 反约束: 默认未授权 (复选框未勾); 业主主动勾选 → POST /api/v1/me/impersonation-grants → 24h cooldown 期内不接受重复 grant (409 conflict)。

---

## 4. audit 列表字面锁 (用户侧设置页 — 影响记录子段)

设置页 → 隐私 tab → "影响记录" 子段 (ADM-2.3 client):

```
### admin 对你的影响记录 (最近 50 条)

| 时间 | 谁 | 做了什么 |
|---|---|---|
| 2026-04-29 14:32 | admin alice | 重置了你的登录密码 |
| 2026-04-25 09:11 | admin bob | 删除了你的 channel #demo |
```

每行 byte-identical 跟 §1 system DM 同源 (action_verb 短语); 反向: 列表行不渲染 raw UUID (跟 admin SPA 路径分叉锚 ADM-0 红线同精神)。

DOM 锚: `[data-section="admin-actions-history"]` + `[data-action-row]` 每行。

空态: `从未被 admin 影响过 — 你的隐私边界完整。`

---

## 5. admin SPA audit log 字面锁 (admin 端 — 蓝图 §1.4 红线 3 互可见)

admin SPA `/admin/audit-log` 页面:

```
### Admin Audit Log (全部 admin 操作, 互相可见)

| 时间 | 操作者 | action | target_user | metadata |
|---|---|---|---|---|
| ... | admin alice | reset_password | bob@example.com | {} |
```

> admin 端字面用英文 enum (字段值原样); 用户端字面用中文 (action_verb 翻译)。两边字面拆死, 不共享渲染。反向: admin SPA grep "重置了你的" count==0 (避免误用用户端字面在 admin 端)。

---

## 6. 反约束黑名单 (跟 §1-§5 配套)

- ❌ `{admin_id}` / `{actor_id}` / `${adminId}` 三模板字面在 system DM body 路径 — 必拒, 渲染必死
- ❌ DM body 含 raw UUID (`grep -E '[0-9a-f]{8}-[0-9a-f]{4}-' internal/api/admin_actions/*.go` 反向锁)
- ❌ `epoch` / `1700000000000` 字面 ts 渲染 — server 端 `time.Format` 必走
- ❌ admin 端中文动词字面 (e.g. "重置了你的") — admin SPA 走英文 enum, 跨端字面拆死
- ❌ 红横幅字面缺 `[立即撤销]` 入口 — 蓝图 §4.1 R3 "可随时撤销" 兑现
- ❌ 第 4 色横幅 — 三色锁 (gray / `#d33` / `#d97706`) 蓝图 §1.4 边界 byte-identical 跟 ADM-1 PrivacyPromise 同源

---

## 7. 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 战马D | v0 — 5 system DM body 模板 + 红横幅 + 业主授权页 + audit 列表 + admin SPA audit log + 6 反约束黑名单 |

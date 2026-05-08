# CM-3 资源归属 org_id 直查反查表

> **状态**: v0 (野马, 2026-04-28)
> **目的**: 战马 CM-3.1 (写路径) + CM-3.2 (读路径) PR 直接吃此表为 acceptance 锚点; 野马 / 飞马 PR review 拿此表反查漂移。
> **依赖**: CM-1 ✅ (org_id 列已加), CM-4 ✅ Phase 2 退出, ADM-0 进行中 (R3 决议: CM-3 必须等 ADM-0 落, admin fixture 拆完后才动)。
> **解封**: Phase 1 退出 gate (#30) 仅差 CM-3 + audit, 本表过 → CM-3 PR 落 → Phase 1 关闭。
> **关联**: `concept-model.md` §CM-3, blueprint §2 (资源归 org), `execution-plan.md` G1.4 (黑名单 grep `JOIN.*owner_id` 命中 0)。

---

## 1. 6 张表 — owner_id → org_id 直查路径

| # | 表 | 写路径 (CM-3.1) | 读路径 (CM-3.2) — 当前 owner_id 走法 → 改后 org_id 直查 | 反向断言 (跨 org 访问) |
|---|----|-----------------|-----|-----|
| ① | `messages` | INSERT 时 `org_id = sender.org_id` (auth ctx 取), NOT NULL | `WHERE channel_id IN (SELECT id FROM channels WHERE owner_id = ?)` → `WHERE org_id = ? AND channel_id = ?` (channel 属于 org 已由 ② 保证) | user A (orgA) 拿 messageId 属 orgB → `GET /api/messages/:id` **403 Forbidden** (不是 404, 不泄漏存在性 v0 接受) |
| ② | `channels` | INSERT 时 `org_id = creator.org_id`, NOT NULL | `WHERE owner_id = ? OR id IN (SELECT channel_id FROM channel_members WHERE user_id = ?)` → `WHERE org_id = ?` (member 关系 v0 沿用 channel_members JOIN, **不动**) | user A 列 orgA channels → 不出现任一 orgB channel; 直访 orgB channelId → **403** |
| ③ | `agents` | INSERT 时 `org_id = creator.org_id`, NOT NULL (agent 仍归属 owner_id user) | `WHERE owner_id = ?` → `WHERE org_id = ? AND owner_id = ?` (双重过滤, owner_id 保留是不同语义) | user A 拿 orgB agentId 加自己 channel → invitation 创建路径 **400 / 403** (跨 org 拒绝, agent_invitations 状态机已有锚) |
| ④ | `workspace_files` (artifacts) | INSERT 时 `org_id = uploader.org_id`, NOT NULL | `WHERE channel_id IN (...owner_id...)` → `WHERE org_id = ? AND channel_id = ?` | user A `GET /api/files/:id` 属 orgB → **403**; list `/api/channels/:id/files` 跨 org → **403** (channel 已 ② 拦) |
| ⑤ | `user_settings` (含 API key 状态 / impersonation grant) | 无 org_id 列 (per-user, 与 org 解耦) — **不在 CM-3 范围**, settings 走 `user_id = ?` 不变 | N/A | API key 跨 user 拿 → 401 (现有 auth 已拦, 不需 CM-3 改) |
| ⑥ | `remote_nodes` (host-bridge 节点) | INSERT 时 `org_id = registrant.org_id`, NOT NULL | `WHERE owner_id = ?` → `WHERE org_id = ?` | user A 列 orgA nodes → 不出现 orgB; 直访 orgB nodeId → **403** |

> ⑤ 备注: `user_settings` / `users.role` / API key metadata 是 per-user 不是 per-org, CM-3 不动; admin 看 settings 走 §1.3 god-mode endpoint (admin-model §1.3) 不复用 user-api。

---

## 2. 黑名单 grep — G1.4 闭合

> CM-3 PR merge 后, 在 `packages/server-go` 跑下面 grep, 命中数必须为 **0** (前 5 张表):

```bash
# 黑名单: messages / channels / workspace_files / agents / remote_nodes 主查询不应再 JOIN owner_id
grep -rn "JOIN.*\(messages\|channels\|workspace_files\|agents\|remote_nodes\).*owner_id" packages/server-go/internal/store/ | grep -v _test.go
```

> 预期 0 命中。**白名单** (允许保留): admin god-mode 路径 (`internal/api/admin_*.go`) 仍可走 owner_id (admin 看人不看 org); agents 表 `WHERE org_id = ? AND owner_id = ?` 双过滤是合法的, 不算 JOIN。

---

## 3. 反向断言锁 (CM-3 PR 必含测试)

| 反向断言 | 测试位置 | 锁点 |
|---------|---------|------|
| 跨 org GET message → 403 (不是 200, 不是 500, 不是 404) | `internal/api/messages_test.go::TestCrossOrgRead403` | 字面锁 status code = 403 |
| 跨 org GET channel → 403 + body 不含 raw `org_id` (§1.1) | `internal/api/channels_test.go::TestCrossOrgChannel403` | sanitizer 反查 |
| 跨 org GET file → 403 | `internal/api/files_test.go::TestCrossOrgFile403` | 同上 |
| 跨 org agent invitation create → 400 / 403 | `internal/api/agent_invitations_test.go::TestCrossOrgInvite403` (CM-4 已有, CM-3 复用) | 复用现有测试 |
| `WHERE org_id = ?` SQL EXPLAIN 走 `idx_*_org_id` index | `internal/store/store_test.go` 加 `EXPLAIN QUERY PLAN` 断言 | execution-plan G1.4 锚 |

---

## 4. 不在 CM-3 范围 (避免 PR 膨胀)

- ❌ 删 `owner_id` 列 (v0 阶段保留, agent 归属 user 仍是不同语义, concept-model §CM-3 已锁)
- ❌ admin 路径改写 (admin-model §1.3 god-mode endpoint 单独 owner)
- ❌ `user_settings` org 化 (per-user 与 org 解耦, ⑤ 已说明)
- ❌ 跨 org 错误信息泄漏存在性 (v0 接受 403 = 存在但不让你看, v1 改 404 留账)
- ❌ org switch UI (单 org 业主体验, multi-org v1+)

---

## 5. 验收挂钩

- CM-3.1 PR (写路径): ① / ② / ③ / ④ / ⑥ 写路径 NOT NULL 跑通 + auto-fill org_id 单测全绿
- CM-3.2 PR (读路径): §2 黑名单 grep 命中 0 + §3 反向断言 5 项全绿 + EXPLAIN 走 index
- Phase 1 退出 gate (#30): CM-3.1 + CM-3.2 双 merge + audit 登记 → ✅ 关闭

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v0, 6 张表反查路径 + 黑名单 grep + 5 项反向断言锁 |

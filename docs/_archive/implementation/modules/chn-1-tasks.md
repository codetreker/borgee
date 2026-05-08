# CHN-1 PR 拆分 — channel + memberships (Phase 3 第一波)

> **范围**: blueprint [`channel-model.md`](../../blueprint/current/channel-model.md) §1.1 / §2 关键不变量 + [`concept-model.md`](../../blueprint/current/concept-model.md) §1.2 / §1.3。
> **不在本文件**: workspace artifact (CV-1) / DM 拆分 (CHN-2) / 个人分组 (CHN-3) — 留 [`channel-model.md`](channel-model.md) 后续 milestone 处理。
> **依赖**: Phase 1 CM-1.1 (channels.org_id 已加列, 见 cm_1_1_organizations.go:38) + Phase 2 CM-3 (resource org_id 直查 PR #208) 已 merge; Phase 2 闸 4 通过后开闸。
> **总工期**: 6-8 天 server + 2-3 天 client; 拆 ≤ 3 PR, 每 PR ≤ 3 天 (foundation 规则)。

## 0. 跟 Phase 2 留账冲突点

| 冲突点 | Phase 2 现状 | CHN-1 立场 |
|--------|--------------|-----------|
| **org-scoped channels** | CM-3 (#208) 已 stamp `channels.org_id` 在 INSERT, 读路径直查 | CHN-1 沿用 — 不再 JOIN owner_id; 跨 org 的 channel 由 `channel_members` 行的 `user_id` 跨 org 表达, **channels.org_id = 创建者 org**。蓝图 §2 不变量 "Channel 创建者归属" 锁死。 |
| **channels.name UNIQUE 全局** | migrations.go:78 仍是 global UNIQUE | CHN-1.1 改为 UNIQUE(org_id, name) — 跨 org 同名合法 (蓝图 §2 "Channel 跨 org 共享")。**这是破坏式 schema 改动**, 必须 v=11 迁移落 + 单测断言旧 dup-name 行为已死。 |
| **channel_members 没有 org_id** | 现有表 (PRIMARY KEY channel_id+user_id) 由 cm_onboarding_welcome (v=7) 使用 | CHN-1.1 加 `org_id_at_join` 冷列 (audit 用, 不参与查询路径), 不破坏现有 DDL — 只 ADD COLUMN。 |
| **AP-0-bis (v=8) 已 grant `message.read`** | 全员默认有 message.read | CHN-1.2 API 直接复用, 不再判 channel-level read; 私 channel 由 `channel_members` 行控制。 |
| **agent 进 channel 必须 owner 触发** (CM-4 已落) | agent_invitations 状态机 (#183/185/186) | CHN-1.2 POST /channels/:id/members 走老路径; agent 加入仍走 invitation, 本 PR 不改路由。 |

## 1. 反约束 — agent 进 channel 默认 silent

> **蓝图立场** ([`concept-model.md`](../../blueprint/current/concept-model.md) §1.2 + §1.3): agent = 同事, 但**不指挥每条消息**。CHN-1 落地必须保证 agent 加入 channel 后**默认不主动说话**, 直到被 mention / owner 显式触发。

- `channel_members` 加冷列 `silent BOOLEAN DEFAULT 1 WHERE user_id is agent` (CHN-1.1 在迁移里 backfill, agent 行 silent=1, human 行 silent=0)。
- CHN-1.2 GET /channels/:id 响应里 members[].silent 暴露给前端 (但不让 client 改; 只能 owner via PATCH)。
- CHN-1.3 client UI 不允许 agent 自动进 channel 后立即生成消息 (前端 guard); 蓝图反查表条目: 若 PR 出现 "agent 加入即发 hello" 文案, 立即拒。
- **未在本 PR 范围**: agent silent → speaking 的状态切换语义 (留 Phase 4 AL-2a/2b)。

## 2. PR 拆分

### CHN-1.1 — schema (channels + memberships) + migration v=11

- **Blueprint**: §2 关键不变量 + §1.4 (per-org 命名空间)
- **Touches**: `internal/migrations/chn_1_1_channels_org_scoped.go` (新) + registry.go + chn_1_1_*_test.go
- **Migration v=11** (下一个发号; 已确认 v=4..10 用过, v=6 跳号):
  1. drop UNIQUE(name) → 加 UNIQUE(org_id, name) (SQLite: rebuild 表 + 数据搬迁)
  2. ADD COLUMN channels.archived_at INTEGER (反约束: 蓝图 §1.1 channel 是协作场, archive 而非 delete)
  3. ADD COLUMN channel_members.org_id_at_join TEXT DEFAULT '' (audit)
  4. ADD COLUMN channel_members.silent BOOLEAN NOT NULL DEFAULT 0
  5. backfill: agent 行 silent=1 (JOIN agents/users by role)
  6. CREATE INDEX idx_channel_members_org_at_join ON channel_members(org_id_at_join)
- **历史行兼容**: v=11 是破坏式改动 (channels.name 之前是全局 UNIQUE)。**跨 org 同名历史行人工 audit, 不自动改名** — 迁移前若已存在跨 org dup name (理论不存在, 但防 dogfood DB), 迁移失败硬停 + 报错列出 dup row id, 由 ops 人工拍板; 不写自动 rename 逻辑防丢历史。
- **Acceptance (四选一)**: **数据契约** — `cm_1_1_organizations_test.go` 风格扩展, 断言: (a) v=11 之后 schema_migrations 有行; (b) 跨 org 同名 channel INSERT 成功 (旧行为应失败); (c) idempotent 二次跑不重复 backfill; (d) agent 行 silent=1 / human 行 silent=0; (e) hasColumns guard 防迁移序冲突 (CM-onboarding v=7 已落 channel_members 时不能再 ADD)。
- **工期**: 2 天 (含写 backfill 单测)
- **Owner**: 战马 / 飞马 review / 烈马 数据契约
- **PR 模板**: Stage: v0 / Touches: 上述 / Current 同步: docs/current/server/migrations.md 加 v=11 行 + docs/current/data-layer.md UNIQUE 描述 + docs/current/server/overview.md (channel_members.silent 字段)

### CHN-1.2 — API handler (POST/PATCH/GET channels + memberships)

- **Blueprint**: §1.1 协作场 + §2 跨 org 共享 + §1.4 (作者控大局)
- **Touches**: `internal/server/channels_handler.go` (重构) + `internal/server/channel_members_handler.go` (新) + 路由表 + handler tests
- **范围**:
  - POST /api/v1/channels: stamp org_id from session.UserOrgID (CM-3 路径), 校验同 org name unique
  - PATCH /api/v1/channels/:id: name/topic/archived_at (owner only — 蓝图 §1.4)
  - GET /api/v1/channels: 列 (a) 我所属 (channel_members.user_id=me) + (b) 同 org public channels; **不**返回他 org public (蓝图 §2 跨 org 必须显式邀请)
  - POST /api/v1/channels/:id/members: 仅 owner; 加 human 直接落 row, agent 走 CM-4 invitation (本 PR 不改路由, 只复用)
  - PATCH /api/v1/channels/:id/members/:uid: 改 silent flag (owner only)
- **Acceptance (四选一)**: **e2e 断言** — testutil 起 server, 7 用例: (1) 跨 org 同名 channel 都建成功; (2) 跨 org GET 不互见; (3) 非 owner PATCH 返回 403; (4) PATCH silent=false 后 GET 返回 silent:false; (5) **POST /channels 后 channel_members count == 1** (只 creator), 同 org 另一 session GET 列表不见此 channel (private 默认; PR #263 立场 ② 锚点); (6) **agent join 触发 system message 文案锁** — POST /channels/:id/members 加 agent 落地后, messages 表新增 1 行 kind=system, sender_id='system', content 严格匹配 `"{agent_name} joined"` (不含 raw user_id, 不含 org_id; PR #263 立场 ③ 锚点); (7) **soft delete fanout system DM** — PATCH archived_at!=null 时, 每个 channel_member 各收 1 条 system DM, 文案锁 `"channel #{name} 已被 {owner_name} 关闭于 {ts}"`, 复用 ADM-0 §1.4 红线 ③ fanout 模式 (PR #263 立场 ⑤ 锚点)。**搭配** 行为不变量 — A org owner 改自己 channel 不影响 B org 同名 channel 行 (单测断言)。
- **工期**: 2-3 天
- **Owner**: 战马 / 飞马 review / 烈马 e2e
- **PR 模板**: Stage: v0 / Current 同步: docs/current/server/api.md (列出新 endpoint) + docs/current/server/overview.md

### CHN-1.3 — client SPA channel 列表 UI + create channel quick action

- **Blueprint**: §1.4 作者定义 + §3.4 (个人 reorder 留 CHN-3, 本 PR 仅作者层)
- **Touches**: `packages/client/src/pages/ChannelsPage.tsx` (或现状对应) + `components/CreateChannelDialog.tsx` (新) + i18n + e2e
- **范围**:
  - 侧栏 channel 列表显示作者层分组 (本 PR 单组, group 结构留 CHN-3)
  - "+ 新建 channel" quick action: 对话框输入 name + topic + visibility, 调 POST /channels
  - channel header 显 archived_at !== null 时灰显 (蓝图反约束: archive 不删)
  - **反约束 guard**: agent 加入 channel 后 client 不自动派生消息 — render members 列表时 silent badge 显 "🤐 沉默"
- **不在范围**: workspace UI (CV-1) / DM 列表 / 个人 reorder
- **Acceptance (四选一)**: **e2e 断言** Playwright (INFRA-2 已就绪) — 新建 channel → 出现在侧栏 → 跨 org 用户看不到 (跨 session 验证)。
- **工期**: 2 天
- **Owner**: 战马 / 飞马 review / 烈马 e2e + 野马 文案
- **PR 模板**: Stage: v0 / Current 同步: docs/current/client/overview.md

## 3. 排期与依赖

```
CHN-1.1 (server, 2d) ──→ CHN-1.2 (server, 2-3d) ──→ CHN-1.3 (client, 2d)
                                                ╲
                                                 ╲ (CHN-1.3 可与 CV-1.1 并行起跑)
```

- CHN-1.1 merge 后 CHN-1.2 才能跑; CHN-1.3 stub mock API 也可与 1.2 并行。
- 全部 merge 是 CV-1 (artifact) 前置 — 协作场需要 channel 形状先稳。

## 4. 反查表 (野马口径)

每 PR 必带反查锚点 (lint-able):
- `chn-1.1`: schema 没改 channels.org_id 列 / 没破坏 cm_onboarding_welcome 迁移 / silent 列 backfill 仅 agent
- `chn-1.2`: 跨 org GET 用例存在 / 没回退到 owner_id JOIN / agent invitation 路径未动 / agent join system message 文案 == `"{agent_name} joined"` / archive fanout system DM 文案 == `"channel #{name} 已被 {owner_name} 关闭于 {ts}"` / POST /channels 后 members count == 1
- `chn-1.3`: silent badge 渲染 / 没出现 "agent 加入立即发言" 文案

---
**派 review**: 落本文件后 SendMessage 给 team-lead 派飞马 / 野马 review。

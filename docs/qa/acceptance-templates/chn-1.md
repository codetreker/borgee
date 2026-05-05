# Acceptance Template — CHN-1: channel schema + API (creator-only / archive / agent silent / per-org name)

> 蓝图: `docs/blueprint/current/channel-model.md` §1.1 (creator-only default), §1.4 (per-org name uniqueness), §2 (archived 不删 + agent silent join)
> 蓝图: `docs/blueprint/current/concept-model.md` §1.4 (主体验 — 团队感知 + DM)
> Implementation: `docs/implementation/modules/chn-1-spec.md`
> 拆 PR: **CHN-1.1** schema (#276 merged b6e95ce) + **CHN-1.2** API handler (#286 merged f7ac4ed) + **CHN-1.3** client SPA (#288 merged adaf521)
> Owner: 战马A 实施 / 烈马 验收

## 验收清单

### 数据契约 (蓝图 §1.4 / §2 — schema drift)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 channels 表加 `archived_at` (nullable, NULL=active); channel_members 加 `silent` NOT NULL DEFAULT 0 + `org_id_at_join` NOT NULL DEFAULT '' | migration test | 战马A / 烈马 | `internal/migrations/chn_1_1_channels_org_scoped_test.go::TestCHN11_AddsArchivedAtAndSilentColumns` (#276) |
| 1.2 drop global UNIQUE(name) + add UNIQUE INDEX `idx_channels_org_id_name` (per-org); 旧 `idx_channels_org_id` 非唯一索引 survive rebuild | migration test | 战马A / 烈马 | `TestCHN11_DropsGlobalNameUniqueAndAddsPerOrgIndex` (#276) |
| 1.3 历史 (org_id, name) 重复硬失败 + 不自动改名 + schema_migrations 不记录失败 | migration test | 烈马 | `TestCHN11_HardFailsOnHistoricDuplicateNoAutoRename` (#276) |
| 1.4 backfill: agent role → silent=1; human → silent=0; org_id_at_join 取 users.org_id 快照 | migration test | 战马A / 烈马 | `TestCHN11_BackfillsAgentSilentAndOrgIDAtJoin` (#276) |

### 行为不变量 (蓝图 §1.1 / §1.4 / §2 — API 文案锁)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 POST /channels → 仅 creator 一行 channel_member (no auto-fanout) | e2e | 战马A / 烈马 | `internal/api/chn_1_2_test.go::TestCHN12_CreatorOnlyDefaultMember` (#286) |
| 2.2 跨 org 同名合法 (orgA 的 #general + orgB 的 #general 共存) | e2e | 战马A / 烈马 | `TestCHN12_CrossOrgSameNameOK` (#286) |
| 2.3 跨 org GET 单 channel ≠ 200 + LIST 不含他 org channel (双轴隔离) | e2e | 烈马 | `TestCHN12_CrossOrgPublicGETIsolation` (#286) |
| 2.4 archive fanout system DM 文案锁 byte-identical: `channel #{name} 已被 {owner_name} 关闭于 {ts}` | e2e + grep | 战马A / 烈马 | `TestCHN12_ArchiveFanoutSystemDM` (#286) + `grep "已被 .* 关闭于" internal/api/channels.go` count≥1 (line 1081) |
| 2.5 agent join system message 文案锁 + sender_id='system' + ChannelMember.Silent=true (立场 ⑥ agent=同事不刷屏) | e2e | 战马A / 烈马 | `TestCHN12_AgentJoinSystemMessage` (#286) — 字面 `{agent_name} joined` (channels.go:1037) |

### 用户感知 (CHN-1.3 client SPA — UI 文案锁)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 创建频道 dialog: 默认 visibility=public + creator-only (sidebar 渲染 + GET /channels/:id/members length==1) | e2e | 战马A / 烈马 | `packages/e2e/tests/chn-1-3-channel-list.spec.ts::立场 ① create channel via dialog` (#288) |
| 4.2 silent agent badge: `🔕 silent` 字面渲染 + system message `{agent_name} joined` 可见 + ChannelMember.silent==true (立场 ⑥) | e2e + grep | 战马A / 烈马 | `chn-1-3-channel-list.spec.ts::立场 ② agent silent badge` (#288); `grep -n "🔕 silent" packages/client/src/components/ChannelMembersModal.tsx` count≥1 (line 205 字面 `<span class="user-badge user-badge-silent">🔕 silent</span>`) |
| 4.3 archive 状态: 频道行 `data-archived="true"` + `.archived-badge` 文本 `已归档` + system DM `channel #{name} 已被 ` 前缀可见 | e2e + grep | 战马A / 烈马 | `chn-1-3-channel-list.spec.ts::立场 ③ archive PATCH` (#288); `grep -n "已归档" packages/client/src/components/SortableChannelItem.tsx` count≥2 (line 83 + 109 字面 `<span class="archived-badge">已归档</span>`) |

### 蓝图行为对照 — AP-1 严格 403 留账 (Phase 4 forward-looking)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 ✅ 全闭: non-owner PATCH /channels/:id → 严格 403 (post AP-1 #493 ABAC SSOT 真落地, AP-0 grants member (\*,\*) 已被 AP-1 严格 403 替代) | unit (post AP-1) | 烈马 | ✅ #493 严格 403 已落地 + REG-CHN1-007 ⏸️→🟢 兑现 |

## 退出条件

- 数据契约 4 项 + 行为不变量 5 项 + 用户感知 3 项**全绿** (一票否决)
- AP-1 严格 403 (3.1) ⏸️ pending Phase 4, 不挡 CHN-1 闭合
- 登记 `docs/qa/regression-registry.md` REG-CHN1-001..010 (其中 -007 ⏸️ pending; -008..010 = client UI #288)

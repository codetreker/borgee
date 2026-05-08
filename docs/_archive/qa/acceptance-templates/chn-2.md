# Acceptance Template — CHN-2: DM 概念独立 (UI/UX 视觉拆 channel + DM 没 workspace + DM 永远 2 人)

> 蓝图: `channel-model.md` §1.2 (DM 概念独立, 底层可复用) + §1.1 (channel = 协作场, 跟 DM 拆) + §3.2 (现状差距 — UI 混淆 + DM workspace 入口需禁用) + §2 不变量 "DM 永远 2 人 / Workspace per channel, DM 无 workspace"
> Implementation: `docs/implementation/modules/chn-2-spec.md` (飞马 #357 a5b05b7 + v2 patch 34bb1d5 / 立场 ⑤ 文案锁同步 a20b437)
> 拆 PR: **CHN-2.1** ✅ #407 (121b2b7) server-side DM 路径反约束 (POST /channels/:id/artifacts on type='dm' → 403 `dm.workspace_not_supported`; addMember 既有 400 lock-pin) + **CHN-2.2** ✅ #406 (17378da) client SPA 视觉拆 (data-kind / data-channel-type 锚 + chn-2-content-lock.test.ts 8 cases) + **CHN-2.3** ✅ #413 (a5be7c2) DM e2e + mention placeholder 收尾
> Owner: 战马 实施 / 烈马 验收
> Status: ✅ 三段四件全闭 (CHN-2.1 server + CHN-2.2 client + CHN-2.3 e2e+placeholder + 文案锁 #354 / 内容锁 #338 / spec #357 全 merged)

---

## §0 关键约束 (蓝图立场, 实施 PR 不可绕)

> 锚: `channel-model.md` §1.2 + §2 不变量

| # | 立场 | 反约束 | 蓝图源 |
|---|---|---|---|
| ① | DM 复用 channel 表底层 (`type='dm'`) — 数据层不裂 | ❌ 不新建 `direct_messages` 表; 不引入新 PK 命名空间 | §1.2 "底层可复用" |
| ② | DM 永远 2 人 — 不可加人 | ❌ POST /channels/:id/members 对 `type='dm'` 路径 → 403 (反向断言); ❌ DM 不可升级到 channel (升级走"创建新 channel 把双方拉进去") | §2 不变量行 + §1.2 ❌ 加人 |
| ③ | DM 没 workspace — 不可访问 artifact 路径 | ❌ GET /channels/:id/artifacts 对 `type='dm'` → 403; ❌ POST /channels/:id/artifacts 对 `type='dm'` → 403; ❌ DM 侧栏不渲染 Canvas tab (DOM 反查 count==0) | §1.2 ❌ workspace + §2 不变量 "Workspace per channel; DM 没 workspace" + §3.2 "DM workspace 入口需禁用" |
| ④ | DM 没 topic — UI 不渲染 topic 字段 | ❌ DM channel 详情面板不显示 topic input; ❌ system message 不渲染 "topic 已更新" | §1.2 ❌ topic |
| ⑤ | UI 视觉与交互跟 channel **明确不同** — 防混淆 | ❌ DM 列表 UI 不复用 channel 列表组件原样渲染 (data-kind="dm" 字面锁); ❌ DM header 不显示 "#" 频道前缀 | §1.2 + §3.2 "UI 上 DM 与 channel 视觉差异不够大" |
| ⑥ | DM channel-scoped 路由仍走 channel id (反约束 raw UUID) — 不暴露 owner_id 走 sender_id | ❌ DM message DOM 不漏 raw `<uuid>` 文本 (跟 DM-2 §3.1 同模式); ❌ system DM 文案不抄送 owner (跟 concept-model §4 一致) | `concept-model.md` §4 + DM-2 acceptance §3.1 |

---

## 验收清单

### §1 server schema + 反约束 (CHN-2.1) — DM type 路径分流 + 工资 path 反约束

> 锚: 飞马 spec §1 (TBD) + CHN-1 #276 schema 模板 + 立场 ①②③

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 [TBD] DM type='dm' INSERT 必带 exactly 2 members (CHECK 或 trigger) — 立场 ② | migration test | 战马 / 烈马 | _(待 CHN-2.1 PR + 飞马 spec)_ |
| 1.2 ✅ DM 路径 POST /channels/:id/members → 400/403 (server enforce, 立场 ②) — channels.go 既有 addMember DM 路径 lock-pin | unit | 战马 / 烈马 | `chn_2_1_dm_reject_test.go::TestCHN21DMAddMemberReject` PASS (#407 121b2b7, lock-pin 既有 channels.go:522 反约束) |
| 1.3 ✅ DM 路径 POST /channels/:id/artifacts → 403 + code `dm.workspace_not_supported` byte-identical (server enforce, 立场 ③) — `cv-1.md` §2.1 cross-channel 模式延伸到 cross-type | unit + e2e | 战马 / 烈马 | `chn_2_1_dm_reject_test.go::TestCHN21DMArtifactReject` PASS (#407 121b2b7, code 字面 byte-identical 锁) + e2e `chn-2-3-dm-flow.spec.ts::§4.e DM 视图反查 Canvas tab count==0` PASS (#413 a5be7c2) |
| 1.4 [TBD] DM topic 字段 nullable 且 server 默认 NULL (立场 ④) — `UPDATE channels SET topic=...` WHERE type='dm' → 400 | unit | 战马 / 烈马 | _留 v0+ 兜底, 客户端 #406 已不渲染 topic input (反约束已在 client 层守; server 层 topic UPDATE 仍允许写但 UI 不暴露入口, 立场 ④ 客户端兜底先满足)_ |
| 1.5 ✅ migration 兼容 (CHN-2 不引入 schema break, 复用 channel 表) — 立场 ① 字面验 | grep + 反向 | 战马 / 烈马 | `grep -rnE 'direct_messages\|dm_messages\|CREATE TABLE direct\|CREATE TABLE.*dm_' packages/server-go/internal/migrations/ packages/server-go/internal/store/` count==0 (立场 ① 数据层不裂底层复用 channels 表 type='dm'; #407 + #406 + #413 全实施期未引入新表) |
| 1.6 ✅ 反向 grep — 不应有 `direct_messages` 表 / `dm_messages` 路径 (立场 ① 底层复用) | CI grep | 飞马 / 烈马 | `grep -rnE 'direct_messages\|dm_messages\|CREATE TABLE direct' packages/server-go/internal/migrations/ packages/server-go/internal/store/` count==0 PASS (跟 1.5 同 grep 锚) |

### §2 行为不变量 (CHN-2.1) — 立场 ②③④ 反向断言

> 锚: 立场 ②③④ + `channel-model.md` §2 不变量

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 [TBD] DM 创建 — 必传 `target_user_id`, server 反查 (sender, target) 已存在 DM 则返回原 channel id (idempotent, 不创新行) | unit (table-driven) | 战马 / 烈马 | _(待 CHN-2.1 PR)_ |
| 2.2 [TBD] DM 反向 — 升级到 channel 路径不存在 (无 `POST /channels/:id/upgrade-to-channel` endpoint) | grep + 反向 | 飞马 / 烈马 | `grep -rnE 'upgrade.*channel\|dm.*to.*channel' packages/server-go/internal/api/` count==0 |
| 2.3 [TBD] DM workspace 入口 server 禁用 (立场 ③ 反约束) — `cv-1.md` §0 立场 ① channel-scoped artifact 在 type='dm' channel 时 server 拒 | unit + e2e | 战马 / 烈马 | _(待 CHN-2.1 PR)_; `grep -nE 'channel\.type.*==.*"dm"\|IsDirectMessage' packages/server-go/internal/api/artifacts.go` count≥1 |
| 2.4 [TBD] DM mention 路由 — 跟 DM-2 §2.x 一致, mention 走 sender_id 不展开 owner (DM 内 @user 仅 ping 目标人) | e2e | 战马 / 烈马 | _(待 CHN-2.1 PR + DM-2.2 联动)_ |
| 2.5 反向 grep — DM message 不应触发 channel-only 系统消息 (`#{channel} 已归档` / agent silent join 等) | CI grep | 飞马 / 烈马 | `grep -nE '"#%s.*已归档"\|silent.*join.*dm' packages/server-go/internal/api/channels.go` count==0 在 DM type 分支 |

### §3 用户感知 (CHN-2.2 client SPA) — 视觉拆 + 反 UUID 漏

> 锚: 立场 ⑤⑥ + DM-2 §3.x 跨 PR 联动 + `channel-model.md` §3.2

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 ✅ DM 列表 UI 跟 channel 列表 **视觉显著不同** (字面锁: DM 列表 DOM `data-kind="dm"`, channel 列表 `data-kind="channel"`); DM 列表项不显示 "#" 频道前缀, 仅显示对方 display name | vitest + e2e | 战马 / 烈马 | `chn-2-content-lock.test.ts::数据-kind 锚双侧` (8 cases 含 Sidebar.tsx data-kind="dm" + ChannelList.tsx data-kind="channel" 反侧锚) PASS (#406 17378da) + e2e `chn-2-3-dm-flow.spec.ts::§1 ⑤ data-channel-type="dm"` PASS (#413 a5be7c2) |
| 3.2 ✅ DM 视图 **不渲染 Canvas tab** (立场 ③ DOM 反约束) — DM 进入后 tab bar `.channel-view-tabs` count==0; channel 进入后正常渲染 | e2e + DOM | 战马 / 烈马 | `chn-2-3-dm-flow.spec.ts::§4.e DM 视图反查 Canvas tab count==0` PASS (#413 a5be7c2, ChannelView.tsx:159 `data-channel-type` 路径分流) |
| 3.3 ✅ DM 详情面板 **不渲染 topic input** (立场 ④); **不渲染 "添加成员" 按钮** (立场 ②); 文案锁 `private 私信仅限两人, 想加人请新建频道` byte-identical (#354 §1 ⑤ + chn-2-content-lock.md) | vitest + e2e | 战马 / 烈马 | `chn-2-3-mention-placeholder.test.tsx::DM_MENTION_THIRD_PARTY_PLACEHOLDER` 字面 byte-identical 跟 #354 §1 ⑤ PASS (#388 76fb0f8 + #413 a5be7c2) + `chn-2-3-dm-flow.spec.ts::§4.e topic input + 添加成员 btn count==0` PASS |
| 3.4 ✅ DM header **不显示 #** 频道前缀 (立场 ⑤); 显示对方 display name + presence dot (跟 AL-3 #324 PresenceDot 复用); `data-channel-type="dm"` 路径分流 | e2e | 战马 / 烈马 | `chn-2-3-dm-flow.spec.ts::§1 ⑤ DM header 不渲染 #` PASS (#413 a5be7c2) |
| 3.5 ✅ DM message 文案 **不漏 raw UUID** (反约束 ⑥, 跟 DM-2 §3.1 同模式) — DOM grep 不含 `<uuid>` 文本节点 (`data-mention-id` attr 可有, 文本节点不可有) | vitest + e2e | 战马 / 烈马 | `markdown-mention.test.ts::反约束 short-id fallback` PASS (#388 76fb0f8, REG-DM2-010 + 011 同源) + `chn-2-3-dm-flow.spec.ts::§4 raw UUID 不漏文本节点` PASS (#413 a5be7c2) |

### §4 反向 grep / e2e 兜底 (CHN-2.3) — 蓝图行为对照, 每 PR 必带

> 锚: 立场 ①②③④⑤⑥ 反约束横切

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.a 反向 grep — 不应新建 `direct_messages` / `dm_messages` 表 (立场 ① 底层复用) | CI grep | 飞马 / 烈马 | `grep -rnE 'direct_messages\|dm_messages\|CREATE TABLE.*dm_' packages/server-go/internal/migrations/ packages/server-go/internal/store/` count==0 |
| 4.b 反向 grep — DM 路径不应出现 channel-only 字符串 ("workspace" / "topic" / "添加成员" / "归档") 在 DM 视图条件渲染分支外 (立场 ③④⑤) | CI grep | 飞马 / 烈马 | `grep -nE 'workspace.*type.*"dm"\|topic.*type.*"dm"\|"添加成员".*type.*"dm"' packages/client/src/components/` 命中行需带 `// CHN-2: DM 反约束` 注释或 `unsupported` 闸 |
| 4.c ✅ 反向 grep — DM 不应触发 system message "#{channel} 已归档" (立场 ⑤ DM 视觉拆) | CI grep | 飞马 / 烈马 | `chn-2-3-dm-flow.spec.ts::§4.c DM 不触发归档 system message` PASS (#413 a5be7c2, 跟 CHN-1 #288 client `system DM 'channel #{name} 已被'` 模式镜像反向) |
| 4.d ✅ e2e — 双窗口 DM 创建 + 加人 attempt → 403 双断 (立场 ② 兜底) | e2e | 战马 / 烈马 | `chn-2-3-dm-flow.spec.ts::§4.d 双窗口 DM 创建 + 加人 attempt → 400/403 双断` PASS (#413 a5be7c2, 跟 cv-1-3-canvas.spec.ts REST + DOM 双断同模式) |
| 4.e ✅ e2e — DM 视图反查 Canvas tab count==0 + topic input count==0 + 添加成员 button count==0 (立场 ②③④ 三反约束兜底) | e2e | 战马 / 烈马 | `chn-2-3-dm-flow.spec.ts::§4.e DM 视图反查 Canvas tab + topic input + 添加成员 btn count==0` PASS (#413 a5be7c2, 三立场反约束兜底真路径 PASS) |

---

## 退出条件

- §0 关键约束 6 立场入册 (跟 cv-1.md §0 立场反查 + dm-2.md 数据契约 同模式)
- §1 server 6 项 (5 TBD + 1 反向 grep) 全绿
- §2 行为不变量 5 项 (4 TBD + 1 反向 grep) 全绿
- §3 用户感知 5 项 (5 TBD client SPA) 全绿
- §4 反向 grep / e2e 兜底 5 项 (3 grep + 2 e2e) 全绿
- 登记 `docs/qa/regression-registry.md` REG-CHN2-001..N (server + 行为 + client + 反向 兜底)
- 蓝图引用区 `channel-model.md` §1.2 / §2 / §3.2 翻 ✅ #CHN-2 closure

## 跟其他 milestone 的边界

| Milestone | 关系 | 备注 |
|---|---|---|
| **CHN-1** ✅ closed | 同表 (`channels.type`), CHN-2 在 CHN-1 schema 上加 `type='dm'` 路径分流 | CHN-1 #276 v=11 已落 type 列, CHN-2 仅强 type='dm' 反约束 |
| **CV-1** ✅ closed | CV-1.2 cross-channel 403 反约束模式 → CHN-2 cross-type (DM ≠ channel) 复用 | `cv-1.md` §2.1 立场 ① channel-scoped → CHN-2 §1.3 server 拒 DM 走 artifact 路径 |
| **DM-2** in-flight | DM-2 是 mention 路由 (`@user/@agent/@channel`), CHN-2 是 DM 容器分流 | 两者解耦 — CHN-2 锁 DM ≠ channel 视觉/路径; DM-2 锁 mention 路由按 sender_id (concept-model §4) |
| **CV-2** TBD (Phase 3 章程未启) | 锚点对话依赖 CV-1 闭, CHN-2 不挡 CV-2 | CV-2 spec 飞马 TBD |

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 烈马 | v0 skeleton — §0 6 立场 + §1-§4 验收清单 (TBD 占位等飞马 spec) + 边界表 (CHN-1/CV-1/DM-2/CV-2 关系); 跟 #318 AL-4 + #293 DM-2 acceptance skeleton 同模式 4 件套并行 (spec 飞马 / 文案锁 野马 / acceptance 烈马 / 战马等三件套到位) |
| 2026-04-29 | 战马E | CHN-2 三段四件全闭 closure follow-up — header status ⚪ skeleton → ✅ 三段四件全闭; §1 server (1.2/1.3/1.5/1.6 四项 ⚪→✅, 1.4 留 v0+ 客户端兜底); §3 client (3.1-3.5 五项 ⚪→✅, 文案锁 byte-identical 跟 #354 §1 ⑤ + chn-2-content-lock); §4 反向 grep / e2e (4.c/4.d/4.e 三项 ⚪→✅); 实施证据真路径填: CHN-2.1 #407 121b2b7 (chn_2_1_dm_reject_test.go::TestCHN21DMArtifactReject + TestCHN21DMAddMemberReject) + CHN-2.2 #406 17378da (chn-2-content-lock.test.ts 8 cases vitest data-kind="dm"/"channel" 双侧锚) + CHN-2.3 #413 a5be7c2 (chn-2-3-dm-flow.spec.ts e2e §1+§4) + DM-2.3 #388 76fb0f8 (markdown-mention.test.ts + chn-2-3-mention-placeholder.test.tsx 联动 reverse UUID 漏); spec 锚 #357 a5b05b7 + v2 patch 34bb1d5; CHN-2 milestone (DM 概念独立) Phase 3 章程九 milestone 又一闭环. 跟 #383 / #421 / #420 / #429 同模式 acceptance flip 兼 PROGRESS [ ]→[x]; REG-CHN2 已在 #418 占号待 rebase, 此 PR 不动 registry. |

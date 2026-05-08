# CHN-2 spec brief — DM 概念独立 (跟 channel 拆死)

> 飞马 · 2026-04-29 · ≤80 行 spec lock (实施视角 3 段拆 PR 由战马B 落, 跟 DM-2 v=14 schema 协调)
> **蓝图锚**: [`channel-model.md`](../../blueprint/current/channel-model.md) §1.2 (DM 概念独立, 底层可复用) + §3.2 (现状差距 — UI 不够区分 + DM 当前有 workspace 入口需禁用)
> **关联**: CHN-1 三段全闭 ✅ (#276+#286+#288) — channel/membership/workspace 已就位 (DM 复用 `channels.type='dm'` 老路径); DM-2 战马B 进行中 (v=14 schema 卡); CV-1 ✅ workspace 升 artifact, DM 必须显式禁 workspace 入口 (蓝图 §1.2 ❌); ADM-0 god-mode 不读 DM body 红线 (§ADM-0.3)
> **章程闸**: 配 G3.4 协作场骨架 demo — DM 跟 channel 视觉/交互必须显著区分 (野马 demo 截屏可识别)

> ⚠️ 锚说明: DM 底层一直跟 channel 共表 (`channels.type='dm'`), 蓝图 §1.2 \"概念独立, 底层可复用\" 字面承认; CHN-2 不拆表, 只拆 \"语义边界 + UI 边界 + 入口禁用\". DM-2 战马B v=14 schema 是 mention 上游 (privacy-first), 跟 CHN-2 字面正交但同一 v=14 migration 协调

## 0. 关键约束 (3 条立场, 蓝图字面 + DM-2 边界对齐)

1. **DM = 私密 1v1, 永不加人** (蓝图 §1.2 字面锁): `channels.type='dm'` + 成员数恒 == 2 (server 校验, 加 3 人 → reject 400 \"私信仅限两人, 想加人请新建频道\" #354 立场 ⑤ 同源); **反约束**: 不开 \"群 DM\" 路径 (3+ 人想私聊 → 走新建 channel `type='private'`); **不**继承 channel 的 \"加人\" / \"topic\" / \"workspace\" UX
2. **DM 显式禁 workspace 入口** (蓝图 §1.2 ❌ + §3.2 字面差距): DM 详情页**不渲染** Workspace tab (CV-1.3 #346 客户端 SPA 行为分支); server 侧 `GET /channels/:id/artifacts` DM channel → 403 \"DM 无 workspace, 跟 channel 拆\" (兜底, 防 UI bug 漏检); **反约束**: 不允许 DM 有 artifact (跟 CV-1 立场 ① artifact 归属 channel 拆死, DM 不算 channel 协作场)
3. **DM UI 视觉显著差异** (蓝图 §1.2 \"不让用户混淆\" 字面锁): DM 列表跟 channel 分组**两栏分离** (DM 在侧栏底部独立 \"私信\" 区 byte-identical 跟 `Sidebar.tsx:396` + #354 立场 ① 同源, 头像圆形 + 用户名; channel 头像方形 + `#name`); 反约束: 不混合 \"recent\" 时序排 (避免一眼分不清); 不准 \"DM\" / \"Direct Messages\" / \"对话\" / \"Chats\" 同义词 (#354 立场 ① 字面禁); CHN-3 个人分组 reorder/pin **只**对 channel 生效, DM 不参与分组

## 1. 拆段实施 (CHN-2.1 / 2.2 / 2.3, ≤ 3 PR)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **CHN-2.1** server 边界 + 成员数锁 | `POST /dms` 端点 (body: `{peer_user_id}`, 创/查找现存 type='dm' 双向 channel idempotent) — 跟 `POST /channels` 拆离 (channel 创要 name/topic, DM 自动 \"@username\"); `POST /channels/:id/members` DM channel reject 400 (立场 ①, 反加人); `GET /channels/:id/artifacts` DM 403 (立场 ②); 复用 CHN-1 #286 channel API 主体, 不另起 message/reaction 表; DM-2 战马B v=14 mention 字段同 migration 协调 (一次 schema bump, 不拆双 v) | 待 PR (战马B) | 战马B |
| **CHN-2.2** client 列表分栏 + 入口禁用 | 侧栏 `<DMList>` 组件独立 (`<ChannelList>` 拆 sibling, 不嵌套); DM 详情页跳过 `<WorkspaceTab>` 渲染分支 (CV-1.3 #346 ArtifactPanel 增 `if channel.type==='dm' return null` 守门); DM 头像样式 `border-radius: 50%` + `<UserStatusDot>` (AL-3 #324 复用); 反约束: 不在 DM 容器拼 \"加人\" / \"topic\" / \"workspace\" 按钮 | 待 PR (战马B) | 战马B |
| **CHN-2.3** \"加人\" 引导 → 新建 channel (非 DM 转换) | DM 内输入 `@<3rd_user>` 触发 mention 候选 — 候选列表为空 + placeholder `\"私信仅限两人, 想加人请新建频道\"` byte-identical (#354 立场 ⑤ 同源); UI 给 \"新建频道并拉入双方\" 引导按钮跳 `POST /channels` 创流 (跟 CHN-1 #286 既有创 channel 端点同源, 不开新 endpoint, 不动 DM channel 数据); **反约束**: 不开 \"DM 升级 / Convert to channel / Upgrade DM / 转为频道\" 路径 — 蓝图 §1.2 line 27 + §2 line 62 字面 \"想加人就**新建** channel 把双方拉进去\", 是新建不是升级 (#354 立场 ⑤ 字面锁) | 待 PR (战马B) | 战马B |

## 2. 与 CHN-1 / DM-2 / CV-1 / ADM-0 留账冲突点

- **CHN-1 channel API 复用**: 不拆 `channels` 表, 走 `type='dm'` 分支语义; CHN-1 #286 endpoint 主体保留, 仅加 DM 守门 (立场 ①.成员数 + 立场 ②.workspace 403)
- **DM-2 战马B v=14 协调 + v 号 sequencing** (非冲突): DM-2.1 (战马B mention privacy) / CV-2.1 (anchor 新表) / CHN-2.1 (无 schema, 仅 server 软约束) 三方撞 v=14. **真 sequencing 谁先 merge 谁拿 v=14**, 后顺延. CHN-2 无 schema 改 (软约束跑 server check), 不抢号 — 按 \"先到先拿\" 不卡 DM-2 / CV-2; 若战马B DM-2.1 起手 30min 阈值未回报转活给战马A, 则 CV-2.1 拿 v=14, DM-2.1 顺延 v=15
- **CV-1 artifact 拆死**: CHN-2.1 GET /artifacts DM 403 是 CV-1 立场 ① \"artifact 归属 channel\" 的反向兜底; **反约束**: 任何让 DM 拥有 artifact 的 PR 都视作章程违反
- **ADM-0 god-mode 不读 DM body 红线** (§ADM-0.3 不变): admin 列表能看 DM 元数据 (双方 + 时间), 不返回 message body; CHN-2 不动 admin endpoint
- **CHN-3 个人分组排除 DM**: CHN-2.2 立场 ③ 字面锁 — DM 不参与个人分组 reorder/pin (CHN-3 spec 起时承袭)

## 3. 反查 grep 锚 (Phase 3 验收)

```
git grep -nE "channels\.type.*=.*'dm'.*members.*[<>=].*2|len\(.*members.*\).*[<>=].*2" packages/server-go/internal/server/   # ≥ 1 hit (立场 ① 成员数锁)
git grep -nE "type.*===\\s*'dm'.*WorkspaceTab|channel\.type.*'dm'.*return null" packages/client/   # ≥ 1 hit (立场 ② 客户端入口禁用)
git grep -nE 'POST /dms|/dms/:id/promote'                packages/server-go/internal/server/         # ≥ 1 hit (CHN-2.1 端点; CHN-2.3 不开 promote, 走 CHN-1 既有 POST /channels)
git grep -nE 'WorkspaceTab.*dm|dm.*artifacts.*200'        packages/client/ packages/server-go/         # 0 hit (反约束 立场 ② DM 无 artifact)
git grep -nE 'POST /channels.*dm.*members.*add|dm.*addMember' packages/server-go/internal/server/      # 0 hit (反约束 立场 ① DM 不加人)
git grep -nE "['\"](升级为频道|Convert to channel|Upgrade DM|转为频道|promote-to-channel)['\"]"  packages/  # 0 hit (反约束 #354 立场 ⑤ 蓝图 §1.2/§2 字面 \"新建\" 非 \"升级\")
```

任一 0 hit (除反约束行) → CI fail.

## 4. 不在本轮范围 (反约束)

- ❌ 群 DM (3+ 人私聊, 立场 ①, 想群聊 → 升 channel)
- ❌ DM workspace / artifact (立场 ② 蓝图 §1.2 字面禁)
- ❌ DM topic / 加人按钮 (立场 ① 蓝图 §1.2 ❌)
- ❌ channel → DM 反向降级 (CHN-2.3 不开降级)
- ❌ DM → channel 单向 \"升级 / 转换\" 路径 (#354 立场 ⑤ + 蓝图 §1.2/§2 字面 \"新建\" 非 \"升级\"; 想加人 → 新建 channel 拉双方)
- ❌ DM 端到端加密 (留 v3+, ADM-0 god-mode 元数据可见前提)
- ❌ DM 跟 channel 混排 \"recent\" 时序栏 (立场 ③ 字面禁)

## 5. Test plan (实施 PR 各自带, 此 spec 不带)

- CHN-2.1: `POST /dms` idempotent (双向重创回同一 id) + 成员数锁 reject 加 3 人 (400 文案锁 \"DM 不可加人, 升 channel\") + DM `GET /artifacts` 403 (立场 ②) + DM-2 v=14 同 migration 不冲突 (战马B 协同)
- CHN-2.2: client e2e DM 详情页**不渲染** Workspace tab (DOM count==0) + 侧栏 DMList / ChannelList 拆 sibling (DOM 路径反断) + DM 头像 `border-radius: 50%` 截屏锁
- CHN-2.3: DM 内 `@<3rd_user>` mention 候选空 + placeholder 字面 byte-identical (#354 立场 ⑤ 同源) + \"新建频道\" 引导跳 `POST /channels` (CHN-1 既有端点) + 反向 grep \"升级为频道\" 同义词 0 hit

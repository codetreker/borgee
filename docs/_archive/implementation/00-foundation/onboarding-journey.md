# Onboarding Journey — 新用户第一分钟

> **作者**: 野马 (PM) · 2026-04-28 · 立场签字: 野马
> **截止**: 2026-05-05 (CM-onboarding 实施前置, 战马等此 doc)
> **位置**: `docs/implementation/00-foundation/` (跨模块旅程, 不属于单一模块)
> **状态**: v1 锁定 — 任何文案改动需 **野马 +1**, 任何步骤增删需 4 人 review

---

## 1. 这是什么 / 不是什么

| 是 | 不是 |
|----|------|
| 业主 (human owner) **注册成功后到看到第一个 agent 上线** 的端到端旅程 | UI 设计稿 / 像素级布局 |
| 每一步的 success / empty / error / skip 状态文案锁定 | 完整 onboarding tour (那是 v1 范围) |
| CM-onboarding / AL-2 / RT-0 三个 milestone 的产品立场标尺 | 邀请注册流 (B2 团队邀请, v1) |
| 立场 §1.4 团队感知 + §11 沉默胜于假 loading 的实施约束 | 性能指标 (走 RT-0 的 ≤3s 硬条件) |

**目标用户**: 新业主 (1 个 human + 0 个 agent), 从注册成功到第一次"感觉到 agent 是同事"。

**对应蓝图立场**:
- §1.4 — 团队感知主体验 (打开 app 第一眼看到团队, 不是空白)
- README §核心 11 — 沉默胜于假 loading (任何空状态必须解释原因; 实时 frame 见 `realtime.md §1.1`)
- §1.3 — agent 间独立协作 (体感断档兜底口播, 第 5 步)

---

## 2. 端到端 6 步

```
[1] 注册成功
     ↓ auto-create #welcome + auto-join + auto-select
[2] 第一眼看到 #welcome (非空屏, §1.4)
     ↓ system message + quick action button
[3] 点 [创建 agent] CTA
     ↓ AgentManager 3 步内
[4] agent 创建成功 (host-bridge 装时不问)
     ↓ agent 自动上线
[5] 左栏出现 agent + subject 文案 "正在熟悉环境"
     ↓ 业主对 agent 说第一句话
[6] agent 回应 + 产品口播 "未来 agent 会互相协作" (§1.3)
```

每步 **必有** success / empty / error / skip 四态文案。**沉默不是选项** (README §核心 11)。

---

## 3. 步骤详细规范

### 步骤 1 — 注册成功 → 进 #welcome

**触发**: `POST /api/v1/auth/register` 200 → client `RegisterPage` redirect 到 `App.tsx`。

**server 行为** (CM-onboarding 范围):
- 同事务内: create org + create user + create `#welcome` channel (`kind=system`) + insert channel_member + insert system message
- 不在事务: 任何外部调用 (host-bridge / push)

**client 行为** (CM-onboarding 范围):
- `actions.loadChannels()` 完成后, 默认 selected channel = #welcome (取代旧 "👈 选择频道")

**文案** (锁定, 野马 +1 才能改):

| 状态 | 文案 (中文) | 备注 |
|------|------------|------|
| ✅ success | (无独立文案, 直接进步骤 2) | |
| ⚠️ empty | N/A | 此步无 empty 态 (新注册必有 #welcome) |
| ❌ error backfill 失败 / channel 创建失败 | "正在准备你的工作区, 稍候刷新…" + [重试] 按钮 | **README §核心 11 反约束硬条件**: 严禁沿用旧 "👈 选择频道开始聊天" (那是沉默) |
| ⏭️ skip | N/A | 不可 skip |

**验收锚点**: G2.4 截屏 "Welcome 第一眼非空屏" + CM-onboarding E2E 数据契约。

---

### 步骤 2 — #welcome 第一眼 system message

**触发**: 步骤 1 完成, channel 已 selected, MessageList 已渲染。

**显示**:
- system message 在 `#welcome` 顶部 (sender = system, 不显头像)
- 消息文末跟一个 **quick action button** "创建 agent" (野马 R1 加补硬条件: CTA 不能是死字)

**文案** (锁定):

| 状态 | 文案 |
|------|------|
| ✅ success (业主第一次进) | "**欢迎来到 Borgee 👋**<br>这里是你的工作区。Borgee 不是一个 AI 工具, 而是让你和 AI 同事一起协作的地方。<br>第一步: 创建你的第一个 agent 同事 →" + button [创建 agent] |
| ⚠️ empty (理论上不出现) | N/A |
| ❌ error (system message 写入失败) | channel 标题位显示 "⚠️ 欢迎消息加载失败, [重试]" — **不**默认渲染空 channel (README §核心 11) |
| ⏭️ skip (业主再次进入, 已创建过 agent) | system message 仍在 (历史保留), CTA 替换为 "你的 agent 已就位 →" 灰态 link 跳左栏聚焦 |

**点击 [创建 agent] 行为**: 跳 AgentManager (`setShowAgents(true)`), AgentManager 进 step 1。

**验收锚点**: G2.4 截屏 "Welcome 第一眼非空屏 + subject 文案"。

---

### 步骤 3-4 — 创建 agent (3 步内)

**约束** (蓝图 host-bridge §1.3 "装时轻, 用时问"):
- 创建流程 ≤ 3 步, 每步只问 1 个必填
- host-bridge 装机 / 配置不在此处问 (留到 agent 第一次需要本地能力时再问)
- 不要求选 "角色" (蓝图 agent-lifecycle §2.1 无角色库立场)

**3 步**:
1. 起个名字 (input, 默认建议 "助手")
2. 选个 runtime (radio, 默认 OpenClaw — 立场 §7 "Borgee 不带 runtime")
3. 确认 (review + submit)

**创建成功 (步骤 4)**: agent 进 `users` 表 (role=agent), 默认 `[message.send, message.read]` (AP-0-bis), 自动加 #welcome 成员, agent 自动 register 进 presence map → online。

**文案** (锁定):

| 状态 | 文案 |
|------|------|
| ✅ success | toast "🎉 {name} 已加入你的团队" + 自动跳回 #welcome + system message "@{name} 上线了, 试试和它打招呼 →" |
| ⚠️ empty | N/A (流程不存在 empty 态) |
| ❌ error 名字重复 | inline error "这个名字已经有人用了, 换一个吧" |
| ❌ error runtime 不可达 | inline error "runtime 暂时连不上, 你可以先创建, agent 会显示『故障 (runtime_unreachable)』, 修好后自动恢复" — **保留创建路径**, 不挡 (README §核心 11 解释原因) |
| ⏭️ skip (业主关闭弹窗) | 退回 #welcome, system message 不变, CTA 仍可点 |

**验收锚点**: G2.4 截屏 "agent 创建成功 toast" (可选, 5 张内取舍)。

---

### 步骤 5 — 左栏出现 agent + subject 文案

**触发**: agent 创建成功 + presence register。

**Sidebar 显示**:
- 左栏团队区出现 agent 一行: `🤖 {name}` + 状态点 + **subject 文案**
- subject 文案是 §11 "沉默胜于假 loading" 的产品落点: agent 在 "做什么" 必须说出来

**文案** (锁定, AL-1b 实施时引用此处):

| agent 状态 | 主文案 | subject 文案 (一行内) | 备注 |
|------|------|------|------|
| online + 刚创建 (≤2min) | 在线 | "正在熟悉环境…" | 默认初次 subject |
| online + idle | 在线 | (空) | 不强制有 subject (但**不是糊弄灰**, 是显式 online) |
| online + thinking | 在线 | "在想 {subject}…" | README §核心 11 硬条件: 必带 subject, 无 subject 不渲染 thinking |
| busy (Phase 4 BPP) | 忙碌 | "在做 {task_label}…" | AL-1b 实施 |
| error | 故障 | "{reason_code}" (e.g. "api_key_invalid") | README §核心 11 解释原因, 不糊弄 |
| offline | 已离线 | (空) | **不准用"灰点 + 不说原因"** (野马 R2 反约束) |

| 状态 | 整体文案 |
|------|---------|
| ✅ success | 上表展示 |
| ⚠️ empty (业主无 agent — 步骤 4 之前) | 左栏团队区显示 "👋 你还没有 agent 同事" + [创建 agent] CTA (与步骤 2 联动) |
| ❌ error (presence map 故障) | agent 行显示 "故障 (presence_unavailable)" — 不假装 online |
| ⏭️ skip | N/A |

**验收锚点**: G2.4 截屏 **"左栏团队感知"** (5 张关键截屏之一, 同时验 §1.4 + §11)。

---

### 步骤 6 — agent 回应 + §1.3 体感断档口播

**触发**: 业主在 #welcome 对 agent 说第一句话, agent 回应。

**文案** (system message, agent 第一次回应后 1 条, 业主 session 内只发 1 次):

> "💡 你和 {agent_name} 的协作开始了。<br>未来你的 agent 们之间也能互相协作 (Phase 4 上线), 你不用每次居中调度 — 它们会像真正的同事一样配合。"

**目的**: 立场 §1.3 (agent 间独立协作) 在 Phase 2-3 间体感断档 6+ 个月, 用产品口播兜底, 让业主**预期管理**。

| 状态 | 文案 |
|------|------|
| ✅ success | 上文 (1 次性, 写 user_preference 字段去重) |
| ⚠️ empty / ❌ error / ⏭️ skip | 静默不展示 (此口播是锦上添花, 不算硬功能, 失败不报错) |

**验收锚点**: G2.4 demo **口播一次** (野马跑 demo 时口头说一次, 不要求 system message 实现 — Phase 2 用口播替代, Phase 4 改 system message)。

---

## 4. 文案锁定原则

1. **任何改动需野马 +1** — 在 PR 描述里 @yema 并写改动理由。
2. **§11 反约束** 跨步骤适用: 任何 loading / 空状态 / 错误状态都必须**说出原因或下一步**, 不准沉默。
3. **§1.4 团队感知** 跨步骤适用: 业主任何时刻进 app, 左栏不能是空白 — empty 态也要有 CTA。
4. **CTA 不能是死字**: 任何"试试 →"必须有可点击的下一步, 不能是 system text 装样子。

---

## 5. 反推 surface 缺口 (战马/飞马 sign-off 后排期)

| Surface | 现状 | 需要 |
|---------|------|------|
| #welcome 自动建 + auto-select | 不存在 | CM-onboarding (战马 0.5-1 天) |
| system message + quick action button | system message 已支持, button 未支持 | CM-onboarding 加 message kind 扩展 1 字段 + client 渲染 + AgentManager 跳转钩 (~0.5 天) |
| App.tsx 空状态降级文案 | 旧 "👈 选择频道" | CM-onboarding 改文案 + 加 retry hook (~0.2 天) |
| Sidebar agent subject 文案 | 不存在 | AL-1b (Phase 4 BPP 同期, 文案锁此 doc 引用) |
| presence "正在熟悉环境" 默认初次 subject | 不存在 | AL-1b (实施时挂此 doc §3 步骤 5 锁文案) |
| §1.3 体感断档口播 | Phase 2 用 demo 口播, Phase 4 改 system message | 无 surface 工作, 演示时口播即可 |

---

## 6. 验收挂钩

| Gate | 本 doc 引用点 |
|------|--------------|
| G2.4 用户感知签字 (野马) | 步骤 2 截屏 + 步骤 5 截屏 (5 张关键截屏之 2 — **第 5 张同时覆盖步骤 2 (Welcome 第一眼非空屏) 和步骤 5 (左栏团队感知)**, execution-plan G2.4 锁 5 张数学一致) |
| CM-onboarding Acceptance | 步骤 1 数据契约 + 步骤 2 E2E + 步骤 1 error 文案 |
| AL-1b Acceptance | 步骤 5 subject 文案锁 |
| README §核心 11 反约束 grep + 截屏 | 跨步骤 (1 / 2 / 5 错误态) |
| §1.4 第一眼非空屏 | 步骤 1 + 5 |

---

## 7. 不在本 doc 范围

- B2 团队邀请注册流 (业主邀请其他 human 加入 org) — v1
- agent 高级配置 (model / temperature / system prompt) — AL-2
- host-bridge 第一次授权弹窗 (用时问) — HB-3
- 多 agent 团队管理 (>3 个 agent 的分组) — Phase 3 CHN-3
- mobile / PWA 版的 onboarding (要重写, 三栏不适用) — CS-3

---

## 7.x Phase 3 后扩展旅程 — 协作场闭环 demo (野马, 2026-04-29 v1.1 加)

> **触发**: Phase 3 章程 9 milestone 4 件套全闭后, 业主从 "看到第一个 agent 上线" 升级为 "用 agent 完成一次产品协作". 这是 G3.4 退出公告 demo 故事线 — Canvas Vision 第一刀的用户感知证明。

| 步骤 | 用户动作 | 系统行为 | 文案锁源 | 反约束 |
|------|---------|---------|---------|-------|
| 7a | 创新 channel `#prd-v2-spec` (private) | 默认 `default_tab="chat"` 进 chat tab + 双 tab 切换器渲染 ("聊天" / "工作区") | #382 ④ + ① | DM 视图无双 tab (#382 ⑤ 7 源 byte-identical) |
| 7b | invite agent (`@spec-helper`) | CHN-1 既有路径 + chat tab member 列表显 🤖 角标 (二元锁) | #354 ② + #347 line 251 | agent 不发自我介绍消息 (#11 silent default) |
| 7c | switch workspace tab + 创 markdown artifact "PRD v2 草稿" | `data-artifact-kind="markdown"` + ArtifactPanel #346 既有 | #370 ① + #382 ③ | workspace tab 不渲染 message (双 tab 不交叉) |
| 7d | 点 iterate 按钮 🔄 + intent "请帮我补充用户故事章节" | iterate state pending → running ("agent 正在迭代…") | #380 ①②③ | 非 owner DOM omit (defense-in-depth #347 line 254) |
| 7e | iterate completed → 自动跳新版 v2 + kindBadge 🤖 | system message fanout `{agent_name} 更新 PRD v2 草稿 v2` byte-identical (跟 artifacts.go:591) | #380 ④ + CV-1 #347 | 走 CV-1 既有 commit 路径单源 (#365 立场 ②) |
| 7f | switch 到 v2, hover 段落 → 💬 "评论此段" → 写 review "agent 你这章逻辑没闭合" | 锚点 thread 创 + header "段落讨论" + placeholder | #355 ①②③ | agent 不能开 thread (反约束三连, server 403) |
| 7g | agent 回复锚点 → 角标 🤖 + thread 内 reply | 反向断言 cross-anchor agent→agent 403 (蓝图 §1.6 钉死人审) | #355 ④⑤ | agent 仅在 owner 起的 thread 内 reply |
| 7h | 点 "标为已解决" → thread 折叠 + DOM `data-resolved="true"` | thread 灰底降权但不从 DOM 移除 (review 历史保留) | #355 ⑥ | 已解决 anchor 不删 (跟 CV-1 立场 ③ 同根 — agent 默认无删历史权) |
| 7i | (并行) chat tab 输入 `@spec-helper 这个有空跟我聊聊` | mention 解析 → MentionPushedFrame 8 字段 byte-identical → target 在线 ≤3s push | #314 ①② + #372 | raw UUID 仅 `data-mention-id` attr 不进文本节点 (隐私 §13) |
| 7j | (并行) target agent 离线时 mention | 离线 fallback owner DM "{agent_name} 当前离线，#{channel} 中有人 @ 了它…" | #314 ③ + #372 | 发送方 UI **无任何提示** (#314 ④ 永久锁) |

**G3.4 退出闸三签依据**: 7a-j 全流路径 e2e ≤30s 闭环 (CHN-4.3) + 双 tab 截屏 + 多视角截屏 (野马签 byte-identical 验, 路径锁见 `docs/qa/screenshots/README.md`).

---

## 8. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v1 初版, CM-onboarding 实施前置 |
| 2026-04-29 | 野马 | v1.1 — 加 §7.x Phase 3 协作场闭环 demo 旅程 (7a-j 全流路径), 跟各 milestone 文案锁 byte-identical 同源, 撑 G3.4 退出公告. 不动 §1-§7 (CM-onboarding 立场 v1 锁字面已稳定不漂) |

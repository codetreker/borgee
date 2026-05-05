# Canvas Vision — Workspace + Artifact 协作

> Borgee 的"对着文档讨论"愿景：在每个 channel 里，AI 团队（PM、Dev、QA、Designer 等）和人一起围绕产物（artifact）协作。
> 状态：建军 + 飞马 + 野马 对齐（2026-04-27）。前置阅读：[`concept-model.md`](concept-model.md)、[`channel-model.md`](channel-model.md)。

## 0. 一句话定义

> **每个 channel 里，AI 团队和人围绕一组 artifact（PRD、代码、设计稿、测试用例…）协作；artifact 既是产物又是讨论锚点。**

## 1. 目标态（Should-be）— 五条立场

### 1.1 用户场景：个人 + AI 团队

- 第一批用户 = **个人效率达人**（独立创业者 / 工作室主理人）
- 拥有一个 AI 团队：PM agent、Dev agent、QA agent、Architect agent、Designer agent ……
- 主交互场景：用户在 channel 里下达意图，AI 团队成员协同产出
- 这是 [concept-model §1.2](concept-model.md) "agent = 同事"在工作流维度的具体落地

### 1.2 文档协作 = 轻量画布（D-lite，不是 Miro）

- **不是**无限画布（Miro/FigJam）——Borgee 没有原创设计资本，不跟人家比布局。
- **不是**纯查看器（A）——跟 Slack 链接预览无差异化。
- **是**：channel 自带轻量协作面板，承载 Markdown 卡片 / 文档 / 文件 / 段落锚点对话。
- 心智更接近 Linear issue + comment，而不是 Miro 白板。

### 1.3 画布与 channel 的关系：channel 自带（A）

- **每个 channel 内置一个 workspace 区域**——画布不是独立资源，权限直接继承 channel 成员。
- 这跟 [channel-model §1.1](channel-model.md) 的"channel = 聊天 + 共享工作空间"双支柱一致。
- 不做 "同一份数据多种视图"（C 基础设施）的复杂方案——CRDT 巨坑，v1 不踩。

### 1.4 Workspace = Artifact 集合

> 这是第 3 轮的关键洞察：把 Cursor/Claude 的 artifact 概念融入 workspace。

- 每个 channel 的 workspace 由一组 **artifact** 组成。每个 artifact 是一份独立产物：
  - PRD 文档（Markdown）
  - 代码片段（带语言标注）
  - 测试用例
  - 设计稿（图片或链接）
  - 看板 / 待办（v2+）
- artifact 可以被消息 `@` 引用，引用时在消息流里自动展开预览。
- artifact 自带**版本历史**：agent 每次修改产生一个版本，人可以回滚。

### 1.5 Agent 在 workspace 上的能力（B）

| 行为 | 默认 | 需要 owner grant |
|------|------|------------------|
| 写内容（创建/编辑 artifact 内容） | ✅ 默认允许 | — |
| 在 artifact 上回复锚点评论 | ✅ 默认允许 | — |
| 创建新 artifact | ✅ 默认允许 | — |
| **修改布局**（重命名、归档、删除、调整顺序） | ❌ | ✅ 需 grant |
| **删除版本历史** | ❌ | ✅ 需 grant |

> **设计直觉**：agent 是同事，能贡献内容；但**重构空间结构**是更高权限，需要 owner 显式授权——这是 [concept-model §1.3](concept-model.md) "协作可以、扩权不行"的具体落地。

### 1.6 锚点对话：人机界面，不是 agent 间通信

- 锚点对话（在 artifact 的某个段落上挂讨论） = **owner review agent 产物**的工具
- agent 之间互相通信走普通 channel message + artifact 引用，**不需要锚点**
- 这避免了"AI 自己跟自己锚点对话"的诡异场景，把锚点的语义钉死为人审产物

---

## 2. v1 实施范围（最小可探索版本）

> 目标：用最小工作量验证"AI 团队产出沉淀到 channel 的 workspace"这个行为是否真实成立。

### v1 做

- 每 channel 一个 workspace 区域（侧边栏 tab，与 chat 平级）
- 可创建 **Markdown artifact**（单文档，无附件）
- agent 可 iterate（再次写入触发新版本）
- 人可编辑
- artifact 可被消息 mention/引用，引用时展开预览
- 简单版本历史（线性，可回滚到前一版）

### v1 **不做**

- ❌ 无限画布（Miro 风格）
- ❌ 多 artifact 关联视图（拖拽连线）
- ❌ realtime CRDT（一人编辑一锁，足够个人场景）
- ❌ PDF / PR diff 渲染
- ❌ 段落锚点对话（v2 加，v1 验证文档形态够不够）
- ❌ 看板、思维导图、白板等非 markdown 形态

---

## 3. 与现状的差距

| 目标态 | 现状 | 差距 |
|--------|------|------|
| Workspace = artifact 集合 | 当前是普通文件树（上传 + 自动归档消息附件） | 需新建 artifact 数据模型（带版本、metadata、引用关系） |
| Markdown artifact 编辑 | 完全没有 | v1 核心新功能 |
| Agent iterate / 版本历史 | 无 | 需要新表 + 写入策略 |
| Mention / 引用展开 | 无 | UI + 消息渲染层增加新组件 |
| "agent 写内容默认允许，动布局需 grant" | 当前权限只在 channel 粒度 | 权限系统加 `workspace.edit_content` / `workspace.modify_structure` 两个 scope |

## 4. 与其它文档的关系

- 跟 [`concept-model.md`](concept-model.md)：本文是"AI 团队 = 同事"在 workspace 维度的展开。
- 跟 [`channel-model.md`](channel-model.md)：本文细化了 §1.3 "Workspace 核心"——把"文件树"升级为"artifact 集合"。
- 跟未来的"权限"轮（第 8 轮）：本文 §1.5 提出的两类 workspace 权限将在那一轮统一收敛。

## 5. 不在本轮范围

- agent 跟 owner / 跨 org agent 的具体协作流程 → 第 4 轮"Agent 接入与生命周期"
- 权限的具体 scope 设计 → 第 8 轮"Auth & 权限"
- artifact 的 SQLite schema 与事件流 → 第 10 轮"数据层"
- artifact 编辑器的 UI 形态 → 第 11 轮"Client (web SPA)"

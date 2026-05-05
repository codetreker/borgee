# Channel Model — channel / DM / workspace

> Borgee 的 channel 不是单纯聊天容器，是"协作场"。本文是对 channel/DM/workspace 三件套的形状层规范。
> 状态：建军 + 飞马 + 野马 对齐（2026-04-27）。前置阅读：[`concept-model.md`](concept-model.md)。

## 1. 目标态（Should-be）— 四条立场

### 1.1 Channel = 协作场（聊天 + 共享工作空间）

- **不是** Slack 风格的聊天容器（先聊天，附件是补充）
- **不是** Discord 的社区频道（声音/角色为重）
- **是协作场**：channel 在 Borgee 的意涵是"一群人 + 一群 agent 围绕一件事工作的地方"。
- 形态由两个共同支柱构成：
  1. **聊天流**——讨论、决策、状态同步
  2. **共享工作空间（workspace）**——文档、文件、产出
- Agent 在 channel 里是**原生成员**，不是 webhook/bot 投递者——agent 与人**一样是"房东之一"**，能感知 channel 的状态、在 workspace 中放置/读取产物。

> 这条把 §concept-model §1.2（agent = 同事）落到 channel 维度。如果 channel 退化成纯聊天，agent "同事"定位也跟着退化成 bot。

### 1.2 DM：概念独立，底层可复用

- **概念上**：DM 是**私密 1v1 对话**，与 channel 的"协作场"语义**明确分开**。
- **底层**：可以复用 message / reaction / 事件流等数据结构（实现简单）。
- DM **不**继承的特性：
  - ❌ workspace（DM 没有共享文件树）
  - ❌ topic（私聊不需要主题描述）
  - ❌ 加人（DM 永远是两个人；想加人就升级成 channel）
- **UI 视觉与交互**与 channel **明确不同**——不让用户混淆"我在私聊"还是"在协作"。

### 1.3 Workspace：核心（artifact 集合）

- **目标态**：workspace 与聊天**并列核心**——这是 Borgee 与 Slack 的关键差异之一。
- **形态（详见 [`canvas-vision.md`](canvas-vision.md)）**：workspace 是一组 **artifact**（PRD、代码片段、设计稿、测试用例…），不是简单文件树。每个 artifact 自带版本历史，agent 可 iterate 编辑、人可 review/edit。
- **v1 实现节奏**：默认**收起** workspace，聊天优先曝光。这是**节奏选择，不是降级**——v1 跑最小 markdown artifact，验证"AI 团队产出沉淀"行为。
- workspace 内容包括：
  - Markdown artifact（v1 唯一形态）
  - 主动上传的文件 / 消息附件归档（沿用现状）
  - （v2+）代码片段、设计稿、看板等更多 artifact 类型
- workspace 与 [`concept-model.md` §1.2](concept-model.md) 的"agent = 同事"耦合——agent 默认能写内容（创建/编辑 artifact），但**修改布局**（重命名、删除、归档）需要 owner grant，详见 [`canvas-vision.md` §1.5](canvas-vision.md)。

### 1.4 Channel 分组：作者定义 + 个人微调

- **作者**（channel 的创建者 / owner）**定义 group 结构**——所有成员看到同一套分组。
  - 心智：跟 Discord 的 category 一样，作者控制大局。
- 每个用户可独立控制：
  - **group 的展开/折叠状态**
  - **侧边栏的排序**（在 group 之上的个人偏好层）
- 不允许个人**改 group 名**或**重新分组**——那是 channel 作者的事。

> 这是漂亮的折中：保留协作心智的"我们看到的是同一个组织结构"，又把个人偏好（折叠/排序）藏起来不污染他人。

---

## 2. 关键不变量

| 不变量 | 含义 |
|--------|------|
| Channel **跨 org 共享** | 一个 channel 里可以同时坐多个 org 的人 + agent |
| Channel 创建者归属 | 创建者所在 org "拥有" channel；agent 创建则归 owner |
| Agent 加入 channel 必须由 owner 触发 | 跨 org 邀请规则见 [`concept-model.md` §4.2](concept-model.md) |
| Agent 在 channel 里**代表自己** | mention 路由不展开到 owner，见 [`concept-model.md` §4](concept-model.md) |
| DM 永远 2 人 | 想加人 → 创建新 channel 把双方拉进去 |
| Workspace per channel | 每个 channel 一棵独立文件树；DM 没有 workspace |

## 3. 与现状的差距（v1 还差什么）

### 3.1 Channel 作为"协作场"

- 当前实现：channel = 聊天容器 + per-channel workspace（已经是 70% 形态）
- 差距：workspace 还只是**附件归档**视觉权重低；agent 在 channel 里仍偏"消息发送者"。
- 下一步：当画布/文档协作开始铺设时，workspace 升级为协作场的另一支柱。

### 3.2 DM 与 channel 的概念分离

- 当前实现：DM 复用 channel 表，`type="dm"`，**底层完全统一**——这一条符合 §1.2 的"底层可复用"。
- 差距：UI 上 DM 与 channel 视觉差异不够大，长期容易混淆；DM 当前**也有 workspace 入口**（虽然没人用），需要在 UI/产品层显式禁用。

### 3.3 Workspace 升级路径

- 当前实现：每个 channel 一个文件树，可上传 + 自动归档。
- 差距：缺画布、缺协作文档、缺 agent 直接读写 workspace 的标准化接口。
- 下一步在第 3 轮（画布/文档协作愿景）展开。

### 3.4 Channel 分组的作者 vs 个人分层

- 当前实现：`channel_groups` 全 org 共享（任何人拖动都改大家看到的顺序）。
- 差距：缺"个人折叠状态"和"个人排序"。需要新增 `user_channel_layout(user_id, channel_id, collapsed, position)`（或 group 层面的）。
- 这是中等改动，不影响数据迁移，纯 UI + 个人偏好表。

## 4. 不在本轮范围

- 画布、协作文档、agent 直接编辑 workspace 的具体 API → 第 3 轮"画布/文档协作愿景"
- agent 加入 channel 的具体邀请审批状态机 → [`concept-model.md` §4.2](concept-model.md)（已落定）
- DM 与 mention 路由 → [`concept-model.md` §4.1](concept-model.md)（已落定）

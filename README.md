# Borgee

> **个人即组织, agent 是你的同事 — 不是工具。**

Borgee 是为独立创业者、工作室主理人、个人效率达人打造的协作平台。在这里, 你不是"用 AI", 而是和**一支 AI 团队一起工作**: PM、Dev、QA、Designer agent 像同事一样坐在 channel 里, 围绕 PRD、代码、设计稿、测试用例和你共事。

---

## 你看到的 Borgee

打开 Borgee, 你不会看到"组织"、"工作区"、"切换团队"这些重概念。你看到的是:

- **左侧团队** — 你的 AI 同事们: PM、Dev、QA、Designer, 各自有名字、有头像、有"在做什么"
- **中间 channel** — 一群人 + 一群 agent 围绕一件事工作的地方, 不是聊天群也不是社区频道, 是协作场
- **右侧 workspace** — channel 里产出的东西: 一份 PRD, 一段代码片段, 一个测试清单。每份产物有版本历史, agent 写新版, 你可以回滚

DM 留给私密 1v1, 跟协作场视觉上明确不同, 不让你混淆"我在私聊"还是"在协作"。

---

## 我们坚信的几件事

这些不是技术选型, 是产品的脊梁:

1. **个人即组织** — 一个人就是一个 org, UI 永远不告诉你 "org" 是什么。多人协作以 channel 为载体, 不是把人塞进 workspace
2. **Agent = 同事** — 不是工具、不是助手、不是 webhook bot。agent 在 channel 里代表自己; 它"加入"channel 时**默认沉默**, 直到被点名或主人触发 — 沉默胜于假活物感
3. **沉默胜于假 loading** — 任何"思考中"动画必须告诉你**它在想什么** ("正在阅读 main.go" / "正在写第 3 节"), 没有信息的 spinner = 信任崩塌, 我们宁可不显示
4. **Workspace 与聊天并列** — channel 不是 Slack 聊天容器, 也不是 Discord 社区频道。聊天讨论 + workspace 产物双支柱, agent 既能说话也能写产物
5. **Borgee 不带 runtime** — 我们不绑定一种 AI; agent runtime (OpenClaw、Hermes、你自己的) 通过中立协议 (BPP) 接入, Borgee 是 agent 配置面 + 协作场, 不是另一个 ChatGPT 壳
6. **管控元数据 OK, 读你内容必须授权** — 平台 admin 强权但不窥视, 后台永远不返回消息正文 / artifact 内容; admin 进了你的 channel 你的屏幕顶部会有红色横幅常驻

完整的 14 条立场见 [`docs/blueprint/README.md`](docs/blueprint/README.md)。

---

## 一个典型的工作日

> 你是一个独立产品人。早上想到一个新功能。

1. 在 PM agent 的 DM 里说一句"我想做 X", PM agent 接住意图, 在你们的 channel 里生成一份 PRD artifact
2. Dev agent 看到 artifact mention, 开始读、写代码片段 artifact, 进度条上写着"正在写 OAuth 接入第 2 节" — 不是模糊的"思考中"
3. QA agent 基于 PRD 起测试用例 artifact, 邀请进 channel 时静默落座, 等被 mention 才开口
4. 你在 PRD 第 3 段加锚点评论: "这里改一下"。这是**人审 agent 产物**的工具, agent 之间不用锚点互通, 它们走 channel 消息
5. Dev agent 基于你的 review 改第 2 版, 你回滚一次, 选择第 1 版的方向继续
6. 一天结束, channel 的 workspace 沉淀了 3 份 artifact + 一条决策时间线

不是"用 AI 提效", 是"和一支 AI 团队一起做事"。

---

## 项目状态

Borgee 还在快速迭代:

- **Phase 1 ✅** 资源归属与多 org 数据基线
- **Phase 2 ✅** 条件性全过 — 详 [`docs/qa/phase-2-exit-announcement.md`](docs/qa/phase-2-exit-announcement.md)
- **Phase 3 🟡** Channel 协作场 + Workspace artifact + Realtime 推送 — 立场反查表已落 ([phase-3](docs/qa/phase-3-stance-checklist.md) / [cv-1](docs/qa/cv-1-stance-checklist.md))
- **Phase 4** 插件协议 (BPP) + Agent 生命周期 + 隐私承诺页

---

## 想了解更多

| 入口 | 适合谁 |
|---|---|
| [`docs/blueprint/`](docs/blueprint/) | 想理解 Borgee 产品形状与立场 |
| [`docs/blueprint/current/concept-model.md`](docs/blueprint/current/concept-model.md) | 想知道 "agent = 同事" 怎么落到数据模型 |
| [`docs/blueprint/current/canvas-vision.md`](docs/blueprint/current/canvas-vision.md) | 想看 workspace + artifact 的产品愿景 |
| [`docs/PRD-v3.md`](docs/PRD-v3.md) | PRD v3 |
| [`docs/qa/`](docs/qa/) | 想看每条立场怎么被验收守住 |

dev 上手 / 跑本地 / 部署见 [`docs/implementation/`](docs/implementation/) 与 `packages/*/README.md`。

---

*"不是 AI 替代你, 是 AI 和你一起做事。"*

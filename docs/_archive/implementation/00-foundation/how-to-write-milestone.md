# How to Write a Milestone

> 写 implementation 模块文档的规范。每个 implementation/&lt;module&gt;.md 都遵循这个模板。

## Milestone 文档结构

```markdown
# Implementation · <Module Name>

> 蓝图: [`../../blueprint/<module>.md`](../../blueprint/<module>.md)
> 现状: [`../../current/<module>/`](../../current/<module>/) 或 inline 简述
> 阶段: ⚡ v0 / 🛡️ v1 (见 README §阶段策略)

## 1. 现状 → 目标 概览

一段话讲清: 当前代码长什么样, 蓝图长什么样, 距离主要在哪几个点。

## 2. Milestones

### M-1 短标题 (例: organizations 表落地)

- **目标**: 一句话(blueprint 的某个具体立场,带 §X.Y 锚点)
- **Owner**: 飞马 (review) / 战马 (实现) / 野马 (立场把关) / 烈马 (acceptance 跑通)
- **范围**: bullets, 5 条以内
- **不在范围**: 明确划掉, 防止 scope creep
- **依赖**: 前置 milestone IDs
- **预估**: ⚡ v0 / 🛡️ v1 阶段各几天/周
- **PR 拆分**: 列出可独立合并的 PR 标题 (每 PR 标 "战马作者 / 飞马 review / 烈马 acceptance")
- **Acceptance spec** (四选一,每 PR ≥1 个):
  - **E2E 断言**: 用户可见的行为, e.g. "新注册 user 自动有一个 org, GET /me 返回 org_id 字段"
  - **蓝图行为对照**: blueprint §X.Y 的某条规则, e.g. "符合 §1.1 — agent 默认只有 message.send"
  - **数据契约**: schema/API/protocol 的具体字段, e.g. "users 表有 org_id 列, NOT NULL, 索引存在"
  - **行为不变量**: 节流 / idempotency / 状态机合法转移 / 用户感知签字 (见 §第 4 类详解)

### M-2 ...
```

## Acceptance Spec 四选一详解

每个 PR 必须挂 **至少一种** 验收方式。粒度按"能轻则轻"原则选。

### 1. E2E 断言

最强,但最贵。

格式:
> 用户/agent 在 X 场景下, 触发 Y, 期望看到 Z。

例:
> 用户 A 在 channel C 中 @ agent B (B 离线), 期望 B 的 owner 在 5 秒内收到 system message "B 离线, channel C 中有人 @ 它"。

适用: 用户感知层的功能、跨多模块协同的功能。

### 2. 蓝图行为对照

中等强度。

格式:
> 实现满足 [`../../blueprint/<module>.md`](../../blueprint/<module>.md) §X.Y 的规则: "<引用原文一段>"。

例:
> 实现满足 blueprint/current/concept-model.md §1.2 的规则: "Agent 默认权限最小化(`message.send`),由 owner 显式 grant 进一步能力"。

适用: 修 bug、refactor、不改用户行为的小改动。

### 3. 数据契约

最轻,纯结构。

格式:
> Schema/API/protocol 的某个具体字段达到 X 状态。

例:
> `users` 表存在 `org_id TEXT NOT NULL` 列, `idx_users_org_id` 索引存在。

适用: 纯 schema 变化、新增字段、加索引。

### 4. 行为不变量

中等到最强, 处理"功能正确但不能这样表达"的场景。分两小节:

#### 4.1 行为不变量 (机器可断言)

格式:
> 在 X 条件下, 系统行为 Y 必须成立 (单测 / 集成测试可跑)。

例:
> 同一 owner 在 5 分钟节流窗口内被多次 @ → 系统只发出 1 条 system message (单测断言计数)。
> 跨 org 邀请状态机: `pending → approved | rejected | expired`, 其它转移非法 (单测覆盖每条边)。
> 同一邀请被重复接受 → 第二次返回 idempotent 响应, agent 不重复 join (集成测试)。

适用: 节流 / 状态机 / idempotency / 并发去重。

#### 4.2 用户感知签字 (人工签字)

格式:
> 标志性 milestone 关闭前, 由野马 (PM) 跑一遍 demo, 主观签字"X 立场成立", **截关键步骤 3-5 张截屏留档** (AI 团队不录视频)。

例:
> 野马跑 CM-4 demo: 邀请 → 接受 → @ 离线 → 收到 system message。主观签字"看起来像同事不像 bot", 截 3-5 张关键截屏存入 `docs/evidence/cm-4/` 目录。截屏 commit 时附 `blueprint-sha.txt` 记录当时 blueprint 版本。

适用: 标志性 milestone (产品立场层); 强制留截屏 (而非视频, AI 团队), 方便后续若有人改坏立场拿截屏对照。

> ⚠️ **底线**: 4.1 和 4.2 不能互相替代。工程端不能用单测把 4.2 糊弄过去, 产品端也不能用主观签字代替 4.1 的可重复断言。

> ⭐ **标志性 milestone 强制 4.1 + 4.2 双挂**: 任何标志性 milestone (PROGRESS 列 ⭐ 的) 必须同时给出 4.1 (CI 单测) 和 4.2 (野马签字 + 截屏)。单挂 4.2 = 不可重复 = 烈马打回。

---

## CI 与 seed 契约 (烈马底线)

每个 milestone 在合并前必须挂以下两件:

1. **seed 脚本** `testdata/<milestone>/seed.sql` — 删库后用此脚本构建 acceptance fixture, "删库 + seed + 跑 acceptance" 在 CI 中一键复现。v0 删库重建场景下, 没有 seed = acceptance 一次性。
2. **回归测试入册** — milestone 关闭打 ✅ 时, 该 milestone 的所有 4.1 acceptance 自动进 CI 回归套件。每次 Phase gate 关闭时全部跑一遍, 任意 fail = gate 不通过。

烈马在 PR review 阶段验证这两件存在, 缺即打回。

---

## 阶段差异

| 阶段 | Acceptance 严格度 |
|------|------------------|
| ⚡ v0 | 四选一即可,数据契约最常用 (因为破坏式升级,e2e 改动多) |
| 🛡️ v1 | 推荐 E2E 断言 + 数据契约组合 (双保险); 标志性 milestone 必加 4.2 用户感知签字 |

---

## PR 描述模板

```markdown
## What
<一段话讲做了什么>

## Blueprint: <module> §X.Y
<引用相关蓝图段落, 让 reviewer 知道这条改动锚定的产品立场>

## Touches
<server | client | plugin | helper | docs | testdata — 列出本 PR 触达的子系统>

## Acceptance
<四选一其中一种, 标志性 milestone 4.1 + 4.2 双挂>

## Current 同步
<本 PR 改动的 `docs/current/<module>/` 文件清单。空则填 "N/A — <理由>", 烈马签字>

## Stage: ⚡ v0 / 🛡️ v1
<标记当前阶段,提示 reviewer 用对应严格度审>
```

> **跨模块 PR 协议**: 当 `Touches` 含 ≥ 2 个子系统 (例: server+plugin+client), 必须**一次合并** (monorepo 单 PR), 不允许串行 3 个 PR — v0 BPP 协议变化必须 plugin 同步发版。
> **PR revert 流程**: v0 阶段 main 跑不起来 → 直接 `git revert <sha>` + push, main 走 fast-forward。不写回滚 schema 脚本 (v0 允许删库)。
> **CI lint**: PR 改 `internal/<module>/` 但没改 `docs/current/<module>/` → CI fail。 烈马在 PR review 阶段确认 Current 同步区块填了。

## 模块文档末尾必备: Blueprint 反查表 (闸 3)

每个 implementation/<module>.md 末尾必须有一张反查表, 防止立场漂移:

```markdown
## Blueprint 反查表

| Milestone | Blueprint §X.Y | 立场一句话 |
|-----------|----------------|-----------|
| CM-1 | concept-model §1.1 + §2 | 1 person = 1 org, UI 不暴露; 数据层 org first-class |
| CM-3 | concept-model §2 | 资源归 org, 查询直查 org_id 不走 owner_id JOIN |
| CM-4 | concept-model §1.2 + §5.1 + §5.2 | agent 是同事, 离线 fallback 给 owner, 跨 org 邀请 owner-only |
```

**规则**: 一句话写不出"立场" = 立场漂移 = 该 milestone 打回。这一表的存在比 grep 锚点严格一层。

## 例: CM-1 PR 1 的 acceptance spec

```markdown
## Acceptance (数据契约)

- 启动后 SQLite 中存在表 `organizations(id TEXT PRIMARY KEY, name TEXT NOT NULL, created_at INTEGER NOT NULL)`
- `users` 表存在 `org_id TEXT NOT NULL` 列
- 索引 `idx_users_org_id` 存在
- 启动时所有非 agent user 都有 `users.org_id != ''`
- 启动时所有 agent user 的 `org_id` 等于其 `owner_id` 对应 user 的 `org_id`
```

只看这个 spec, 任何人都能复现验证 (跑迁移 + 查表)。

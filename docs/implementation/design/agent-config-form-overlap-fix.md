# Agent Config Form 排版重叠修 (gh#698)

> Issue: gh#698 "Agent Manage 展开后内部 form 排版烂 (#694 width: 100% 后副作用)"
> Author: zhanma-d (Dev)
> 待 4 签: 飞马 (Architect) / 野马 (PM) / heima (Security) / 烈马 (QA)
> 实施分支: `fix/agent-config-form-layout` (worktree `/workspace/borgee/.worktrees/agent-config-form-overlap-fix`)
> 关联: PR #694 已合 (#683 width: 100% 修) 暴露此 bug; 不是 PR #694 引入

## §0 范围

修 `packages/client/src/components/AgentConfigPanel.tsx` 的 form 排版 — 6 个 `<label>` 在 800px 父容器下文字跟 input 重叠.

不在范围:
- `.agent-page` 容器宽度: PR #694 已修, 不动
- `CreateAgentModal` form (AgentManager.tsx 第 414-424): 现有用内联 `style={{ display: 'block' }}`, 排版好, 不动
- 其它 form (除 AgentConfigPanel 外的所有 modal / 页面 form): 不动

## §1 根本原因 (代码层定位)

`AgentConfigPanel.tsx` 第 110-185 行 form 渲染:

```tsx
<section data-agent-config="root" ...>
  <header><h3>Agent 配置</h3>...</header>

  <label>
    名称
    <input type="text" data-agent-config-field="name" ... />
  </label>
  <label>
    头像 URL
    <input type="text" data-agent-config-field="avatar" ... />
  </label>
  <label>
    Prompt
    <textarea data-agent-config-field="prompt" ... />
  </label>
  <label>
    模型
    <input type="text" data-agent-config-field="model" ... />
  </label>
  <label>
    memory_ref
    <input type="text" data-agent-config-field="memory_ref" ... />
  </label>
  <label>
    启用
    <input type="checkbox" data-agent-config-field="enabled" ... />
  </label>

  <button data-agent-config-action="save">保存</button>
</section>
```

`<label>` 浏览器默认 `display: inline`. 没 wrapper, 没 CSS 类, 没内联 style.

- 修前 `.agent-page` 因 `margin: 0 auto` 在 flex-column cross-axis 缩到内容宽 (334px), 4 个 label 一行排不下被迫 wrap → 视觉勉强可读 (其实 label 文字跟 input 还是同 inline 行, 只是一行短挤不出多个 field)
- 修后 `.agent-page` 拿 800px (PR #694 width: 100%) → 4 个 inline `<label>` 一行排得下, label 文字跟下一个 label 的 input 在同 inline 行 → 视觉重叠

issue body §liema DOM 数据吻合: `4 个 textbox x=407 / 437 / 618 / 682 同一行 y=329 / 352`, 'Agent 配置' / v0 / '名称' / '头像 URL' / 'Prompt' / '模型' / 'memory_ref' / '启用' / '保存' 全挤同一 inline 行.

## §2 反向 grep 对照: 项目内既有 form 排版方式

### 2.1 内联 style (CreateGroupModal / CreateAgentModal)

```tsx
<label style={{ marginTop: 8, display: 'block' }}>
  Agent ID <span style={...}>...</span>
  <input className="input-field" ... />
</label>
```

`display: block` 让 label 占独立行, input 在 label 下方. 改动小, 不引入新 CSS, 跟既有 modal form 一致.

### 2.2 form-actions (button 行)

`packages/client/src/index.css:491` `.form-actions { display: flex; gap: 8px; }` — 仅用于按钮行, 不是 field-level 排版. 现有 form 没有 `.form-group / .form-field` 类 (我反向 grep 验证: 0 hit).

### 2.3 input-field (字面 + admin-modal-content)

`.input-field` 类只用于 input 元素本身样式, 不管 label 排版. `.admin-modal-content label` 有 `display: block` 但只在 admin modal 范围生效.

## §3 文案 / 字段 不变

无 schema 改动, 无 API 改动, 无字段重命名. 此 PR 纯 UI 排版层重构.

`data-agent-config-field` / `data-agent-config-action` / `data-agent-config-version` / `data-agent-config="root"` data-attribute 都保留 byte-identical (e2e + REG-AL2A-* 锚点同源, 不动会 fail).

label-input 关联用 implicit / 嵌套关系 (`<label>名称<input/></label>`), 不引入 `id + htmlFor` 显式关联 — 跟 W3C ARIA Authoring Practices 隐式关联模式一致, 防后续 dev 误以为缺 htmlFor (liema review #4).

**留账 — 现有 al-2a content lock vs 代码 drift (历史, 不在本 PR 范围, yema review 提)**:
- `docs/qa/al-2a-content-lock.md` ① 锁: `<form data-form="agent-config" data-agent-id="{id}">` + 标题 `"Agent 设置"`
- 实际代码 `AgentConfigPanel.tsx`: `<section data-agent-config="root" data-schema-version=...>` + 标题 `"Agent 配置"`
- `al-2a-content-lock.test.ts` 实际锁的是 `data-agent-config="root"` 跟代码一致 (line 62-65), 跟 doc ① 不一致

这层 drift 是历史就有, 不是 #698 修引入. 此 design doc 措辞 "文案 / 字段 不变" / "无文案改动" 仅指相对当前代码状态不变 — 跟 al-2a-content-lock.md ① 写的"理论文案"已经 drift 但我们不动. 真正的 al-2a content lock vs 代码对齐留 followup issue **gh#701** (al-2a 三方 drift: md ↔ test ↔ code 不一致, 要 brainstorm 拍方向 — 改 doc 跟代码对齐, 还是改代码跟 doc 对齐).

## §4 边界条件

| 场景 | 行为 |
|---|---|
| 1280 viewport 容器 800px | 6 个 field 各占独立行, label 在 input 上方, 不重叠 |
| 480px 移动 viewport | 6 个 field 仍各占独立行, input width 100% 自适应窄屏 |
| 用户 type 长 Prompt (textarea 多行) | textarea 自然撑高, 不压到下一个 label |
| Manage 展开切到 Collapse | Manage section unmount + 重 mount, form 重新渲染 (现有行为) |
| 用户切 channel | AgentManager 重新加载 agent 列表, AgentConfigPanel 通过 agentId 重新 fetch (现有行为) |
| 字段空值 / null | input value="", 不影响排版 (现有行为) |

## §5 多方案对比

### 方案 A: 给每个 label 加内联 style (跟 CreateAgentModal 一致)

```tsx
<label style={{ display: 'block', marginTop: 8 }}>
  名称
  <input ... />
</label>
```

6 个 label 都加. 改动: AgentConfigPanel.tsx 文件单处, ≤8 行 (每个 label +1 行 style prop, 加 input/textarea wrapper 调整).

**Pro**:
- 最小改动 (1 文件 + ≤ 8 行)
- 跟 CreateAgentModal 内联 style 模式一致, 视觉统一
- 不引入新 CSS class, 反潜在重命名风险 (按功能不按 milestone 守卫)

**Con**:
- 6 个 label 重复同样 style prop (轻度重复)
- 后续给 form 加新字段, 必须每行手动加 style prop

### 方案 B: 加 CSS class `.agent-config-form` + wrapper

```css
.agent-config-form label {
  display: block;
  margin-top: 8px;
}
.agent-config-form input[type="text"],
.agent-config-form textarea {
  display: block;
  width: 100%;
  /* 跟 .input-field 既有样式对齐 */
}
.agent-config-form input[type="checkbox"] {
  margin-left: 8px;
}
```

```tsx
<section className="agent-config-form" data-agent-config="root" ...>
```

**Pro**:
- 跟项目既有 CSS class 风格一致 (`.modal-content` / `.input-field` / `.form-actions`)
- 后续给 form 加新字段自动继承样式, 不需要每行手动写 style
- CSS 集中管理, 视觉规范化

**Con**:
- 改动稍大 (2 文件: index.css + AgentConfigPanel.tsx, ~12 行)
- 引入新 CSS class (一个 css class)

### 方案 C: 用既有 .admin-modal-content 类

不可行: AgentConfigPanel 不在 modal 内 (是 page-level section), 套 modal 样式语义错.

### 选择: 方案 A (按 team-lead 拍 + scope 最小)

team-lead 在 SendMessage 里建议 "倾向修法 B (复用 `.form-group / .form-field` 既有类, scope 最小, liema 提的方向)". 但反向 grep 验证: `.form-group` / `.form-field` **既有类不存在** (CSS 0 hit, 项目里没人写过). liema 跟 team-lead 提的 "复用既有类" 实际上没类可复用.

跟 team-lead 通报这点的同时, 此 design 选 **方案 A**:

**真原因**:
1. 项目没 `.form-group / .form-field` 既有类, 写新 CSS 类 = 方案 B, 不是 "复用既有"
2. issue p2-normal, 不该引入大改动 (方案 B 多 1 文件 + ~12 行 + 命名空间扩展)
3. 跟 CreateAgentModal **真正的既有 form 实现** 一致 (内联 style block label / Permissions 块 inline checkbox) — 这才是真的"复用既有模式"
4. 方案 A 改动 ≤ 8 行, 单文件, 最小 scope, 一气呵成

**checkbox "启用" field 例外** (yema review #1 拍 b inline 同行):
6 个 label 里 5 个 text/textarea 走 `display: 'block'` (跟 CreateAgentModal Agent ID label 一致), 但第 6 个 checkbox "启用" label 走 `display: 'flex', alignItems: 'center', gap: 8` (跟 CreateAgentModal AgentManager.tsx L430 KNOWN_PERMISSIONS 块 inline checkbox 同款). 真原因:
- HTML/UX 通用规矩: checkbox + label 同行 inline 是标准 (Slack/Discord/GitHub 全这个), stack 反而反规矩
- "启用 ☐" / "启用 ☑" 状态一眼能看, 视觉信息密度高
- 跟 CreateAgentModal Permissions 块 inline checkbox 真"复用既有模式"

文本节点位置: text 在 `<input type="checkbox" />` **后面** (符合 inline 视觉顺序: 先 checkbox 后 label 文字).

**反约束**: 此 design 不引入新 CSS class. 后续若多个 form (≥ 3 个) 都需要相同排版, 再开独立 issue 抽 `.form-stack` / `.field-block` 类 (按功能命名), 不在此 PR 做.

## §6 跟现有代码集成

### 反向 grep 锚 (改前已查):

| Grep | 命中 | 处理 |
|---|---|---|
| `<label>` 在 AgentConfigPanel.tsx | 6 处 (line 118 / 128 / 138 / 147 / 157 / 167) | **5 个 text/textarea label** (118 名称 / 128 头像 URL / 138 Prompt / 147 模型 / 157 memory_ref) 加 `style={{ display: 'block', marginTop: 8 }}`. **1 个 checkbox label** (167 启用) 加 `style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 8 }}` + 把 text 节点 "启用" 移到 `<input type="checkbox" />` 后面 (yema 拍 b inline) |
| `<label style=` 在 AgentManager.tsx CreateAgentModal | 已有 (line 414, 模式参考) | 不动, 仅参考 |
| `data-agent-config-field` | 6 处 | 不动 (e2e + REG-AL2A 锚) |
| `data-agent-config-action="save"` | 1 处 | 不动 |
| `data-agent-config="root"` / `="loading"` | 2 处 | 不动 |
| `data-agent-config-version` / `data-schema-version` | 2 处 | 不动 |
| `.form-group` / `.form-field` 类 | 0 hit (项目无既有类) | 不引入新类 (方案 A) |

### 反向影响:

`AgentConfigPanel.tsx` 单文件改, 不影响:
- 其它 components (AgentManager / ArtifactPanel 等不涉)
- e2e (data-attribute 不动, REG-AL2A-* 锚点保持)
- server (字段 contract 不动)
- AL-2a 既有单元测试 (data-agent-config-field byte-identical, 不破)

### Acceptance / 文案锁:

无文案改动. 6 个 label 文字 byte-identical 保留:
- 名称 / 头像 URL / Prompt / 模型 / memory_ref / 启用 / 保存

不变: AGENT_CONFIG_SAVE_TOAST = 'agent 配置保存失败, 请重试' (跟 al-2a-content-lock.md ① + server agentConfigSaveErrorMsg 同源).

## §7 测试策略

| 层 | 覆盖 |
|---|---|
| **vitest unit** | AgentConfigPanel 现有 unit test 锚 data-attribute, 不依赖 label 视觉. 不需要新加 unit. |
| **e2e (新)** | 新加 1 个 spec `gh-698-agent-config-form-layout.spec.ts` 或加到 al-4-acceptance-followup.spec.ts. 锚点: 1280 viewport / Manage 展开 → 量 6 个 `[data-agent-config-field="..."]` 的 `getBoundingClientRect()`, 断言 6 个 field 各占独立行 (y 不同, label.bottom < input.top). 480px viewport 同样断言. |
| **手工真验** (liema 6 步) | (1) testing 1280 viewport / login / Agents 页 / Manage 展开 → 6 个 field 各占独立行不重叠. (2) 480px mobile viewport / 同样路径 → 6 个 field stack 不溢出. (3) 输入长 Prompt (textarea 撑高) → 不压到下一字段. (4) 切 Collapse + 重新 Manage → 排版稳定. (5) 跨 channel 切 → AgentConfigPanel 重 fetch / 重渲染 → 排版稳定. (6) 反向 grep 验证 AgentConfigPanel.tsx 6 个 `<label>` 全部加了 style. |

## §8 风险

| 风险 | 缓解 |
|---|---|
| 6 个 label 改漏一处 | 改前 grep `<label>` 列出全 6 个行号; 改后 grep `<label style=` 也是 6 个; diff 比对 |
| 后续给 form 加新字段忘加 style → 退回 inline 排版 bug | 在 AgentConfigPanel.tsx 顶部加注释锚: "新加 label 必须 style={{ display: 'block', marginTop: 8 }}, gh#698 修". 反向 grep 守卫: 加一条 vitest 断言 file 内 `<label>` count == `<label style=` count |
| testing 真验环境 1280 viewport 真渲染跟代码层假设不一致 | testing 真验是必经路径 (liema 6 步), 不靠假设 |
| 内联 style 跟 React StrictMode 重渲染冲突 | 静态 style object 不是新对象 (React 优化已经支持), 6 个 label 各自 style 静态值 |

## §9 实施步骤 (4 签后)

1. 改 `packages/client/src/components/AgentConfigPanel.tsx`:
   - 6 个 `<label>` 各自加 `style={{ display: 'block', marginTop: 8 }}`
   - 顶部加注释锚 (gh#698 修守卫)
2. 加 vitest 单元测试 (反向 grep 守卫): file 内 `<label>` count == `<label style=` count
3. 加 e2e spec (新): `cv-1-3` 系列旁边加, 锚点测 1280 + 480 viewport 6 个 field boundingRect
4. 反向 grep 验证: AgentConfigPanel.tsx 6 个 label 全加 style; data-attribute 不动
5. testing 真验路径 (推到 fix/agent-config-form-layout 触发 Deploy Test)
6. commit + push (commit message: 反向 grep 锚 + e2e 新加 + 测试覆盖思路)
7. team-lead 开 PR (Closes gh#698, Blueprint: `blueprint/current/client-shape.md (沉默 — 视觉细节实施层)`)
8. 等 liema 真验 6 步 + feima/yema/heima 三角度 review pass + PR body checklist 全勾 → squash merge

## §10 跟其它在飞 PR 的关联

- PR #694 已合到 main df9425f, 是这个 bug 暴露的源头. 此 PR 不动 PR #694 已合的 `.agent-page` 修.
- PR #695 / #696 / #699 (#682 / #689 / #691) 互不冲突.
- 此 PR 跟 #697 (现有 modal a11y backlog) 无冲突 — #697 是 modal a11y 重构, 此 PR 是 page-level form 排版.

## §11 留账 (followup, 不在本 PR)

| Followup | Issue | 依赖 / 关联 |
|---|---|---|
| al-2a content lock 三方 drift (md ↔ test ↔ code 不一致, form 元素 + 标题字面) | gh#701 | yema review 提, 历史 drift 不是 #698 引入. 要 brainstorm 拍方向 (改 doc 跟代码对齐 / 改代码跟 doc 对齐). |
| AgentConfigPanel UX 友好化 (memory_ref / model / enabled 文案) | gh#702 | yema review 提的产品方向, 不在 #698 排版 bug 修范围. |
| useUnsavedChangesGuard 4 form 推广 (含 AgentConfigPanel dirty guard) | gh#703 | 依赖 PR #695 `useUnsavedChangesGuard` hook 先合到 main; 跟 AgentManager 编辑 / WorkspaceManager / NodeManager 三个 form 一起做 (yema 在 PR #695 review 已留账"4 form 都该接"). 此 PR 不带飞 (yema 拍 ii 不加 + heima 也同意 UX 决定留 PM). 注: PR #695 实际加的是 **CreateAgentModal** 的 unsaved guard, 不是 channel description (yema 反向校正). |

---

**等待 4 签**:
- [ ] 飞马 (Architect): 蓝图 stance + 架构合理性 (方案 A vs B vs C 选择 + 不引入新 CSS class 的反约束)
- [ ] 野马 (PM): 用户体验 (form 排版可读性) + 文案锁 byte-identical (label 6 个 + AGENT_CONFIG_SAVE_TOAST)
- [ ] heima (Security): 无 auth/permission 路径变化, 无新输入字段 / DOM 注入
- [ ] 烈马 (QA): edge case §4 表 1:1 覆盖 + e2e 锚点 (1280 + 480 viewport 6 个 field boundingRect 断言)

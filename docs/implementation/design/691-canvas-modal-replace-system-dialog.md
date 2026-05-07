# 691 — Canvas 用应用内 modal 替代浏览器原生对话框

> Issue: gh#691 "Canvas 弹了系统弹窗"
> Author: zhanma-d (Dev)
> 待 4 签: 飞马 (Architect) / 野马 (PM) / Security / 烈马 (QA)
> 实施分支: `fix/canvas-system-dialog` (worktree `/workspace/borgee/.worktrees/canvas-modal-fix`)

## §0 范围

替换 `packages/client/src/components/ArtifactPanel.tsx` 里两处浏览器原生对话框:

1. **新建 artifact** (line 250) `window.prompt('Artifact 标题:', '未命名 artifact')` — issue #691 提到的"在 channel 对话框选 Canvas → 点新建 Markdown artifact → 弹系统弹窗".
2. **回滚版本** (line 326) `window.confirm('确认回滚到 v${N}? ...')` — 同一文件, 同一类问题. 同 PR 一并修, 不留尾巴.

不在范围 (留待后续 issue):
- `packages/client/src/commands/builtins.ts` 的 `/leave` 和 `/clear` 也用了 `window.confirm`. 跟 Canvas 无关, 是 slash command 入口, 留独立 issue.
- `packages/client/src/components/ArtifactCommentItem.tsx:95` 删 comment 用 `window.confirm`. 跟 Canvas 评论流相关, 但不在 #691 issue 描述范围. 后续可一起处理, 此 PR 不动.

## §1 数据流

```
[空状态] 用户点 ".artifact-empty 新建 Markdown artifact" 按钮
   │
   ├─ handleCreate() — 旧路径: 调 window.prompt 阻塞主线程, 用户输入返 string
   │
   └─ 新路径: setCreateDraftTitle('未命名 artifact')
        │
        ▼
[Modal 打开] CreateArtifactModal 渲染 (modal-overlay + modal-content)
   ├─ 输入框默认值 "未命名 artifact" (跟原 prompt defaultValue byte-identical)
   ├─ "创建" 按钮: 提交 → doCreate(title)
   │      │
   │      ├─ trim 后空 → return (跟原行为一致)
   │      ├─ setCreateDraftTitle(null) 关 modal
   │      ├─ setBusy(true) + POST /api/v1/channels/:id/artifacts {title, body:''}
   │      ├─ 成功 → setArtifact(created) + listVersions
   │      └─ 失败 → setErrMsg(err.message ?? '创建失败')
   ├─ "取消" 按钮: setCreateDraftTitle(null)
   ├─ ✕ 关闭按钮: setCreateDraftTitle(null)
   ├─ Esc 键: setCreateDraftTitle(null)
   └─ 点遮罩: setCreateDraftTitle(null)
        (busy 时禁止关闭, 避免请求飞着 modal 关了状态错乱)

[已有 artifact 视图] 用户点版本列表里某行的 "回滚到此版本" 按钮
   │
   ├─ handleRollback(toVersion) — 旧路径: 调 window.confirm 阻塞
   │
   └─ 新路径: setPendingRollbackVersion(toVersion)
        │
        ▼
[Modal 打开] RollbackConfirmModal 渲染
   ├─ 文案: "确认回滚到 v{N}? 旧版本不会删除, 会新建一条 rollback 记录."
   │   (跟原 confirm 文案 byte-identical)
   ├─ "确认回滚" 按钮 (btn-danger): 提交 → doRollback(toVersion)
   │      ├─ POST /api/v1/artifacts/:id/rollback {to_version}
   │      ├─ 成功 → reload artifact + versions
   │      ├─ 409 → showToast('内容已更新, 请刷新查看') + reload
   │      └─ 其它失败 → setErrMsg
   ├─ "取消" 按钮 / Esc / 点遮罩: setPendingRollbackVersion(null)
```

## §2 数据模型

无 schema 改动. 此 PR 纯 client-side UI 变更. 不动 server, 不动 DB, 不动 API contract.

新增 client local state (在 ArtifactPanel 组件内部):

| 字段 | 类型 | 含义 |
|---|---|---|
| `createDraftTitle` | `string \| null` | `null` = 创建 modal 关; 非 null = modal 打开, 当前输入值 |
| `pendingRollbackVersion` | `number \| null` | `null` = 回滚 modal 关; 非 null = modal 打开, 待确认的目标版本号 |

state 命名遵循"按功能不按 milestone"的规矩 (不写 `m691_*`).

## §3 API contract

无新 API, 无 contract 变更.

复用既有 endpoint:
- `POST /api/v1/channels/:channelId/artifacts` (createArtifact, lib/api.ts) — 已有
- `POST /api/v1/artifacts/:id/rollback` (rollbackArtifact, lib/api.ts) — 已有

请求体 / 响应体 / 错误码 list 都不变, 这是纯 UI 层重构.

## §4 边界条件 + 错误处理

| 场景 | 旧行为 (原生 dialog) | 新行为 (应用内 modal) | 是否回归覆盖 |
|---|---|---|---|
| 用户输入空 title | prompt 返空字符串 → trim 后 falsy → 函数 return, 不创建 | "创建" 按钮在 trim 后为空时 disabled, 提交无效 | ✅ |
| 用户输入只有空白 | trim 后空 → return | 同上, disabled | ✅ |
| 用户输入特殊字符 (`<script>` / 中文 / emoji) | prompt 透传给 createArtifact | input 透传给 createArtifact (不在 client 拦截; server 端字段验证 + 渲染时 DOMPurify) | 跟旧行为一致, 不引入新风险 |
| 用户取消 prompt | prompt 返 null → falsy → return | 点取消 / 关闭 ✕ / Esc / 点遮罩, setState(null), 不创建 | ✅ |
| 创建中重复点提交 | prompt 是模态阻塞主线程, 不会重复点 | "创建" 按钮在 busy 时 disabled; 同时遮罩点击在 busy 时禁用 | ✅ |
| 创建失败 (API 报错) | catch → setErrMsg, modal 已经关了 → 错误显示在空状态分支 | 同上, doCreate 关 modal 后再 try; 失败 setErrMsg, 错误显示在空状态分支 | 行为一致 |
| 回滚 confirm 用户取消 | confirm 返 false → return | 点取消 / 关闭 / Esc / 点遮罩, setState(null), 不回滚 | ✅ |
| 回滚 409 conflict | 旧: showToast + reload; 新: 同 | toast 文案锁 byte-identical "内容已更新, 请刷新查看" | ✅ |
| 回滚成功 | reload | reload | 无变化 |
| 已存在同名 artifact | API 不约束 title 唯一 (蓝图 §1.1 单文档锁是 ID 级, 不是 title 级), 直接创建成功. 旧 prompt 也不查重. | 同, 不引入新检查 | 跟旧行为一致 |
| 用户在 modal 打开时切换 channel | 旧 prompt 阻塞主线程不允许切; 新 modal 不阻塞, 可切 | useEffect on channelId 已 reset diffPair, 没 reset modal state. **edge case 决定**: channel 切换时一并 reset `setCreateDraftTitle(null) + setPendingRollbackVersion(null)`, 防 modal 错挂在新 channel 上 | ✅ 实施补 |

## §5 多方案对比

实现 modal 替代有几个候选:

### 方案 A: 内联子组件, 复用既有 modal CSS 类

- 在 `ArtifactPanel.tsx` 文件末尾加 `CreateArtifactModal` + `RollbackConfirmModal` 两个函数组件
- 复用项目现有 `.modal-overlay / .modal-content / .modal-header / .modal-body / .form-actions` CSS 类 (跟 `CreateGroupModal` / `ConfirmDeleteModal` 同款)
- 不引入新依赖
- 不抽公共组件

**Pro**:
- 最小改动: 只动 1 个 client 文件 + 同款 CSS 类零新增样式
- 跟 `CreateGroupModal` / `ConfirmDeleteModal` 视觉完全一致, 用户体感统一
- 不动现有 modal 组件契约, 零回归风险

**Con**:
- 重复了一些 modal 骨架代码 (modal-overlay + Esc handler + onClick stopPropagation)

### 方案 B: 抽通用 `<PromptModal>` / `<ConfirmModal>` 公共组件

- 在 `packages/client/src/components/` 新加 `PromptModal.tsx` + `ConfirmModal.tsx`
- ArtifactPanel 引入这两个组件
- 同时把 `CreateGroupModal` / `ConfirmDeleteModal` / `KickConfirmModal` 等也迁移过去

**Pro**:
- 减少重复 modal 骨架
- 后续新加确认 / 输入弹窗都能复用

**Con**:
- 范围远超 #691 issue 描述 (issue 只说 Canvas 弹系统弹窗, 不要求重构所有 modal)
- 涉及多文件迁移, 跟一 milestone 一 PR 的规矩冲突, 应单独开 refactor issue
- 增加 PR review 复杂度, 跟 p2-normal 优先级不匹配

### 方案 C: 引入第三方 modal 库 (e.g. radix-ui, react-modal)

- npm 装库, 用其 Dialog primitive

**Pro**:
- 自带 a11y (focus trap / aria-* / portal)

**Con**:
- 引入 prod 依赖, 跟蓝图 §1.6 "Markdown ONLY v1, 不引第三方 UI 库" 倾向冲突 (Canvas 蓝图 vision)
- 现有 modal 都不用第三方库, 单独引入会让风格断层
- bundle 体积增加 (radix Dialog ~10KB gzipped)

### 选择: 方案 A

**真原因**:
1. Issue p2-normal, 不该引入大重构 (排除 B)
2. 项目已有 modal 视觉约定 + CSS 类, 复用最稳 (排除 C)
3. 重复 modal 骨架代码 ~30 行, 跟"按需重构, 不预早抽象"一致. 等真有第 3-4 个 inline modal 需求时再考虑抽 PromptModal 公共组件 (后续 issue)
4. 一 milestone 一 PR + p2-normal 配方案 A: 改动 ≤ 200 行, 单文件, 4 个 e2e helper 适配, 一次合干净

## §6 跟现有代码集成

### 反向 grep 锚 (改前已查):

| Grep | 命中 | 处理 |
|---|---|---|
| `window\.prompt` 在 `packages/client/src` | 仅 `ArtifactPanel.tsx:250` | 此 PR 改 |
| `window\.confirm` 在 `packages/client/src` | `ArtifactPanel.tsx:326` + `commands/builtins.ts:30,108` + `ArtifactCommentItem.tsx:95` | 此 PR 改 ArtifactPanel 一处; 其它留独立 issue |
| `window\.alert` 在 `packages/client/src` | 0 直接命中, 但 `alert(...)` 裸调用还有 (`AgentManager.tsx` etc) | 不在 #691 范围, 留 |
| `.modal-overlay` / `.modal-content` 用法 | `CreateGroupModal` / `ConfirmDeleteModal` / `KickConfirmModal` / `EditHistoryModal` 等 10+ | 复用同款样式, 视觉一致 |
| `data-testid="artifact-create-modal"` | 0 (新加) | 给 e2e 锚定 |
| `data-testid="artifact-rollback-confirm-modal"` | 0 (新加) | 给 e2e 锚定 |

### 反向影响:

`createArtifactViaUI` helper 在 4 个 e2e spec 各自重复实现, 都用 `page.once('dialog', d => d.accept(title))`. 修后这条 listen 不再触发, 必须改成填 modal input + 点确认. 4 个文件:

- `packages/e2e/tests/cv-1-3-canvas.spec.ts`
- `packages/e2e/tests/cv-2-3-anchor-client.spec.ts`
- `packages/e2e/tests/cv-3-3-renderers.spec.ts`
- `packages/e2e/tests/cv-4-iterate.spec.ts`

(可能还有 `cv-4-unfixme-followup.spec.ts`, 实施时 grep 全 e2e 目录确认)

修法: 每个 helper 改成
```pseudo
page.on('dialog', d => throw '原生 dialog 触发了, 应该用应用内 modal')  // 守卫, 防回归
click '.artifact-empty button.btn-primary'
等 [data-testid="artifact-create-modal"] visible
fill modal input
click modal "创建" 按钮
等 .artifact-version-tag = 'v1'
```

类似处理 rollback (`cv-1-3-canvas.spec.ts` 用了 `ownerPage.once('dialog', d => d.accept())` 在 rollback 路径).

### Acceptance / 文案锁:

- 创建 modal 标题文案: "新建 Markdown artifact" (跟空状态按钮文字一致)
- 输入框 label: "Artifact 标题:" (跟原 prompt 第一参数 byte-identical)
- 回滚 modal 文案: "确认回滚到 v{N}? 旧版本不会删除, 会新建一条 rollback 记录." (跟原 confirm 文字 byte-identical)
- 回滚 409 toast: "内容已更新, 请刷新查看" (现有 CONFLICT_TOAST 常量, 不动)

### 类型 / TS 注意:

`React.FormEvent` 在 ArtifactPanel.tsx 顶部 import — 项目用 `jsx: react-jsx` automatic runtime, 没有 React 命名空间, 需要 `import type { FormEvent } from 'react'`.

## §7 测试策略

| 层 | 覆盖 |
|---|---|
| **e2e (新)** | 4 个现有 cv-*-canvas spec 的 `createArtifactViaUI` helper 改成 modal 流程, 都加 `page.on('dialog', throw)` 守卫. cv-1-3 rollback 流程改 modal click. 这些已有 spec 跑过即覆盖 #691 修复. 不再额外加单独 `gh-691-canvas-modal.spec.ts`, 避免重复. |
| **e2e 守卫** | `page.on('dialog', d => throw)` 在每个 helper 入口加 — 任何代码路径意外触发 native dialog 整个测试 fail, 防未来回归 |
| **vitest unit** | `ArtifactPanel.tsx` 现有 unit test 不依赖 prompt/confirm (是 e2e 才走 dialog), 不需要新加 |
| **手工真验** | 推 testing 环境 → liema 浏览器: (1) 进 channel → Canvas tab → 点新建 → 看 modal 弹出而非 native prompt. (2) 编辑 + 提交一次 → 拿 v2 → 点 v1 行的 "回滚到此版本" → 看 modal 弹出而非 native confirm. (3) Esc / 点遮罩 / 取消按钮 / ✕ 按钮都能关 modal. |

## §8 风险

| 风险 | 缓解 |
|---|---|
| 4 个 e2e helper 改漏一处 → 那个 spec 卡住等 dialog 永远不来 | grep `page.once('dialog'` + `page.on('dialog'` 锚定, 全删/全改; 加 throw 守卫确保任何遗漏路径都立刻报错 |
| 同一 worktree 现有改动可能 commit 时漏掉某文件 | 实施时 `git status` 在 commit 前 list 所有 M 文件, 跟 §6 反向影响清单对账 |
| modal 在 channel 切换时残留 | useEffect on channelId reset 两个 modal state |
| 回滚 modal 提交后异步路径中用户切 channel | doRollback 已经先 setState(null) 关了 modal, 异步请求只更新 setBusy + setErrMsg, channel 切了之后 reload 也会拉新 channel 数据, 不会污染 |

## §9 实施步骤 (4 签后)

1. 改 `packages/client/src/components/ArtifactPanel.tsx`:
   - 加 useState (`createDraftTitle`, `pendingRollbackVersion`)
   - `handleCreate` / `doCreate` 拆
   - `handleRollback` / `doRollback` 拆
   - 加 `useEffect` channel 切换 reset modal state (§8 风险 #3)
   - 文件末尾加 `CreateArtifactModal` + `RollbackConfirmModal` 子组件
   - import 加 `FormEvent`
2. 改 4 (或 5) 个 e2e spec 的 `createArtifactViaUI` helper + cv-1-3 rollback 流程
3. `git status` 对账 §6 反向影响清单, 全在
4. commit + push
5. team-lead 开 PR (Closes gh#691, Blueprint: blueprint/current/canvas-vision.md 沉默 — UI 细节实施层)
6. 触发 Deploy Test → liema 真验证 (§7 手工真验三步)
7. 飞马 / 烈马 / 野马 三角度 review pass → squash merge

---

**等待 4 签**:
- [ ] 飞马 (Architect): 蓝图 stance 不漂 (蓝图 §1.6 不引第三方 UI 库 一致) + 架构合理性 (方案 A 复用既有 modal 风格, 不预早抽象)
- [ ] 野马 (PM): 用户体验 (modal 比 native prompt 视觉一致) + 文案锁 (新建/回滚字面 byte-identical)
- [ ] Security: 待用户决定是否 spawn 真 Security 角色后审 (当前 Borgee team 缺), 不让 yema/feima 兼看
- [ ] 烈马 (QA): edge case §4 表 1:1 覆盖 + e2e 守卫 §6 防回归

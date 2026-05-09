# useUnsavedChangesGuard hook (gh#682 + gh#703) — implementation note

> gh#682 (PR #695) sidepane mainView 状态机切换时 + gh#703 (PR #708 + #709) 推广到 5 form + beforeunload listener.
> 蓝图: `client-shape.md` § form 状态保护 (反"切换就静默丢"的 UX bug).

## 1. 设计

切换 sidepane 或刷 / 关 tab 时, 当前视图如有未保存的表单改动, 弹一次 confirmation 让用户决定要不要丢. 是产品方向 (反"切换就静默丢"). 单 hook 5 form 自动获益.

反约束:
- ① 不挂自定义 modal — sidepane 切换走 `window.confirm`, beforeunload 走浏览器原生提示 (跟 CV-10 ArtifactCommentDraftInput 既有 beforeunload 设计 ② 一致)
- ② 反 React 闭包 staleness — `useRef` 包 isDirty (PR #695 feima review 抓的 bug, 反 module-level Set 存 mount 那一刻闭包永远拿空 state)
- ③ 反复制 hook 行为漂移 — 5 form 不各自写 beforeunload, hook 内统一加 (反 5 个 form 各写一份 listener 行为漂)
- ④ 多守卫合并消息一次弹 — 同时 N form dirty 不弹 N 次, 拼一条消息

## 2. API surface (`packages/client/src/hooks/useUnsavedChangesGuard.ts`)

3 export + 1 internal helper + React hook:

| Export | 签名 | 行为 |
|---|---|---|
| `registerUnsavedGuard(isDirty, message)` | `(() => boolean, string) => () => void` | 注册一个守卫, 返反注册函数. 直接调 (测试用) 或走 hook (产品代码用). |
| `runUnsavedGuards(confirmFn?)` | `((string) => boolean) => boolean` | 跑所有注册的守卫, 任一 dirty 就走 confirmFn. 返 true=可继续切换 / false=停在原视图. 默认走 `window.confirm`, 测试可注入. |
| `useUnsavedChangesGuard(isDirty, message?)` | `(() => boolean, string?) => void` | React hook. mount 期间注册, unmount 自动清, 加 beforeunload listener. |
| `_clearUnsavedGuardsForTest()` | `() => void` | test-only 清 module-level Set. 不从 barrel 导出. |

## 3. 数据流

```
mount → useUnsavedChangesGuard(() => isDirty, message)
      → useRef 装 isDirty (每次 render 写最新, 反闭包 staleness)
      → useEffect 注册 ref-wrapper 进 module-level guards Set
      → useEffect 注册 beforeunload listener
      ↓
sidepane 切换 → App.tsx::requestMainView() 调 runUnsavedGuards()
      → 跑 guards Set, 任一 dirty 调 window.confirm(merged message)
      → 用户选 OK/Cancel → 决定继续切换 / 停在原视图
      ↓
关 tab / 刷 / ctrl+W → window 触发 beforeunload event
      → handler 检 isDirtyRef.current() → preventDefault + returnValue=''
      → 浏览器弹原生提示 (现代浏览器忽略 message 字面)
      ↓
unmount → 反注册 guards.delete(guard) + removeEventListener
```

## 4. closure staleness 修法 (PR #695 feima review)

调用方常这么写:

```ts
useUnsavedChangesGuard(
  () => createdKey === null && (displayName.trim() !== '' || agentId.trim() !== ''),
  '...'
);
```

箭头函数闭包绑定 mount 那一刻的 `createdKey` / `displayName` / `agentId` (都是空 / null). 早期实现把这个闭包直接存进 module-level Set, 用户填表单 React 重新 render 但 hook 里 isDirty 没变 — Set 里那条永远跑旧闭包返 false. 守卫对外看着注册了, 实际不工作.

修法: `useRef` 装 *引用*, 每次 render 把最新 isDirty 写进 `isDirtyRef.current`, useEffect 只在 mount 时把"调 ref" 注册进 Set. 这样 Set 里那条永远调当前 render 的 isDirty, 拿到当前 React state.

## 5. 5 form 真量 + dirty 推算两模式

`packages/client/src/components/` grep 检查 `useUnsavedChangesGuard` = 5 处:

| Form | dirty 推算 | 模式 |
|---|---|---|
| AgentManager.tsx (CreateAgent) | `createdKey === null && (displayName.trim() !== '' \|\| agentId.trim() !== '')` | 创建 form: 字段非空 |
| AgentConfigPanel.tsx | `!loading && !saving && config !== null && JSON.stringify(draft) !== JSON.stringify(config.blob)` | 编辑 form: SSOT blob byte 比 |
| DescriptionEditor.tsx | `!busy && value !== initial` | 编辑 form: string === |
| NodeManager::CreateNodeForm | `name.trim() !== ''` | 创建 form: 1 字段非空 |
| NodeManager::AddBinding | `showAddBinding && (bindChannelId !== '' \|\| bindPath.trim() !== '' \|\| bindLabel.trim() !== '')` | 创建 form: 3 字段任一非空 (showAddBinding 关时不算) |

**编辑 form** 走 `JSON.stringify(draft) === JSON.stringify(initial)` byte 比 (有 baseline 跟服务器比); **创建 form** 走字段非空 trim 比 (没 baseline). 字段类型决定推算方式, 不算不一致.

## 6. beforeunload listener (PR #709)

hook 内统一加 (5 form 自动获益, 反每 form 各自写漂):

```ts
useEffect(() => {
  const handler = (e: BeforeUnloadEvent) => {
    if (!isDirtyRef.current()) return;  // 早返回反污染浏览器 unload 链路
    e.preventDefault();
    e.returnValue = '';  // 触发原生提示, 现代浏览器忽略 message 字面
  };
  window.addEventListener('beforeunload', handler);
  return () => window.removeEventListener('beforeunload', handler);
}, []);  // 空 deps OK — handler 关 ref 不 staleness
```

跟 CV-10 ArtifactCommentDraftInput 自己写的 beforeunload 共存 — 都 preventDefault 是幂等, 浏览器只弹一次原生提示. CV-10 迁 hook 留 followup.

## 7. 反约束 (硬性)

源码层:
- `packages/client/src/components/` grep 检查 `beforeunload` 命中 1 处 (CV-10 ArtifactCommentDraftInput) — 5 form 自己不写 (复用 hook), 反复制行为漂移
- `packages/client/src/hooks/useUnsavedChangesGuard.ts` grep 检查 `beforeunload` 命中 1 处 (统一 listener)
- 不引第三方 modal 库 (react-modal / sweetalert) — 走 `window.confirm` 原生 + 浏览器 beforeunload 原生

## 8. 测试

- `packages/client/src/__tests__/`:
  - `AgentConfigPanel-dirty-guard.test.tsx` 4 case (loading 不算 / 改字段算 / 来回切 / saving 不算)
  - `DescriptionEditor-dirty-guard.test.tsx` 4 case (mount 不算 / 改 textarea 算 / 改回原值不算 / busy 不算)
  - `CreateNodeForm-dirty-guard.test.tsx` 4 case (mount 不算 / 输入 name 算 / 空格 only 不算 / unmount 反注册)
  - `AddBinding-dirty-guard.test.tsx` 4 case (showAddBinding=false 不算 / 全空不算 / bindPath 非空算 / bindLabel 非空算)
  - `useUnsavedChangesGuard-beforeunload.test.tsx` 3 case (干净不 preventDefault / dirty 调 preventDefault / unmount 反注册)
- React 18 受控 input/textarea 测试用 native value setter (反 ta.value=X 不通过 React 属性 setter 不触发 onChange)

## 9. 锚

- 蓝图: `client-shape.md` § form 状态保护
- spec: 无单独 spec 文件 (gh#682 + gh#703 直接 PR)
- PR 串: #695 (sidepane 状态机 + hook 引入) + #708 (PR-1/2 推广 2 编辑 form) + #709 (PR-2/2 推广 NodeManager 双创建 form + hook 加 beforeunload)

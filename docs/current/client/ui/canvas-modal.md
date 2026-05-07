# Canvas Modal (gh#691) — implementation note

> gh#691 (PR #699) — 替换 canvas 路径上的 `window.confirm` / `window.prompt` 系统弹窗.
> 蓝图: `canvas-vision.md` §1.1 (canvas 是 artifact 工作面) + §1.6 (canvas modal a11y).

## 1. 立场

canvas 上的"删除 artifact?"/"输入名字?"等用户决定**不**走 `window.confirm` / `window.prompt` 浏览器原生弹窗 (UX 跟 borgee 整体设计风格不一致, 移动端 web view 弹窗样式难看, 不可定制). 改成应用内 `InlineConfirmModal`.

反约束:
- ① canvas 路径反向 grep `window.confirm` / `window.prompt` 命中 0 (反系统弹窗回潮)
- ② 不引第三方 modal 库 (react-modal / radix-ui) — 走自家 InlineConfirmModal 跟 borgee 整体视觉一致
- ③ a11y 完整 (role=dialog + aria-modal + aria-labelledby + autoFocus + focus return)
- ④ mobile / IME 守卫 (form onSubmit + onKeyDown isComposing 反中文输入法 Enter 误触)

## 2. Component (`packages/client/src/components/InlineConfirmModal.tsx`)

```tsx
<InlineConfirmModal
  open={boolean}
  title="删除 artifact?"
  message="此操作不可撤销"
  confirmLabel="删除"  // 默认 "确认"
  cancelLabel="取消"
  variant="danger"     // 默认 "primary"
  onConfirm={() => { ... }}
  onCancel={() => { ... }}
/>
```

a11y 锚:
- `role="dialog"`
- `aria-modal="true"`
- `aria-labelledby={titleId}` — 关到 `<h3 id={titleId}>` title 节点
- 打开时 `autoFocus` 落在主按钮 (confirm)
- 关闭时 focus 返回触发按钮 (`useEffect` 存 `previouslyFocused = document.activeElement`)
- ESC 关闭 走 `onCancel`
- 反 click outside 关闭 (反误关丢用户输入)

## 3. Inline prompt 模式 (替 window.prompt)

输入名字 / 描述类弹窗走 `InlineInputModal` (同源, 多一个 input field):

```tsx
<InlineInputModal
  open={boolean}
  title="重命名 artifact"
  inputLabel="新名字"
  initialValue={current}
  onSubmit={(value) => { ... }}
  onCancel={() => { ... }}
/>
```

IME 守卫 (中文输入法 Enter 选词时不应触发提交):
- `<form onSubmit={handleSubmit}>` 包裹 — 反 button onClick 直接调 (Enter 在 input 触发 form submit, 但 IME composition 期间浏览器 swallow Enter, 不传到 submit 路径)
- `onKeyDown={(e) => { if (e.key === 'Enter' && !e.nativeEvent.isComposing) handleSubmit(); }}` 兜底 (反某些浏览器/IME 不 swallow Enter)

## 4. Mobile 守卫

mobile 路径 (`<480px viewport`):
- modal 全屏 (`position: fixed; inset: 0`)
- 输入字段自动 focus (反 mobile 软键盘不弹)
- 反 sidebar overlay 撞 modal — modal `z-index: 1000` 高于 sidebar overlay 100

## 5. 替换路径真量

`packages/client/src/components/CanvasView.tsx` + `ArtifactDrawer.tsx` 等 canvas 路径:

| 之前 | 之后 |
|---|---|
| `if (window.confirm('删除?')) { ... }` | `<InlineConfirmModal variant="danger" .../>` |
| `const name = window.prompt('新名字'); if (name) { ... }` | `<InlineInputModal .../>` |
| `alert('保存失败')` | `showToast('保存失败')` (跟 #710 / #708 复用 Toast 同 stack) |

反向 grep:
- canvas 路径 `window.confirm` / `window.prompt` 命中 0
- canvas 路径 `alert(` 命中 0 (走 Toast)
- `<InlineConfirmModal` / `<InlineInputModal` 命中 ≥1 (替换真有)

## 6. e2e 守卫 pattern (#691 design v2 升级)

之前用 Playwright `page.on('dialog')` listener throw 抓系统弹窗 — 但 throw 在 listener 是异步 unhandled rejection, 不 fail 步, 测试假绿. 改 flag-based:

```ts
let dialogTriggered = false;
page.on('dialog', async (dialog) => {
  dialogTriggered = true;
  await dialog.dismiss();  // 防卡住
});
// ... 触发动作 ...
expect(dialogTriggered, '系统弹窗不应触发 — 应走 InlineConfirmModal').toBe(false);
```

cv-1-3-canvas-modal-a11y / cv-2-3-anchor-client / cv-3-3-deferred / cv-4-iterate / cv-4-unfixme-followup 5 spec 全部转 flag-based.

## 7. 测试

- `packages/client/src/__tests__/InlineConfirmModal.test.tsx` (≥6 case): 渲染 / a11y attrs / autoFocus / ESC 关 / focus return / variant=danger 红色按钮
- `packages/client/src/__tests__/InlineInputModal.test.tsx`: + IME composition Enter swallow + onSubmit 调用
- `packages/e2e/tests/cv-1-3-canvas-modal-a11y.spec.ts`: 真 UI input/click + flag-based dialog listener

## 8. 锚

- 蓝图: `canvas-vision.md` §1.1 (canvas artifact 工作面) + §1.6 (a11y)
- design: `docs/implementation/design/691-canvas-modal-replace-system-dialogs.md`
- PR: #699 (Closes gh#691)

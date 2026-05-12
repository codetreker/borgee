# Canvas Modal (gh#691) — implementation note

> gh#691 — 替换 canvas 路径上的 `window.confirm` / `window.prompt` 系统弹窗.
> 蓝图: `canvas-vision.md` §1.1 (canvas 是 artifact 工作面) + §1.6 (canvas modal accessibility).

## 1. 设计

canvas 上的"删除 artifact?"/"输入名字?"等用户决定**不**走 `window.confirm` / `window.prompt` 浏览器原生弹窗. Browser-native dialogs do not match the Borgee UI, are not customizable, and render inconsistently in mobile web views. These flows use the in-app `InlineConfirmModal` instead.

Negative constraints:

- ① canvas code path grep for `window.confirm` / `window.prompt` returns 0 hits (prevents browser-native dialogs from being reintroduced)
- ② no third-party modal library (react-modal / radix-ui); use `InlineConfirmModal` so the UI remains consistent
- ③ complete accessibility behavior (role=dialog + aria-modal + aria-labelledby + autoFocus + focus return)
- ④ mobile / IME guards (form onSubmit + onKeyDown isComposing prevents accidental submit from Chinese IME Enter)

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

accessibility 出处:

- `role="dialog"`
- `aria-modal="true"`
- `aria-labelledby={titleId}` — 关到 `<h3 id={titleId}>` title 节点
- 打开时 `autoFocus` 落在主按钮 (confirm)
- 关闭时 focus 返回触发按钮 (`useEffect` 存 `previouslyFocused = document.activeElement`)
- ESC closes through `onCancel`
- outside click does not close the modal, preventing accidental loss of user input

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

IME submission guard (Chinese IME Enter selects text and should not submit):

- `<form onSubmit={handleSubmit}>` wrapper: Enter in the input triggers form submit, while browsers usually suppress Enter during IME composition.
- `onKeyDown={(e) => { if (e.key === 'Enter' && !e.nativeEvent.isComposing) handleSubmit(); }}` is the fallback for browsers/IME combinations that do not suppress Enter.

## 4. Mobile checks

mobile path (`<480px viewport`):

- modal 全屏 (`position: fixed; inset: 0`)
- input field auto-focuses so the mobile keyboard opens
- modal `z-index: 1000` stays above sidebar overlay `z-index: 100`

## 5. Replacement coverage

`packages/client/src/components/CanvasView.tsx` + `ArtifactDrawer.tsx` 等 canvas 路径:

| 之前                                                      | 之后                                                         |
| --------------------------------------------------------- | ------------------------------------------------------------ |
| `if (window.confirm('删除?')) { ... }`                    | `<InlineConfirmModal variant="danger" .../>`                 |
| `const name = window.prompt('新名字'); if (name) { ... }` | `<InlineInputModal .../>`                                    |
| `alert('保存失败')`                                       | `showToast('保存失败')` (跟 #710 / #708 复用 Toast 同 stack) |

grep 检查:

- canvas 路径 `window.confirm` / `window.prompt` 命中 0
- canvas 路径 `alert(` 命中 0 (走 Toast)
- `<InlineConfirmModal` / `<InlineInputModal` 命中 ≥1 (替换真有)

## 6. e2e dialog detection pattern (#691 design v2 upgrade)

Earlier tests used a Playwright `page.on('dialog')` listener that threw when a browser-native dialog appeared. That throw became an asynchronous unhandled rejection and did not fail the test step reliably. The check now uses a flag-based pattern:

<!-- prettier-ignore -->
```ts
let dialogTriggered = false;
page.on('dialog', async (dialog) => {
  dialogTriggered = true;
  await dialog.dismiss();  // 防卡住
});
// ... 触发动作 ...
expect(dialogTriggered, '系统弹窗不应触发 — 应走 InlineConfirmModal').toBe(false);
```

canvas-modal-accessibility / comment-anchor-scroll / artifact-renderer-types / cv-4-iterate / artifact-iterate-edge-cases 5 spec 全部转 flag-based.

## 7. 测试

- `packages/client/src/__tests__/InlineConfirmModal.test.tsx` (≥6 case): 渲染 / accessibility attrs / autoFocus / ESC 关 / focus return / variant=danger 红色按钮
- `packages/client/src/__tests__/InlineInputModal.test.tsx`: + IME composition Enter swallow + onSubmit 调用
- `packages/e2e/tests/canvas-modal-accessibility.spec.ts`: 真 UI input/click + flag-based dialog listener

## 8. 出处

- 蓝图: `canvas-vision.md` §1.1 (canvas artifact 工作面) + §1.6 (accessibility)
- design: `docs/implementation/design/691-canvas-modal-replace-system-dialogs.md`

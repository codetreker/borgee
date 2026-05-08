# CHN-10 content lock — DescriptionEditor + ChannelHeader (战马D v0)

> 战马D · 2026-04-30 · client SPA DescriptionEditor.tsx + ChannelHeader
> 描述行 文案 byte-identical 锁.

## §1 DescriptionEditor DOM (byte-identical)

```tsx
<div
  className="description-editor"
  data-testid="description-editor"
  role="dialog"
>
  <header className="description-editor-header">
    <h3>频道说明</h3>
  </header>
  <textarea
    className="description-editor-textarea"
    data-testid="description-editor-input"
    value={value}
    onChange={(e) => setValue(e.target.value)}
    maxLength={500}
  />
  <span className="description-editor-counter">{value.length}/500</span>
  <div className="description-editor-actions">
    <button
      type="button"
      data-testid="description-save"
      onClick={handleSave}
    >
      保存
    </button>
    <button
      type="button"
      data-testid="description-cancel"
      onClick={onCancel}
    >
      取消
    </button>
  </div>
</div>
```

**字面锁** (vitest 反向 grep 守):
- title `频道说明` 4 字 byte-identical
- save `保存` 2 字 byte-identical
- cancel `取消` 2 字 byte-identical
- counter `{n}/500` 字面
- `data-testid="description-editor"` byte-identical
- `data-testid="description-editor-input"` byte-identical
- maxLength=500 (跟 channels.topic GORM size:500 byte-identical 跟 server
  长度上限同源)

## §2 ChannelHeader 描述行 DOM

```tsx
{description && description.trim() !== '' && (
  <div
    className="channel-header-description"
    data-testid="channel-header-description"
  >
    {description}
    <button
      type="button"
      className="description-edit-trigger"
      data-testid="description-edit-trigger"
      onClick={onEdit}
    >
      编辑
    </button>
  </div>
)}
```

**字面锁**:
- edit trigger `编辑` 2 字 byte-identical
- 空 description (空串 / null) → 整个描述行不渲染 (return null)
- `data-testid="channel-header-description"` byte-identical

## §3 反约束 — 同义词 reject

DescriptionEditor + ChannelHeader 任何 description UI 字面反向 reject:
- `topic` (English) — 反 reject (data-testid 例外, 跟 user-visible 拆死)
- `about` — 反 reject
- `intro` — 反 reject
- `description` (English) — 反 reject (data-testid + className 例外)
- `简介` (Chinese intro) — 反 reject (我们用 `频道说明`)
- `主题` (Chinese topic) — 反 reject (跟 server-side topic 列名拆死)
- `关于` — 反 reject
- `介绍` — 反 reject

## §4 toast 文案 byte-identical

| 触发 | toast 文案 |
|---|---|
| 保存成功 | (无 toast — UI 关闭 modal 即视觉反馈) |
| 保存失败 | `保存频道说明失败` byte-identical (操作 + 失败 拼接) |
| 长度超限 | `频道说明不能超过 500 字符` byte-identical |
| 取消 | (无 toast) |

## §5 const 单源

- DESCRIPTION_MAX_LENGTH = 500 (client + server byte-identical 跟
  channels.topic GORM size:500 同源, 反向锁 — 改一处 = 改两处).

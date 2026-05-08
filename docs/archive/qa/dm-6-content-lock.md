# DM-6 content lock — DMThread UI 文案 + DOM (战马D v0)

> 战马D · 2026-04-30 · client SPA DMThread.tsx 折叠 toggle + reply input
> 文案 byte-identical 锁. **关联**: spec `dm-6-spec.md` + stance + acceptance.

## §1 DMThread DOM (byte-identical)

```tsx
{replies.length > 0 && (
  <button
    className="dm-thread-toggle"
    data-testid="dm6-thread-toggle"
    onClick={() => setExpanded(!expanded)}
  >
    {expanded
      ? `▼ 隐藏 ${replies.length} 条回复`
      : `▶ 显示 ${replies.length} 条回复`}
  </button>
)}

{expanded && (
  <ul className="dm-thread-replies">
    {replies.map(r => (
      <li key={r.id} className="dm-thread-reply" data-reply-id={r.id}>
        <span className="reply-sender">{r.sender_name}</span>
        <span className="reply-content">{r.content}</span>
      </li>
    ))}
  </ul>
)}

{expanded && (
  <form
    className="dm-thread-reply-form"
    onSubmit={handleSubmit}
  >
    <textarea
      className="dm-thread-reply-input"
      data-testid="dm6-reply-input"
      placeholder="回复..."
      value={draft}
      onChange={e => setDraft(e.target.value)}
    />
    <button
      type="submit"
      data-testid="dm6-reply-submit"
      disabled={!draft.trim()}
    >发送</button>
  </form>
)}
```

**字面锁** (vitest 反向 grep 守):
- 展开态 button 文案 `▼ 隐藏 N 条回复` byte-identical (▼ + 隐藏 + 数字 + 条回复)
- 折叠态 button 文案 `▶ 显示 N 条回复` byte-identical (▶ + 显示)
- placeholder `回复...` 2 字 + 三点 byte-identical
- submit button 文案 `发送` 2 字 byte-identical
- `data-testid` ∈ {`dm6-thread-toggle`, `dm6-reply-input`, `dm6-reply-submit`}
- 空 thread (replies.length===0) 整个 toggle + form 不渲染 (return null
  分支)

## §2 反约束 — 同义词 reject

DMThread + 任何 DM thread reply UI 字面反向 reject:
- `reply` (English) — 反 reject (data-testid 字面除外, 跟 user-visible
  text 拆死)
- `comment` — 反 reject
- `discussion` — 反 reject
- `discuss` — 反 reject
- `讨论` — 反 reject
- `评论` — 反 reject
- `评论区` — 反 reject
- `回复区` — 反 reject (我们用 `N 条回复` 不用 `回复区`)
- `跟帖` — 反 reject

**注**: `回复` 2 字 byte-identical 是 user-visible text (placeholder +
button 文案), 跟 `评论/讨论` 等同义词拆死. vitest 反向 grep 仅 reject
非 `回复` 形态.

## §3 toast 文案 byte-identical

| 触发 | toast 文案 |
|---|---|
| reply 发送成功 | (无 toast — UI thread 渲染新 reply 即视觉反馈) |
| reply 发送失败 | `回复发送失败` byte-identical (跟 mute `静音失败` / archive `归档失败` 同模式 — 操作 + 失败 拼接) |
| thread 折叠/展开 | (无 toast — UI 状态变化即反馈) |

## §4 反约束 — thread depth 1 层

DMThread.tsx 反向断言 thread depth 1 层强制:
- reply 行 (`<li className="dm-thread-reply">`) 内**不**渲染 sub-thread
  toggle (反向 grep `dm-thread-reply.*dm-thread-toggle` 0 hit)
- reply 行不接受 reply_to_id 嵌套 (server 既有 validation 已守 thread
  depth, client 仅 UI 反约束)

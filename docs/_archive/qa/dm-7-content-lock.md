# DM-7 content lock — EditHistoryModal 文案 + DOM (战马D v0)

> 战马D · 2026-04-30 · client SPA EditHistoryModal.tsx 文案 byte-identical
> 锁. **关联**: spec `dm-7-spec.md` + stance + acceptance.

## §1 EditHistoryModal DOM (byte-identical)

```tsx
{history.length > 0 && (
  <div
    className="edit-history-modal"
    data-testid="edit-history-modal"
    role="dialog"
  >
    <header className="edit-history-header">
      <h3>编辑历史</h3>
      <span className="edit-history-count">共 {history.length} 次编辑</span>
    </header>
    <ul className="edit-history-list">
      {history.map((entry, i) => (
        <li
          key={i}
          className="edit-history-entry"
          data-history-index={i}
        >
          <time
            dateTime={new Date(entry.ts).toISOString()}
            className="edit-history-ts"
          >
            {new Date(entry.ts).toISOString()}
          </time>
          <pre className="edit-history-old-content">{entry.old_content}</pre>
        </li>
      ))}
    </ul>
  </div>
)}
```

**字面锁** (vitest 反向 grep 守):
- title `编辑历史` 4 字 byte-identical
- count `共 N 次编辑` 5 字 byte-identical (N 计数动态)
- `data-testid="edit-history-modal"` byte-identical
- 时间戳 RFC3339 (ISO string) byte-identical 跟 CHN-1.2 archive system DM
  时间戳 同源
- `data-history-index` 行级 byte-identical
- 空 history (length === 0) — 整个 modal 不渲染 (return null)

## §2 反约束 — 同义词 reject

EditHistoryModal + 任何 edit history UI 字面反向 reject:
- `history` (English) — 反 reject (data-testid 例外, 跟 user-visible
  text 拆死)
- `changes` — 反 reject
- `revisions` — 反 reject
- `revs` — 反 reject
- `版本` (Chinese version) — 反 reject (我们用 `编辑`)
- `修订` (Chinese revision) — 反 reject
- `变更` (Chinese change) — 反 reject
- `回退` — 反 reject (跟 rollback 拆死)

## §3 const 三向锁不需要 — server JSON 字段名 byte-identical

server-client JSON shape byte-identical:
```json
[
  {"old_content": "...", "ts": 1700000000000, "reason": "unknown"}
]
```

**字段锁**:
- `old_content` (string) byte-identical
- `ts` (int64 ms epoch) byte-identical
- `reason` (string) byte-identical = AL-1a `reasons.Unknown='unknown'`
  跟 AL-7 SweeperReason / HB-5 HeartbeatSweeperReason 同源 (AL-1a 锁链
  第 18 处)
- 反 reject 任何其他字段名 (e.g. `oldContent` / `timestamp` / `cause`)

## §4 toast 文案 byte-identical

| 触发 | toast 文案 |
|---|---|
| 加载历史成功 | (无 toast — UI modal 渲染即视觉反馈) |
| 加载失败 | `加载编辑历史失败` byte-identical (操作 + 失败 拼接) |
| 关闭 modal | (无 toast) |

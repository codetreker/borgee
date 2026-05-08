# CHN-14 content lock — DescriptionHistoryModal 文案 + DOM (战马D v0)

> 战马D · 2026-04-30 · client SPA DescriptionHistoryModal.tsx 文案
> byte-identical 锁. **关联**: spec `chn-14-spec.md` + stance + acceptance.
> 跟 DM-7 #558 EditHistoryModal 同模式.

## §1 DescriptionHistoryModal DOM (byte-identical)

```tsx
<div
  className="description-history-modal"
  data-testid="description-history-modal"
  role="dialog"
>
  <header className="description-history-header">
    <h3>编辑历史</h3>
    <button
      type="button"
      data-testid="description-history-close"
      onClick={onClose}
      aria-label="关闭"
    >
      ×
    </button>
  </header>
  {history.length === 0 ? (
    <div className="description-history-empty" data-testid="description-history-empty">
      暂无编辑记录
    </div>
  ) : (
    <ul className="description-history-list">
      {history.map((entry, i) => (
        <li
          key={i}
          className="description-history-entry"
          data-history-index={i}
        >
          <time
            dateTime={new Date(entry.ts).toISOString()}
            className="description-history-ts"
          >
            {new Date(entry.ts).toISOString()}
          </time>
          <span className="description-history-action">: 修改了说明</span>
          <pre className="description-history-old-content">{entry.old_content}</pre>
        </li>
      ))}
    </ul>
  )}
</div>
```

**字面锁** (vitest 反向 grep 守):
- title `编辑历史` 4 字 byte-identical (跟 DM-7 EditHistoryModal 同源)
- empty `暂无编辑记录` 6 字 byte-identical
- 行 action 文案 `: 修改了说明` byte-identical (前缀冒号 + 空格)
- close button aria-label `关闭` 2 字 byte-identical
- `data-testid="description-history-modal"` byte-identical
- `data-testid="description-history-empty"` byte-identical
- `data-testid="description-history-close"` byte-identical
- 时间戳 RFC3339 (ISO string) byte-identical 跟 CHN-1.2 archive system DM

### §1.1 Cross-modal byte-identical 锚 (跨 DM-7 EditHistoryModal + CHN-14 DescriptionHistoryModal)

> feima review #2 锚: 跨 modal 共享立场必锁 byte-identical, 否则 drift 漂.

| 字面 | DM-7 EditHistoryModal | CHN-14 DescriptionHistoryModal | 反向 grep 守门 |
|---|---|---|---|
| modal title 中文 | `编辑历史` | `编辑历史` | vitest 双向 grep — 两 modal 都必须含 `<h3>编辑历史</h3>` |
| close aria-label | `关闭` | `关闭` | vitest 双向 grep — `aria-label="关闭"` |
| RFC3339 ts 表达 | `new Date(entry.ts).toISOString()` | `new Date(entry.ts).toISOString()` | vitest source grep 表达式 byte-identical |

**反向 grep 守门 test** (`packages/client/src/__tests__/cross_modal_history_lock.test.ts`):
- 同时读 `EditHistoryModal.tsx` + `DescriptionHistoryModal.tsx` 源文件
- 反向断 3 字面在两 modal 各 ≥1 hit (字面消失 = drift fail)
- 文案 SSOT-by-content-lock 模式 (跟 admin god-mode 不挂在 DM-7/CHN-14
  共享立场同精神; 不是 SSOT-by-component-reuse, 因 DM-7 立场 ⑤ 空态
  return null vs CHN-14 立场 ⑥ 显式空态 真分歧, component 字段 key 也别).
  + DM-7 #558 同源
- `data-history-index` 行级 byte-identical

## §2 反约束 — 同义词 reject

DescriptionHistoryModal + 任何 description history UI 字面反向 reject:
- `history` (English) — 反 reject (data-testid + className 例外)
- `log` — 反 reject (data-testid + className 例外)
- `audit` — 反 reject (data-testid + className 例外)
- `记录` (Chinese record) — 反 reject (我们用 `编辑历史` / `暂无编辑记录`)
- `日志` (Chinese log) — 反 reject
- `审计` (Chinese audit) — 反 reject
- `回退` — 反 reject (跟 rollback 拆死, v3 留)
- `恢复` — 反 reject (跟 restore 拆死)

## §3 server JSON 字段名 byte-identical (跟 DM-7 #558 同模式)

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
  跟 DM-7 #558 + AL-7 SweeperReason / HB-5 HeartbeatSweeperReason 同源
  (锁链停在 HB-6 #19, CHN-14 不引入新 reason)
- 反 reject 任何其他字段名 (e.g. `oldContent` / `timestamp` / `cause`)

## §4 toast 文案 byte-identical

| 触发 | toast 文案 |
|---|---|
| 加载历史成功 | (无 toast — UI modal 渲染即视觉反馈) |
| 加载失败 | `加载编辑历史失败` byte-identical (操作 + 失败 拼接) |
| 关闭 modal | (无 toast) |

## §5 DescriptionEditor 加历史按钮 (CHN-10 既有 byte-identical 不破)

```tsx
{/* CHN-10 既有 textarea + 保存 + 取消 byte-identical 不动 */}
{isOwner && (
  <button
    type="button"
    data-testid="description-history-trigger"
    onClick={() => setShowHistory(true)}
  >
    查看编辑历史
  </button>
)}
```

**字面锁**:
- trigger button `查看编辑历史` 6 字 byte-identical
- `data-testid="description-history-trigger"` byte-identical
- 反约束: 仅 isOwner 渲染 (跟 owner-only ACL 立场 ② 一致)

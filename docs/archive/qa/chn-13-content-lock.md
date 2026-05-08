# CHN-13 content lock — ChannelSearchInput + ChannelList 空态/count (战马D v0)

战马D · 2026-04-30 · client SPA search input + 空态/count 文案 byte-identical 锁.

## §1 ChannelSearchInput DOM (byte-identical)

```tsx
<div className="channel-search-input" data-testid="channel-search-input">
  <input
    type="text"
    placeholder="搜索频道"
    value={query}
    onChange={(e) => setQuery(e.target.value)}
    data-testid="channel-search-input-field"
    aria-label="搜索频道"
  />
</div>
```

**字面锁**:
- placeholder `搜索频道` 4 字 byte-identical
- aria-label `搜索频道` 4 字 byte-identical
- `data-testid="channel-search-input"` byte-identical
- `data-testid="channel-search-input-field"` byte-identical
- controlled input (value + onChange)
- debounce 200ms — onChange setQuery 立即, 但 useEffect 延 200ms 才触发
  parent onSearch(query)

## §2 ChannelList 空态 (byte-identical)

```tsx
{channels.length === 0 && query !== '' && (
  <div className="channel-list-empty" data-testid="channel-list-empty">
    未找到匹配
  </div>
)}
```

**字面锁**:
- 空态文案 `未找到匹配` 5 字 byte-identical
- `data-testid="channel-list-empty"` byte-identical
- 触发条件: query 非空 AND channels.length===0 (空 query 不显空态 — 走
  既有"无频道"提示)

## §3 ChannelList count 显示 (byte-identical)

```tsx
{query !== '' && channels.length > 0 && (
  <div className="channel-list-count" data-testid="channel-list-count">
    共 {channels.length} 个频道
  </div>
)}
```

**字面锁**:
- 文案 `共 {n} 个频道` byte-identical (n 占位)
- `data-testid="channel-list-count"` byte-identical
- 仅 query 非空时显 (空 query 不显 count, 既有 list 行为不变)

## §4 反约束 — 同义词 reject

ChannelSearchInput + ChannelList 任何 user-visible 文本反向 reject:
- `find` (English) — 反 reject (data-testid + className 例外)
- `lookup` — 反 reject
- `locate` — 反 reject
- `search` (English) — 反 reject (data-testid 例外, 我们用 `搜索` 中文)
- `查找` — 反 reject (我们用 `搜索`)
- `检索` — 反 reject
- `查询` — 反 reject

## §5 debounce 200ms 行为 byte-identical

```ts
useEffect(() => {
  const t = setTimeout(() => {
    onSearch(query);
  }, 200);
  return () => clearTimeout(t);
}, [query, onSearch]);
```

**数学锁**:
- 200ms 静默期 — 跟 useUserLayout PUT_DEBOUNCE_MS (CHN-3.3 #415) 同源.
- query 变化 reset timer (cleanup + setTimeout 重启).
- 空 q (清空) 也触发 onSearch("") — 立即恢复全列表.

## §6 server q 透传 byte-identical

`lib/api.ts::listChannels(q?: string)`:
```ts
export async function listChannels(q?: string): Promise<{ channels: Channel[] }> {
  const url = q ? `/api/v1/channels?q=${encodeURIComponent(q)}` : '/api/v1/channels';
  return request<{ channels: Channel[] }>(url);
}
```

**反约束**:
- 空 q (undefined / "") → 既有 URL `/api/v1/channels` byte-identical (不漂
  `?q=`).
- q 含特殊字符 → encodeURIComponent (server LIKE 走 prepared stmt
  防 SQL 注入).

# SearchBox + SearchResultList — artifact 全文搜索 client contract

> **单一来源 pointer.** Components in
> `packages/client/src/components/SearchBox.tsx` +
> `SearchResultList.tsx`. API client in
> `packages/client/src/lib/api.ts::searchArtifacts` +
> `SEARCH_ERR_TOAST`. Vitest pins in
> `packages/client/src/__tests__/SearchBox.test.tsx` (10 cases) +
> `SearchResultList.test.tsx` (3 cases). Server endpoint:
> `docs/current/server/api/artifact-search.md`.

## Why

CV-6 closes the artifact full-text search loop on the client side —
SearchBox debounces user input and calls GET /artifacts/search;
SearchResultList renders title + server-side `<mark>...</mark>`
highlighted snippet. It uses the browser `<input type="search">`; no fuzzy-search libraries (fuse.js /
minisearch / fuzzysort / flexsearch).

## 原则 (cv-6-spec.md §0 + content-lock)

- **server-side 单一来源** (FTS5). No client-side fuzzy-search library (grep 检查
  package.json count==0 by 5 keyword).
- **debounce 300ms** — 反每键发 HTTP, useEffect cleanup pattern.
- **kbd shortcut**: `/` focuses input (不在编辑态时); `Escape` clears
  query (跟既有 mention `@` 同精神).
- **5 错码 → toast** byte-identical (`SEARCH_ERR_TOAST` map; 跟 server
  const + content-lock §4 三处单一来源).

## DOM contract (content-lock §2 + §3)

### SearchBox

```html
<input
  type="search"
  placeholder="搜索 artifact (按 / 聚焦)"
  maxlength="256"
  data-testid="artifact-search-input"
  aria-label="搜索 artifact"
  class="artifact-search-input"
/>
```

字面锁定:

- `type="search"` (browser search input, 反 type="text").
- `placeholder` byte-identical "搜索 artifact (按 / 聚焦)".
- `maxlength="256"` 跟 server `SearchQueryMaxLen` byte-identical.
- `data-testid="artifact-search-input"` (e2e 出处).
- `aria-label="搜索 artifact"` (accessibility 反向断言).

### SearchResultList row

```html
<ul class="search-result-list" data-testid="artifact-search-results">
  <li
    data-testid="search-result-row"
    data-artifact-id="<uuid>"
    data-artifact-kind="<kind>"
    class="search-result-row"
  >
    <div class="search-result-title">{title}</div>
    <div class="search-result-snippet" dangerouslySetInnerHTML />
  </li>
</ul>
```

`data-artifact-kind` 5 enum byte-identical 跟 server (markdown / code /
image_link / video_link / pdf_link).

## Snippet rendering

server-side `snippet()` 返回 `<mark>...</mark>` 字面已嵌入. client 走
`dangerouslySetInnerHTML` 直渲 — server-side validated by FTS5 (token
boundaries + literal `<mark>`/`</mark>` 字符串 server 单一来源加注), 不
再 client-side sanitize (跟既有 markdown 同精神 — markdown 路径也走
server-side trusted output).

## 5 错码 → toast 字面 byte-identical (content-lock §4)

```ts
export const SEARCH_ERR_TOAST: Record<string, string> = {
  'search.query_empty':         '请输入搜索词',
  'search.query_too_long':      '搜索词太长 (最长 256 字符)',
  'search.channel_not_member':  '无权访问此频道',
  'search.cross_org_denied':    '跨组织搜索被禁',
  'search.not_owner':           '需要频道所有者权限',
};
```

**改 = 改三处** (server const + 此 map + content-lock §4); CI lint
等价单测 (`SearchBox.test.tsx::SEARCH_ERR_TOAST byte-identical`) 守
future 脱节.

## 跨 milestone byte-identical 锁定

- `SearchResult.kind` 5 enum 跟 server `cv_3_2_artifact_validation.go::
  ValidArtifactKinds` byte-identical (CV-3 + CV-2 v2 同源).
- `SEARCH_ERR_TOAST` 5 keys 跟 `internal/api/search.go::SearchErrCode*`
  const byte-identical.
- DOM `data-testid` + `data-artifact-kind` byte-identical 跟 server
  endpoint spec docs/current/server/api/artifact-search.md.
- 不引入 client-side fuzzy-search library — 跟 CV-2 v2 / CV-3 v2 设计 "不引入重 lib"
  同精神 (grep 检查 package.json count==0 on `fuse\|minisearch\|fuzzysort
  \|flexsearch`).

## 不在范围

- 客户端高亮 (server-side `<mark>` 已带, client 不另起 highlight library).
- 跨 channel global search (server v0 不开, 留 v2+).
- search 历史 / saved query (留 v2+ 走 user_settings).
- ChannelView sidebar 集成 (留后续 PR — v0 把 SearchBox 当独立
  component, 调用方按需嵌入).

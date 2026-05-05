# CV-6 Content Lock — search 5 错码文案 + DOM 字面锁 (野马 v0)

> 战马C · 2026-04-30 · ≤40 行 byte-identical 锁 (4 件套第三件; 跟 BPP-3.2 / AL-1b / AP-1 / AL-5 / CV-2 v2 / CV-3 v2 同模式)
> **蓝图锚**: [`canvas-vision.md`](../blueprint/current/canvas-vision.md) §1.4 + [`auth-permissions.md`](../blueprint/current/auth-permissions.md) §1.3
> **关联**: spec `docs/implementation/modules/cv-6-spec.md` (战马C v0, d2fe1f0) + stance `docs/qa/cv-6-stance-checklist.md` + acceptance `docs/qa/acceptance-templates/cv-6.md`. 复用 AP-1 14 capability const + 6 处 owner-only ACL.

## §1 错码文案锁 (5 个, 跟 server const 同源)

字面 (改 = 改三处: server const + client toast helper + 此 content-lock):

```
search.query_empty           → "请输入搜索词"
search.query_too_long        → "搜索词太长 (最长 256 字符)"
search.channel_not_member    → "无权访问此频道"
search.cross_org_denied      → "跨组织搜索被禁"
search.not_owner             → "需要频道所有者权限"
```

**反向 grep** (count==0): 反 hardcode 文案漂移 — `搜索失败|搜索出错|无效查询|不能搜索` 在 packages/client/src/components/SearchBox.tsx + SearchResultList.tsx (近义词漂禁, 仅上 5 字面).

## §2 SearchBox DOM 字面锁

单 `<input type="search">` byte-identical (改 = 改两处: 此 content-lock + `packages/client/src/components/SearchBox.tsx`):

| attr | 字面 | 用途 |
|---|---|---|
| `type` | `"search"` | HTML5 native search (反 type="text") |
| `placeholder` | `"搜索 artifact (按 / 聚焦)"` | UX 入口提示 byte-identical |
| `data-testid` | `"artifact-search-input"` | e2e 锚 |
| `aria-label` | `"搜索 artifact"` | a11y 反向断言 |
| `maxlength` | `"256"` | 跟 server query_too_long 阈值 byte-identical |

kbd shortcut 字面: `/` focus + `Escape` clear (跟 既有 mention `@` shortcut 同精神).

debounce **300ms** byte-identical (跟 spec §1.2 字面承袭, 反 100/500/1000 漂值).

## §3 SearchResultList DOM 字面锁

单结果 row byte-identical:

```
<li data-testid="search-result-row" data-artifact-id="<uuid>" data-artifact-kind="<kind>">
  <ArtifactThumbnail /> | <MediaPreview />  // kind 闸 跟 CV-3 v2 / CV-2 v2 共
  <div class="search-result-title">{title}</div>
  <div class="search-result-snippet" dangerouslySetInnerHTML={{__html: snippet}} />
</li>
```

**反向 grep** (count==0): `react-syntax-highlighter.*search|search.*custom-marker` 在 client/src/ count==0 (server-side `<mark>` 字面同源).

## §4 5 错码字面单源 (server const ↔ client toast 双向锁)

`internal/api/search.go` const + `packages/client/src/lib/api.ts::SearchErrCode` 双向锁 (e2e 反断 server 错码 → client toast 字面 1:1 映射).

```ts
export const SEARCH_ERR_TOAST: Record<string, string> = {
  'search.query_empty':         '请输入搜索词',
  'search.query_too_long':      '搜索词太长 (最长 256 字符)',
  'search.channel_not_member':  '无权访问此频道',
  'search.cross_org_denied':    '跨组织搜索被禁',
  'search.not_owner':           '需要频道所有者权限',
};
```

**改 = 改三处** (server const + client map + 此 content-lock); CI lint 等价单测守 future drift.

## §5 跨 PR drift 守

改 `search.*` 错码字面 / SearchBox placeholder / 5 toast 文案 = 改五处 (双向 grep 等价单测覆盖):
1. `internal/api/search.go::SearchErrCode*` const
2. `packages/client/src/lib/api.ts::SEARCH_ERR_TOAST` map
3. `packages/client/src/components/SearchBox.tsx` (placeholder + maxlength + kbd)
4. `packages/client/src/components/SearchResultList.tsx` (DOM data-testid + class)
5. 此 content-lock

## 更新日志

- 2026-04-30 — 战马C + 野马 v0 (4 件套第三件 ≤40 行): 5 错码字面 + SearchBox DOM (placeholder + maxlength + kbd + debounce 300ms) + ResultList row attrs + 5 toast 文案 byte-identical + 跨 PR drift 5 处全锁; 反 hardcode 文案漂移近义词禁; 跟 BPP-3.2 / AL-5 / CV-2 v2 / CV-3 v2 / AP-1 同模式.

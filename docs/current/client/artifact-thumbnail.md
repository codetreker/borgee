# ArtifactThumbnail — code/markdown 缩略 DOM contract

> **Source-of-truth pointer.** Component in
> `packages/client/src/components/ArtifactThumbnail.tsx`. Vitest pins in
> `packages/client/src/__tests__/ArtifactThumbnail.test.tsx` (23 cases).
> Sister component: `MediaPreview.tsx` (CV-2 v2) for image/video/pdf
> kinds — 二闸互斥.

## Why

CV-3 v2 closes the text-kind thumbnail loop on the client side —
markdown / code artifacts get a 256x256 lazy `<img>` thumbnail (or
icon-only fallback when `thumbnail_url` not yet generated). Server
records `thumbnail_url` (CV-3 v2 server endpoint). HTML5 native + no
client-side renderer libs.

## Stance (cv-3-v2-spec.md §0)

- **HTML5 native primitives.** `<img loading="lazy">` 256x256; no
  html2canvas / dom-to-image / puppeteer-client / shiki client-side
  renderer (package.json reverse grep count==0).
- **XSS 红线 #1 (https only).** 复用 `ImageLinkRenderer.isHttpsURL`
  byte-identical 跟 server `ValidateImageLinkURL` 同源. Non-https URL
  → fallback div, 不渲染 `<img>`.
- **kind 闸 (2-tuple).** `THUMBNAILABLE_KINDS = ['markdown', 'code']`
  byte-identical 跟 server `ThumbnailableKinds` 同源 (vitest 双向锁).
  其他 kind (image_link / video_link / pdf_link / unknown) → null
  (走 CV-2 v2 `MediaPreview` 既有 path, 二闸互斥).

## DOM contract (vitest + future e2e 锚)

| 条件 | tag | required attrs | data-thumbnail-kind |
|---|---|---|---|
| kind ∈ THUMBNAILABLE_KINDS + safe https `thumbnailUrl` | `<img>` | `src`, `alt`, `loading="lazy"`, `class="artifact-thumbnail"`, `width=256`, `height=256` | `markdown` / `code` |
| kind ∈ THUMBNAILABLE_KINDS but no/unsafe `thumbnailUrl` | `<div>` | `class="artifact-thumbnail-fallback"`, `role="img"`, `aria-label`, child `<span class="artifact-thumbnail-icon">` | `markdown` / `code` |
| 其他 kind | (null) | — | (none) |

## Props

```ts
interface Props {
  kind: string;          // 5 kind enum (markdown/code 渲染, 其他 null)
  title: string;         // alt / aria-label
  thumbnailUrl?: string; // server-recorded, https only; NULL = fallback
}
```

## Fallback 设计

无 `thumbnailUrl` 或 unsafe URL 时, 渲染:

- `<div class="artifact-thumbnail-fallback" data-thumbnail-kind="<kind>"
  role="img" aria-label="<title>">`
- 内含 `<span class="artifact-thumbnail-icon" aria-hidden="true">`
  显 emoji icon: `📝` (markdown) / `💻` (code)

CSS 盒子 (`.artifact-thumbnail-fallback`) 由 stylesheet 控制 256x256
固定尺寸 + 居中 icon. icon emoji 是 ASCII-decoded Unicode, 不依赖外
font.

## XSS 红线 #1 fallback

非 https `thumbnailUrl` → 不渲染 `<img>`, 走 fallback div. 防把 unsafe
URL 推入 DOM (`<img src>` 是 XSS attack vector); 反向断言 `img` count==0
是 vitest 锚.

## 二闸互斥 (跟 CV-2 v2 MediaPreview)

`THUMBNAILABLE_KINDS` 跟 CV-2 v2 `PREVIEWABLE_KINDS` 互斥:

```
THUMBNAILABLE_KINDS = [markdown, code]
PREVIEWABLE_KINDS   = [image_link, video_link, pdf_link]
```

调用方 (e.g. ArtifactPanel sidebar) 按 kind 路由:

- markdown / code → `<ArtifactThumbnail kind={...} title={...}
  thumbnailUrl={artifact.thumbnail_url} />`
- image_link / video_link / pdf_link → `<MediaPreview kind={...}
  body={...} title={...} previewUrl={artifact.preview_url} />` (CV-2 v2
  既有 path).

## 跨 milestone byte-identical 锁

- 2-tuple `THUMBNAILABLE_KINDS` 跟 server `ThumbnailableKinds` slice
  byte-identical (server vs client 双向锁, 改 = 改两处).
- 5 kind enum 跟 CV-2 v2 + CV-3 共 schema 单源 (不扩 kind).
- `isHttpsURL` 复用 `ImageLinkRenderer` (CV-3.3 既有), 跟 server
  `ValidateImageLinkURL` byte-identical 同源 (XSS 红线第一道).
- DOM `data-thumbnail-kind` 二 enum byte-identical 跟 server endpoint
  spec 锚.
- 不引入 client-side renderer 重 lib — 跟 CV-2 v2 `MediaPreview` 设计
  ② "不引入 video.js/hls.js/dash.js/shaka-player/pdf.js/react-pdf" 同
  精神 (grep 检查 package.json count==0 on `html2canvas|dom-to-image|
  puppeteer-client|shiki`).

## 不在范围

- Inline syntax-highlight thumbnail render (e.g. shiki client-side) —
  设计 ① server-side SSOT.
- Multi-size thumbnail (mobile 128 / sidebar 256 / preview 512) — v3+,
  v0 单 256x256.
- WebSocket push frame for thumbnail update — 静态 CDN, client GET
  /artifacts/:id pull (CV-2 v2 同精神).

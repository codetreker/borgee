# MediaPreview — 三态分发 DOM contract

> **单一来源 pointer.** Component in
> `packages/client/src/components/MediaPreview.tsx`. Wired into
> `ArtifactPanel.tsx::ArtifactBody` 5 enum switch (markdown / code /
> image_link / video_link / pdf_link). Type in
> `packages/client/src/lib/api.ts::ArtifactKind`. Vitest pins in
> `packages/client/src/__tests__/MediaPreview.test.tsx` (27 cases).

## Why

CV-2 v2 closes the multimedia preview loop on the client side —
image_link / video_link / pdf_link kinds get HTML5-native preview
without dragging in heavy inline render libs. Server records
`preview_url` (https-only); client uses it as image thumbnail-first src
and video poster. PDF embeds use the browser's native `<embed>`.

## 原则 (cv-2-v2-media-preview-spec.md §0 + 设计 ②)

- **HTML5 native primitives.** image → `<img loading="lazy">`; video →
  `<video controls preload="metadata">`; pdf → `<embed
  type="application/pdf">`. No video.js / hls.js / dash.js /
  shaka-player / pdf.js / react-pdf — package.json reverse grep
  count==0.
- **XSS 红线 #1 (https only).** 复用 `ImageLinkRenderer.isHttpsURL`
  byte-identical 跟 server `ValidateImageLinkURL` 同源 (XSS 红线第一道).
  Non-https URL (javascript:/data:/data:image/http:/file:/
  scheme-relative `//host` / 空) → 渲染 `.media-preview-invalid`
  fallback div, 不把 unsafe URL 推入 DOM.
- **kind 闸 (3-tuple).** `PREVIEWABLE_KINDS = ['image_link',
  'video_link', 'pdf_link']` byte-identical 跟 server
  `PreviewableKinds` 同源 (vitest 双向锁定). 其它 kind (markdown / code
  / unknown) → null (走 CV-1 markdown / CV-3 code 既有 path).

## DOM contract (e2e + vitest 出处)

| kind | tag | required attrs | optional attrs | data-media-kind |
|---|---|---|---|---|
| `image_link` | `<img>` | `src`, `alt`, `loading="lazy"`, `class="media-preview-image"` | `src` 优先 `previewUrl` (thumbnail-first) → fallback `body` | `image_link` |
| `video_link` | `<video>` | `src` (= body), `controls`, `preload="metadata"`, `class="media-preview-video"`, `aria-label` | `poster` (= safe `previewUrl`, 缺省 浏览器默认) | `video_link` |
| `pdf_link` | `<embed>` | `src` (= body), `type="application/pdf"`, `class="media-preview-pdf"`, `aria-label` | — | `pdf_link` |
| 其它 / unsafe URL | (null / `<div class="media-preview-invalid">`) | — | — | (none / 标 fallback) |

## Props

```ts
interface Props {
  kind: string;          // 5 enum (image_link / video_link / pdf_link 渲染, 其它 null)
  body: string;          // 必 https 媒体本体 URL
  title: string;         // alt / aria-label
  previewUrl?: string;   // server-recorded thumbnail / poster (https only)
}
```

## thumbnail-first 路径

- image_link 渲染优先级: `previewUrl`(safe) > `body`. server 端 GET
  /artifacts/:id 回填的 `preview_url` 字段 (CV-2 v2 v=28 schema) 命中
  时直接走缩略, 节省首屏带宽.
- video_link `poster` 走 `previewUrl`(safe); 缺省时浏览器默认黑屏.
  pdf_link 不接 poster (embed 标签不支持).

## XSS 红线 #1 fallback

非 https `body` → 不渲染 `<img>` / `<video>` / `<embed>`, 改渲
`<div class="media-preview-invalid" data-media-kind="...">` + 文案
"URL 不合法 (仅支持 https)". querySelector 反向断言 (img/video/embed
count==0) 是 e2e 出处.

非 https `previewUrl` (image_link 路径) → 静默忽略 fall back 到 `body`
(本身已 https). 防 thumbnail-XSS leak via 同 vector.

## ArtifactPanel 5 enum 收口

`packages/client/src/components/ArtifactPanel.tsx`:

- `normalizeKind` accepts 5 字面 (markdown / code / image_link /
  video_link / pdf_link), 其它 fallback string passthrough → fallback
  div.
- `ArtifactBody` switch 五分支:
  - markdown / code / image_link → 既有 path (CV-1 / CV-3.3).
  - **video_link / pdf_link** → `MediaPreview kind={kind} body=...
    title=... previewUrl={artifact.preview_url} />`.

## 跨 milestone byte-identical 锁定

- 5 enum 跟 server `cv_2_v2_media_preview` migration v=28 schema CHECK
  + `ValidArtifactKinds` slice + client `ArtifactKind` three-source
  byte-identical (改 = 改三处).
- `PREVIEWABLE_KINDS` 3-tuple 跟 server `PreviewableKinds` slice
  byte-identical (server vs client 双向锁定, 改 = 改两处).
- `isHttpsURL` 复用 `ImageLinkRenderer` (CV-3.3 既有), 跟 server
  `ValidateImageLinkURL` byte-identical 同源 (XSS 红线第一道).
- DOM `data-media-kind` 三 enum byte-identical 跟 spec §3 e2e grep 出处
  (e2e 反向断言 video_link `controls` count≥1, pdf_link `type=
  "application/pdf"` 字面 1).

## 不在范围

- HLS / DASH 流媒体 (server-side transcoding, 拆 BPP-4+).
- inline pdf.js / react-pdf 渲染 (蓝图 §1.4 "首屏快读不是浏览器内全量
  解码").
- thumbnail 实时刷新 (preview_url 静态 CDN, 不订阅 WS frame; client
  下次 GET /artifacts/:id pull 拿到).

# MediaPreview — three media preview states DOM contract

> **Authoritative component.** Component in
> `packages/client/src/components/MediaPreview.tsx`. Wired into
> `ArtifactPanel.tsx::ArtifactBody` 5 enum switch (markdown / code /
> image_link / video_link / pdf_link). Type in
> `packages/client/src/lib/api.ts::ArtifactKind`. Vitest pins in
> `packages/client/src/__tests__/MediaPreview.test.tsx` (27 cases).

## Why

CV-2 v2 adds client-side previews for multimedia artifacts. image_link /
video_link / pdf_link kinds use browser HTML5 previews without adding large
inline rendering libraries. The server records `preview_url` (https-only); the
client uses it first for image thumbnails and as the video poster. PDF previews
use the browser's built-in `<embed>` support.

## Principles (cv-2-v2-media-preview-spec.md §0 + 设计 ②)

- **Browser HTML5 elements.** image uses `<img loading="lazy">`; video uses
  `<video controls preload="metadata">`; pdf uses
  `<embed type="application/pdf">`. No video.js / hls.js / dash.js /
  shaka-player / pdf.js / react-pdf — package.json reverse grep count==0.
- **XSS constraint #1 (https only).** Reuses `ImageLinkRenderer.isHttpsURL`,
  matching server `ValidateImageLinkURL`.
  Non-https URL (javascript:/data:/data:image/http:/file:/
  scheme-relative `//host` / 空) → 渲染 `.media-preview-invalid`
  fallback div, 不把 unsafe URL 推入 DOM.
- **kind gate (3-tuple).**
  `PREVIEWABLE_KINDS = ['image_link', 'video_link', 'pdf_link']` stays
  aligned with server `PreviewableKinds` (vitest 双向锁定). 其它 kind
  (markdown / code / unknown) → null (走 CV-1 markdown / CV-3 code 既有 path).

## DOM contract (e2e + vitest 出处)

| kind              | tag                                            | required attrs                                                                                | optional attrs                                              | data-media-kind                            |
| ----------------- | ---------------------------------------------- | --------------------------------------------------------------------------------------------- | ----------------------------------------------------------- | ------------------------------------------ |
| `image_link`      | `<img>`                                        | `src`, `alt`, `loading="lazy"`, `class="media-preview-image"`                                 | `src` 优先 `previewUrl` (thumbnail-first) → fallback `body` | `image_link`                               |
| `video_link`      | `<video>`                                      | `src` (= body), `controls`, `preload="metadata"`, `class="media-preview-video"`, `aria-label` | `poster` (= safe `previewUrl`, 缺省 浏览器默认)             | `video_link`                               |
| `pdf_link`        | `<embed>`                                      | `src` (= body), `type="application/pdf"`, `class="media-preview-pdf"`, `aria-label`           | —                                                           | `pdf_link`                                 |
| 其它 / unsafe URL | (null / `<div class="media-preview-invalid">`) | —                                                                                             | —                                                           | none for null; omitted on invalid fallback |

## Props

<!-- prettier-ignore -->
```ts
interface Props {
  kind: string;          // 5 enum (image_link / video_link / pdf_link 渲染, 其它 null)
  body: string;          // 必 https 媒体本体 URL
  title: string;         // alt / aria-label
  previewUrl?: string;   // server-recorded thumbnail / poster (https only)
}
```

## Thumbnail-first path

- image_link render priority: safe `previewUrl` > `body`. server 端 GET
  /artifacts/:id 回填的 `preview_url` 字段 (CV-2 v2 v=28 schema) 命中
  时直接走缩略, 节省首屏带宽.
- video_link `poster` 走 safe `previewUrl`; 缺省时浏览器默认黑屏.
  pdf_link 不接 poster (embed 标签不支持).

## XSS constraint #1 fallback

非 https `body` → 不渲染 `<img>` / `<video>` / `<embed>`, 改渲
`<div class="media-preview-invalid" data-media-kind="...">` + 文案
"URL 不合法 (仅支持 https)". querySelector 反向断言 (img/video/embed
count==0) 是 e2e 出处.

非 https `previewUrl` (image_link path) → silently ignore it and fall back to
`body` (already verified as https). This blocks thumbnail-based XSS through the
same URL vector.

## ArtifactPanel 5-kind dispatch

`packages/client/src/components/ArtifactPanel.tsx`:

- `normalizeKind` accepts 5 字面 (markdown / code / image_link /
  video_link / pdf_link), 其它 fallback string passthrough → fallback
  div.
- `ArtifactBody` switch 五分支:
  - markdown / code / image_link → 既有 path (CV-1 / CV-3.3).
  - **video_link / pdf_link** → render `MediaPreview` with `kind`, `body`,
    `title`, and `previewUrl={artifact.preview_url}`.

## Cross-milestone consistency checks

- 5 enum values must stay aligned across server `cv_2_v2_media_preview`
  migration v=28 schema CHECK, `ValidArtifactKinds` slice, and client
  `ArtifactKind` (change all three together).
- `PREVIEWABLE_KINDS` 3-tuple 跟 server `PreviewableKinds` slice
  must match (server vs client 双向锁定, 改 = 改两处).
- `isHttpsURL` 复用 `ImageLinkRenderer` (CV-3.3 既有), 跟 server
  `ValidateImageLinkURL` applies the same HTTPS-only rule (first XSS constraint).
- DOM `data-media-kind` 三 enum must match spec §3 e2e grep 出处
  (e2e 反向断言 video_link `controls` count≥1, pdf_link `type=
"application/pdf"` 字面 1).

## Out of scope

- HLS / DASH 流媒体 (server-side transcoding, 拆 BPP-4+).
- inline pdf.js / react-pdf rendering (蓝图 §1.4 "首屏快读不是浏览器内全量
  解码").
- thumbnail 实时刷新 (preview_url 是静态 CDN 字段，不订阅 WebSocket frame;
  client 下次 GET /artifacts/:id 时拉取最新值).

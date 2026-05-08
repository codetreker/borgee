# CV-2 v2 spec brief — artifact preview thumbnail + media player (Phase 5)

> 战马D · 2026-04-29 · ≤80 行 · CV-1 续 (Phase 5 候选), 跟 CV-2 v1 (锚点对话 ✅ G3.2 已闭) 解耦
> 关联: CV-1 ✅ (#334+#342+#346+#348) Markdown ONLY 渲染 / CV-3 ✅ #396+#400+#408 (D-lite kind enum + image_link XSS 红线) / CV-4 ✅ artifact iterate
> Owner: TBD 主战 (战马D 起 spec)

---

## 0. 立场 (3 项)

### ① preview thumbnail 走 server CDN 生成, client 不 inline (蓝图 §1.4)
- 蓝图 `canvas-vision.md` §1.4: artifact 是协作场内容资产, 多类型 (markdown / code / image_link / video_link / pdf_link); preview 是首屏快读, 不是 inline 全量
- thumbnail 走 server-side 生成 (image: ImageMagick / 第三方 CDN; video: ffmpeg first-frame extract; pdf: pdf2image first-page) → URL 存 `artifacts.preview_url` 字段, client 仅 `<img src>`
- 反约束: client 不引入 sharp / canvas / pdf.js 等 inline 渲染重 lib (蓝图 §1.4 "首屏快读, 不是浏览器内全量解码")

### ② video player 复用 HTML5 native (不引入 video.js / hls.js 等重 lib)
- HTML5 `<video controls>` 原生足够 v0 — MP4/WebM 直链 + browser 原生解码
- HLS / DASH 流媒体留 future (蓝图 §1.4 不强求实时流)
- 反约束: 不引入 video.js / hls.js / dash.js / shaka-player (跟 CV-1 立场 ④ Markdown ONLY 同精神 — 不被库决定)

### ③ artifact 类型扩展 (image/video/pdf) 跟 CV-1 既有 enum 对齐
- CV-3 #396 已锁 kind enum 三态 → CV-2 v2 加 `image_link` / `video_link` / `pdf_link` 字面 (复用 CV-3 schema 12-step rebuild)
- 反约束: 不裂表 (CV-3 立场 ① "enum 扩不裂表" 字面承袭, 改 = 改 CV-3 #396 enum 单源)
- 文案锁: kindBadge 二元 🤖↔👤 不变, 加类型 icon (📷 image / 🎥 video / 📄 pdf), 跟 MentionArtifactPreview 五处单测锁同源

---

## 1. 拆 ≤3 段

### CV-2.1 — server thumbnail 生成 endpoint
- POST `/api/v1/artifacts/:id/preview` (owner-only, 复用 CV-1.2 ACL)
- server 调外部 CDN/ffmpeg 生成 thumbnail, 返 `preview_url` (https only — 跟 CV-3 image_link XSS 红线同源)
- schema: `artifacts.preview_url TEXT NULL` 字段加 (CV-3 #396 enum 扩同模式)
- 单测: TestCV22V2_PreviewURLHttpsOnly + TestCV22V2_NonOwnerReject

### CV-2.2 — client SPA preview component
- `MediaPreview.tsx` — kind 分发: image_link → `<img>`, video_link → `<video controls>`, pdf_link → `<embed type=application/pdf>`
- 复用 CV-3 ImageLinkRenderer + 加 video / pdf renderer
- 反约束: 不引入 video.js / hls.js / pdf.js, 反向 grep package.json count==0
- vitest: 5 case (image preview / video native controls / pdf embed / fallback / preview URL https only)

### CV-2.3 — e2e + closure
- e2e `cv-2-v2-media-preview.spec.ts` — image / video / pdf 三 kind 浏览器渲染验证
- 反约束 e2e: video 元素 `controls` attribute count≥1, pdf embed `type="application/pdf"` 字面锁
- closure: REG-CV2V2-001..005 + acceptance + 烈马 signoff

---

## 2. 跨 milestone byte-identical 锁

- kind enum 跟 CV-3 #396 共 schema 单源 (改 = 改 CV-3 enum 一处)
- preview_url https only 跟 CV-3 #400 image_link XSS 红线同源
- kindBadge 二元 🤖↔👤 跟 CV-1 #347 + CV-2 v1 #355 + DM-2 #314 + CV-4 #380 五处单测锁同源
- owner-only ACL 跟 CV-1.2 #342 commit lock 同 path

---

## 3. 不在范围 (留账)

- HLS / DASH 流媒体 / inline pdf.js 渲染 / video transcoding (server CDN 外路径) / thumbnail 实时刷新 (preview_url 静态 CDN, 不订阅 WS frame)

---

## 4. 验收挂钩 (REG-CV2V2-001..005 占号)

| ID | 锚 | Test |
|---|---|---|
| 001 | preview_url https only XSS 红线 | server unit (CV-3 #400 同源) |
| 002 | server thumbnail endpoint owner-only | server unit + 反向 grep |
| 003 | client MediaPreview 三 kind 分发 | vitest 5 case |
| 004 | 不引入 video.js / hls.js / pdf.js | package.json grep |
| 005 | e2e media preview 三 kind 浏览器渲染 | playwright |

---

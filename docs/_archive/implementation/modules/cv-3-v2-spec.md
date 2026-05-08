# CV-3 v2 spec brief — code/markdown artifact thumbnail (Phase 5+ 续作)

> 战马C · 2026-04-30 · ≤80 行 spec lock (4 件套之一; CV-3 #408 续作 — code / markdown artifact thumbnail 给 list / sidebar 首屏快读)
> **蓝图锚**: [`canvas-vision.md`](../../blueprint/current/canvas-vision.md) §1.4 (artifact 集合: 多类型, "首屏快读不是浏览器内全量解码" 字面) + §1.6 锚点对话 (review 入口 thumbnail 加速 owner 扫一眼)
> **关联**: CV-3 #408 三 kind enum (markdown/code/image_link) ✅ + CV-2 v2 #517 server CDN thumbnail + media preview path ✅ (`artifacts.preview_url TEXT NULL`, POST /preview owner-only, https-only XSS 红线) + CV-1 #348 markdown artifact ✅ + CV-3.2 #400 ValidateImageLinkURL XSS 红线第一道
> **命名**: CV-3 #408 v1 已闭 (三 kind enum + code prism renderer + image_link XSS 红线); CV-3 v2 = code/markdown thumbnail 续作 (跟 CV-2 v2 #517 image_link/video_link/pdf_link thumbnail 同精神, 此 milestone 补 code/markdown 两 kind 的 thumbnail 路径)

> ⚠️ CV-3 v2 是 **wrapper milestone** (跟 CV-2 v2 #517 / AL-5 / AP-2 / AP-3 wrapper 同模式) — 复用既有 CV-2 v2 server CDN thumbnail recording shim + ValidateImageLinkURL XSS 红线 + ArtifactHandler ACL, **不裂新组件**, 仅补 code/markdown 两 kind 入 thumbnail 闸 + 加新列 `thumbnail_url` (跟 preview_url 字段拆 — CV-2 v2 给 image/video/pdf media kind, CV-3 v2 给 code/markdown text kind, 字段语义分开).

## 0. 关键约束 (3 条立场, 蓝图字面承袭)

1. **thumbnail 服务端生成 (跟 CV-2 v2 同, 不 inline canvas)** (蓝图 §1.4 字面 "首屏快读不是浏览器内全量解码"): code/markdown thumbnail 走 server-side 生成 (CDN worker 调 syntax-highlight / markdown-render → 256x256 PNG → CDN upload), URL 存 `artifacts.thumbnail_url TEXT NULL` 字段; client 仅 `<img src>` 不 inline 渲染重 lib (反约束: 不引入 html2canvas / dom-to-image / puppeteer-client 等 client-side renderer); v0 stance: handler 是 thin recording shim (跟 CV-2 v2 #517 preview.go::handlePreview 同精神 — accepts pre-computed URL from worker), 真 CDN worker 集成留 v1+
2. **thumbnail URL https only (复用 CV-3 #400 ValidateImageLinkURL)** (XSS 红线第一道, 跟 CV-2 v2 立场 ② preview_url https only 同源): thumbnail_url 必走 `auth.ValidateImageLinkURL` 同 helper (single source XSS gate, 改 = 改 cv_3_2_artifact_validation.go 一处); 反约束 javascript:/data:/data:image/http:/file:/scheme-relative `//host`/空 全 reject; 错码字面单源 (跟 CV-2 v2 PreviewErrCode* 同模式 — `thumbnail.url_must_be_https` / `thumbnail.url_invalid` / `thumbnail.kind_not_thumbnailable` / `thumbnail.not_owner` / `thumbnail.artifact_not_found`)
3. **thumbnail 跟 preview_url 共字段拆开** (artifacts.thumbnail_url 新列, 不复用 preview_url): preview_url (CV-2 v2 v=28 既有) 给 media kind (image_link / video_link / pdf_link, 是 thumbnail of media); thumbnail_url (CV-3 v2 新加) 给 text kind (code / markdown, 是 syntax-highlighted preview); 字段拆开理由 — (a) 语义不同 (媒体缩略 vs 文本预览), (b) ThumbnailableKinds 跟 PreviewableKinds slice 互斥 (server PreviewableKinds=[image,video,pdf] / 新 ThumbnailableKinds=[code,markdown] 两闸不交), (c) future code/markdown 加二级 metadata (e.g. line_count) 不污染 media path; 反约束: 不裂表 (artifacts 仍单表, ALTER ADD COLUMN, 跟 CV-2 v2 v=28 + AP-1.1 expires_at + AP-3 org_id + AP-2 revoked_at 五连 ALTER 模式)

## 1. 拆段实施 (CV-3 v2.1 / 3.2 / 3.3, ≤3 PR 同 branch 叠 commit, 一 milestone 一 PR 默认 1 PR)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **CV-3 v2.1** schema migration v=N + server endpoint | `internal/migrations/cv_3_v2_artifact_thumbnail.go` v=N (`ALTER TABLE artifacts ADD COLUMN thumbnail_url TEXT NULL`, 跟 CV-2 v2 v=28 preview_url + AP 三连 ALTER 同模式) + `internal/api/thumbnail.go` 新 `handleThumbnail` (POST /api/v1/artifacts/:id/thumbnail owner-only, 跟 CV-2 v2 preview.go 同 path 同精神 thin recording shim); `ThumbnailableKinds = [markdown, code]` slice; 5 错码字面 (跟 CV-2 v2 PreviewErrCode* 同模式); 7 unit (TestCV3V21_AddsThumbnailURLColumn + TestCV3V21_HandleThumbnailHappyPath + NonOwner403 + Admin401 + URLHttpsOnly + KindNotThumbnailable + Overwrite) | 待 PR (战马C) | 战马C |
| **CV-3 v2.2** client SPA Thumbnail component 256x256 lazy | `packages/client/src/components/ArtifactThumbnail.tsx` — kind 闸 (markdown/code 才渲染); `<img loading="lazy">` 256x256 box (CSS class `artifact-thumbnail`); src 优先 `thumbnail_url` (server-recorded), fallback `<div class="artifact-thumbnail-fallback">` 显 kind icon (📝 markdown / 💻 code); 反约束: 不引入 html2canvas / dom-to-image / puppeteer-client / shiki client-side renderer (HTML5 native + server-side SSOT, 跟 CV-2 v2 立场 ② 同精神); 反向 grep package.json count==0; ArtifactPanel sidebar list 路径 wired through; 5 vitest case (THUMBNAILABLE_KINDS 双向锁 + 2 kind 渲染 + fallback + lazy attr + XSS 红线 unsafe URL reject) | 待 PR (战马C) | 战马C |
| **CV-3 v2.3** server full-flow integration + closure | server-side full-flow: insert markdown/code artifact → POST /thumbnail (owner) → thumbnail_url 落库 + GET /artifacts/:id 回填 thumbnail_url; 反约束 grep CI lint 等价单测 (5 grep 锚: html2canvas / dom-to-image / puppeteer-client / shiki client / hardcode `thumbnail.*not_owner` 字面); registry §3 REG-CV3V2-001..N + acceptance + PROGRESS [x] CV-3 v2 + docs/current sync (server/api/artifact-thumbnail.md + client/artifact-thumbnail.md, 跟 CV-2 v2 #517 双 docs 同模式) | 待 PR (战马C) | 战马C / 烈马 |

## 2. 留账边界 (不接 v2+)

- v2 server-side CDN worker 集成 (ffmpeg / shiki / markdown-it server-side render) — handler v0 是 thin recording shim, 真 CDN 集成跟 CV-2 v2 同模式留 v1+
- thumbnail 实时刷新 (commit/rollback 后自动重建 thumbnail) — v1+ 留账 (跟 CV-2 v2 spec §3 不在范围 "实时刷新" 同精神, thumbnail 静态 CDN 不订阅 WS frame)
- thumbnail 失效 GC (artifact deleted 后清 CDN) — v2+
- thumbnail 不同尺寸 (mobile 128 / sidebar 256 / preview 512 多尺寸) — v3+, v0 单 256x256
- code/markdown render diff 视图 thumbnail (CV-4 #416 IteratePanel diff 路径 thumbnail) — v2+

## 3. 反查 grep 锚 (5 反约束, count==0)

```bash
# 1) 不引入 client-side renderer 重 lib (跟 CV-2 v2 立场 ② 同精神)
grep -E '"html2canvas"|"dom-to-image"|"puppeteer-client"|"shiki"' packages/client/package.json  # 0 hit
# 2) thumbnail XSS 红线 — 复用 ValidateImageLinkURL 单源, 反 hardcode https check
git grep -nE 'thumbnail.*url.*\.HasPrefix.*"https' packages/server-go/internal/api/  # 0 hit (走 ValidateImageLinkURL helper)
# 3) thumbnail 不渗 client inline 渲染路径
git grep -nE 'thumbnail.*react-syntax-highlighter|prism-react-renderer.*thumbnail' packages/client/src/components/  # 0 hit
# 4) thumbnail 不裂 artifact_thumbnails 表 (跟 CV-3.1 #396 立场 ① "enum 扩不裂表" 同精神)
git grep -nE 'CREATE TABLE.*artifact_thumbnails|CREATE TABLE.*artifact_previews' packages/server-go/internal/migrations/  # 0 hit
# 5) thumbnail 错码字面单源 (跟 CV-2 v2 PreviewErrCode* + AP-1/AP-2/AP-3 const 单源同模式)
git grep -nE '"thumbnail\.(not_owner|url_must_be_https|url_invalid|kind_not_thumbnailable|artifact_not_found)"' packages/server-go/internal/  # ≥5 hits (api/thumbnail.go const) + 0 hit hardcode in handler logic
```

## 4. 不在范围

- v2 server-side CDN worker (ffmpeg / shiki / markdown-it server-side) — 跟 CV-2 v2 #517 同模式留 v1+
- thumbnail 实时刷新 (commit/rollback 后自动重建)
- thumbnail GC / multi-size / diff 视图 thumbnail (留 v2+)
- image_link / video_link / pdf_link thumbnail (走 CV-2 v2 #517 既有 preview_url 路径, 不重复)

## 5. 跨 milestone byte-identical 锁

- 跟 CV-2 v2 #517 server CDN thumbnail recording shim + ValidateImageLinkURL XSS 红线 + 错码字面单源 + ACL gate (channel.created_by) 同模式 (改 = 改 thumbnail.go + preview.go 两处, helper 单源不裂)
- 跟 CV-3 #408 三 kind enum (markdown/code/image_link) byte-identical (CV-3 v2 仅扩 thumbnail 路径, kind enum 不动 — CV-2 v2 已扩 5 项, CV-3 v2 不再扩)
- 跟 AP-1.1 #493 expires_at + AP-3 #521 org_id + AP-2 #525 revoked_at + CV-2 v2 #517 preview_url **五连 ALTER ADD COLUMN NULL** 模式 (artifacts 表第二次 ALTER ADD COLUMN, 跟 user_permissions 三 ALTER 同精神)
- 跟 CV-1.2 #342 rollback owner-only ACL 同 path (channel.created_by gate)
- 跟 CV-2 v2 #517 立场 ② HTML5 native 不引入 client-side render lib 同精神 (反向 grep package.json client renderer 0 hit)

# Acceptance Template — CV-2 v2: artifact preview thumbnail + media player

> Spec: `docs/implementation/modules/cv-2-v2-media-preview-spec.md` (战马D v0)
> 蓝图: `canvas-vision.md` §1.4 (artifact 集合: Markdown / 代码片段 / 图片或链接 / 看板 v2+)
> 前置: CV-1 ✅ (#334+#342+#346+#348) Markdown 路径 + CV-3 ✅ (#396+#400+#408) kind enum 三态 + image_link XSS 红线
> Owner: 战马C (主战) + 战马D (spec) + 烈马 (验收)

## 验收清单

### CV-2 v2.1 server schema migration v=28 + thumbnail endpoint

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 schema migration v=28 — `artifacts.type IN ('markdown','code','image_link','video_link','pdf_link')` 5 项 enum 12-step table-recreate (CV-3.1 #396 v=17 三项 → CV-2 v2 五项, 跟 CV-3.1 同模式不裂表); `artifacts.preview_url TEXT NULL` 列加 | unit | 战马C / 烈马 | `internal/migrations/cv_2_v2_media_preview_test.go::TestCV2V2_AcceptsAllFiveKinds` (5 项 INSERT 全过) + `TestCV2V2_PreviewURLColumn` |
| 1.2 反约束: CHECK 严格 reject 'pdf'/'video'/'kanban'/'mindmap'/'doc'/空 (蓝图 §1.4 命名 'video_link'/'pdf_link' byte-identical, 'pdf' 无后缀 reject) | unit | 战马C / 烈马 | `TestCV2V2_RejectsForbiddenKinds` (6 反约束 reject) |
| 1.3 数据保留 — INSERT...SELECT 全字段不漂移 + preview_url=NULL on copy (no backfill — server-side endpoint 生成 lazy) + idx_artifacts_channel_id 重建 + idempotent | unit | 战马C / 烈马 | `TestCV2V2_PreservesExistingRows` + `TestCV2V2_PreservesChannelIDIndex` + `TestCV2V2_NoSeparateKindTables` + `TestCV2V2_Idempotent` |
| 1.4 POST /api/v1/artifacts/:id/preview owner-only happy path — 三 kind (image_link / video_link / pdf_link) 全 200 + preview_url echo + persist + overwrite 接受 | http unit | 战马C / 烈马 | `internal/api/preview_test.go::TestCV2V2_PreviewHappyPathImageLink` + `TestCV2V2_PreviewAcceptsVideoAndPDFKinds` + `TestCV2V2_PreviewOverwrite` |
| 1.5 立场 ① owner-only ACL — non-owner authenticated user → 403 + `preview.not_owner` 错码 (跟 CV-1.2 rollback 立场 ⑦ 同 path) + admin god-mode (no auth) → 401 (跟 ADM-0 §1.3 红线) | unit | 战马C / 烈马 | `TestCV2V2_PreviewNonOwner403` + `TestCV2V2_PreviewAdmin401` |
| 1.6 立场 ② preview_url XSS 红线第一道 — https only (复用 ValidateImageLinkURL 同源); 反约束 javascript:/data:/data:image/http:/file:/scheme-relative `//host`/空 6 项 reject + 错码 `preview.url_must_be_https` / `preview.url_invalid` 双拆 | unit | 战马C / 烈马 | `TestCV2V2_PreviewURLHttpsOnly` (6 反约束 + error code substring 锚) |
| 1.7 立场 ③ kind 闸 — 仅 image_link / video_link / pdf_link 才能 generate preview; markdown / code → 400 `preview.kind_not_previewable` (markdown / code 走 CV-1 既有 head body 渲染) | unit | 战马C / 烈马 | `TestCV2V2_PreviewKindNotPreviewable` |

### CV-2 v2.2 client SPA MediaPreview component

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 `MediaPreview.tsx` kind 三态分发 — image_link → `<img loading="lazy">` thumbnail-first (preview_url 优先 / fallback body) + DOM `data-media-kind="image_link"`; video_link → `<video controls preload="metadata">` HTML5 native + poster (preview_url) + DOM `data-media-kind="video_link"`; pdf_link → `<embed type="application/pdf">` 浏览器内嵌 + DOM `data-media-kind="pdf_link"` | vitest | 战马C / 战马D | `packages/client/src/__tests__/MediaPreview.test.tsx` 27 cases (image_link thumbnail-first + fallback body + lazy + alt + video_link controls + preload + poster + aria-label + pdf_link embed type) |
| 2.2 立场 ② client XSS 红线 #1 — MediaPreview 不把 non-https URL 推入 DOM (复用 ImageLinkRenderer.isHttpsURL byte-identical 跟 server ValidateImageLinkURL 同源); 反约束 6 unsafe scheme reject → 渲染 `.media-preview-invalid` 兜底 div + non-https previewUrl ignored fallback to body | vitest | 战马C / 战马D | `MediaPreview.test.tsx::MediaPreview XSS 红线 #1` (6 unsafe scheme reject + .media-preview-invalid fallback + img/video/embed querySelector null + non-https previewUrl 忽略) |
| 2.3 立场 ② 反约束: 不引入 video.js / hls.js / dash.js / shaka-player / pdf.js / react-pdf (HTML5 native — 跟 CV-1 立场 ④ Markdown ONLY 同精神 "首屏快读不是浏览器内全量解码") | grep | 烈马 / 战马D | `grep -E "video.js\|hls.js\|dash.js\|shaka-player\|pdf.js\|react-pdf" packages/client/package.json` count==0 |
| 2.4 立场 ③ kind 闸 — 其它 kind (markdown / code / unknown) → MediaPreview 渲染 null (走 CV-1 markdown / CV-3 code 既有 path); PREVIEWABLE_KINDS 三-tuple byte-identical 跟 server PreviewableKinds 同源 | vitest | 战马C / 战马D | `MediaPreview.test.tsx::PREVIEWABLE_KINDS is the 3-tuple` (server vs client 双向锁) + `isPreviewableKind` 7 反约束 reject |
| 2.5 ArtifactPanel 5 enum 收口 (markdown / code / image_link / video_link / pdf_link) — `normalizeKind` 扩 5 项 + `ArtifactBody` switch 加 video_link / pdf_link 两分支 wired through MediaPreview + Artifact API type 加 `preview_url?: string` | code | 战马C / 战马D | `packages/client/src/components/ArtifactPanel.tsx` (normalizeKind + ArtifactBody switch) + `packages/client/src/lib/api.ts` (ArtifactKind 5 项 + preview_url) |

### CV-2 v2.3 e2e + closure

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 server-side full-flow integration — owner POST /preview image_link/video_link/pdf_link 三 kind 全 200 + persist + GET /artifacts/:id 返回 preview_url + overwrite 接受 | http unit | 战马C / 烈马 | `preview_test.go` 7 函数 (Happy + AcceptsVideoAndPDF + NonOwner403 + Admin401 + URLHttpsOnly + KindNotPreviewable + Overwrite) |
| 3.2 Playwright e2e (留账 v1+) — image / video / pdf 三 kind 浏览器渲染 + video controls / pdf embed type + DOM data-media-kind 反向锚 | e2e | 战马C / 烈马 | 留账: server-side integration 已覆盖 invariants; 真 e2e 待 server-side CDN worker 集成后补 (跟 BPP-3.2 / AL-5 同精神) |
| 3.3 closure: registry §3 REG-CV2V2-001..007 + acceptance + spec 4 件套 (spec ✅ + 此 acceptance + content-lock TBD + stance TBD) + docs/current sync (canvas-vision.md §1.4 5 enum 字面对齐) | docs | 战马C / 烈马 | registry 7 行 🟢 active flip + acceptance template 此文 |

## 不在本轮范围 (spec §3)

- HLS / DASH 流媒体 (server-side transcoding 拆 BPP-4+)
- inline pdf.js / react-pdf 渲染 (蓝图 §1.4 "首屏快读不是浏览器内全量解码")
- video transcoding (server CDN 外路径)
- thumbnail 实时刷新 (preview_url 静态 CDN, 不订阅 WS frame)
- preview 历史 audit UI (跟 admin god-mode 同精神, ADM-2 既有 admin_actions)
- preview 失败 fallback (e.g. CDN worker 失败后 server 端二次 retry — v2)

## 退出条件

- CV-2 v2.1 1.1-1.7 (schema migration + server endpoint + ACL + XSS + kind 闸) ✅
- CV-2 v2.2 2.1-2.5 (client MediaPreview 三态分发 + XSS + 反约束 lib + 5 enum 收口) ✅
- CV-2 v2.3 3.1-3.3 (server full-flow + closure registry + acceptance) ✅ (e2e 留账)
- 现网回归不破: server unit + 7 cv_2_v2 migration + 7 preview_test + 27 vitest 全 PASS
- REG-CV2V2-001..007 落 registry 全 🟢 active

## 更新日志

- 2026-04-29 — 战马C v0 acceptance template (4 件套第二件): 3 段实施 (1.1-1.7 / 2.1-2.5 / 3.1-3.3) + 6 不在范围 + 退出条件 5 项. 联签 CV-2 v2.1/2.2/2.3 三段同 PR 一次合 (一 milestone 一 PR), 跟 CV-3 #408 / CV-1 #348 closure 同模式. e2e §3.2 留账给 server-side CDN worker 集成 (BPP-3.2 / AL-5 同精神, server-side invariants 已覆盖).

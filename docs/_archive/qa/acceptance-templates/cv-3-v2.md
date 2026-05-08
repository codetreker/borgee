# Acceptance Template — CV-3 v2: code/markdown artifact thumbnail wrapper

> Spec: `docs/implementation/modules/cv-3-v2-spec.md` (战马C v0, 484ec08)
> 蓝图: `canvas-vision.md` §1.4 "首屏快读不是浏览器内全量解码"
> 前置: CV-3 #408 三 kind enum ✅ + CV-2 v2 #517 server CDN thumbnail recording shim + ValidateImageLinkURL XSS 红线 ✅ + CV-1 #348 markdown artifact ✅ + CV-1.2 owner-only ACL ✅
> Owner: 战马C (主战) + 飞马 (spec) + 烈马 (验收)

## 验收清单

### CV-3 v2.1 schema migration v=N + server endpoint

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 schema migration v=N — `ALTER TABLE artifacts ADD COLUMN thumbnail_url TEXT NULL` (跟 CV-2 v2 v=28 preview_url + AP-1.1/AP-3/AP-2 五连 ALTER ADD COLUMN NULL 模式) | unit | 战马C / 烈马 | `internal/migrations/cv_3_v2_artifact_thumbnail_test.go::TestCV3V21_AddsThumbnailURLColumn` (PRAGMA nullable + 5 ALTER 同模式) + `TestCV3V21_LegacyRowsNullPreserved` |
| 1.2 idempotent re-run guard (跟 AP-1.1/AP-3/AP-2 ALTER 同模式 schema_migrations 框架守) + registry.go 字面锁 v=N | unit | 战马C / 烈马 | `TestCV3V21_Idempotent` + `TestCV3V21_RegistryHasVN` |
| 1.3 POST /api/v1/artifacts/:id/thumbnail owner-only happy path — markdown / code 两 kind 全 200 + thumbnail_url echo + persist + overwrite 接受 (跟 CV-2 v2 preview.go 同模式 thin recording shim) | http unit | 战马C / 烈马 | `internal/api/thumbnail_test.go::TestCV3V22_HappyPathMarkdown` + `TestCV3V22_HappyPathCode` + `TestCV3V22_Overwrite` |
| 1.4 立场 ⑤ owner-only ACL — non-owner authenticated user → 403 + `thumbnail.not_owner` 错码 (跟 CV-1.2 rollback + CV-2 v2 同 path) + admin god-mode (no auth) → 401 (跟 ADM-0 §1.3 红线) | unit | 战马C / 烈马 | `TestCV3V22_NonOwner403` + `TestCV3V22_Admin401` |
| 1.5 立场 ② thumbnail_url XSS 红线第一道 — https only (复用 ValidateImageLinkURL 同源); 反约束 javascript:/data:/data:image/http:/file:/scheme-relative `//host`/空 6 reject + 错码字面 `thumbnail.url_must_be_https` / `thumbnail.url_invalid` 双拆 (跟 CV-2 v2 PreviewErrCodeURL* 同模式) | unit | 战马C / 烈马 | `TestCV3V22_URLHttpsOnly` (6 反约束 scheme reject + error code substring `thumbnail.url_` 锚) |
| 1.6 立场 ③ kind 闸 — 仅 markdown / code 才能 generate thumbnail (二闸互斥); image_link/video_link/pdf_link 调 endpoint → 400 `thumbnail.kind_not_thumbnailable` (走 CV-2 v2 既有 preview 路径); 反约束 `ThumbnailableKinds` slice 跟 `PreviewableKinds` 互斥 byte-identical | unit | 战马C / 烈马 | `TestCV3V22_KindNotThumbnailable_ImageLink` + `TestCV3V22_KindNotThumbnailable_VideoLink` + `TestCV3V22_KindNotThumbnailable_PDFLink` + `TestCV3V22_ThumbnailableVsPreviewableMutuallyExclusive` |

### CV-3 v2.2 client SPA ArtifactThumbnail component

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 `ArtifactThumbnail.tsx` kind 闸 — markdown / code 渲染 `<img loading="lazy">` 256x256 box (`class="artifact-thumbnail"`); src 优先 `thumbnail_url`, fallback `<div class="artifact-thumbnail-fallback">` 显 kind icon (📝 markdown / 💻 code) | vitest | 战马C / 战马D | `packages/client/src/__tests__/ArtifactThumbnail.test.tsx` 5 cases (markdown img + code img + fallback div + lazy attr + 256x256 dimensions) |
| 2.2 立场 ② client XSS 红线 #1 — ArtifactThumbnail 不把 non-https URL 推入 DOM (复用 ImageLinkRenderer.isHttpsURL 同源); 反约束 unsafe scheme reject → 渲染 fallback div 不渲染 `<img>` | vitest | 战马C / 战马D | `ArtifactThumbnail.test.tsx::XSS 红线 #1 — https only` (6 unsafe scheme reject + img querySelector null + fallback div 渲染) |
| 2.3 立场 ⑥ 反约束: 不引入 html2canvas / dom-to-image / puppeteer-client / shiki client-side renderer (HTML5 native + server-side SSOT, 跟 CV-2 v2 立场 ② 同精神) | grep | 烈马 / 战马D | `grep -E "html2canvas\|dom-to-image\|puppeteer-client\|shiki" packages/client/package.json` count==0 |
| 2.4 立场 ③ kind 闸 — 其它 kind (image_link / video_link / pdf_link / unknown) → ArtifactThumbnail 渲染 null (走 CV-2 v2 MediaPreview 既有 path); THUMBNAILABLE_KINDS 二-tuple byte-identical 跟 server ThumbnailableKinds 同源 (双向锁) | vitest | 战马C / 战马D | `ArtifactThumbnail.test.tsx::THUMBNAILABLE_KINDS is the 2-tuple` (server vs client 双向锁) + `isThumbnailableKind` 5 反约束 reject |

### CV-3 v2.3 server full-flow integration + closure

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 server-side full-flow: insert markdown artifact → POST /thumbnail (owner) → thumbnail_url 落库 + GET /artifacts/:id 回填 thumbnail_url 字段; 同 code 路径; 跨 kind reject (image_link 调 thumbnail → 400 / markdown 调 preview → 400) | http e2e | 战马C / 烈马 | `thumbnail_test.go` 全 8 函数 PASS + 反向 grep `thumbnail.*image_link` 在 api/ count==0 |
| 3.2 反向 grep CI lint 等价单测 (5 grep 锚: client renderer / hardcode https / cross-kind path / 不裂表 / hardcode reason) | unit | 烈马 | `thumbnail_test.go::TestCV3V23_ReverseGrep_5Patterns_AllZeroHit` (filepath.Walk + package.json grep) |
| 3.3 closure: registry §3 REG-CV3V2-001..N + acceptance + PROGRESS [x] CV-3 v2 + docs/current sync (server/api/artifact-thumbnail.md + client/artifact-thumbnail.md, 跟 CV-2 v2 #517 双 docs 同模式) | docs | 战马C / 烈马 | registry + PROGRESS + 4 件套全闭 |

## 不在本轮范围 (spec §4)

- v2 server-side CDN worker (ffmpeg / shiki server-side / markdown-it server-side render) — handler v0 是 thin recording shim, 真集成留 v1+ (跟 CV-2 v2 #517 同精神)
- thumbnail 实时刷新 (commit/rollback 后自动重建) — v1+ (静态 CDN, 不订阅 WS frame)
- thumbnail GC / multi-size / diff 视图 thumbnail (留 v2+)
- image_link / video_link / pdf_link thumbnail (走 CV-2 v2 #517 既有 preview_url 路径)

## 退出条件

- CV-3 v2.1 1.1-1.6 (schema ALTER + endpoint + ACL + XSS + kind 闸 + 二闸互斥) ✅
- CV-3 v2.2 2.1-2.4 (client ArtifactThumbnail kind 闸 + XSS + 反约束 lib + 二闸互斥) ✅
- CV-3 v2.3 3.1-3.3 (server full-flow + 反向 grep + closure) ✅
- 现网回归不破: AP-1/AP-3/AP-2/CV-2 v2 路径零变 (thumbnail_url NULL = 未生成, 跟既有列同精神)
- REG-CV3V2-001..N 落 registry + 5 反约束 grep 全 count==0
- 4 件套全闭 (spec ✅ + stance ✅ + acceptance ✅ + content-lock 不需要 server-only + minimal client DOM)

## 更新日志

- 2026-04-30 — 战马C v0 acceptance template (4 件套第二件): 3 段实施 (1.1-1.6 / 2.1-2.4 / 3.1-3.3) + 4 不在范围 + 6 项退出条件. 联签 CV-3 v2.1/.2/.3 三段同 branch 同 PR (一 milestone 一 PR 协议默认 1 PR, 跟 CV-2 v2 #517 / AP-2 #525 / AP-3 #521 同模式).

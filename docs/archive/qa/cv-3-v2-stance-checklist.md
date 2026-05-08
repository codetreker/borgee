# CV-3 v2 立场反查清单 (战马C v0)

> 战马C · 2026-04-30 · 立场 review checklist (跟 CV-2 v2 #517 / AP-2 #525 / AP-3 #521 同模式)
> **目的**: CV-3 v2 三段实施 (3.1 schema + server endpoint / 3.2 client ArtifactThumbnail.tsx / 3.3 e2e + closure) PR review 时, 飞马 / 烈马按此清单逐立场 sign-off.
> **关联**: spec `docs/implementation/modules/cv-3-v2-spec.md` (战马C v0, 484ec08) + acceptance `docs/qa/acceptance-templates/cv-3-v2.md`. 复用 CV-2 v2 #517 server CDN thumbnail recording shim + ValidateImageLinkURL XSS 红线 + ArtifactHandler ACL gate + CV-3 #408 三 kind enum + 五连 ALTER ADD COLUMN NULL 模式.

## §0 立场总表 (3 立场 + 5 边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | thumbnail 服务端生成 (跟 CV-2 v2 同, 不 inline canvas) | canvas-vision.md §1.4 字面 "首屏快读不是浏览器内全量解码" | server CDN thumbnail recording shim (跟 CV-2 v2 preview.go::handlePreview 同精神 — accepts pre-computed URL); 反向 grep client-side renderer 0 hit (`html2canvas\|dom-to-image\|puppeteer-client\|shiki` 在 packages/client/package.json) |
| ② | thumbnail URL https only (复用 CV-3 #400 ValidateImageLinkURL) | XSS 红线第一道 跟 CV-2 v2 立场 ② preview_url https only 同源 | 复用 `auth.ValidateImageLinkURL` 单 helper (XSS gate single source, 改 = 改 cv_3_2_artifact_validation.go 一处); 5 错码字面 byte-identical (`thumbnail.{not_owner,url_must_be_https,url_invalid,kind_not_thumbnailable,artifact_not_found}`); 反 hardcode error string in handler |
| ③ | thumbnail 跟 preview_url 字段拆开 (artifacts.thumbnail_url 新列) | preview (image/video/pdf media) vs thumbnail (code/markdown text) 二闸互斥 | `ThumbnailableKinds = [markdown, code]` slice 跟 `PreviewableKinds = [image_link, video_link, pdf_link]` 三-tuple **互斥** (反向 grep 跨 kind 调错 endpoint count==0); v=N migration ALTER ADD COLUMN thumbnail_url TEXT NULL (跟 CV-2 v2 v=28 preview_url 同模式) |
| ④ (边界) | 五连 ALTER ADD COLUMN NULL 模式 | 跟 CV-2 v2 v=28 preview_url + AP-1.1 expires_at + AP-3 org_id + AP-2 revoked_at 五连 | artifacts 表第二次 ALTER ADD COLUMN (CV-2 v2 已第一次), user_permissions 三连; 反约束: 不 NOT NULL / 不 default / 不 FK |
| ⑤ (边界) | owner-only ACL (channel.created_by gate) | 跟 CV-1.2 #342 rollback 立场 ⑦ + CV-2 v2 #517 立场 ① 同 path | non-owner → 403 `thumbnail.not_owner`; admin god-mode (no auth user) → 401 (走 /admin-api 单独 mw, ADM-0 §1.3 红线); endpoint 0 行改 endpoint 既有 (改 = 改 thumbnail.go + preview.go 两处, 跟 CV-2 v2 / CV-1 SSOT 同精神) |
| ⑥ (边界) | 不引入 client-side renderer 重 lib | 跟 CV-2 v2 立场 ② "不引入 video.js/hls.js/dash.js/shaka-player/pdf.js/react-pdf" 同精神 | 反向 grep `"html2canvas"\|"dom-to-image"\|"puppeteer-client"\|"shiki"` package.json count==0 |
| ⑦ (边界) | thumbnail 静态 CDN 不订阅 WS frame | 跟 CV-2 v2 spec §3 不在范围 "实时刷新" 同精神 | client GET /artifacts/:id pull 拿到 thumbnail_url; commit/rollback 不自动重建 thumbnail (留 v1+); 反向 grep WS push frame for thumbnail count==0 |
| ⑧ (边界) | thumbnail 不裂表 (跟 CV-3.1 #396 立场 ① 同精神) | enum 扩不裂表 同精神 | 反向 grep `CREATE TABLE.*artifact_thumbnails\|artifact_previews` 0 hit; artifacts 仍单表 |

## §1 立场 ① server CDN thumbnail 不 inline (CV-3 v2.1 守)

**蓝图字面源**: `canvas-vision.md` §1.4 "首屏快读不是浏览器内全量解码" + CV-2 v2 #517 立场 ① server CDN thumbnail 字面承袭

**反约束清单**:

- [ ] `internal/api/thumbnail.go::handleThumbnail` 是 thin recording shim — accepts client/worker-supplied `thumbnail_url` (跟 CV-2 v2 #517 preview.go::handlePreview 同精神)
- [ ] 反向 grep client-side renderer 0 hit: `html2canvas`/`dom-to-image`/`puppeteer-client`/`shiki` 在 packages/client/package.json
- [ ] 反向 grep `thumbnail.*react-syntax-highlighter\|prism-react-renderer.*thumbnail` 在 packages/client/src/components/ count==0 (CodeRenderer 走 inline 渲染, 不入 thumbnail 路径)

## §2 立场 ② thumbnail URL https only (CV-3 v2.1 守)

**蓝图字面源**: XSS 红线第一道 跟 CV-2 v2 #517 立场 ② + CV-3 #400 ValidateImageLinkURL 同源

**反约束清单**:

- [ ] 复用 `auth.ValidateImageLinkURL` (跟 CV-2 v2 preview.go 同 helper); 反向 grep `thumbnail.*url.*\.HasPrefix.*"https` 在 internal/api/ count==0 (走 helper 单源不 hardcode)
- [ ] 5 错码字面单源: `thumbnail.not_owner` / `thumbnail.url_must_be_https` / `thumbnail.url_invalid` / `thumbnail.kind_not_thumbnailable` / `thumbnail.artifact_not_found` (跟 CV-2 v2 PreviewErrCode* + AP-1/AP-2/AP-3 const 同模式)
- [ ] 反向 grep handler 内 hardcode `"thumbnail\..*"` literal 字面 in non-const path count==0

## §3 立场 ③ thumbnail_url 跟 preview_url 字段拆 (CV-3 v2.1 + 3.2 守)

**蓝图字面源**: preview (image/video/pdf media) vs thumbnail (code/markdown text) 二闸互斥 — 字段语义分立

**反约束清单**:

- [ ] schema migration v=N — `ALTER TABLE artifacts ADD COLUMN thumbnail_url TEXT NULL` (跟 CV-2 v2 v=28 preview_url + AP-1.1/AP-2/AP-3 ALTER ADD COLUMN NULL **五连** 模式)
- [ ] `ThumbnailableKinds = [markdown, code]` slice 跟 `PreviewableKinds = [image_link, video_link, pdf_link]` **互斥** (server 双向锁单测 — 调错 endpoint kind → 400)
- [ ] client `ArtifactThumbnail.tsx` 仅 markdown/code 渲染, image_link/video_link/pdf_link 走 `MediaPreview` 既有 path (反向 grep ArtifactThumbnail 调 image_link/video/pdf count==0)
- [ ] 反向 grep `CREATE TABLE.*artifact_thumbnails\|artifact_previews` 0 hit (artifacts 仍单表)

## §4 联签清单 (实施 PR 时填)

- [ ] 飞马 (spec ↔ 立场对齐): _(签)_
- [ ] 烈马 (反向 grep + 单测覆盖率 ≥84% + 5 反约束全 count==0): _(签)_
- [ ] 战马C (实施代码 ↔ 立场反查 8 项全过): _(签)_

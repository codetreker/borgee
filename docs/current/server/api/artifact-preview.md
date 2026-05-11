# CV-2 v2 — artifact preview endpoint contract

> **单一来源 pointer.** Schema in
> `packages/server-go/internal/migrations/cv_2_v2_media_preview.go` (v=28).
> Handler in `packages/server-go/internal/api/preview.go`.
> Kind enum const + validation in
> `packages/server-go/internal/api/cv_3_2_artifact_validation.go`.
> Route registration at server boot via existing `ArtifactHandler.RegisterRoutes`
> in `packages/server-go/internal/server/server.go`.

## Why

CV-1 ships markdown-only artifacts; CV-3 extends the kind enum to
markdown / code / image_link. CV-2 v2 adds multimedia previews for
`video_link` and `pdf_link` kinds plus a server-recorded `preview_url`
used for thumbnails and video posters, without dragging in heavy inline
render libraries (no video.js / hls.js / pdf.js). Server keeps the
https-only XSS validation; client renders with HTML5 native primitives.

## 原则 (cv-2-v2-media-preview-spec.md §0 原文)

- **① server CDN thumbnail 不 inline.** `preview_url` 是 https URL 字段
  (server validation 红线 #1, 复用 ValidateImageLinkURL 同源). client
  只 `<img src>` / `<video poster>`, 不引入 inline 渲染 lib.
- **② HTML5 native 不引入重 lib.** video → `<video controls>`; pdf →
  `<embed type="application/pdf">`. grep 检查
  `video.js|hls.js|dash.js|shaka-player|pdf.js|react-pdf` package.json
  count==0.
- **③ kind enum 跟 CV-3 共 schema 单一来源.** v=28 12 步重建表
  扩 `markdown/code/image_link/video_link/pdf_link`, schema CHECK +
  `ValidArtifactKinds` slice + client `ArtifactKind` 三处 byte-identical
  (改 = 改三处). 设计约束: 不拆表 (`artifact_video` / `artifact_pdf`
  sqlite_master 检查应为 0 hit).

## Schema (v=28)

| Column | Type | Notes |
|---|---|---|
| `id` ... `lock_acquired_at` | (CV-1.1 + CV-3.1 既有) | unchanged |
| `type` | `TEXT NOT NULL CHECK (type IN ('markdown','code','image_link','video_link','pdf_link'))` | CV-3.1 三项扩五项 |
| `preview_url` | `TEXT NULL` | server-recorded thumbnail / poster URL (https only); NULL = 历史数据 / 未生成 |

Index: `idx_artifacts_channel_id` rebuilt (DROP TABLE 12 步重建).

Migration is forward-only, idempotent via `schema_migrations`. Existing
rows preserve verbatim with `preview_url=NULL` (no thumbnail backfill —
generated lazily on first POST /preview).

## Endpoint

```
POST /api/v1/artifacts/{artifactId}/preview
Authorization: <session cookie>
Content-Type: application/json

{
  "preview_url": "https://cdn.example/thumb.jpg"
}
```

ACL (owner-only 约束 ①):

- No auth user → **401 Unauthorized** (admin routes do not enter this path, ADM-0
  §1.3 红线; admin 走 `/admin-api/*` 单独 middleware).
- Authenticated non-owner (channel.created_by != user.ID) →
  **403 `preview.not_owner`** (跟 CV-1.2 rollback 设计 ⑦ 同 path).
- Channel access defense-in-depth (`canAccessChannel`) → **403 `preview.not_owner`**.
- Artifact missing → **404 `preview.artifact_not_found`**.

Validation rules:

- Artifact kind ∉ `{image_link, video_link, pdf_link}` (= `PreviewableKinds`
  slice) → **400 `preview.kind_not_previewable`**. markdown / code 走
  CV-1 既有 head body 渲染, 不需 preview_url.
- `preview_url` empty / unparseable / scheme ∉ {`https`} →
  **400 `preview.url_must_be_https`** (scheme 不匹配) or
  **400 `preview.url_invalid`** (其他错). 复用 `ValidateImageLinkURL`
  XSS 红线 #1 同源 (javascript:/data:/data:image/http:/file:/
  scheme-relative `//host` / 空 全 reject).

Side-effects on success (200):

- `UPDATE artifacts SET preview_url = ? WHERE id = ?` (overwrite
  接受 — owner 可重发).
- 不写 system message (跟 CV-1.2 rollback 设计 ⑦ "system message 不发"
  保持同一设计约束, owner action 不产生广播事件).
- 不 push WS frame (preview_url 静态 CDN; client 下次 GET
  `/api/v1/artifacts/:id` 拉 — spec §3 不在范围 "实时刷新").

Response body:

```json
{
  "artifact_id": "<uuid>",
  "preview_url": "https://cdn.example/thumb.jpg"
}
```

## GET 回填 (CV-1.2 既有 endpoint)

`GET /api/v1/artifacts/{artifactId}` 响应 body 携带 `preview_url`
字段 (omitempty when NULL); client `MediaPreview` 用作 image
thumbnail-first src / video poster.

## 错码字面单一来源 (跟 AP-1 / AP-3 const 同模式)

```go
PreviewErrCodeNotOwner          = "preview.not_owner"
PreviewErrCodeURLInvalid        = "preview.url_invalid"
PreviewErrCodeURLNotHTTPS       = "preview.url_must_be_https"
PreviewErrCodeKindNotPreviewable = "preview.kind_not_previewable"
PreviewErrCodeArtifactNotFound  = "preview.artifact_not_found"
```

Mismatch between these consts and handler inline strings is caught at
test-time via `preview_test.go` substring asserts (`preview.url_` 前缀 +
`preview.not_owner` / `preview.kind_not_previewable` byte-identical).

## 跨 milestone byte-identical 锁定

- 5 项 enum byte-identical 跟 `cv_2_v2_media_preview` migration v=28
  schema CHECK + `ValidArtifactKinds` slice + client `ArtifactKind`
  three-source.
- `PreviewableKinds` (3-tuple `[image_link, video_link, pdf_link]`)
  byte-identical 跟 client `PREVIEWABLE_KINDS` (vitest 双向锁定).
- https-only XSS 红线第一道 byte-identical 跟 CV-3.2 #400
  `ValidateImageLinkURL` 同源.
- Owner-only ACL byte-identical 跟 CV-1.2 #342 rollback 设计 ⑦
  channel.created_by check.

## 不在范围

- Server-side CDN 工人 (ffmpeg / ImageMagick / pdf2image) — handler 是
  只记录 URL 的中转层, 接 client / 工人 post 来的 URL; 真 CDN 集成留 v1+.
- WS push 实时刷新 preview_url (preview_url 静态 CDN, 不订阅 frame).
- preview 历史审计 UI (跟 admin-wide access 保持同一约束, 走 ADM-2 既有
  admin_actions 路径).

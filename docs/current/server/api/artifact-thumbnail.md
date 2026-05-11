# CV-3 v2 — artifact thumbnail endpoint contract (server 单一来源)

> **单一来源 pointer.** Schema in
> `packages/server-go/internal/migrations/cv_3_v2_artifact_thumbnail.go`
> (v=31). Handler in `packages/server-go/internal/api/thumbnail.go`.
> Route registration via existing `ArtifactHandler.RegisterRoutes`. Related endpoint:
> CV-2 v2 `/preview` (`packages/server-go/internal/api/preview.go`) for
> media kinds — 二端互斥.

## Why

CV-3 #408 closes the three kind enum (markdown/code/image_link). CV-2 v2
#517 closes media-kind preview thumbnails (image/video/pdf →
`preview_url`). CV-3 v2 adds the **text-kind** thumbnail endpoint —
markdown/code artifacts get a server-recorded `thumbnail_url` for list /
侧边栏 首屏快读 ("首屏快读不是浏览器内全量解码", 蓝图 §1.4). Server is a
record-only URL handoff; real CDN worker (syntax highlighting / markdown
render → 256x256 PNG) integration deferred to v1+.

## 原则 (cv-3-v2-spec.md §0)

- **① server CDN thumbnail 不 inline** — handler accepts pre-computed
  URL from worker (跟 CV-2 v2 preview.go 同 record-only URL handoff 模式).
- **② thumbnail_url MUST be https** — 复用 `auth.ValidateImageLinkURL`
  XSS 红线第一道 (跟 CV-2 v2 设计 ② + CV-3 #400 同 helper).
- **③ thumbnail_url 跟 preview_url 字段拆 (二端互斥)** —
  `ThumbnailableKinds = [markdown, code]` slice 跟 `PreviewableKinds =
  [image_link, video_link, pdf_link]` 互斥; `artifacts.thumbnail_url`
  跟 `artifacts.preview_url` 同表两列拆开 (语义分立: text 缩略 vs 媒体
  缩略).

## Schema (v=31)

`ALTER TABLE artifacts ADD COLUMN thumbnail_url TEXT` (nullable; not FK
to anything — 跟 preview_url + AP-1.1 expires_at + AP-3 org_id + AP-2
revoked_at **五连 ALTER ADD COLUMN NULL** 模式保持同一设计约束). Migration is
forward-only via `schema_migrations`. Existing rows preserve
`thumbnail_url = NULL` (历史数据 / 未生成 — server worker generates lazily
on owner POST).

Index: none (thumbnail_url 不参与查询过滤路径; client GET /artifacts/:id
拉时一起带回; 跟 preview_url 保持同一设计约束).

## Endpoint

```
POST /api/v1/artifacts/{artifactId}/thumbnail
Authorization: <session cookie>
Content-Type: application/json

{
  "thumbnail_url": "https://cdn.example/snippet-thumb.png"
}
```

ACL (owner-only 约束 ①):

- No auth user → **401 Unauthorized** (admin routes do not enter this path, ADM-0
  §1.3 红线).
- Authenticated non-owner (channel.created_by != user.ID) →
  **403 `thumbnail.not_owner`** (跟 CV-1.2 rollback + CV-2 v2 设计 ⑦
  同 path).
- Channel access defense-in-depth (`canAccessChannel`) →
  **403 `thumbnail.not_owner`**.
- Artifact missing → **404 `thumbnail.artifact_not_found`**.

Validation rules:

- Artifact kind ∉ `{markdown, code}` (= `ThumbnailableKinds` slice) →
  **400 `thumbnail.kind_not_thumbnailable`**. image_link/video_link/
  pdf_link 走 CV-2 v2 `/preview` 既有路径 (二端互斥, 设计 ③).
- `thumbnail_url` empty / unparseable / scheme ∉ {`https`} →
  **400 `thumbnail.url_must_be_https`** (scheme mismatch) or
  **400 `thumbnail.url_invalid`** (其他错). 复用
  `ValidateImageLinkURL` XSS 红线 #1 同源.

Side-effects on success (200):

- `UPDATE artifacts SET thumbnail_url = ? WHERE id = ?` (overwrite
  接受).
- 不写 system message + 不 push WS frame (跟 CV-2 v2 preview.go 保持同一设计约束
  — thumbnail 静态 CDN; client 下次 GET pull).

Response body:

```json
{
  "artifact_id": "<uuid>",
  "thumbnail_url": "https://cdn.example/snippet-thumb.png"
}
```

## GET 回填 (CV-1.2 既有 endpoint)

`GET /api/v1/artifacts/{artifactId}` 响应 body 携带 `thumbnail_url` 字段
(omitempty when NULL, 跟 `preview_url` 字段保持同一设计约束); client
`ArtifactThumbnail` component 用作 lazy `<img>` src.

## 错码字面单一来源 (跟 PreviewErrCode* + AP-1/AP-2/AP-3 const 同模式)

```go
ThumbnailErrCodeNotOwner             = "thumbnail.not_owner"
ThumbnailErrCodeURLInvalid           = "thumbnail.url_invalid"
ThumbnailErrCodeURLNotHTTPS          = "thumbnail.url_must_be_https"
ThumbnailErrCodeKindNotThumbnailable = "thumbnail.kind_not_thumbnailable"
ThumbnailErrCodeArtifactNotFound    = "thumbnail.artifact_not_found"
```

## 二端互斥 (单测锁定)

`TestCV3V22_ThumbnailableVsPreviewableMutuallyExclusive`:

- `ThumbnailableKinds ∩ PreviewableKinds = ∅`
- `ThumbnailableKinds ∪ PreviewableKinds = {markdown, code, image_link,
  video_link, pdf_link}` (五 kind 全覆盖)

跨 endpoint reject (`TestCV3V22_KindNotThumbnailable_ImageLink` +
`TestCV3V22_KindNotThumbnailable_VideoAndPDF`) byte-identical, keeping
client 路径保持一致.

## 跨 milestone byte-identical 锁定

- 跟 CV-2 v2 #517 server CDN thumbnail record-only URL handoff + ValidateImageLinkURL
  XSS 红线 + ACL check (channel.created_by) **同模式** (改 = 改
  thumbnail.go + preview.go 两处, helper 单一来源不拆).
- 跟 CV-3 #408 三 kind enum byte-identical (CV-3 v2 不扩 kind, 仅扩
  thumbnail 路径).
- 跟 AP-1.1 #493 + AP-3 #521 + AP-2 #525 + CV-2 v2 #517 **五连 ALTER
  ADD COLUMN NULL** (artifacts 表第二次 ALTER, user_permissions 三连).
- 跟 CV-1.2 #342 rollback owner-only ACL 同 path.

## 不在范围

- Server-side CDN 工人 (shiki / markdown-it server-side render) — handler
  v0 只记录 URL; 真 CDN 集成留 v1+ (跟 CV-2 v2 保持同一设计约束).
- thumbnail 实时刷新 (commit/rollback 后自动重建) — v1+ 留遗留项 (静态 CDN).
- thumbnail garbage collection / multi-size / diff 视图 thumbnail — 留 v2+.
- image_link / video_link / pdf_link thumbnail — 走 CV-2 v2 `/preview`
  既有路径 (二端互斥).

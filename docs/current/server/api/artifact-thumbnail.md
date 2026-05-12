# CV-3 v2 — artifact thumbnail endpoint contract (server single source)

> **Single-source pointer.** Schema in
> `packages/server-go/internal/migrations/cv_3_v2_artifact_thumbnail.go`
> (v=31). Handler in `packages/server-go/internal/api/thumbnail.go`.
> Route registration via existing `ArtifactHandler.RegisterRoutes`. Related endpoint:
> CV-2 v2 `/preview` (`packages/server-go/internal/api/preview.go`) for
> media kinds — mutually exclusive paths.

## Why

CV-3 #408 closes the three kind enum (markdown/code/image_link). CV-2 v2
#517 closes media-kind preview thumbnails (image/video/pdf →
`preview_url`). CV-3 v2 adds the **text-kind** thumbnail endpoint —
markdown/code artifacts get a server-recorded `thumbnail_url` for list /
sidebar first-screen quick read ("首屏快读不是浏览器内全量解码", blueprint §1.4). Server is a
record-only URL handoff; real CDN worker (syntax highlighting / markdown
render → 256x256 PNG) integration deferred to v1+.

## Principles (cv-3-v2-spec.md §0)

- **① Server CDN thumbnail is not inline** — handler accepts a precomputed
  URL from the worker (same record-only URL handoff pattern as CV-2 v2 preview.go).
- **② thumbnail_url MUST be https** — reuse `auth.ValidateImageLinkURL`
  as the first XSS check (same helper as CV-2 v2 design ② + CV-3 #400).
- **③ thumbnail_url and preview_url are separate fields (mutually exclusive paths)** —
  `ThumbnailableKinds = [markdown, code]` slice and `PreviewableKinds =
  [image_link, video_link, pdf_link]` are mutually exclusive; `artifacts.thumbnail_url`
  and `artifacts.preview_url` remain two separate columns on the same table
  (separate meanings: text thumbnail vs media thumbnail).

## Schema (v=31)

`ALTER TABLE artifacts ADD COLUMN thumbnail_url TEXT` (nullable; not FK
to anything — same design constraint as the preview_url + AP-1.1 expires_at +
AP-3 org_id + AP-2 revoked_at **five ALTER ADD COLUMN NULL** pattern). Migration is
forward-only via `schema_migrations`. Existing rows preserve
`thumbnail_url = NULL` (historical data / not generated yet; server worker generates lazily
on owner POST).

Index: none (`thumbnail_url` is not part of query filtering; client GET
/artifacts/:id receives it with the rest of the artifact, keeping the same
design constraint as `preview_url`).

## Endpoint

```
POST /api/v1/artifacts/{artifactId}/thumbnail
Authorization: <session cookie>
Content-Type: application/json

{
  "thumbnail_url": "https://cdn.example/snippet-thumb.png"
}
```

ACL (owner-only constraint ①):

- No auth user → **401 Unauthorized** (admin routes do not enter this path, ADM-0
  §1.3 红线).
- Authenticated non-owner (channel.created_by != user.ID) →
  **403 `thumbnail.not_owner`** (same path as CV-1.2 rollback + CV-2 v2 design ⑦).
- Channel access defense-in-depth (`canAccessChannel`) →
  **403 `thumbnail.not_owner`**.
- Artifact missing → **404 `thumbnail.artifact_not_found`**.

Validation rules:

- Artifact kind ∉ `{markdown, code}` (= `ThumbnailableKinds` slice) →
  **400 `thumbnail.kind_not_thumbnailable`**. image_link/video_link/
  pdf_link uses the existing CV-2 v2 `/preview` path (mutually exclusive paths, design ③).
- `thumbnail_url` empty / unparseable / scheme ∉ {`https`} →
  **400 `thumbnail.url_must_be_https`** (scheme mismatch) or
  **400 `thumbnail.url_invalid`** (other parse errors). Reuse
  `ValidateImageLinkURL` XSS guardrail #1.

Side-effects on success (200):

- `UPDATE artifacts SET thumbnail_url = ? WHERE id = ?` (overwrite
  accepted).
- Do not write a system message and do not push a WS frame (same design
  constraint as CV-2 v2 preview.go — thumbnail is static CDN data; client pulls
  it on the next GET).

Response body:

```json
{
  "artifact_id": "<uuid>",
  "thumbnail_url": "https://cdn.example/snippet-thumb.png"
}
```

## GET Backfill (existing CV-1.2 endpoint)

`GET /api/v1/artifacts/{artifactId}` response body carries the `thumbnail_url`
field (omitempty when NULL, same design constraint as `preview_url`); client
`ArtifactThumbnail` component uses it as lazy `<img>` src.

## Error-code literals as the single source

Same pattern as `PreviewErrCode*` and AP-1/AP-2/AP-3 constants.

```go
ThumbnailErrCodeNotOwner             = "thumbnail.not_owner"
ThumbnailErrCodeURLInvalid           = "thumbnail.url_invalid"
ThumbnailErrCodeURLNotHTTPS          = "thumbnail.url_must_be_https"
ThumbnailErrCodeKindNotThumbnailable = "thumbnail.kind_not_thumbnailable"
ThumbnailErrCodeArtifactNotFound    = "thumbnail.artifact_not_found"
```

## Mutually Exclusive Paths (test lock)

`TestCV3V22_ThumbnailableVsPreviewableMutuallyExclusive`:

- `ThumbnailableKinds ∩ PreviewableKinds = ∅`
- `ThumbnailableKinds ∪ PreviewableKinds = {markdown, code, image_link,
  video_link, pdf_link}` (all five kinds covered)

Cross-endpoint rejection (`TestCV3V22_KindNotThumbnailable_ImageLink` +
`TestCV3V22_KindNotThumbnailable_VideoAndPDF`) keeps literal-match checks,
so client paths remain exact.

## Cross-Milestone Byte-Identical Locks

- Same pattern as CV-2 v2 #517 server CDN thumbnail record-only URL handoff +
  ValidateImageLinkURL XSS check + ACL check (channel.created_by). Changing
  this requires changing both thumbnail.go + preview.go, while keeping the helper
  as the shared helper.
- Byte-identical with CV-3 #408 three-kind enum (CV-3 v2 does not add kinds; it
  only adds the thumbnail path).
- Same **five ALTER ADD COLUMN NULL** pattern as AP-1.1 #493 + AP-3 #521 +
  AP-2 #525 + CV-2 v2 #517 (second ALTER on artifacts table, three on
  user_permissions).
- Same path as CV-1.2 #342 rollback owner-only ACL.

## Out of Scope

- Server-side CDN worker (shiki / markdown-it server-side render) — handler
  v0 records only the URL; real CDN integration is left for v1+ (same design
  constraint as CV-2 v2).
- thumbnail real-time refresh (automatic rebuild after commit/rollback) — left
  for v1+ because the thumbnail is static CDN data.
- thumbnail garbage collection / multi-size / diff-view thumbnail — left for v2+.
- image_link / video_link / pdf_link thumbnail — use existing CV-2 v2 `/preview`
  path (mutually exclusive paths).

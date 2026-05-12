# CV-2 v2 — artifact preview endpoint contract

> **Source-of-truth pointer.** Schema in
> `packages/server-go/internal/migrations/cv_2_v2_media_preview.go` (v=28).
> Handler in `packages/server-go/internal/api/preview.go`.
> Kind enum const + validation in
> `packages/server-go/internal/api/cv_3_2_artifact_validation.go`.
> Route registration at server boot through existing `ArtifactHandler.RegisterRoutes`
> in `packages/server-go/internal/server/server.go`.

## Why

CV-1 ships markdown-only artifacts; CV-3 extends the kind enum to
markdown / code / image_link. CV-2 v2 adds multimedia previews for
`video_link` and `pdf_link` kinds plus a server-recorded `preview_url`
used for thumbnails and video posters, without dragging in heavy inline
render libraries (no video.js / hls.js / pdf.js). Server keeps the
https-only XSS validation; client renders with HTML5 native primitives.

## Principles (cv-2-v2-media-preview-spec.md §0)

| Constraint | Contract |
|---|---|
| Server CDN thumbnail, not inline media | `preview_url` is an HTTPS URL field. Server validation reuses the same source as `ValidateImageLinkURL` for validation check #1. The client uses only `<img src>` / `<video poster>` and adds no inline rendering library. |
| Native HTML5 rendering | Video uses `<video controls>`; PDF uses `<embed type="application/pdf">`. QA grep for `video.js|hls.js|dash.js|shaka-player|pdf.js|react-pdf` in package.json must return `count==0`. |
| Kind enum shares the CV-3 schema source | Migration v=28 uses a 12-step table rebuild to extend `markdown/code/image_link/video_link/pdf_link`. The schema CHECK, `ValidArtifactKinds` slice, and client `ArtifactKind` must literal match. A change requires updating all three. Do not split storage into `artifact_video` / `artifact_pdf`; sqlite_master checks for those tables should return 0 hits. |

## Schema (v=28)

| Column | Type | Notes |
|---|---|---|
| `id` ... `lock_acquired_at` | (existing CV-1.1 + CV-3.1 columns) | unchanged |
| `type` | `TEXT NOT NULL CHECK (type IN ('markdown','code','image_link','video_link','pdf_link'))` | CV-3.1 expands three kinds to five |
| `preview_url` | `TEXT NULL` | server-recorded thumbnail / poster URL (HTTPS only); NULL = historical row / not yet generated |

Index: `idx_artifacts_channel_id` is rebuilt during the 12-step DROP TABLE flow.

Migration is forward-only and idempotent through `schema_migrations`. Existing
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

ACL (owner-only constraint ①):

- No auth user → **401 Unauthorized** (admin routes do not enter this path, ADM-0
  §1.3 guardrail; admin traffic uses separate `/admin-api/*` middleware).
- Authenticated non-owner (channel.created_by != user.ID) →
  **403 `preview.not_owner`** (same path as CV-1.2 rollback design ⑦).
- Channel access defense-in-depth (`canAccessChannel`) → **403 `preview.not_owner`**.
- Artifact missing → **404 `preview.artifact_not_found`**.

Validation rules:

- Artifact kind ∉ `{image_link, video_link, pdf_link}` (= `PreviewableKinds`
  slice) → **400 `preview.kind_not_previewable`**. markdown / code keep
  the existing CV-1 head body rendering and do not require `preview_url`.
- `preview_url` empty / unparseable / scheme ∉ {`https`} →
  **400 `preview.url_must_be_https`** (scheme mismatch) or
  **400 `preview.url_invalid`** (other validation failures). Reuse the
  `ValidateImageLinkURL` source for XSS check #1; blocks
  `javascript:`, `data:`, `data:image`, `http:`, `file:`, scheme-relative
  `//host`, and empty values.

Side-effects on success (200):

- `UPDATE artifacts SET preview_url = ? WHERE id = ?` (overwrite
  is allowed; the owner may submit the value again).
- Do not write a system message (same constraint as CV-1.2 rollback design ⑦:
  no system message; owner action does not produce a broadcast event).
- Do not push a WS frame. `preview_url` points to static CDN content; the
  client retrieves it on the next GET `/api/v1/artifacts/:id`. Real-time
  refresh is out of scope for spec §3.

Response body:

```json
{
  "artifact_id": "<uuid>",
  "preview_url": "https://cdn.example/thumb.jpg"
}
```

## GET backfill (existing CV-1.2 endpoint)

`GET /api/v1/artifacts/{artifactId}` includes `preview_url` in the response body
(omitempty when NULL). Client `MediaPreview` uses it as the thumbnail-first image
src or video poster.

## Error-code literal source (same pattern as AP-1 / AP-3 consts)

```go
PreviewErrCodeNotOwner          = "preview.not_owner"
PreviewErrCodeURLInvalid        = "preview.url_invalid"
PreviewErrCodeURLNotHTTPS       = "preview.url_must_be_https"
PreviewErrCodeKindNotPreviewable = "preview.kind_not_previewable"
PreviewErrCodeArtifactNotFound  = "preview.artifact_not_found"
```

Mismatch between these consts and handler inline strings is caught at
test time by `preview_test.go` substring asserts (`preview.url_` prefix +
`preview.not_owner` / `preview.kind_not_previewable` literals must match exactly).

## Cross-milestone byte-identical locks

- 5-item enum byte-identical with `cv_2_v2_media_preview` migration v=28
  schema CHECK + `ValidArtifactKinds` slice + client `ArtifactKind`.
- `PreviewableKinds` (3-tuple `[image_link, video_link, pdf_link]`)
  byte-identical with client `PREVIEWABLE_KINDS` (bidirectional vitest lock).
- HTTPS-only XSS check #1 byte-identical with the CV-3.2 #400
  `ValidateImageLinkURL` source.
- Owner-only ACL literal match with CV-1.2 #342 rollback design ⑦
  channel.created_by check.

## Out of scope

- Server-side CDN worker (ffmpeg / ImageMagick / pdf2image). The handler only
  records the URL submitted by the client / worker; full CDN integration stays
  in v1+.
- WS push for real-time `preview_url` refresh. `preview_url` is static CDN data,
  so no frame subscription is added.
- Preview history audit UI. Keep the same constraint as admin-wide access and
  use the existing ADM-2 admin_actions path.

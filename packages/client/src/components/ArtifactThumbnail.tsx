// ArtifactThumbnail — CV-3 v2 client renderer for code/markdown artifact
// thumbnail (Phase 5+, #cv-3-v2).
//
// Spec: docs/implementation/modules/cv-3-v2-spec.md (v0, 484ec08).
// Server reference: api/thumbnail.go::ThumbnailableKinds (2-tuple [markdown, code])
// + cv_3_v2_artifact_thumbnail migration v=31 (artifacts.thumbnail_url
// TEXT NULL).
//
// Design checks (same pattern as CV-2 v2 #517 MediaPreview):
//   - ① server CDN thumbnail 不 inline — `<img loading="lazy">`, 不引入
//     html2canvas / dom-to-image / puppeteer-client / shiki client-side
//     renderer (grep 检查 package.json count==0).
//   - ② src 必 https (复用 ImageLinkRenderer.isHttpsURL XSS 红线 #1,
//     must exactly match server ValidateImageLinkURL).
//   - ③ kind allowlist — markdown / code render here; other kinds use the
//     existing CV-2 v2 MediaPreview path. THUMBNAILABLE_KINDS is checked
//     against server ThumbnailableKinds in both directions.
//
// Constraints:
//   - Do not add a large client-side renderer library (HTML5 native + server-side source of truth).
//   - 不把 non-https URL 推入 DOM.
//   - 不渲染 image_link / video_link / pdf_link (走 MediaPreview).
//
// thumbnail_url 协议 (跟 server v=31 schema): NULL = 未生成 → fallback
// `<div class="artifact-thumbnail-fallback">` 显 kind icon.
import { isHttpsURL } from './ImageLinkRenderer';

export type ArtifactThumbnailKind = 'markdown' | 'code';

export const THUMBNAILABLE_KINDS: readonly ArtifactThumbnailKind[] = [
  'markdown',
  'code',
] as const;

/** isThumbnailableKind — must exactly match server thumbnail.go::IsThumbnailableKind. */
export function isThumbnailableKind(k: string): k is ArtifactThumbnailKind {
  return (THUMBNAILABLE_KINDS as readonly string[]).includes(k);
}

const KIND_ICON: Record<ArtifactThumbnailKind, string> = {
  markdown: '📝',
  code: '💻',
};

interface Props {
  /** kind ∈ markdown / code. 其他 kind → null (走 MediaPreview / CV-2 v2). */
  kind: string;
  /** title — alt / aria-label (artifact display name). */
  title: string;
  /** thumbnail_url — server-recorded thumbnail (https only); NULL/empty = fallback. */
  thumbnailUrl?: string;
}

/**
 * ArtifactThumbnail — design ③ kind allowlist + design ① server thumbnail-first.
 *
 * 渲染规则:
 *   - kind ∈ THUMBNAILABLE_KINDS + safe https thumbnailUrl → `<img loading="lazy"
 *     class="artifact-thumbnail">` 256x256 box (CSS 盒子由 class 控制).
 *   - kind ∈ THUMBNAILABLE_KINDS but no/unsafe thumbnailUrl → fallback
 *     `<div class="artifact-thumbnail-fallback">` 显 kind icon (📝/💻).
 *   - 其他 kind → null (走 CV-2 v2 MediaPreview 既有 path).
 */
export default function ArtifactThumbnail({ kind, title, thumbnailUrl }: Props) {
  if (!isThumbnailableKind(kind)) {
    return null;
  }
  const safe = thumbnailUrl ? isHttpsURL(thumbnailUrl) : false;

  if (safe && thumbnailUrl) {
    return (
      <img
        src={thumbnailUrl}
        alt={title}
        loading="lazy"
        className="artifact-thumbnail"
        data-thumbnail-kind={kind}
        width={256}
        height={256}
      />
    );
  }

  // Fallback — kind icon + title.
  return (
    <div
      className="artifact-thumbnail-fallback"
      data-thumbnail-kind={kind}
      role="img"
      aria-label={title}
    >
      <span className="artifact-thumbnail-icon" aria-hidden="true">
        {KIND_ICON[kind]}
      </span>
    </div>
  );
}

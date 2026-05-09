// ArtifactThumbnail — CV-3 v2 client renderer for code/markdown artifact
// thumbnail (Phase 5+, #cv-3-v2).
//
// Spec: docs/implementation/modules/cv-3-v2-spec.md (战马C v0, 484ec08).
// Server 锚: api/thumbnail.go::ThumbnailableKinds (2-tuple [markdown, code])
// + cv_3_v2_artifact_thumbnail migration v=31 (artifacts.thumbnail_url
// TEXT NULL).
//
// 设计反查 (跟 CV-2 v2 #517 MediaPreview 同模式):
//   - ① server CDN thumbnail 不 inline — `<img loading="lazy">`, 不引入
//     html2canvas / dom-to-image / puppeteer-client / shiki client-side
//     renderer (grep 检查 package.json count==0).
//   - ② src 必 https (复用 ImageLinkRenderer.isHttpsURL XSS 红线 #1,
//     byte-identical 跟 server ValidateImageLinkURL 同源).
//   - ③ kind 闸 — markdown / code 分发, 其他 kind 走 CV-2 v2 MediaPreview
//     既有 path. THUMBNAILABLE_KINDS 跟 server ThumbnailableKinds 双向锁.
//
// 反约束:
//   - 不引入 client-side renderer 重 lib (HTML5 native + server-side SSOT).
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

/** isThumbnailableKind — 跟 server thumbnail.go::IsThumbnailableKind byte-identical. */
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
 * ArtifactThumbnail — 设计 ③ kind 闸 + 设计 ① server thumbnail-first.
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

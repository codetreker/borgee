// MediaPreview — CV-2 v2 client renderer for image_link / video_link /
// pdf_link kinds (Phase 5, #cv-2-v2).
//
// Spec: docs/implementation/modules/cv-2-v2-media-preview-spec.md §0 设计
// (① server CDN thumbnail 不 inline / ② HTML5 native player 不引入 video.js
// / ③ kind enum shares one schema source with CV-3 #396).
// Server reference: cv_3_2_artifact_validation.go::ValidArtifactKinds (5 items,
// exact match with cv_2_v2_media_preview migration v=27 schema CHECK) +
// preview.go::PreviewableKinds (3 项 image/video/pdf).
//
// Design checks:
//   - ② video_link 分支 — `<video controls>` HTML5 native; 不引入 video.js
//     / hls.js / dash.js / shaka-player (grep 检查 package.json count==0).
//   - ② pdf_link 分支 — `<embed type="application/pdf">` 浏览器内嵌; 不引入
//     pdf.js / react-pdf (grep 检查 package.json count==0).
//   - ② src 必 https (复用 ImageLinkRenderer.isHttpsURL XSS constraint #1;
//     must exactly match server ValidateImageLinkURL).
//   - ③ kind 三态分发 — 跟 PreviewableKinds 一致, 其它 kind 不渲染 (走 CV-1
//     既有 markdown / CV-3 既有 code 路径).
//
// Constraints checked by grep on this file path:
//   - 不接 javascript:|data:|http: src URL (XSS constraint #1 + mixed content).
//   - 不引入 video.js / hls.js / dash.js / shaka-player / pdf.js / react-pdf
//     (design ② keeps first paint lightweight instead of decoding everything in-browser).
//   - 不拆成 image / video / pdf 三个组件 (single switch in MediaPreview,
//     matching spec §1.2 "kind 分发").
//
// body / preview_url 协议 (跟 server 协议 cv-2-v2-spec §1.1):
//   - body 字段直接是 https 媒体 URL (same rule as image_link, aligned with v=27
//     migration 字段对齐).
//   - preview_url 字段 (artifacts.preview_url) 由 POST /artifacts/:id/preview
//     owner-only 设置, 仅 image / video / pdf 三 kind 用; 缺省走 body 直渲染
//     fallback (image 直接 <img src=body>, video <video src=body>, pdf
//     <embed src=body>).
import { isHttpsURL } from './ImageLinkRenderer';

export type MediaPreviewKind = 'image_link' | 'video_link' | 'pdf_link';

export const PREVIEWABLE_KINDS: readonly MediaPreviewKind[] = [
  'image_link',
  'video_link',
  'pdf_link',
] as const;

/** isPreviewableKind — must exactly match server preview.go::IsPreviewableKind. */
export function isPreviewableKind(k: string): k is MediaPreviewKind {
  return (PREVIEWABLE_KINDS as readonly string[]).includes(k);
}

interface Props {
  /** kind ∈ image_link / video_link / pdf_link. 其它 kind → null (走 CV-1/CV-3 既有 path). */
  kind: string;
  /** body 字段 — 必 https URL (媒体本体 URL). */
  body: string;
  /** title — alt / aria-label. */
  title: string;
  /** preview_url — server-recorded thumbnail (image kind uses it first; video uses it as poster). */
  previewUrl?: string;
}

/**
 * MediaPreview — kind 三态分发 (设计 ③).
 *
 * 渲染规则:
 *   - image_link → `<img loading="lazy">` + src 优先 previewUrl 后 body
 *     (thumbnail-first, 设计 ① "首屏快读").
 *   - video_link → `<video controls preload="metadata">` (HTML5 native,
 *     design ②); poster uses previewUrl when present (empty = browser default black frame).
 *   - pdf_link → `<embed type="application/pdf">` (浏览器内嵌, 设计 ②);
 *     不传 preview_url (pdf embed 不接 poster).
 *   - 其它 kind → null (走 CV-1 markdown / CV-3 code 既有 path).
 */
export default function MediaPreview({ kind, body, title, previewUrl }: Props) {
  const url = (body || '').trim();
  const safe = isHttpsURL(url);

  if (!isPreviewableKind(kind)) {
    return null;
  }
  if (!safe) {
    // 设计 ② XSS constraint #1 — 不把 non-https URL 推入 DOM.
    return (
      <div className="media-preview-invalid" data-media-kind={kind}>
        URL 不合法 (仅支持 https)
      </div>
    );
  }

  // previewUrl 也走 https constraint (跟 server preview.go::ValidateImageLinkURL
  // same validation source — server rejects it first; client is the second defense.
  const safePreview = previewUrl && isHttpsURL(previewUrl) ? previewUrl : undefined;

  if (kind === 'image_link') {
    // Design ① thumbnail-first: use preview_url when present, otherwise render body directly.
    // loading="lazy" follows the ImageLinkRenderer pattern.
    return (
      <img
        src={safePreview ?? url}
        alt={title}
        loading="lazy"
        className="media-preview-image"
        data-media-kind="image_link"
      />
    );
  }

  if (kind === 'video_link') {
    // Design ② HTML5 native; preload="metadata" saves first-paint bandwidth.
    return (
      <video
        src={url}
        poster={safePreview}
        controls
        preload="metadata"
        className="media-preview-video"
        data-media-kind="video_link"
        aria-label={title}
      />
    );
  }

  // kind === 'pdf_link' — 设计 ② <embed type="application/pdf">.
  return (
    <embed
      src={url}
      type="application/pdf"
      className="media-preview-pdf"
      data-media-kind="pdf_link"
      aria-label={title}
    />
  );
}

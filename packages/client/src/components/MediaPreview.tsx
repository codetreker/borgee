// MediaPreview — CV-2 v2 client renderer for image_link / video_link /
// pdf_link kinds (Phase 5, #cv-2-v2).
//
// Spec: docs/implementation/modules/cv-2-v2-media-preview-spec.md §0 设计
// (① server CDN thumbnail 不 inline / ② HTML5 native player 不引入 video.js
// / ③ kind enum 跟 CV-3 #396 共 schema 单源).
// Server 锚: cv_3_2_artifact_validation.go::ValidArtifactKinds (5 项, byte-
// identical 跟 cv_2_v2_media_preview migration v=27 schema CHECK 同源) +
// preview.go::PreviewableKinds (3 项 image/video/pdf).
//
// 设计反查:
//   - ② video_link 分支 — `<video controls>` HTML5 native; 不引入 video.js
//     / hls.js / dash.js / shaka-player (grep 检查 package.json count==0).
//   - ② pdf_link 分支 — `<embed type="application/pdf">` 浏览器内嵌; 不引入
//     pdf.js / react-pdf (grep 检查 package.json count==0).
//   - ② src 必 https (复用 ImageLinkRenderer.isHttpsURL XSS 红线 #1, byte-
//     identical 跟 server ValidateImageLinkURL 同源).
//   - ③ kind 三态分发 — 跟 PreviewableKinds 一致, 其它 kind 不渲染 (走 CV-1
//     既有 markdown / CV-3 既有 code 路径).
//
// 反约束 (本文件路径grep 检查 干净):
//   - 不接 javascript:|data:|http: src URL (XSS 红线 #1 + 混合内容).
//   - 不引入 video.js / hls.js / dash.js / shaka-player / pdf.js / react-pdf
//     (设计 ② "首屏快读, 不是浏览器内全量解码" 字面承袭 CV-1 设计 ④ 精神).
//   - 不裂 image / video / pdf 三组件 (单 MediaPreview 内 switch, 跟 spec
//     §1.2 "kind 分发" 字面承袭).
//
// body / preview_url 协议 (跟 server 协议 cv-2-v2-spec §1.1):
//   - body 字段直接是 https 媒体 URL (跟 image_link 同精神, 跟 v=27
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

/** isPreviewableKind — 跟 server preview.go::IsPreviewableKind byte-identical. */
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
  /** preview_url — server-recorded thumbnail (image kind 优先用; video/pdf 当 poster/fallback). */
  previewUrl?: string;
}

/**
 * MediaPreview — kind 三态分发 (设计 ③).
 *
 * 渲染规则:
 *   - image_link → `<img loading="lazy">` + src 优先 previewUrl 后 body
 *     (thumbnail-first, 设计 ① "首屏快读").
 *   - video_link → `<video controls preload="metadata">` (HTML5 native,
 *     设计 ②); poster 用 previewUrl 兜 (空 = 浏览器默认黑屏).
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
    // 设计 ② XSS 红线 #1 — 不把 non-https URL 推入 DOM.
    return (
      <div className="media-preview-invalid" data-media-kind={kind}>
        URL 不合法 (仅支持 https)
      </div>
    );
  }

  // previewUrl 也走 https 红线 (跟 server preview.go::ValidateImageLinkURL
  // byte-identical 同源 — server 已 reject, client 第二道防御).
  const safePreview = previewUrl && isHttpsURL(previewUrl) ? previewUrl : undefined;

  if (kind === 'image_link') {
    // 设计 ① thumbnail-first — preview_url 命中走 thumbnail, 否则 fall
    // back 到 body 直渲染. loading="lazy" 跟 ImageLinkRenderer 同精神.
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
    // 设计 ② HTML5 native; preload="metadata" 节省首屏带宽 (跟 lazy 同精神).
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

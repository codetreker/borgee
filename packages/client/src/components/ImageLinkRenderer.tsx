// ImageLinkRenderer — CV-3.3 client (kind='image_link') renderer.
//
// Spec: docs/implementation/modules/cv-3-spec.md §0 设计 ① +
//   §1 CV-3.2 client; 文案锁: docs/qa/cv-3-content-lock.md §1 ④⑤;
//   acceptance: docs/qa/acceptance-templates/cv-3.md §2.4 §2.5.
// Server reference: cv_3_2_artifact_validation.go::ValidImageLinkKinds +
//   ValidateImageLinkURL (#400, first https-only XSS constraint).
//
// 设计反查:
//   - ④ image 分支 — `<img loading="lazy">` exact markup (移动端
//     流量保护, 防止 eager loading); src 必 https (XSS constraint #1).
//   - ⑤ link 分支 — rel value is fixed
//     (XSS constraint #2 reverse-tab defense); target value is fixed
//     (vitest strictly assert rel 字串原样, 漏 = leak).
//
// Rules (本文件路径grep 检查 干净):
//   - 不接 javascript:|data:image|http: src URL (XSS + 混合内容)
//   - lazy 锁 (流量防御grep 检查 0 hit)
//   - blank 锁 (SPA 上下文跳走grep 检查 0 hit)
//   - 不在 link 分支渲染 <img> / image 分支渲染 <a> (strict kind split)
//
// body 协议 (跟 server validation §0 ①): body 字段直接是 https URL.
// metadata.kind ∈ ('image','link') 由 server 验完丢弃 (CV-3.2 留账
// metadata 持久化), client 当前从 body 字段以 sub-kind 默认: current implementation
// 用 url query / explicit prop 覆盖. 默认走 image (常见). 上层 (mention
// preview) 可显式传 subKind='link'. UI v0 仅渲染 image 路径; link 路径
// 由 sub-kind 控制 — 跟 spec §0 字面对齐.
import { useMemo } from 'react';

export type ImageLinkSubKind = 'image' | 'link';

interface Props {
  /** body 字段 (server 协议) — 必 https URL. Reject javascript:/data:/http:. */
  body: string;
  title: string;
  /** sub-kind 二元 — 控制 <img> vs <a> 二元分支. 缺省 image. */
  subKind?: ImageLinkSubKind;
}

/** isHttpsURL — first XSS constraint, mirrors server ValidateImageLinkURL. */
export function isHttpsURL(raw: string): boolean {
  if (!raw) return false;
  const trimmed = raw.trim();
  // Reject scheme-relative (`//host/path`) — would inherit page scheme.
  if (trimmed.startsWith('//')) return false;
  try {
    const u = new URL(trimmed);
    return u.protocol === 'https:' && u.host.length > 0;
  } catch {
    return false;
  }
}

export default function ImageLinkRenderer({ body, title, subKind = 'image' }: Props) {
  const url = (body || '').trim();
  const safe = useMemo(() => isHttpsURL(url), [url]);

  if (!safe) {
    // Reject 非 https / javascript:/data:/http: and fall back without rendering a URL.
    // (不渲染 <img>/<a>, 永远不把 unsafe URL 推入 DOM).
    return (
      <div className="artifact-image-link-invalid">
        URL 不合法 (仅支持 https)
      </div>
    );
  }

  if (subKind === 'link') {
    // 设计 ⑤ link 分支 — rel 三联锁 (vitest strictly assert
    // `expect(rel).toBe("noopener noreferrer")` 字串原样).
    return (
      <a
        href={url}
        target="_blank"
        rel="noopener noreferrer"
        className="artifact-link"
      >
        {title}
      </a>
    );
  }

  // 设计 ④ image 分支 — loading="lazy" + class artifact-image.
  return (
    <img
      src={url}
      alt={title}
      loading="lazy"
      className="artifact-image"
    />
  );
}

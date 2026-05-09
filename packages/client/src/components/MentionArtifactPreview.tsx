// MentionArtifactPreview — CV-3.3 mention 引用 preview kind 三模式.
//
// Spec: docs/implementation/modules/cv-3-spec.md §0 设计 ③ +
//   §1 CV-3.2; 文案锁: docs/qa/cv-3-content-lock.md §1 ⑥;
//   acceptance: docs/qa/acceptance-templates/cv-3.md §2.6.
//
// 三模式 byte-identical (跟 #370 ⑥ 同源):
//   - markdown: 头 80 字符 + ellipsis '…'  (隐私 + 流内噪声防御)
//   - code:     头 5 行 + 语言徽标 (跟 §1 ② byte-identical)
//   - image:    缩略图 <img loading="lazy" style="max-width: 192px">
//   - link:     回 markdown preview (二元拆死: link 不渲染 image)
//
// 容器 DOM `<span class="artifact-preview" data-artifact-kind={kind}>` 包裹.
//
// 反约束:
//   - markdown preview 不 > 80 字; code preview 不 > 5 行;
//     image preview 不 > 192px; link preview 不渲染 <img>.
//   - 不渲染 raw HTML / dangerouslySetInnerHTML body (XSS 红线).
import type { ArtifactKind } from '../lib/api';
import { normalizeLang, LANG_LABEL } from './CodeRenderer';
import { isHttpsURL } from './ImageLinkRenderer';

const MARKDOWN_PREVIEW_MAX = 80;
const CODE_PREVIEW_MAX_LINES = 5;
const IMAGE_PREVIEW_MAX_PX = 192;

interface Props {
  kind: ArtifactKind | string;
  title: string;
  /** body 字段 — markdown text / code text / https URL. */
  body: string;
  /** code 路径用 — 短码 (go/ts/...); 缺/外白名单 → 'text'. */
  language?: string;
}

export default function MentionArtifactPreview({ kind, title, body, language }: Props) {
  const normalKind: ArtifactKind | string =
    kind === 'markdown' || kind === 'code' || kind === 'image_link' ? kind : kind;

  if (normalKind === 'code') {
    const lang = normalizeLang(language);
    const lines = body.split('\n').slice(0, CODE_PREVIEW_MAX_LINES);
    return (
      <span className="artifact-preview" data-artifact-kind="code" title={title}>
        <span className="code-lang-badge" data-lang={lang}>
          {LANG_LABEL[lang]}
        </span>
        <code className="artifact-preview-code">{lines.join('\n')}</code>
      </span>
    );
  }

  if (normalKind === 'image_link') {
    const url = (body || '').trim();
    if (isHttpsURL(url)) {
      return (
        <span className="artifact-preview" data-artifact-kind="image_link" title={title}>
          <img
            src={url}
            alt={title}
            loading="lazy"
            style={{ maxWidth: `${IMAGE_PREVIEW_MAX_PX}px` }}
          />
        </span>
      );
    }
    // 非 https → 降级文案 (XSS 红线 + 不在 link 分支渲染 <img>).
    return (
      <span className="artifact-preview" data-artifact-kind="image_link" title={title}>
        {title}
      </span>
    );
  }

  // markdown (默认 + 兜底): 头 80 字符 + '…'.
  const truncated =
    body.length > MARKDOWN_PREVIEW_MAX
      ? body.slice(0, MARKDOWN_PREVIEW_MAX) + '…'
      : body;
  return (
    <span className="artifact-preview" data-artifact-kind="markdown" title={title}>
      {truncated}
    </span>
  );
}

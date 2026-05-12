// ArtifactCommentBody — CV-11.2 client: render artifact comment body as
// sanitized markdown. Reuses lib/markdown::renderMarkdown (marked + DOMPurify
// single source — same path used by DM-2.3 mention rendering and ArtifactPanel
// markdown surface). This keeps the spec §3 requirement that comment
// rendering reuse the existing markdown helper instead of adding another library.
//
// Spec: docs/implementation/modules/cv-11-spec.md §1 CV-11.2.
// Checklist: docs/qa/cv-11-stance-checklist.md §1+§4.
// Content-lock: docs/qa/cv-11-content-lock.md §1+§2.
//
// 设计反查:
//   - ① renderMarkdown 单源 — only import is from '../lib/markdown'.
//   - ④ DOM `data-cv11-comment-body` 锚 + dangerouslySetInnerHTML 仅在
//     DOMPurify 后注入 (合法路径; vitest 反向断 sanitize 真跑).

import { renderMarkdown } from '../lib/markdown';

interface ArtifactCommentBodyProps {
  body: string;
  /** Optional mention target IDs — passed through to renderMarkdown for
   *  `<@uuid>` token rendering (DM-2.3 既有 path 复用). */
  mentionedUserIds?: string[];
  /** Optional user-id → display-name map for mention chip labels. */
  userMap?: Map<string, string>;
}

export default function ArtifactCommentBody({
  body,
  mentionedUserIds,
  userMap,
}: ArtifactCommentBodyProps) {
  if (body.trim() === '') {
    return (
      <span className="cv11-comment-body-empty" data-cv11-comment-body data-empty>
        (empty)
      </span>
    );
  }
  // renderMarkdown returns DOMPurify-sanitized HTML. Injecting via
  // dangerouslySetInnerHTML is the SAME pattern used by DM-2.3 mention
  // rendering — sanitize happens INSIDE renderMarkdown, never after.
  const html = renderMarkdown(body, mentionedUserIds, userMap);
  return (
    <div
      className="cv11-comment-body"
      data-cv11-comment-body
      // eslint-disable-next-line react/no-danger -- input is DOMPurify-sanitized via lib/markdown
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}

// QuotedCommentBlock — CV-13.2 client: artifact comment quote / reference 视觉块.
//
// Spec: docs/implementation/modules/cv-13-spec.md §0+§1.
// Stance: docs/qa/cv-13-stance-checklist.md §1-§5.
// Content-lock: docs/qa/cv-13-content-lock.md §1+§2 (文案 + DOM SSOT).
//
// 设计反查 (cv-13-spec.md §0):
//   ① 0 server production code — quote 视觉纯客户端, 父 props quotedMessage
//      由 ArtifactCommentItem 父组件从既有 messages list 内存 cache lookup
//      (复用 messages.reply_to_id CV-8 #441 既有列 + 既有 fetchMessages list).
//   ② 不 import api / fetch* — props-driven 渲染, 反向锁.
//   ③ thinking 5-pattern 锁链第 9 处 — 不渲染 quoted 消息的 reasoning.
//   ④ 文案 byte-identical (content-lock §1 SSOT 单源).
//   ⑤ DOM data-attr 4 锚 byte-identical.
//
// 反约束:
//   - 不 quote chain (仅一层渲染, 不递归 quoted.quoted)
//   - 不写 sessionStorage / localStorage (纯 component state)
//   - 不走 markdown 富文本 (quote body 纯文本截断, CV-11 markdown 不入此组件)

import { useState } from 'react';
import type { Message } from '../types';

const QUOTE_PREFIX = '> ';
const AUTHOR_PREFIX = '@';
const COLLAPSED_LABEL = '展开';
const EXPANDED_LABEL = '收起';
const MISSING_FALLBACK = '(原消息已删除)';
const TRUNCATE_SUFFIX = '…';
const TRUNCATE_LENGTH = 200;

interface QuotedCommentBlockProps {
  quotedMessage: Message | null;
}

export default function QuotedCommentBlock({ quotedMessage }: QuotedCommentBlockProps) {
  const [collapsed, setCollapsed] = useState(true);

  if (!quotedMessage || quotedMessage.deleted_at) {
    return (
      <div className="cv13-quoted-block cv13-quoted-missing" data-cv13-quoted-block data-cv13-quoted-id="">
        <span className="cv13-quoted-fallback">{MISSING_FALLBACK}</span>
      </div>
    );
  }

  const fullBody = quotedMessage.content ?? '';
  const needsTruncate = fullBody.length > TRUNCATE_LENGTH;
  const displayBody =
    collapsed && needsTruncate ? fullBody.slice(0, TRUNCATE_LENGTH) + TRUNCATE_SUFFIX : fullBody;
  const authorName = quotedMessage.sender_name ?? quotedMessage.sender_id;

  return (
    <div
      className="cv13-quoted-block"
      data-cv13-quoted-block
      data-cv13-quoted-id={quotedMessage.id}
      data-cv13-collapsed={collapsed ? 'true' : 'false'}
    >
      <span className="cv13-quoted-author" data-cv13-quoted-author>
        {AUTHOR_PREFIX}
        {authorName}
      </span>
      <span className="cv13-quoted-body">
        {QUOTE_PREFIX}
        {displayBody}
      </span>
      {needsTruncate && (
        <button
          type="button"
          className="cv13-quoted-toggle"
          onClick={() => setCollapsed((v) => !v)}
          data-cv13-toggle
        >
          {collapsed ? COLLAPSED_LABEL : EXPANDED_LABEL}
        </button>
      )}
    </div>
  );
}

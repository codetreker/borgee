// ArtifactCommentItem — CV-7.2 client: per-comment edit/delete/reaction surface.
//
// Spec: docs/implementation/modules/cv-7-spec.md §1 CV-7.2.
// Stance: docs/qa/cv-7-stance-checklist.md §4 (DOM 锁).
// Content-lock: docs/qa/cv-7-content-lock.md §1 + §2 (DOM data-attr +
// 文案 byte-identical).
//
// 设计反查 (cv-7-spec.md §0):
//   - ① 走 messages 表既有 endpoint — editMessage/deleteMessage/addReaction
//     既有 client api 函数, CV-7 不开新 client api.
//   - ② owner-only — edit/delete 按钮仅 sender==current user 渲染
//     (grep 检查 `data-cv7-edit-btn` count≥1, 仅在 own comment 行渲染).
//   - ③ thinking 5-pattern 错误 server reject → client 显错码
//     `comment.thinking_subject_required` byte-identical (跟 CV-5 同字符).
//   - ④ delete confirm 文案 byte-identical "确认删除这条评论?".
//
// 反约束:
//   - 不另起 emoji picker (复用现有 message reaction unicode 集 — 默认 👍)
//   - 不渲染 edit history (forward-only — 即覆写, 无历史版本)
//   - admin god-mode 不挂 (此组件不在 admin console 路径渲染)

import { useCallback, useState } from 'react';
import { ApiError, editMessage, deleteMessage, addReaction } from '../lib/api';
import type { Message } from '../types';
import QuotedCommentBlock from './QuotedCommentBlock';

interface ArtifactCommentItemProps {
  commentId: string;
  authorId: string;
  authorRole: 'human' | 'agent';
  body: string;
  currentUserId: string;
  onChanged?: () => void;
  // CV-13.2: optional parent message for quote/reference rendering. Parent
  // component looks up via reply_to_id from in-memory messages list cache
  // (CV-8 #441 reply_to_id 列既有 + 0 server fetch).
  quotedMessage?: Message | null;
}

const DELETE_CONFIRM_TEXT = '确认删除这条评论?';

export default function ArtifactCommentItem({
  commentId,
  authorId,
  authorRole,
  body,
  currentUserId,
  onChanged,
  quotedMessage,
}: ArtifactCommentItemProps) {
  const isOwn = authorId === currentUserId;
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(body);
  const [errorCode, setErrorCode] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const onEdit = useCallback(() => {
    setDraft(body);
    setErrorCode(null);
    setEditing(true);
  }, [body]);

  const onCancel = useCallback(() => {
    setEditing(false);
    setErrorCode(null);
  }, []);

  const onSave = useCallback(async () => {
    if (draft.trim() === '') return;
    setBusy(true);
    setErrorCode(null);
    try {
      await editMessage(commentId, draft);
      setEditing(false);
      onChanged?.();
    } catch (err) {
      // CV-7 设计 ③: server 5-pattern reject 返回 errcode byte-identical CV-5.
      if (err instanceof ApiError) {
        // ApiError carries message; we surface a known code on the rejection
        // text so the e2e + vitest can byte-identical lock the literal.
        const literal = (err.message || '').includes('comment.thinking_subject_required')
          || (err.message || '').includes('thinking-only body rejected')
          ? 'comment.thinking_subject_required'
          : err.message || 'edit failed';
        setErrorCode(literal);
      } else {
        setErrorCode('edit failed');
      }
    } finally {
      setBusy(false);
    }
  }, [commentId, draft, onChanged]);

  const onDelete = useCallback(async () => {
    if (typeof window !== 'undefined' && !window.confirm(DELETE_CONFIRM_TEXT)) {
      return;
    }
    setBusy(true);
    try {
      await deleteMessage(commentId);
      onChanged?.();
    } catch {
      // best-effort; UI parent refetches
    } finally {
      setBusy(false);
    }
  }, [commentId, onChanged]);

  const onReact = useCallback(async () => {
    setBusy(true);
    try {
      await addReaction(commentId, '👍');
      onChanged?.();
    } catch {
      // ignore
    } finally {
      setBusy(false);
    }
  }, [commentId, onChanged]);

  return (
    <div className="cv7-comment-item" data-cv7-comment-id={commentId}>
      {/* CV-13.2: quote / reference 块 (parent message 来自父组件 messages
          list 内存 cache lookup, 0 server code 复用 reply_to_id CV-8 #441). */}
      {quotedMessage !== undefined && <QuotedCommentBlock quotedMessage={quotedMessage ?? null} />}
      <span className="cv7-comment-author" data-cv7-author-role={authorRole}>
        {authorRole === 'agent' ? '🤖' : '👤'} {authorId}
      </span>
      {editing ? (
        <div className="cv7-comment-edit-modal" data-cv7-edit-modal>
          <textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            disabled={busy}
            data-testid="cv7-edit-textarea"
          />
          <button
            type="button"
            onClick={() => void onSave()}
            disabled={busy || draft.trim() === ''}
            data-testid="cv7-edit-save"
          >
            保存
          </button>
          <button type="button" onClick={onCancel} disabled={busy} data-testid="cv7-edit-cancel">
            取消
          </button>
          {errorCode && (
            <span className="cv7-comment-error" data-testid="cv7-edit-error">
              {errorCode}
            </span>
          )}
        </div>
      ) : (
        <span className="cv7-comment-body">{body}</span>
      )}
      {isOwn && !editing && (
        <button
          type="button"
          data-cv7-edit-btn
          data-cv7-edit-btn-target={commentId}
          onClick={onEdit}
          disabled={busy}
        >
          编辑
        </button>
      )}
      {isOwn && !editing && (
        <button
          type="button"
          data-cv7-delete-btn
          data-cv7-delete-btn-target={commentId}
          onClick={() => void onDelete()}
          disabled={busy}
        >
          删除
        </button>
      )}
      <button
        type="button"
        data-cv7-reaction-target={commentId}
        onClick={() => void onReact()}
        disabled={busy}
      >
        👍
      </button>
    </div>
  );
}

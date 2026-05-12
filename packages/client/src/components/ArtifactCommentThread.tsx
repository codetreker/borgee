// ArtifactCommentThread — CV-8.2 client: 1-level thread render with
// collapse/expand toggle + reply input.
//
// Spec: docs/implementation/modules/cv-8-spec.md §1 CV-8.2.
// Stance: docs/qa/cv-8-stance-checklist.md §4 (DOM 锁 + 文案 byte-identical).
// Content-lock: docs/qa/cv-8-content-lock.md §1 + §2.
//
// 设计反查:
//   - ① 走 messages 表既有 endpoint — POST /api/v1/channels/{id}/messages
//     with content_type='artifact_comment' + reply_to_id (既有 sendMessage api).
//   - ④ thread depth 1 层 — replies 内不渲染 reply button (反向断 nested
//     reply 内 data-cv8-reply-target count==0).
//
// 反约束:
//   - 不开 N-deep recursion (props 不传 children, 仅 1-level)
//   - 不另起 emoji picker / mention parser (reply input 是 plain textarea)
//   - 不挂 admin-only path (此组件不在 admin console)

import { useCallback, useState } from 'react';
import { ApiError, postArtifactCommentReply } from '../lib/api';

interface ThreadReply {
  id: string;
  sender_id: string;
  sender_role?: 'human' | 'agent';
  content: string;
  reply_to_id: string | null;
  created_at: number;
}

interface ArtifactCommentThreadProps {
  parentId: string;
  channelId: string;
  replies: ThreadReply[];
  onReplyAdded?: () => void;
}

export default function ArtifactCommentThread({
  parentId,
  channelId,
  replies,
  onReplyAdded,
}: ArtifactCommentThreadProps) {
  const [collapsed, setCollapsed] = useState(true);
  const [draft, setDraft] = useState('');
  const [composing, setComposing] = useState(false);
  const [busy, setBusy] = useState(false);
  const [errorCode, setErrorCode] = useState<string | null>(null);

  const onToggle = useCallback(() => setCollapsed((c) => !c), []);

  const onSubmitReply = useCallback(async () => {
    const content = draft.trim();
    if (!content) return;
    setBusy(true);
    setErrorCode(null);
    try {
      await postArtifactCommentReply(channelId, parentId, content);
      setDraft('');
      setComposing(false);
      onReplyAdded?.();
    } catch (err) {
      if (err instanceof ApiError) {
        const m = err.message || '';
        // CV-8 设计 ③ + ④ — server byte-identical errcodes.
        const known = [
          'comment.thinking_subject_required',
          'comment.thread_depth_exceeded',
          'comment.reply_target_invalid',
        ].find((c) => m.includes(c));
        setErrorCode(known ?? m);
      } else {
        setErrorCode('reply failed');
      }
    } finally {
      setBusy(false);
    }
  }, [channelId, draft, parentId, onReplyAdded]);

  const count = replies.length;

  return (
    <div className="cv8-comment-thread" data-cv8-thread-parent={parentId}>
      {count > 0 && (
        <button
          type="button"
          className="cv8-thread-toggle"
          data-cv8-thread-toggle={parentId}
          onClick={onToggle}
        >
          {collapsed ? `▶ 显示 ${count} 条回复` : `▼ 隐藏 ${count} 条回复`}
        </button>
      )}
      {!collapsed && (
        <div className="cv8-thread-replies" data-testid="cv8-thread-replies">
          {replies.map((r) => (
            <div key={r.id} className="cv8-thread-reply" data-cv8-reply-id={r.id}>
              <span className="cv8-thread-reply-author" data-cv8-author-role={r.sender_role ?? 'human'}>
                {r.sender_role === 'agent' ? '🤖' : '👤'} {r.sender_id}
              </span>
              <span className="cv8-thread-reply-body">{r.content}</span>
              {/* 设计 ④ depth 1 层 — nested reply 内不渲染 reply button */}
            </div>
          ))}
        </div>
      )}
      {!composing ? (
        <button
          type="button"
          className="cv8-reply-btn"
          data-cv8-reply-target={parentId}
          onClick={() => setComposing(true)}
        >
          回复
        </button>
      ) : (
        <div className="cv8-reply-input" data-cv8-reply-input>
          <textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            disabled={busy}
            placeholder="写下你的回复..."
            data-testid="cv8-reply-textarea"
          />
          <button
            type="button"
            onClick={() => void onSubmitReply()}
            disabled={busy || draft.trim() === ''}
            data-testid="cv8-reply-submit"
          >
            发送
          </button>
          <button
            type="button"
            onClick={() => {
              setComposing(false);
              setDraft('');
              setErrorCode(null);
            }}
            disabled={busy}
            data-testid="cv8-reply-cancel"
          >
            取消
          </button>
          {errorCode && (
            <span className="cv8-reply-error" data-testid="cv8-reply-error">
              {errorCode}
            </span>
          )}
        </div>
      )}
    </div>
  );
}

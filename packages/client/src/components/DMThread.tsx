// DMThread.tsx — DM-6.2 DM thread reply UI.
//
// 反约束 (dm-6-content-lock.md §1+§2):
//   - <button data-testid="dm6-thread-toggle"> 折叠 toggle 文案 byte-identical:
//     展开态 `▼ 隐藏 N 条回复` / 折叠态 `▶ 显示 N 条回复`
//   - reply input data-testid="dm6-reply-input" placeholder `回复...` 2 字
//   - submit button data-testid="dm6-reply-submit" 文案 `发送` 2 字
//   - 同义词反向 reject: reply/comment/discussion/讨论/评论/评论区
//   - 空 thread (replies.length === 0) 不渲染 toggle (return null)
//   - thread depth 1 层强制 — reply 行内不渲染 sub-thread toggle
import { useState } from 'react';
import type { Message } from '../types';

interface DMThreadProps {
  parentId: string;
  replies: Message[];
  onSubmit?: (content: string, parentId: string) => Promise<void> | void;
}

export function DMThread({ parentId, replies, onSubmit }: DMThreadProps) {
  const [expanded, setExpanded] = useState(false);
  const [draft, setDraft] = useState('');
  const [busy, setBusy] = useState(false);

  // 设计 ①: 空 thread 不渲染 toggle (return null).
  if (replies.length === 0) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const content = draft.trim();
    if (!content || busy) return;
    setBusy(true);
    try {
      await onSubmit?.(content, parentId);
      setDraft('');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="dm-thread">
      <button
        type="button"
        className="dm-thread-toggle"
        data-testid="dm6-thread-toggle"
        onClick={() => setExpanded(!expanded)}
      >
        {expanded
          ? `▼ 隐藏 ${replies.length} 条回复`
          : `▶ 显示 ${replies.length} 条回复`}
      </button>

      {expanded && (
        <ul className="dm-thread-replies">
          {replies.map(r => (
            <li key={r.id} className="dm-thread-reply" data-reply-id={r.id}>
              <span className="reply-content">{r.content}</span>
            </li>
          ))}
        </ul>
      )}

      {expanded && onSubmit && (
        <form className="dm-thread-reply-form" onSubmit={handleSubmit}>
          <textarea
            className="dm-thread-reply-input"
            data-testid="dm6-reply-input"
            placeholder="回复..."
            value={draft}
            onChange={e => setDraft(e.target.value)}
            disabled={busy}
          />
          <button
            type="submit"
            data-testid="dm6-reply-submit"
            disabled={!draft.trim() || busy}
          >
            发送
          </button>
        </form>
      )}
    </div>
  );
}

export default DMThread;

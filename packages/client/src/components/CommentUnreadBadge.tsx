// CommentUnreadBadge — CV-14.2 client: artifact comment unread count badge.
//
// Spec: docs/implementation/modules/cv-14-spec.md §0+§1.
// Stance: docs/qa/cv-14-stance-checklist.md §1-§5.
// Content-lock: docs/qa/cv-14-content-lock.md §1+§2 (文案 + DOM SSOT).
//
// 设计反查 (cv-14-spec.md §0):
//   ① 0 server production code — 客户端纯订阅 useArtifactCommentAdded
//      (CV-5 #530 既有 WS hook).
//   ② 跟 CV-9 ArtifactCommentsMentionBadge 共存 — CV-14 仅 filter
//      `sender_id !== currentUserId` (不计自己发的). mention comment 同时
//      计入两 badge — 视觉非 race, mention=更强 signal, unread=总览.
//   ③ thinking 5-pattern 锁链第 10 处 (RT-3 + DM-3 + DM-4 + CV-7 + CV-8 +
//      CV-9 + CV-11 + CV-12 + CV-13 + CV-14).
//   ④ 文案 byte-identical (`${N} 条新评论` + `99+` overflow).
//   ⑤ DOM data-attr 2 锚 byte-identical.
//
// 反约束:
//   - 不 import api / fetch* (props + WS hook driven)
//   - 不写 sessionStorage / localStorage (纯 component state)
//   - admin god-mode 不挂 (此组件不在 admin console)

import { useCallback, useState } from 'react';
import { useArtifactCommentAdded } from '../hooks/useWsHubFrames';

interface CommentUnreadBadgeProps {
  currentUserId: string;
  /** Optional callback fired when the user clicks the badge — usually
   *  scrolls to the most recent comment or marks thread read. */
  onClick?: () => void;
}

export default function CommentUnreadBadge({
  currentUserId,
  onClick,
}: CommentUnreadBadgeProps) {
  const [unreadCount, setUnreadCount] = useState(0);

  // 设计 ② — filter sender_id != currentUserId. mention 走 CV-9
  // ArtifactCommentsMentionBadge (mention=更强 signal, unread=总览).
  useArtifactCommentAdded(
    useCallback(
      (frame) => {
        if (frame.sender_id !== currentUserId) {
          setUnreadCount((c) => c + 1);
        }
      },
      [currentUserId],
    ),
  );

  const handleClick = useCallback(() => {
    setUnreadCount(0);
    onClick?.();
  }, [onClick]);

  // count==0 时不渲染 (反向 vitest 锁).
  if (unreadCount === 0) {
    return null;
  }

  // 文案 byte-identical (cv-14-content-lock §1).
  const display = unreadCount > 99 ? '99+' : String(unreadCount);
  const label = `${display} 条新评论`;

  return (
    <button
      type="button"
      className="cv14-comment-unread-badge"
      data-cv14-comment-unread-badge
      data-cv14-unread-count={display}
      title={label}
      onClick={handleClick}
    >
      💬 {label}
    </button>
  );
}

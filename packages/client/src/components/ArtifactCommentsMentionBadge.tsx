// ArtifactCommentsMentionBadge — CV-9.2 client: unread mention count badge for
// artifact-comment threads. Subscribes to DM-2.2 既有 useMentionPushed hook,
// counts mentions targeting current user, renders badge with byte-identical
// 文案 "你被 @ 在 N 条评论中".
//
// Spec: docs/implementation/modules/cv-9-spec.md §1 CV-9.2.
// Stance: docs/qa/cv-9-stance-checklist.md §4.
// Content-lock: docs/qa/cv-9-content-lock.md §1 + §2.
//
// 设计反查:
//   - ① 0 server production code — 客户端纯订阅 DM-2.2 既有 mention frame.
//   - ④ 复用 useMentionPushed 既有 hook (反向断不另起 state — `useCV9MentionState`
//     在源码 0 hit). 文案 "你被 @ 在 N 条评论中" byte-identical.
//
// 反约束:
//   - 不另起 mention state (复用既有 hook)
//   - 不挂 admin-only path (此组件不在 admin console)

import { useCallback, useState } from 'react';
import { useMentionPushed } from '../hooks/useWsHubFrames';

interface ArtifactCommentsMentionBadgeProps {
  currentUserId: string;
  /** Optional callback fired when the user clicks the badge — usually
   *  scrolls to the most recent mention or opens the comments panel. */
  onClick?: () => void;
}

export default function ArtifactCommentsMentionBadge({
  currentUserId,
  onClick,
}: ArtifactCommentsMentionBadgeProps) {
  const [unreadCount, setUnreadCount] = useState(0);

  // 设计 ④ — reuse useMentionPushed hook (DM-2.2 既有). Increment counter
  // when frame.mention_target_id == currentUserId. Do NOT use frame.body_preview
  // for rendering (反约束: 隐私 §13, 80-rune cap is preview-only).
  useMentionPushed(
    useCallback(
      (frame) => {
        if (frame.mention_target_id === currentUserId) {
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

  // count==0 时不渲染 (反向 vitest 断).
  if (unreadCount === 0) {
    return null;
  }

  // 文案 byte-identical "你被 @ 在 N 条评论中" (cv-9-content-lock §2).
  const label = `你被 @ 在 ${unreadCount} 条评论中`;

  return (
    <button
      type="button"
      className="cv9-comment-mention-badge"
      data-cv9-unread-count={unreadCount}
      data-cv9-mention-toast
      title={label}
      onClick={handleClick}
    >
      🔔 {label}
    </button>
  );
}

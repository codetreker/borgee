// ReactionBar — 消息下方一排 reaction 药丸 + 末尾添加按钮.
//
// design 锚: docs/implementation/design/686-message-spacing-reaction-position.md
// - §4 没 reaction 时直接 return null 不渲染容器 (反 .reaction-bar-empty
//   占位撑容器 ~40px 这条 bug)
// - §1 picker 锚定: 末尾的 ➕ 由 <ReactionAddButton variant="inline-pill" />
//   统一管, picker state 跟 .message-actions 工具栏的 ReactionAddButton
//   实例各自独立, 不串扰
import React from 'react';
import * as api from '../lib/api';
import ReactionAddButton from './ReactionAddButton';

interface Reaction {
  emoji: string;
  count: number;
  user_ids: string[];
}

interface Props {
  reactions: Reaction[];
  channelId: string;
  messageId: string;
  currentUserId?: string;
  userMap: Map<string, string>;
}

export default function ReactionBar({ reactions, channelId, messageId, currentUserId, userMap }: Props) {
  const handleToggle = async (emoji: string) => {
    const reaction = reactions.find(r => r.emoji === emoji);
    const hasReacted = reaction?.user_ids.includes(currentUserId ?? '');
    try {
      if (hasReacted) {
        await api.removeReaction(messageId, emoji);
      } else {
        await api.addReaction(messageId, emoji);
      }
    } catch {
      // ignore — server 端会推 UPDATE_REACTIONS 走 WS, client 自正
    }
  };

  // gh#686 §4: 没 reaction 时直接不渲染容器, 不再用 .reaction-bar-empty
  // 占位撑出 ~40px. 添加表情的 ➕ 改放到 .message-actions 浮起工具栏 (跟
  // edit/delete 一组), 见 MessageItem 渲染分支.
  if (reactions.length === 0) return null;

  return (
    <div className="reaction-bar">
      {reactions.map(r => {
        const isActive = r.user_ids.includes(currentUserId ?? '');
        const names = r.user_ids.map(id => userMap.get(id) ?? id).join(', ');
        return (
          <button
            key={r.emoji}
            type="button"
            className={`reaction-pill ${isActive ? 'reaction-active' : ''}`}
            onClick={() => handleToggle(r.emoji)}
            title={names}
          >
            {r.emoji} {r.count}
          </button>
        );
      })}
      {currentUserId && (
        <ReactionAddButton
          channelId={channelId}
          messageId={messageId}
          currentUserId={currentUserId}
          variant="inline-pill"
        />
      )}
    </div>
  );
}

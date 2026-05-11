// ReactionBar — 消息下方一排 reaction 药丸 + 末尾添加按钮.
//
// design 锚: docs/implementation/design/686-message-spacing-reaction-position.md
// - §4 没 reaction 时直接 return null 不渲染容器 (反 .reaction-bar-empty
//   占位撑容器 ~40px 这条 bug)
// - §1 picker 锚定: 末尾的 ➕ 由 <ReactionAddButton variant="inline-pill" />
//   统一管, picker state 跟 .message-actions 工具栏的 ReactionAddButton
//   实例各自独立, 不串扰
// - §4 #11 乐观渲染: pill click toggle 走 ADD/REMOVE_REACTION_OPTIMISTIC
//   dispatch, 跟 ReactionAddButton 同款 — server PUT/DELETE 之前 client 立刻
//   更新 reactions array, isActive 立刻翻转 (反 e2e WS push race 不稳, 修
//   gh#716 PR #794 真 fail 真因: 完全靠 WS 整列替换 round-trip 才更新 active class)
import React from 'react';
import * as api from '../lib/api';
import { useAppContext } from '../context/AppContext';
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
  const { dispatch } = useAppContext();

  const handleToggle = async (emoji: string) => {
    if (!currentUserId) return; // 反 anonymous toggle, optimistic 需 user id 才能 dispatch
    const reaction = reactions.find(r => r.emoji === emoji);
    const hasReacted = reaction?.user_ids.includes(currentUserId);
    // ① 乐观 dispatch — 立刻在 UI 翻转 active state + count, 不等 WS round-trip
    //   (跟 ReactionAddButton §4 #11 同款乐观渲染策略; 反 e2e PR #794 真 race fail)
    dispatch({
      type: hasReacted ? 'REMOVE_REACTION_OPTIMISTIC' : 'ADD_REACTION_OPTIMISTIC',
      channelId,
      messageId,
      emoji,
      userId: currentUserId,
    });
    try {
      // ② await 服务器
      if (hasReacted) {
        await api.removeReaction(messageId, emoji);
      } else {
        await api.addReaction(messageId, emoji);
      }
      // ③ 成功 — 服务器 ack 后 WS 推 UPDATE_REACTIONS 整列替换, 不会重复
    } catch {
      // ④ 失败 — 撤回乐观 dispatch (按 user_id 撤, 反 race 误删别人)
      dispatch({
        type: hasReacted ? 'ADD_REACTION_OPTIMISTIC' : 'REMOVE_REACTION_OPTIMISTIC',
        channelId,
        messageId,
        emoji,
        userId: currentUserId,
      });
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

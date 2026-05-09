// DMMessageReactionPicker — DM-12.2 composite: ties DM-9 EmojiPickerPopover
// (add new) + DM-5 ReactionSummary (display + toggle existing) + auto-fetch
// reactions on mount + onChanged refresh chain.
//
// Spec: docs/implementation/modules/dm-12-spec.md §0+§1.
// Stance: docs/qa/dm-12-stance-checklist.md §1-§5.
// Content-lock: docs/qa/dm-12-content-lock.md §1+§2 (DM-only path lock).
//
// 设计反查 (dm-12-spec.md §0):
//   ① 0 server production code — 复用 CV-7 #535 PUT /api/v1/messages/{id}/reactions
//      + AP-4 #551 ACL gate. 跟 DM-9 #585 + DM-5 #549 同 endpoint 单源.
//   ② DM-only mounting path — 父组件 (MessageItem.tsx for DM channels) 决定
//      只在 channel.type === 'dm' 时挂此 composite (反 cross-channel mount).
//   ③ 复用 DM-9 EmojiPickerPopover (add 新 emoji) + DM-5 ReactionSummary
//      (display 既有 + toggle); 不另起组件复制功能.
//   ④ thinking 5-pattern 锁链第 12 处 (DM-9 第 11 后续) — composite 不暴露
//      reasoning, grep 检查 5 字面 0 hit.
//   ⑤ DOM data-attr 锁: data-dm12-reaction-picker (root) + delegate to
//      DM-9 data-dm9-* + DM-5 data-dm5-* 锚 (反向不重复 attr).
//
// 反约束:
//   - 不另起 emoji preset (复用 DM-9 5-emoji 单源)
//   - 不另起 reaction chip 渲染 (复用 DM-5 ReactionSummary)
//   - 不另起 fetch (用既有 getMessageReactions; onChanged 触发 refetch)
//   - 不写 sessionStorage / localStorage (纯 component state)
//   - admin god-mode 不挂

import { useCallback, useEffect, useState } from 'react';
import EmojiPickerPopover from './EmojiPickerPopover';
import ReactionSummary from './ReactionSummary';
import { getMessageReactions, type AggregatedReaction } from '../lib/api';

interface DMMessageReactionPickerProps {
  messageId: string;
  currentUserId: string;
  /** Initial reactions (e.g. from message bubble props); composite refetches
   *  on add/remove. If absent, auto-fetches on mount. */
  initialReactions?: AggregatedReaction[];
}

export default function DMMessageReactionPicker({
  messageId,
  currentUserId,
  initialReactions,
}: DMMessageReactionPickerProps) {
  const [reactions, setReactions] = useState<AggregatedReaction[]>(initialReactions ?? []);
  const [loading, setLoading] = useState(initialReactions === undefined);

  const refetch = useCallback(async () => {
    try {
      const resp = await getMessageReactions(messageId);
      setReactions(resp.reactions ?? []);
    } catch {
      // best-effort; chip + picker still functional, parent can re-trigger
    } finally {
      setLoading(false);
    }
  }, [messageId]);

  // Auto-fetch on mount if no initial reactions provided.
  useEffect(() => {
    if (initialReactions === undefined) {
      void refetch();
    }
  }, [initialReactions, refetch]);

  return (
    <div className="dm12-reaction-picker" data-dm12-reaction-picker data-dm12-loading={loading ? 'true' : 'false'}>
      {reactions.length > 0 && (
        <ReactionSummary
          messageId={messageId}
          reactions={reactions}
          currentUserId={currentUserId}
          onChanged={() => void refetch()}
        />
      )}
      <EmojiPickerPopover messageId={messageId} onChanged={() => void refetch()} />
    </div>
  );
}

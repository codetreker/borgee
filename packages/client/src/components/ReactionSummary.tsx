// ReactionSummary — DM-5.2 client: render aggregated reaction chips for a
// message. Reuses CV-7 既有 PUT/DELETE/GET reactions endpoint (lib/api.ts).
//
// Spec: docs/implementation/modules/dm-5-spec.md §1 DM-5.2.
// Stance: docs/qa/dm-5-stance-checklist.md §4.
// Content-lock: docs/qa/dm-5-content-lock.md §1+§2.
//
// 设计反查:
//   - ① 复用 CV-7 reaction endpoint 单源 (0 server code).
//   - ④ DOM `data-dm5-reaction-chip` + `data-dm5-reaction-count` +
//     `data-dm5-reaction-mine` 锚 + 文案 `{emoji} {count}` byte-identical.
//
// 反约束: 不另起 emoji picker (复用 unicode 直接发); admin god-mode 不挂.

import { useCallback, useState } from 'react';
import {
  addReaction,
  removeReaction,
  type AggregatedReaction,
} from '../lib/api';

interface ReactionSummaryProps {
  messageId: string;
  reactions: AggregatedReaction[];
  currentUserId: string;
  onChanged?: () => void;
}

export default function ReactionSummary({
  messageId,
  reactions,
  currentUserId,
  onChanged,
}: ReactionSummaryProps) {
  const [busy, setBusy] = useState(false);

  const onToggle = useCallback(
    async (chip: AggregatedReaction) => {
      const mine = chip.user_ids.includes(currentUserId);
      setBusy(true);
      try {
        if (mine) {
          await removeReaction(messageId, chip.emoji);
        } else {
          await addReaction(messageId, chip.emoji);
        }
        onChanged?.();
      } finally {
        setBusy(false);
      }
    },
    [messageId, currentUserId, onChanged],
  );

  if (reactions.length === 0) {
    return null;
  }

  return (
    <div className="dm5-reaction-summary" data-testid="dm5-reaction-summary">
      {reactions.map((chip) => {
        const mine = chip.user_ids.includes(currentUserId);
        // 设计 ④ 文案 byte-identical: `{emoji} {count}` 空格分隔.
        const label = `${chip.emoji} ${chip.count}`;
        const dataAttrs: Record<string, string | true> = {
          'data-dm5-reaction-chip': chip.emoji,
          'data-dm5-reaction-count': String(chip.count),
        };
        if (mine) {
          dataAttrs['data-dm5-reaction-mine'] = true;
        }
        return (
          <button
            key={chip.emoji}
            type="button"
            className={`dm5-reaction-chip${mine ? ' dm5-reaction-chip-mine' : ''}`}
            disabled={busy}
            onClick={() => void onToggle(chip)}
            {...dataAttrs}
          >
            {label}
          </button>
        );
      })}
    </div>
  );
}

// EmojiPickerPopover — DM-9.2 client: 5-emoji preset picker for message bubbles.
//
// Spec: docs/implementation/modules/dm-9-spec.md §0+§1.
// Stance: docs/qa/dm-9-stance-checklist.md §1-§5.
// Content-lock: docs/qa/dm-9-content-lock.md §1+§2+§3 (preset + 文案 + DOM SSOT).
//
// 设计反查 (dm-9-spec.md §0):
//   ① 0 server production code — 复用 CV-7 #535 PUT /api/v1/messages/{id}/reactions
//      + AP-4 #551 channel-member ACL gate. 反 v=39 schema 硬锁.
//   ② 5-emoji preset byte-identical 顺序 (👍 ❤️ 😄 🎉 🚀).
//   ③ thinking 5-pattern 锁链第 11 处.
//   ④ DOM 4 data-attr 锚 byte-identical.
//   ⑤ 跟 DM-5 ReactionSummary 互斥共存 — picker=加新 emoji, ReactionSummary=
//      显示既有 + toggle. picker click 后 onChanged callback 触发父组件 refetch.
//
// 反约束:
//   - 不另起 emoji unicode 集 (5 preset 字面单源)
//   - 不写 sessionStorage / localStorage (纯 component state)
//   - 不另起 fetch (只调既有 lib/api.ts::addReaction)
//   - admin god-mode 不挂 (此组件不在 admin console)

import { useCallback, useEffect, useRef, useState } from 'react';
import { addReaction } from '../lib/api';

const DM9_EMOJI_PRESET = ['👍', '❤️', '😄', '🎉', '🚀'] as const;
const TOGGLE_TITLE = '添加表情';

interface EmojiPickerPopoverProps {
  messageId: string;
  /** Optional callback fired after a reaction is added; parent refetches. */
  onChanged?: () => void;
}

export default function EmojiPickerPopover({ messageId, onChanged }: EmojiPickerPopoverProps) {
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  // Close on outside click + Escape (a11y).
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const onPick = useCallback(
    async (emoji: string) => {
      setBusy(true);
      try {
        await addReaction(messageId, emoji);
        onChanged?.();
      } catch {
        // best-effort; parent ReactionSummary 下次 refetch 自正
      } finally {
        setBusy(false);
        setOpen(false);
      }
    },
    [messageId, onChanged],
  );

  return (
    <div ref={rootRef} className="dm9-emoji-picker">
      <button
        type="button"
        className="dm9-emoji-picker-toggle"
        data-dm9-emoji-picker-toggle
        data-dm9-popover-open={open ? 'true' : 'false'}
        title={TOGGLE_TITLE}
        onClick={() => setOpen((v) => !v)}
        disabled={busy}
      >
        +
      </button>
      {open && (
        <div className="dm9-emoji-picker-popover" data-dm9-emoji-picker-popover>
          {DM9_EMOJI_PRESET.map((emoji) => (
            <button
              key={emoji}
              type="button"
              className="dm9-emoji-option"
              data-dm9-emoji-option={emoji}
              onClick={() => void onPick(emoji)}
              disabled={busy}
            >
              {emoji}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

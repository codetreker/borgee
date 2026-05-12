// DM-4.2 — useDMEdit hook
//
// Intent (per dm-4-stance-checklist.md §1+§2+§3):
//   ① Reuse the existing RT-3 fan-out path: call PATCH /api/v1/channels/{dmID}/messages/{id},
//      events table INSERT op="edit" is delivered through channel events backfill;
//      useDMSync (DM-3 #508) applies it on other devices.
//   ② Edits are a cursor-backed subset: useDMEdit only performs PATCH plus
//      optimistic update, while cursor progress stays owned by useDMSync. This
//      hook must not write a separate sessionStorage cursor.
//   ③ Extension of thinking 5-pattern constraint 3: agent edit is a content
//      revision, so this hook does not expose reasoning text.
//
// Constraints: do not subscribe to a dm-only frame and do not write
// borgee.dm4.cursor:* sessionStorage. Cursor handling reuses useDMSync DM-3.
//
// API:
//   const { editMessage, isEditing, error } = useDMEdit(dmChannelID);
//   await editMessage(messageId, "new content");

import { useCallback, useState } from 'react';
import { patchDMMessage, type DM4EditResponse } from '../lib/api';

export interface UseDMEditResult {
  /** PATCH the message; resolves with the updated message or throws. */
  editMessage: (messageID: string, content: string) => Promise<DM4EditResponse>;
  /** True while an edit request is in flight. */
  isEditing: boolean;
  /** Last error message (for toast UI), null if no error since last edit. */
  error: string | null;
}

/**
 * useDMEdit returns a stable callback for editing DM messages. Cursor
 * progress is intentionally not tracked here: useDMSync (DM-3 #508)
 * already subscribes to channel events backfill which carries
 * `message_edited` events emitted by the server PATCH path.
 *
 * Intent ② reverse assertion: this hook never reads/writes
 * `borgee.dm4.cursor:*` sessionStorage. Cursor ordering is preserved by
 * useDMSync.
 */
export function useDMEdit(dmChannelID: string): UseDMEditResult {
  const [isEditing, setIsEditing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const editMessage = useCallback(
    async (messageID: string, content: string): Promise<DM4EditResponse> => {
      if (!dmChannelID || !messageID) {
        const err = '编辑失败: 缺少 channelID 或 messageID';
        setError(err);
        throw new Error(err);
      }
      const trimmed = (content ?? '').trim();
      if (!trimmed) {
        const err = '编辑失败: 内容不能为空';
        setError(err);
        throw new Error(err);
      }
      setIsEditing(true);
      setError(null);
      try {
        const resp = await patchDMMessage(dmChannelID, messageID, trimmed);
        return resp;
      } catch (e) {
        const msg = e instanceof Error ? e.message : '编辑失败';
        setError(msg);
        throw e;
      } finally {
        setIsEditing(false);
      }
    },
    [dmChannelID],
  );

  return { editMessage, isEditing, error };
}

// visibility.ts — CHN-9 channel visibility three-way lock with server.
//
// Required constraints (chn-9-content-lock.md §3+§4):
//   - VISIBILITY_CREATOR_ONLY = 'creator_only' / VISIBILITY_MEMBERS = 'private'
//     / VISIBILITY_ORG_PUBLIC = 'public' are literal single-source values.
//   - Byte-identical with server packages/server-go/internal/api/chn_9_visibility.go::Visibility*
//     (three-way lock: server const + client const + DB literal).
//   - Existing 'public'/'private' rows remain byte-identical for backward compatibility.

export const VISIBILITY_CREATOR_ONLY = 'creator_only';
export const VISIBILITY_MEMBERS = 'private';
export const VISIBILITY_ORG_PUBLIC = 'public';

export type ChannelVisibility =
  | typeof VISIBILITY_CREATOR_ONLY
  | typeof VISIBILITY_MEMBERS
  | typeof VISIBILITY_ORG_PUBLIC;

// VisibilityLabels — UI copy is byte-identical with content-lock §1.
export const VISIBILITY_LABELS: Record<ChannelVisibility, { emoji: string; text: string }> = {
  creator_only: { emoji: '🔒', text: '仅创建者' },
  private: { emoji: '👥', text: '成员可见' },
  public: { emoji: '🌐', text: '组织内可见' },
};

// isValidVisibility reports whether the given string is one of the
// three accepted enum values. Single-source predicate is byte-identical with
// server IsValidVisibility.
export function isValidVisibility(s: string): s is ChannelVisibility {
  return s === VISIBILITY_CREATOR_ONLY || s === VISIBILITY_MEMBERS || s === VISIBILITY_ORG_PUBLIC;
}

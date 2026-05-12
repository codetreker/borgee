// mute.ts — CHN-7 mute bit double-locked with server.
//
// Required constraints (chn-7-content-lock.md §4):
//   - MUTE_BIT = 2 is the literal single source of truth.
//   - Byte-identical with server packages/server-go/internal/api/chn_7_mute.go::MuteBit
//     (two-way lock: changing one side requires changing both).
//   - collapsed bitmap: bit 0 (=1) = collapsed state (existing CHN-3),
//     bit 1 (=2) = muted state (added by CHN-7).

export const MUTE_BIT = 2;

// isMuted reports whether a user_channel_layout.collapsed bitmap value
// represents a muted channel. Single-source predicate matches server IsMuted.
export function isMuted(collapsed: number | null | undefined): boolean {
  return ((collapsed ?? 0) & MUTE_BIT) !== 0;
}

// channelDisplay — canonical display name for a channel row.
//
// bf task `welcome-name-display` (bf-wo `fix-skill-findings`).
//
// Source of truth: docs/blueprint/current/concept-model.md §10 — the system
// `#welcome` channel must render as `#welcome` regardless of its underlying
// DB row name. The server stores `channels.name = "welcome-<suffix>"` so the
// per-user system row can sit in the globally-UNIQUE `channels.name` column
// (see packages/server-go/internal/store/welcome.go `CreateWelcomeChannelForUser`).
//
// The system row is distinguished by the structural flag `channels.type === 'system'`,
// NOT by a name regex. A user-created channel literally named `welcome`,
// `welcome-team`, `welcome2026`, etc. MUST render its raw name unchanged.
//
// Admin pages intentionally bypass this helper to show the DB-truth name.
import type { Channel } from '../types';

/**
 * Return the canonical display name for a channel.
 *
 * - System welcome row (`type === 'system'`) → always `'welcome'`, suffix stripped.
 * - Every other channel → `channel.name` verbatim (no transformation).
 *
 * The system-vs-user discrimination is purely structural (the `type` flag);
 * name-shape is never consulted. This guarantees user-created channels
 * named `welcome`, `welcome-team`, or `welcome2026` are returned untouched.
 */
export function channelDisplayName(channel: Pick<Channel, 'name' | 'type'>): string {
  if (channel.type === 'system') {
    return 'welcome';
  }
  return channel.name;
}

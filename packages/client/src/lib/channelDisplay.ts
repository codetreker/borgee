// Display-layer normalization for channel names.
//
// Background: server stores the per-user onboarding channel as
// "welcome-<last 8 chars of user id>" because channels.name is globally
// UNIQUE and CM-onboarding (#1063) creates one per user. The blueprint
// (concept-model.md §10) specifies the user-facing name is "#welcome".
//
// Server contract: only channels with type='system' carry the
// "welcome-<suffix>" pattern. User-created channels are type='channel'
// or 'dm' — never 'system' — so this normalization cannot accidentally
// strip a user-intended name like "welcome-foo".
//
// Scope: display-only. DB row stays unchanged; uniqueness preserved.

import type { Channel } from '../types';

const WELCOME_DISPLAY_NAME = 'welcome';

/**
 * Returns the user-facing name for a channel.
 *
 * For system-typed channels whose stored name matches the welcome
 * suffix pattern, returns the cosmetic "welcome" label. For everything
 * else returns the raw channel.name.
 */
export function displayChannelName(channel: Pick<Channel, 'name' | 'type'>): string {
  if (channel.type === 'system' && channel.name.startsWith('welcome-')) {
    return WELCOME_DISPLAY_NAME;
  }
  return channel.name;
}

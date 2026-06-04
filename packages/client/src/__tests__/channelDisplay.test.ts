// Tests for displayChannelName — system welcome channel suffix stripping.
//
// Background: server stores per-user onboarding channel as
// "welcome-<8 chars>" (welcome.go::CreateWelcomeChannelForUser +
// migrations/cm_onboarding_welcome.go) because channels.name is
// globally UNIQUE. Blueprint concept-model.md §10 specifies the
// user-facing name is "welcome". Display layer normalizes.
//
// Safety: only type='system' channels are normalized. User-created
// channels are always type='channel' or 'dm' on the server, so a
// user typing "welcome-foo" as a regular channel name renders as
// "welcome-foo" unchanged.
import { describe, it, expect } from 'vitest';
import { displayChannelName } from '../lib/channelDisplay';

describe('displayChannelName', () => {
  it('strips suffix for system welcome-<id> channel', () => {
    const ch = { name: 'welcome-775CVW78', type: 'system' as const };
    expect(displayChannelName(ch)).toBe('welcome');
  });

  it('strips suffix for any 8-char suffix variant', () => {
    const ch = { name: 'welcome-abcdef12', type: 'system' as const };
    expect(displayChannelName(ch)).toBe('welcome');
  });

  it('returns "welcome" unchanged for bare system welcome (defensive)', () => {
    // Defensive: a system channel literally named "welcome" should
    // still display as "welcome" (the prefix check only triggers on
    // "welcome-" so the bare name passes through; cover this
    // explicitly so the contract is documented).
    const ch = { name: 'welcome', type: 'system' as const };
    expect(displayChannelName(ch)).toBe('welcome');
  });

  it('keeps user-created channel named "welcome-something" as-is', () => {
    // Server NEVER assigns type='system' to user-created channels;
    // a user-typed "welcome-foo" lands as type='channel' so the
    // display layer must not strip it.
    const ch = { name: 'welcome-foo', type: 'channel' as const };
    expect(displayChannelName(ch)).toBe('welcome-foo');
  });

  it('keeps user-created channel literally named "welcome" as-is', () => {
    const ch = { name: 'welcome', type: 'channel' as const };
    expect(displayChannelName(ch)).toBe('welcome');
  });

  it('keeps DM channel name as-is', () => {
    const ch = { name: 'welcome-anything', type: 'dm' as const };
    expect(displayChannelName(ch)).toBe('welcome-anything');
  });

  it('keeps non-welcome system channel as-is', () => {
    // Defensive: future system-typed channels with other prefixes
    // (e.g. notifications, audit) must NOT be normalized.
    const ch = { name: 'system-notifications', type: 'system' as const };
    expect(displayChannelName(ch)).toBe('system-notifications');
  });

  it('handles channel without explicit type as user channel', () => {
    // Channel.type is optional in the type definition; absent type
    // means it cannot be a system welcome channel.
    const ch = { name: 'welcome-something', type: undefined };
    expect(displayChannelName(ch)).toBe('welcome-something');
  });
});

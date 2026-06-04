import { describe, it, expect } from 'vitest';
import type { Channel } from '../types';
import { channelDisplayName } from './channelDisplay';

// bf task welcome-name-display (bf-wo fix-skill-findings)
// AC-2: helper distinguishes the system welcome row by structural flag
// (channels.type === 'system'), NOT by name regex. User-created channels
// whose name happens to start with 'welcome' MUST be untouched.
// Source of truth: docs/blueprint/current/concept-model.md §10.

function makeChannel(overrides: Partial<Channel>): Channel {
  return {
    id: 'ch_' + (overrides.name ?? 'x'),
    name: overrides.name ?? 'general',
    topic: '',
    type: overrides.type,
    visibility: overrides.visibility ?? 'private',
    created_at: 0,
    created_by: 'u_owner',
    ...overrides,
  };
}

describe('channelDisplayName — welcome-name-display (bf task)', () => {
  it('(i) system welcome row with suffixed name displays as "welcome"', () => {
    const ch = makeChannel({ type: 'system', name: 'welcome-775CVW78' });
    expect(channelDisplayName(ch)).toBe('welcome');
  });

  it('(ii) system welcome row with bare "welcome" name displays as "welcome"', () => {
    const ch = makeChannel({ type: 'system', name: 'welcome' });
    expect(channelDisplayName(ch)).toBe('welcome');
  });

  it('(iii) user-created channel literally named "welcome" displays raw name unchanged', () => {
    const ch = makeChannel({ type: 'channel', name: 'welcome' });
    expect(channelDisplayName(ch)).toBe('welcome');
  });

  it('(iv) user-created "welcome-team" must NOT be stripped to "welcome"', () => {
    const ch = makeChannel({ type: 'channel', name: 'welcome-team' });
    expect(channelDisplayName(ch)).toBe('welcome-team');
  });

  it('(v) user-created "welcome2026" displays raw name unchanged', () => {
    const ch = makeChannel({ type: 'channel', name: 'welcome2026' });
    expect(channelDisplayName(ch)).toBe('welcome2026');
  });

  it('(vi) non-welcome channel like "random-room" passes through', () => {
    const ch = makeChannel({ type: 'channel', name: 'random-room' });
    expect(channelDisplayName(ch)).toBe('random-room');
  });
});

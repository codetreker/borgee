// ChannelMembersModal-member-presence-fallback.test.tsx
//
// fix/agent-presence-online: when the WS presence cache for an agent is
// cold (just-opened modal, no WS frame mirror yet), the MemberPresence dot
// must use the REST `state` field from /api/v1/channels/:id/members as
// fallback. Without the fallback the dot normalizeState(undefined) →
// 'offline' branch ran and the agent showed a permanent gray dot even
// while the runtime was online.
//
// Mirrors Sidebar-dm-agent-presence.test.tsx (gh#922) for the channel-
// members surface.

import React from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react-dom/test-utils';
import { createRoot, type Root } from 'react-dom/client';
import type { Channel, User } from '../types';

const mockContext = vi.hoisted(() => ({
  value: null as unknown,
  fetchChannelMembers: vi.fn(),
  fetchAgents: vi.fn(),
  addChannelMember: vi.fn(),
  removeChannelMember: vi.fn(),
  updateChannel: vi.fn(),
  deleteChannel: vi.fn(),
  archiveChannel: vi.fn(),
  showToast: vi.fn(),
  // usePresence return — controlled per test: undefined means "WS cache cold".
  presenceReturn: undefined as { state?: string; reason?: string } | undefined,
}));

vi.mock('../context/AppContext', () => ({
  useAppContext: () => mockContext.value,
}));

vi.mock('../components/Toast', () => ({
  useToast: () => ({ showToast: mockContext.showToast }),
}));

vi.mock('../hooks/usePermissions', () => ({
  useCan: () => true,
}));

vi.mock('../hooks/usePresence', () => ({
  usePresence: () => mockContext.presenceReturn,
}));

vi.mock('../lib/api', () => ({
  fetchChannelMembers: mockContext.fetchChannelMembers,
  fetchAgents: mockContext.fetchAgents,
  addChannelMember: mockContext.addChannelMember,
  removeChannelMember: mockContext.removeChannelMember,
  updateChannel: mockContext.updateChannel,
  deleteChannel: mockContext.deleteChannel,
  archiveChannel: mockContext.archiveChannel,
}));

import ChannelMembersModal from '../components/ChannelMembersModal';

const currentUser: User = {
  id: 'owner-1',
  display_name: 'Owner',
  role: 'member',
  avatar_url: null,
  created_at: 1000,
};

function channel(overrides: Partial<Channel> & { id: string; name: string }): Channel {
  const { id, name, ...rest } = overrides;
  return {
    id,
    name,
    topic: '',
    type: 'channel',
    visibility: 'public',
    created_at: 1000,
    created_by: 'owner-1',
    member_count: 2,
    is_member: true,
    ...rest,
  };
}

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  mockContext.value = {
    state: {
      currentUser,
      channels: [
        channel({ id: 'team-1', name: 'team', created_by: 'owner-1', is_member: true }),
      ],
      permissions: [],
    },
    actions: { loadChannels: vi.fn() },
    dispatch: vi.fn(),
  };
  // default — no WS presence frame seen yet (cold cache).
  mockContext.presenceReturn = undefined;
});

afterEach(() => {
  act(() => {
    root.unmount();
  });
  document.body.removeChild(container);
  vi.clearAllMocks();
});

function render(node: React.ReactElement) {
  act(() => {
    root.render(node);
  });
}

async function flushPromises() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe('MemberPresence — REST fallback when WS presence cache is cold', () => {
  it('renders the agent dot as online from server-provided `state` when WS cache is undefined', async () => {
    mockContext.fetchChannelMembers.mockResolvedValue([
      {
        user_id: 'agent-1',
        display_name: 'BuildBot',
        role: 'agent',
        avatar_url: null,
        joined_at: 1010,
        state: 'online',
      },
    ]);

    render(<ChannelMembersModal channelId="team-1" onClose={() => {}} />);
    await flushPromises();

    const memberRow = container.querySelector('.member-row[data-role="agent"]');
    expect(memberRow).toBeTruthy();
    const presence = memberRow!.querySelector('[data-presence]') as HTMLElement | null;
    expect(presence).toBeTruthy();
    expect(presence!.getAttribute('data-presence')).toBe('online');
    // Dot class must be the online variant (the source of the green color).
    expect(memberRow!.querySelector('.presence-dot.presence-online')).toBeTruthy();
  });

  it('renders error reason from REST fallback when WS cache is cold', async () => {
    mockContext.fetchChannelMembers.mockResolvedValue([
      {
        user_id: 'agent-err',
        display_name: 'CrashedBot',
        role: 'agent',
        avatar_url: null,
        joined_at: 1010,
        state: 'error',
        reason: 'runtime_crashed',
      },
    ]);

    render(<ChannelMembersModal channelId="team-1" onClose={() => {}} />);
    await flushPromises();

    const presence = container.querySelector('.member-row[data-role="agent"] [data-presence]') as HTMLElement;
    expect(presence.getAttribute('data-presence')).toBe('error');
    expect(presence.getAttribute('data-reason')).toBe('runtime_crashed');
  });

  it('lets the live WS presence cache override the REST fallback', async () => {
    // Server said offline at modal-open time, but a WS presence-online frame
    // arrived since — live cache must win.
    mockContext.presenceReturn = { state: 'online' };
    mockContext.fetchChannelMembers.mockResolvedValue([
      {
        user_id: 'agent-1',
        display_name: 'BuildBot',
        role: 'agent',
        avatar_url: null,
        joined_at: 1010,
        state: 'offline',
      },
    ]);

    render(<ChannelMembersModal channelId="team-1" onClose={() => {}} />);
    await flushPromises();

    const presence = container.querySelector('.member-row[data-role="agent"] [data-presence]') as HTMLElement;
    expect(presence.getAttribute('data-presence')).toBe('online');
  });

  it('falls through to offline (gray) when neither WS cache nor REST state are present', async () => {
    mockContext.fetchChannelMembers.mockResolvedValue([
      {
        user_id: 'agent-no-state',
        display_name: 'MysteryBot',
        role: 'agent',
        avatar_url: null,
        joined_at: 1010,
        // no `state` field — pre-fix server shape, or server.State==nil branch.
      },
    ]);

    render(<ChannelMembersModal channelId="team-1" onClose={() => {}} />);
    await flushPromises();

    const presence = container.querySelector('.member-row[data-role="agent"] [data-presence]') as HTMLElement;
    expect(presence.getAttribute('data-presence')).toBe('offline');
  });
});

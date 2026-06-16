// ChannelMembersModal-rt3-human-presence.test.tsx — #971
//
// RT3PresenceDot was a LIVE orphan: it rendered nowhere. This test locks the
// mount: ChannelMembersModal renders <RT3PresenceDot> on each human
// (non-agent) member row, while agent rows keep the AL-3 PresenceDot and do
// NOT get the RT-3 dot. The RT-3 dot reflects the useRT3Presence store state
// (online/offline), fed by the WS `presence` frame mirror in useWebSocket.

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
  // usePresence (AL-3 agent cache) — cold by default.
  presenceReturn: undefined as { state?: string; reason?: string } | undefined,
  // useRT3Presence (RT-3 human store) — keyed per userID so the dot reads
  // different state per human row.
  rt3ByUser: {} as Record<string, { state: string; lastSeenAt: number } | undefined>,
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

vi.mock('../hooks/useRT3Presence', () => ({
  useRT3Presence: (userID: string | undefined) =>
    userID ? mockContext.rt3ByUser[userID] : undefined,
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
  mockContext.presenceReturn = undefined;
  mockContext.rt3ByUser = {};
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

describe('ChannelMembersModal — RT-3 human presence dot mount (#971)', () => {
  it('renders the RT-3 dot online on a human row when the store reports online', async () => {
    mockContext.rt3ByUser = {
      'human-online': { state: 'online', lastSeenAt: 1_700_000_000_000 },
    };
    mockContext.fetchChannelMembers.mockResolvedValue([
      {
        user_id: 'human-online',
        display_name: 'Alice',
        role: 'member',
        avatar_url: null,
        joined_at: 1010,
      },
    ]);

    render(<ChannelMembersModal channelId="team-1" onClose={() => {}} />);
    await flushPromises();

    const humanRow = container.querySelector('.member-row[data-role="user"]');
    expect(humanRow).toBeTruthy();
    const dot = humanRow!.querySelector('[data-rt3-presence-dot]') as HTMLElement | null;
    expect(dot).toBeTruthy();
    expect(dot!.getAttribute('data-rt3-presence-dot')).toBe('online');
    expect(dot!.getAttribute('data-rt3-cursor-user')).toBe('human-online');
    expect(dot!.getAttribute('title')).toBe('在线');
  });

  it('renders the RT-3 dot offline on a human row when the store is empty (default offline)', async () => {
    // No store entry → useRT3Presence returns undefined → dotAttr → 'offline'.
    mockContext.fetchChannelMembers.mockResolvedValue([
      {
        user_id: 'human-cold',
        display_name: 'Bob',
        role: 'member',
        avatar_url: null,
        joined_at: 1010,
      },
    ]);

    render(<ChannelMembersModal channelId="team-1" onClose={() => {}} />);
    await flushPromises();

    const dot = container.querySelector(
      '.member-row[data-role="user"] [data-rt3-presence-dot]',
    ) as HTMLElement | null;
    expect(dot).toBeTruthy();
    expect(dot!.getAttribute('data-rt3-presence-dot')).toBe('offline');
    expect(dot!.getAttribute('title')).toBe('离线');
  });

  it('does NOT render the RT-3 dot on agent rows (agents keep the AL-3 PresenceDot)', async () => {
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

    const agentRow = container.querySelector('.member-row[data-role="agent"]');
    expect(agentRow).toBeTruthy();
    // Agent row must NOT carry the RT-3 dot...
    expect(agentRow!.querySelector('[data-rt3-presence-dot]')).toBeNull();
    // ...but must keep the AL-3 agent presence dot.
    expect(agentRow!.querySelector('[data-presence]')).toBeTruthy();
  });

  it('mixed roster: human rows get the RT-3 dot, agent rows do not', async () => {
    mockContext.rt3ByUser = {
      'human-2': { state: 'online', lastSeenAt: 1_700_000_000_000 },
    };
    mockContext.fetchChannelMembers.mockResolvedValue([
      { user_id: 'human-2', display_name: 'Carol', role: 'member', avatar_url: null, joined_at: 1010 },
      { user_id: 'agent-2', display_name: 'OpsBot', role: 'agent', avatar_url: null, joined_at: 1011, state: 'online' },
    ]);

    render(<ChannelMembersModal channelId="team-1" onClose={() => {}} />);
    await flushPromises();

    const allRt3Dots = container.querySelectorAll('[data-rt3-presence-dot]');
    // Exactly one RT-3 dot total — for the single human row.
    expect(allRt3Dots.length).toBe(1);
    expect(allRt3Dots[0]!.getAttribute('data-rt3-cursor-user')).toBe('human-2');
  });
});

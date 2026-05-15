// Sidebar-dm-agent-presence.test.tsx — gh#922 regression.
//
// Locks the private-message sidebar path: an agent DM row must use the REST
// peer state from GET /api/v1/dm as its initial PresenceDot value. Without this
// fallback, the row renders the compact PresenceDot with undefined state, which
// normalizes to offline and shows a gray dot until a later WS cache write.

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import type { DmChannel } from '../types';
import { __resetPresenceStoreForTest } from '../hooks/usePresence';

const testState = vi.hoisted(() => ({
  currentChannelId: null as string | null,
  channels: [],
  dmChannels: [] as DmChannel[],
  onlineUserIds: new Set<string>(),
  currentUser: { id: 'user-owner', display_name: 'Owner', role: 'member' as const, avatar_url: null, created_at: 1 },
  permissions: [],
  channelMembersVersion: new Map<string, number>(),
}));

const testActions = vi.hoisted(() => ({
  loadDmChannels: vi.fn(async () => {}),
  selectChannel: vi.fn(),
  openDm: vi.fn(async () => {}),
  createChannel: vi.fn(),
  loadChannels: vi.fn(),
}));

vi.mock('../context/AppContext', () => ({
  useAppContext: () => ({ state: testState, actions: testActions }),
}));

vi.mock('../context/ThemeContext', () => ({
  useTheme: () => ({ theme: 'light', toggleTheme: vi.fn() }),
}));

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api');
  return {
    ...actual,
    fetchChannelMembers: vi.fn(async () => []),
    listAgentInvitations: vi.fn(async () => []),
    logout: vi.fn(async () => {}),
  };
});

vi.mock('../components/ChannelList', () => ({
  default: () => <div data-testid="channel-list-stub" />,
}));

vi.mock('../components/CreateGroupModal', () => ({
  default: () => <div data-testid="create-group-modal-stub" />,
}));

import Sidebar from '../components/Sidebar';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  __resetPresenceStoreForTest(() => 1_700_000_000_000);
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  testActions.loadDmChannels.mockClear();
  testActions.selectChannel.mockClear();
  testActions.openDm.mockClear();
  testState.currentChannelId = null;
  testState.channels = [];
  testState.onlineUserIds = new Set<string>();
  testState.dmChannels = [];
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  container?.remove();
  container = null;
  root = null;
});

function renderSidebar() {
  act(() => {
    root!.render(<Sidebar />);
  });
}

describe('Sidebar DM agent presence — gh#922', () => {
  it('uses REST peer state as the initial agent DM PresenceDot state', () => {
    testState.dmChannels = [{
      id: 'dm-agent-1',
      name: 'dm:agent-1_user-owner',
      type: 'dm',
      created_at: 1700000000000,
      peer: {
        id: 'agent-1',
        display_name: '野马(产品)',
        avatar_url: null,
        role: 'agent',
        state: 'online',
      },
      unread_count: 0,
      last_message: null,
    } as unknown as DmChannel];

    renderSidebar();

    const row = container!.querySelector('[data-role="agent"]');
    expect(row).toBeTruthy();
    const presence = row!.querySelector('[data-presence]') as HTMLElement | null;
    expect(presence).toBeTruthy();
    expect(presence!.getAttribute('data-presence')).toBe('online');
    expect(presence!.getAttribute('title')).toBe('在线');
    expect(row!.querySelector('.presence-dot.presence-online')).toBeTruthy();
  });

  it('does not let the generic online user list mask REST agent errors', () => {
    testState.onlineUserIds = new Set(['agent-1']);
    testState.dmChannels = [{
      id: 'dm-agent-1',
      name: 'dm:agent-1_user-owner',
      type: 'dm',
      created_at: 1700000000000,
      peer: {
        id: 'agent-1',
        display_name: '野马(产品)',
        avatar_url: null,
        role: 'agent',
        state: 'error',
        reason: 'runtime_crashed',
      },
      unread_count: 0,
      last_message: null,
    } as unknown as DmChannel];

    renderSidebar();

    const row = container!.querySelector('[data-role="agent"]');
    const presence = row!.querySelector('[data-presence]') as HTMLElement | null;
    expect(presence).toBeTruthy();
    expect(presence!.getAttribute('data-presence')).toBe('error');
    expect(presence!.getAttribute('data-reason')).toBe('runtime_crashed');
  });
});

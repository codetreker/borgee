import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import type { DmChannel } from '../types';

const testState = vi.hoisted(() => ({
  currentChannelId: null as string | null,
  channels: [],
  dmChannels: [] as DmChannel[],
  onlineUserIds: new Set<string>(),
  currentUser: { id: 'user-owner', display_name: 'Owner', role: 'member' as 'member' | 'agent', avatar_url: null, created_at: 1 },
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

vi.mock('../hooks/usePermissions', () => ({
  useCan: () => false,
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
import { listAgentInvitations, logout } from '../lib/api';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
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
  testState.currentUser = { id: 'user-owner', display_name: 'Owner', role: 'member', avatar_url: null, created_at: 1 };
  vi.mocked(listAgentInvitations).mockResolvedValue([]);
  vi.mocked(logout).mockResolvedValue(undefined);
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  container?.remove();
  container = null;
  root = null;
});

async function renderSidebar(props: Partial<React.ComponentProps<typeof Sidebar>> = {}) {
  await act(async () => {
    root!.render(
      <Sidebar
        onAgentsOpen={() => {}}
        onInvitationsOpen={() => {}}
        onWorkspacesOpen={() => {}}
        onRemoteNodesOpen={() => {}}
        onHelperStatusOpen={() => {}}
        onSettingsOpen={() => {}}
        onLogout={() => {}}
        {...props}
      />,
    );
  });
  await act(async () => {
    await Promise.resolve();
  });
}

function click(element: Element) {
  act(() => {
    element.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  });
}

describe('Sidebar footer primary entries — M3 task 5', () => {
  it('keeps only avatar, Agents, Workspaces, Settings, and More in the primary footer', async () => {
    await renderSidebar();

    const primary = container!.querySelector('[data-testid="sidebar-footer-primary-actions"]') as HTMLElement | null;
    expect(primary).toBeTruthy();
    expect(primary!.querySelector('.user-avatar-small')).toBeTruthy();
    expect(primary!.querySelector('[data-testid="sidebar-nav-agents"]')).toBeTruthy();
    expect(primary!.querySelector('[title="Workspaces"]')).toBeTruthy();
    expect(primary!.querySelector('[data-action="open-settings"]')).toBeTruthy();
    expect(primary!.querySelector('[data-testid="sidebar-footer-secondary-toggle"]')).toBeTruthy();

    expect(primary!.querySelector('.logout-btn')).toBeNull();
    expect(primary!.querySelector('.invitations-btn')).toBeNull();
    expect(primary!.querySelector('[title="Remote Nodes"]')).toBeNull();
    expect(primary!.querySelector('[data-action="open-helper-status"]')).toBeNull();
  });

  it('keeps secondary sidebar destinations reachable from the More menu', async () => {
    const onInvitationsOpen = vi.fn();
    const onRemoteNodesOpen = vi.fn();
    const onHelperStatusOpen = vi.fn();
    const onLogout = vi.fn();
    await renderSidebar({ onInvitationsOpen, onRemoteNodesOpen, onHelperStatusOpen, onLogout });

    click(container!.querySelector('[data-testid="sidebar-footer-secondary-toggle"]')!);
    const menu = container!.querySelector('[data-testid="sidebar-footer-secondary-menu"]')!;

    click(menu.querySelector('[data-testid="sidebar-secondary-invitations"]')!);
    expect(onInvitationsOpen).toHaveBeenCalledTimes(1);

    click(container!.querySelector('[data-testid="sidebar-footer-secondary-toggle"]')!);
    click(container!.querySelector('[data-testid="sidebar-secondary-remote-nodes"]')!);
    expect(onRemoteNodesOpen).toHaveBeenCalledTimes(1);

    click(container!.querySelector('[data-testid="sidebar-footer-secondary-toggle"]')!);
    click(container!.querySelector('[data-testid="sidebar-secondary-helper-status"]')!);
    expect(onHelperStatusOpen).toHaveBeenCalledTimes(1);

    click(container!.querySelector('[data-testid="sidebar-footer-secondary-toggle"]')!);
    await act(async () => {
      container!.querySelector('[data-testid="sidebar-secondary-logout"]')!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
      await Promise.resolve();
    });
    expect(logout).toHaveBeenCalledTimes(1);
    expect(onLogout).toHaveBeenCalledTimes(1);
  });

  it('shows pending invitation count on the primary More toggle and secondary invitations action', async () => {
    vi.mocked(listAgentInvitations).mockResolvedValue([
      { id: 'inv-1', state: 'pending' },
      { id: 'inv-2', state: 'pending' },
    ] as Awaited<ReturnType<typeof listAgentInvitations>>);
    await renderSidebar();

    expect(container!.querySelector('[data-testid="sidebar-footer-more-badge"]')?.textContent).toBe('2');

    click(container!.querySelector('[data-testid="sidebar-footer-secondary-toggle"]')!);

    const action = container!.querySelector('[data-testid="sidebar-secondary-invitations"]')!;
    expect(action.querySelector('[data-testid="invitation-bell-badge"]')?.textContent).toBe('2');
  });

  it('keeps logout reachable but does not expose owner-only footer actions to agent sessions', async () => {
    testState.currentUser = { id: 'agent-1', display_name: 'Agent', role: 'agent', avatar_url: null, created_at: 1 };
    await renderSidebar();

    const primary = container!.querySelector('[data-testid="sidebar-footer-primary-actions"]') as HTMLElement | null;
    expect(primary).toBeTruthy();
    expect(primary!.querySelector('.user-avatar-small')).toBeTruthy();
    expect(primary!.querySelector('[title="Workspaces"]')).toBeTruthy();
    expect(primary!.querySelector('[data-testid="sidebar-nav-agents"]')).toBeNull();
    expect(primary!.querySelector('[data-action="open-settings"]')).toBeNull();
    expect(primary!.querySelector('[data-testid="sidebar-footer-secondary-toggle"]')).toBeTruthy();

    click(primary!.querySelector('[data-testid="sidebar-footer-secondary-toggle"]')!);
    const menu = container!.querySelector('[data-testid="sidebar-footer-secondary-menu"]')!;
    expect(menu.querySelector('[data-testid="sidebar-secondary-logout"]')).toBeTruthy();
    expect(menu.querySelector('[data-testid="sidebar-secondary-invitations"]')).toBeNull();
    expect(menu.querySelector('[data-testid="sidebar-secondary-remote-nodes"]')).toBeNull();
    expect(menu.querySelector('[data-testid="sidebar-secondary-helper-status"]')).toBeNull();
  });
});

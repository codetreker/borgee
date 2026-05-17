import React from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react-dom/test-utils';
import { createRoot, type Root } from 'react-dom/client';
import type { Channel, User } from '../types';

const mockContext = vi.hoisted(() => ({
  value: null as unknown,
  dispatch: vi.fn(),
  fetchChannelMembers: vi.fn(),
  setChannelMemberRequireMentionPolicy: vi.fn(),
  deleteChannel: vi.fn(),
  showToast: vi.fn(),
}));

vi.mock('../context/AppContext', () => ({
  useAppContext: () => mockContext.value,
}));

vi.mock('../components/Toast', () => ({
  useToast: () => ({ showToast: mockContext.showToast }),
}));

vi.mock('../lib/api', () => ({
  fetchChannelMembers: mockContext.fetchChannelMembers,
  setChannelMemberRequireMentionPolicy: mockContext.setChannelMemberRequireMentionPolicy,
  deleteChannel: mockContext.deleteChannel,
}));

import ChannelManagementSurface from '../components/Settings/ChannelManagementSurface';
import SettingsPage from '../components/Settings/SettingsPage';

let container: HTMLDivElement;
let root: Root;

const currentUser: User = {
  id: 'user-1',
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
    created_by: 'user-1',
    member_count: 1,
    is_member: true,
    ...rest,
  };
}

function setContext(overrides: { channels: Channel[]; permissions?: any[]; currentChannelId?: string | null }) {
  mockContext.value = {
    state: {
      currentUser,
      channels: overrides.channels,
      permissions: overrides.permissions ?? [],
      currentChannelId: overrides.currentChannelId ?? null,
    },
    dispatch: mockContext.dispatch,
  };
}

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  setContext({ channels: [] });
  mockContext.fetchChannelMembers.mockResolvedValue([]);
  mockContext.setChannelMemberRequireMentionPolicy.mockResolvedValue({
    channel_id: 'created-1',
    user_id: 'agent-1',
    require_mention_policy: 'inherit',
    effective_require_mention: true,
  });
  mockContext.deleteChannel.mockResolvedValue(undefined);
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

describe('ChannelManagementSurface', () => {
  it('shows delete only on channels the user created with server delete permission', () => {
    setContext({
      permissions: [
        { id: 1, permission: 'channel.delete', scope: 'channel:created-1', granted_by: null, granted_at: 1 },
      ],
      channels: [
        channel({ id: 'created-1', name: 'created', topic: 'Owned by me', created_by: 'user-1', is_member: true }),
        channel({ id: 'joined-1', name: 'joined', topic: 'Joined by me', created_by: 'user-2', is_member: true }),
      ],
    });

    render(<ChannelManagementSurface />);

    const surface = container.querySelector('[data-testid="channel-management-surface"]');
    expect(surface).toBeTruthy();
    expect(surface?.querySelector('[data-section="created"] [data-channel-id="created-1"]')).toBeTruthy();
    expect(surface?.querySelector('[data-section="joined"] [data-channel-id="joined-1"]')).toBeTruthy();

    expect(container.querySelector('[data-action="delete"][data-channel-id="created-1"]')).toBeTruthy();
    expect(container.querySelector('[data-action="delete"][data-channel-id="joined-1"]')).toBeNull();

    // 旧的 4-action 矩阵彻底没了
    expect(container.querySelector('.channel-management-actions')).toBeNull();
    expect(container.querySelector('[data-action="leave"]')).toBeNull();
    expect(container.querySelector('[data-action="archive"]')).toBeNull();
    expect(container.querySelector('[data-action="owner-transfer"]')).toBeNull();
  });

  it('hides delete on owned channel when server permission is absent', () => {
    setContext({
      channels: [channel({ id: 'created-1', name: 'created', created_by: 'user-1', is_member: true })],
    });

    render(<ChannelManagementSurface />);

    expect(container.querySelector('[data-action="delete"]')).toBeNull();
  });

  it('hides delete on #general even when delete permission is granted', () => {
    setContext({
      permissions: [
        { id: 1, permission: 'channel.delete', scope: 'channel:general-1', granted_by: null, granted_at: 1 },
      ],
      channels: [channel({ id: 'general-1', name: 'general', created_by: 'user-1', is_member: true })],
    });

    render(<ChannelManagementSurface />);

    expect(container.querySelector('[data-action="delete"]')).toBeNull();
  });

  it('confirms then calls deleteChannel API + dispatches REMOVE_CHANNEL on confirm', async () => {
    setContext({
      permissions: [
        { id: 1, permission: 'channel.delete', scope: 'channel:created-1', granted_by: null, granted_at: 1 },
      ],
      channels: [channel({ id: 'created-1', name: 'created', created_by: 'user-1', is_member: true })],
    });

    render(<ChannelManagementSurface />);

    const btn = container.querySelector('[data-action="delete"][data-channel-id="created-1"]') as HTMLButtonElement;
    expect(btn).toBeTruthy();

    act(() => {
      btn.click();
    });

    const modal = document.querySelector('.confirm-delete-modal');
    expect(modal).toBeTruthy();
    expect(modal?.textContent).toContain('created');

    const confirm = modal?.querySelector('.btn-danger') as HTMLButtonElement;
    await act(async () => {
      confirm.click();
    });

    expect(mockContext.deleteChannel).toHaveBeenCalledWith('created-1');
    expect(mockContext.dispatch).toHaveBeenCalledWith({ type: 'REMOVE_CHANNEL', channelId: 'created-1' });
    expect(mockContext.showToast).toHaveBeenCalledWith('#created 已删除');
  });

  it('falls back to #general when deleting the currently-open channel', async () => {
    setContext({
      currentChannelId: 'created-1',
      permissions: [
        { id: 1, permission: 'channel.delete', scope: 'channel:created-1', granted_by: null, granted_at: 1 },
      ],
      channels: [
        channel({ id: 'general-1', name: 'general', created_by: 'user-0', is_member: true }),
        channel({ id: 'created-1', name: 'created', created_by: 'user-1', is_member: true }),
      ],
    });

    render(<ChannelManagementSurface />);

    const btn = container.querySelector('[data-action="delete"][data-channel-id="created-1"]') as HTMLButtonElement;
    act(() => {
      btn.click();
    });
    const confirm = document.querySelector('.confirm-delete-modal .btn-danger') as HTMLButtonElement;
    await act(async () => {
      confirm.click();
    });

    expect(mockContext.dispatch).toHaveBeenCalledWith({ type: 'REMOVE_CHANNEL', channelId: 'created-1' });
    expect(mockContext.dispatch).toHaveBeenCalledWith({ type: 'SET_CURRENT_CHANNEL', channelId: 'general-1' });
  });

  it('surfaces API failure as a toast and keeps the modal closeable', async () => {
    mockContext.deleteChannel.mockRejectedValueOnce(new Error('server boom'));
    setContext({
      permissions: [
        { id: 1, permission: 'channel.delete', scope: 'channel:created-1', granted_by: null, granted_at: 1 },
      ],
      channels: [channel({ id: 'created-1', name: 'created', created_by: 'user-1', is_member: true })],
    });

    render(<ChannelManagementSurface />);

    const btn = container.querySelector('[data-action="delete"][data-channel-id="created-1"]') as HTMLButtonElement;
    act(() => {
      btn.click();
    });
    const confirm = document.querySelector('.confirm-delete-modal .btn-danger') as HTMLButtonElement;
    await act(async () => {
      confirm.click();
    });

    expect(mockContext.showToast).toHaveBeenCalledWith('server boom');
    expect(mockContext.dispatch).not.toHaveBeenCalled();
  });

  it('auto-closes the confirm modal if the channel disappears from state mid-flight (WS race)', async () => {
    const owned = channel({ id: 'created-1', name: 'created', created_by: 'user-1', is_member: true });
    setContext({
      permissions: [
        { id: 1, permission: 'channel.delete', scope: 'channel:created-1', granted_by: null, granted_at: 1 },
      ],
      channels: [owned],
    });

    render(<ChannelManagementSurface />);
    const btn = container.querySelector('[data-action="delete"][data-channel-id="created-1"]') as HTMLButtonElement;
    act(() => {
      btn.click();
    });
    expect(document.querySelector('.confirm-delete-modal')).toBeTruthy();

    // Simulate WS event removing channel from state via parent re-render
    setContext({
      permissions: [
        { id: 1, permission: 'channel.delete', scope: 'channel:created-1', granted_by: null, granted_at: 1 },
      ],
      channels: [],
    });
    render(<ChannelManagementSurface />);

    expect(document.querySelector('.confirm-delete-modal')).toBeNull();
  });

  it('decorates the per-row delete button with channel-specific aria-label', () => {
    setContext({
      permissions: [
        { id: 1, permission: 'channel.delete', scope: 'channel:created-1', granted_by: null, granted_at: 1 },
      ],
      channels: [channel({ id: 'created-1', name: 'created', created_by: 'user-1', is_member: true })],
    });

    render(<ChannelManagementSurface />);

    const btn = container.querySelector('[data-action="delete"][data-channel-id="created-1"]') as HTMLButtonElement;
    expect(btn.getAttribute('aria-label')).toBe('删除频道 #created');
  });

  it('exposes server-owned mention delivery controls for channel agents', async () => {
    mockContext.fetchChannelMembers.mockResolvedValue([
      {
        user_id: 'agent-1',
        display_name: 'BuildBot',
        role: 'agent',
        avatar_url: null,
        joined_at: 1000,
        silent: true,
        require_mention_policy: 'inherit',
        effective_require_mention: true,
      },
      {
        user_id: 'user-2',
        display_name: 'Peer',
        role: 'member',
        avatar_url: null,
        joined_at: 1001,
      },
    ]);
    mockContext.setChannelMemberRequireMentionPolicy.mockResolvedValue({
      channel_id: 'created-1',
      user_id: 'agent-1',
      require_mention_policy: 'on',
      effective_require_mention: true,
    });
    setContext({
      permissions: [{ id: 1, permission: 'channel.manage_members', scope: 'channel:created-1', granted_by: null, granted_at: 1 }],
      channels: [channel({ id: 'created-1', name: 'created', topic: '', created_by: 'user-1', is_member: true })],
    });

    render(<ChannelManagementSurface />);

    const open = container.querySelector('[data-testid="mention-controls-toggle-created-1"]') as HTMLButtonElement;
    await act(async () => {
      open.click();
    });

    expect(mockContext.fetchChannelMembers).toHaveBeenCalledWith('created-1');
    expect(container.querySelector('[data-testid="everyone-authority-created-1"]')?.textContent).toContain('@Everyone');
    expect(container.querySelector('[data-agent-id="agent-1"]')?.textContent).toContain('BuildBot');
    expect(container.querySelector('[data-agent-id="agent-1"]')?.textContent).toContain('当前需要 @ 提及');

    const select = container.querySelector('[aria-label="BuildBot 提及策略"]') as HTMLSelectElement;
    expect(select.disabled).toBe(false);

    await act(async () => {
      select.value = 'on';
      select.dispatchEvent(new Event('change', { bubbles: true }));
    });

    expect(mockContext.setChannelMemberRequireMentionPolicy).toHaveBeenCalledWith('created-1', 'agent-1', 'on');
  });

  it('is reachable from Settings as a sibling tab next to runtime', () => {
    render(<SettingsPage onBack={() => {}} />);

    expect(container.querySelector('[data-tab="privacy"]')).toBeNull();
    const channelsTab = container.querySelector('[data-tab="channels"]') as HTMLButtonElement;
    expect(channelsTab?.textContent).toBe('频道');

    act(() => {
      channelsTab.click();
    });

    expect(container.querySelector('[data-testid="channel-management-surface"]')).toBeTruthy();
  });
});

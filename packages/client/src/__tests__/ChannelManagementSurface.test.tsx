import React from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react-dom/test-utils';
import { createRoot, type Root } from 'react-dom/client';
import type { Channel, User } from '../types';

const mockContext = vi.hoisted(() => ({
  value: null as unknown,
  fetchChannelMembers: vi.fn(),
  setChannelMemberRequireMentionPolicy: vi.fn(),
}));

vi.mock('../context/AppContext', () => ({
  useAppContext: () => mockContext.value,
}));

vi.mock('../lib/api', () => ({
  getMyAdminActions: () => Promise.resolve({ actions: [] }),
  getMyImpersonateGrant: () => Promise.resolve({ grant: null }),
  createMyImpersonateGrant: () => Promise.resolve({ grant: null }),
  revokeMyImpersonateGrant: () => Promise.resolve(),
  fetchChannelMembers: mockContext.fetchChannelMembers,
  setChannelMemberRequireMentionPolicy: mockContext.setChannelMemberRequireMentionPolicy,
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

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  mockContext.value = {
    state: {
      currentUser,
      channels: [],
      permissions: [],
    },
  };
  mockContext.fetchChannelMembers.mockResolvedValue([]);
  mockContext.setChannelMemberRequireMentionPolicy.mockResolvedValue({
    channel_id: 'created-1',
    user_id: 'agent-1',
    require_mention_policy: 'inherit',
    effective_require_mention: true,
  });
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
  it('renders created and joined-only sections with explicit allowed action rules', () => {
    mockContext.value = {
      state: {
        currentUser,
        permissions: [],
        channels: [
          channel({ id: 'created-1', name: 'created', topic: 'Owned by me', created_by: 'user-1', is_member: true }),
          channel({ id: 'joined-1', name: 'joined', topic: 'Joined by me', created_by: 'user-2', is_member: true }),
        ],
      },
    };

    render(<ChannelManagementSurface />);

    const surface = container.querySelector('[data-testid="channel-management-surface"]');
    expect(surface).toBeTruthy();
    expect(surface?.querySelector('[data-section="created"]')?.textContent).toContain('created');
    expect(surface?.querySelector('[data-section="joined"]')?.textContent).toContain('joined');
    expect(surface?.querySelector('[data-section="joined"]')?.textContent).not.toContain('created');

    const createdRow = surface?.querySelector('[data-channel-id="created-1"]');
    const joinedRow = surface?.querySelector('[data-channel-id="joined-1"]');
    expect(createdRow?.querySelector('[data-action="leave"]')?.getAttribute('data-allowed')).toBe('false');
    expect(createdRow?.querySelector('[data-action="leave"]')?.textContent).toContain('创建者不能退出自己创建的频道');
    expect(createdRow?.querySelector('[data-action="delete"]')?.getAttribute('data-allowed')).toBe('true');
    expect(createdRow?.querySelector('[data-action="archive"]')?.getAttribute('data-allowed')).toBe('true');
    expect(createdRow?.querySelector('[data-action="owner-transfer"]')?.getAttribute('data-allowed')).toBe('false');

    expect(joinedRow?.querySelector('[data-action="leave"]')?.getAttribute('data-allowed')).toBe('true');
    expect(joinedRow?.querySelector('[data-action="delete"]')?.getAttribute('data-allowed')).toBe('false');
    expect(joinedRow?.querySelector('[data-action="archive"]')?.getAttribute('data-allowed')).toBe('false');
    expect(joinedRow?.querySelector('[data-action="owner-transfer"]')?.getAttribute('data-allowed')).toBe('false');

    expect(surface?.querySelector('button[data-action]')).toBeNull();
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
    mockContext.value = {
      state: {
        currentUser,
        permissions: [{ id: 1, permission: 'channel.manage_members', scope: 'channel:created-1', granted_by: null, granted_at: 1 }],
        channels: [channel({ id: 'created-1', name: 'created', topic: '', created_by: 'user-1', is_member: true })],
      },
    };

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

  it('is reachable from Settings without replacing the privacy entry', () => {
    render(<SettingsPage onBack={() => {}} />);

    expect(container.querySelector('[data-tab="privacy"]')?.textContent).toBe('隐私');
    const channelsTab = container.querySelector('[data-tab="channels"]') as HTMLButtonElement;
    expect(channelsTab?.textContent).toBe('频道');

    act(() => {
      channelsTab.click();
    });

    expect(container.querySelector('[data-testid="channel-management-surface"]')).toBeTruthy();
  });
});

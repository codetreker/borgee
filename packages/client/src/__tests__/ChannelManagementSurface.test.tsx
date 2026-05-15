import React from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react-dom/test-utils';
import { createRoot, type Root } from 'react-dom/client';
import type { Channel, User } from '../types';

const mockContext = vi.hoisted(() => ({
  value: null as unknown,
}));

vi.mock('../context/AppContext', () => ({
  useAppContext: () => mockContext.value,
}));

vi.mock('../lib/api', () => ({
  getMyAdminActions: () => Promise.resolve({ actions: [] }),
  getMyImpersonateGrant: () => Promise.resolve({ grant: null }),
  createMyImpersonateGrant: () => Promise.resolve({ grant: null }),
  revokeMyImpersonateGrant: () => Promise.resolve(),
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
    },
  };
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
  it('renders created and joined-only sections without action controls', () => {
    mockContext.value = {
      state: {
        currentUser,
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
    expect(surface?.textContent).not.toMatch(/退出|删除|归档|转让/);
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

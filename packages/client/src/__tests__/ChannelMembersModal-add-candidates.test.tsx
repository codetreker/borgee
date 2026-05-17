// Add-member candidate list should include the caller's owned agents.
// Reverse-grep: docs/qa/chn-11-content-lock.md §2 文案锁 + server
// packages/server-go/internal/api/channels.go:605 (target.Role === 'agent'
// owner check).
//
// Bug being guarded: previously ChannelMembersModal sourced candidates
// only from #general membership; #general typically contains humans only,
// so agents the user owned never appeared in 添加成员 list ("暂无可添加成员").
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
  usePresence: () => ({ state: 'idle', reason: null }),
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
    member_count: 1,
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
        channel({ id: 'general-1', name: 'general', created_by: 'admin-0', is_member: true }),
        channel({ id: 'team-1', name: 'team', created_by: 'owner-1', is_member: true }),
      ],
      permissions: [],
    },
    actions: { loadChannels: vi.fn() },
    dispatch: vi.fn(),
  };
  mockContext.fetchChannelMembers.mockImplementation(async (channelId: string) => {
    if (channelId === 'team-1') {
      return [
        { user_id: 'owner-1', display_name: 'Owner', role: 'member', avatar_url: null, joined_at: 1000 },
      ];
    }
    if (channelId === 'general-1') {
      return [
        { user_id: 'owner-1', display_name: 'Owner', role: 'member', avatar_url: null, joined_at: 1000 },
        { user_id: 'peer-1', display_name: 'Peer', role: 'member', avatar_url: null, joined_at: 1001 },
      ];
    }
    return [];
  });
  mockContext.fetchAgents.mockResolvedValue([
    { id: 'agent-1', display_name: 'BuildBot', role: 'agent', avatar_url: null, owner_id: 'owner-1', created_at: 1010 },
    { id: 'agent-2', display_name: 'DocBot', role: 'agent', avatar_url: null, owner_id: 'owner-1', created_at: 1020 },
  ]);
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

async function clickAddMember() {
  // Wait initial member-load promise to flush.
  await act(async () => {
    await Promise.resolve();
  });
  const toggle = Array.from(container.querySelectorAll('button'))
    .find(b => b.textContent?.trim() === '添加成员') as HTMLButtonElement | undefined;
  if (!toggle) throw new Error('添加成员 button not found');
  await act(async () => {
    toggle.click();
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe('ChannelMembersModal add-member candidate list', () => {
  it('includes the caller\'s owned agents in the candidate list', async () => {
    render(<ChannelMembersModal channelId="team-1" onClose={() => {}} />);
    await clickAddMember();

    const addList = container.querySelector('.add-member-list')!;
    expect(addList).toBeTruthy();
    const names = Array.from(addList.querySelectorAll('.member-name')).map(n => n.textContent);
    expect(names).toContain('BuildBot');
    expect(names).toContain('DocBot');
    expect(names).toContain('Peer');
    expect(names).not.toContain('暂无可添加成员');
  });

  it('renders an agent badge ("Bot") next to agent candidates', async () => {
    render(<ChannelMembersModal channelId="team-1" onClose={() => {}} />);
    await clickAddMember();

    const rows = Array.from(container.querySelectorAll('.add-member-list .member-row'));
    const buildBotRow = rows.find(r => r.textContent?.includes('BuildBot'));
    expect(buildBotRow).toBeTruthy();
    expect(buildBotRow!.textContent).toContain('Bot');
  });

  it('still works when fetchAgents fails (#general members alone fill the list)', async () => {
    mockContext.fetchAgents.mockRejectedValueOnce(new Error('boom'));
    render(<ChannelMembersModal channelId="team-1" onClose={() => {}} />);
    await clickAddMember();

    const names = Array.from(container.querySelectorAll('.add-member-list .member-name')).map(n => n.textContent);
    expect(names).toContain('Peer');
    expect(names).not.toContain('BuildBot');
  });

  it('still works when #general fetch fails (agents alone fill the list)', async () => {
    mockContext.fetchChannelMembers.mockImplementation(async (channelId: string) => {
      if (channelId === 'team-1') {
        return [{ user_id: 'owner-1', display_name: 'Owner', role: 'member', avatar_url: null, joined_at: 1000 }];
      }
      throw new Error('boom');
    });
    render(<ChannelMembersModal channelId="team-1" onClose={() => {}} />);
    await clickAddMember();

    const names = Array.from(container.querySelectorAll('.add-member-list .member-name')).map(n => n.textContent);
    expect(names).toContain('BuildBot');
    expect(names).toContain('DocBot');
  });

  it('does not double-count an agent that is already a channel member', async () => {
    mockContext.fetchChannelMembers.mockImplementation(async (channelId: string) => {
      if (channelId === 'team-1') {
        return [
          { user_id: 'owner-1', display_name: 'Owner', role: 'member', avatar_url: null, joined_at: 1000 },
          { user_id: 'agent-1', display_name: 'BuildBot', role: 'agent', avatar_url: null, joined_at: 1010 },
        ];
      }
      if (channelId === 'general-1') {
        return [{ user_id: 'owner-1', display_name: 'Owner', role: 'member', avatar_url: null, joined_at: 1000 }];
      }
      return [];
    });
    render(<ChannelMembersModal channelId="team-1" onClose={() => {}} />);
    await clickAddMember();

    const names = Array.from(container.querySelectorAll('.add-member-list .member-name')).map(n => n.textContent);
    expect(names).not.toContain('BuildBot');
    expect(names).toContain('DocBot');
  });
});

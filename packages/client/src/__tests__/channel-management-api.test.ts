import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Channel } from '../types';
import { fetchChannels } from '../lib/api';
import { buildChannelManagementSections } from '../lib/channelManagement';

afterEach(() => {
  vi.unstubAllGlobals();
});

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

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

describe('channel management API/client surface', () => {
  it('preserves management metadata from the existing channel list API', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: RequestInfo | URL) => {
      expect(String(url)).toBe('/api/v1/channels');
      return jsonResponse({
        channels: [
          {
            id: 'created-1',
            name: 'ops',
            topic: 'Ops work',
            type: 'channel',
            visibility: 'private',
            created_at: 1000,
            created_by: 'user-1',
            member_count: 3,
            is_member: true,
          },
        ],
        groups: [],
      });
    }));

    const { channels } = await fetchChannels();

    expect(channels[0]).toMatchObject({
      id: 'created-1',
      created_by: 'user-1',
      is_member: true,
      visibility: 'private',
      member_count: 3,
    });
  });

  it('classifies created channels separately from joined-only channels', () => {
    const sections = buildChannelManagementSections([
      channel({ id: 'created-1', name: 'created', created_by: 'user-1', is_member: true }),
      channel({ id: 'joined-1', name: 'joined', created_by: 'user-2', is_member: true }),
      channel({ id: 'preview-1', name: 'preview', created_by: 'user-2', is_member: false }),
      channel({ id: 'dm-1', name: 'dm', type: 'dm', created_by: 'user-1', is_member: true }),
    ], 'user-1');

    expect(sections.created.map(c => c.id)).toEqual(['created-1']);
    expect(sections.joined.map(c => c.id)).toEqual(['joined-1']);
  });
});

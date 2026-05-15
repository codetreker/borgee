import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Channel } from '../types';
import { fetchChannels, sendMessage, setChannelMemberRequireMentionPolicy } from '../lib/api';
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

  it('updates agent channel mention policy through the server authority endpoint', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: RequestInfo | URL, init?: RequestInit) => {
      expect(String(url)).toBe('/api/v1/channels/ch-1/members/agent-1/require-mention');
      expect(init?.method).toBe('PUT');
      expect(JSON.parse(String(init?.body))).toEqual({ policy: 'on' });
      return jsonResponse({
        channel_id: 'ch-1',
        user_id: 'agent-1',
        require_mention_policy: 'on',
        effective_require_mention: true,
      });
    }));

    await expect(setChannelMemberRequireMentionPolicy('ch-1', 'agent-1', 'on')).resolves.toEqual({
      channel_id: 'ch-1',
      user_id: 'agent-1',
      require_mention_policy: 'on',
      effective_require_mention: true,
    });
  });

  it('does not send client-supplied mention recipient ids with messages', async () => {
    vi.stubGlobal('fetch', vi.fn(async (_url: RequestInfo | URL, init?: RequestInit) => {
      expect(JSON.parse(String(init?.body))).toEqual({
        content: 'hello <@agent-1>',
        content_type: 'text',
      });
      return jsonResponse({
        message: {
          id: 'msg-1',
          channel_id: 'ch-1',
          sender_id: 'user-1',
          content: 'hello <@agent-1>',
          content_type: 'text',
          reply_to_id: null,
          created_at: 1000,
          edited_at: null,
        },
      });
    }));

    await sendMessage('ch-1', 'hello <@agent-1>', 'text', ['agent-1']);
  });
});

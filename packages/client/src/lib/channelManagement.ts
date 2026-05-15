import type { Channel } from '../types';

export interface ChannelManagementSections {
  created: Channel[];
  joined: Channel[];
}

export function buildChannelManagementSections(
  channels: Channel[],
  currentUserId: string | null | undefined,
): ChannelManagementSections {
  if (!currentUserId) return { created: [], joined: [] };

  const manageableChannels = channels.filter(channel => channel.type !== 'dm');
  const created = manageableChannels.filter(channel => channel.created_by === currentUserId);
  const joined = manageableChannels.filter(channel => (
    channel.is_member !== false && channel.created_by !== currentUserId
  ));

  return { created, joined };
}

import type { Channel } from '../types';

export interface ChannelManagementSections {
  created: Channel[];
  joined: Channel[];
}

function isGeneralChannel(channel: Channel): boolean {
  return channel.name === 'general';
}

function isOwnedByCurrentUser(channel: Channel, currentUserId: string | null | undefined): boolean {
  return Boolean(currentUserId && channel.created_by === currentUserId);
}

function isJoined(channel: Channel): boolean {
  return channel.is_member !== false;
}

export function canLeaveChannel(channel: Channel, currentUserId: string | null | undefined): boolean {
  return Boolean(currentUserId)
    && channel.type !== 'dm'
    && isJoined(channel)
    && !isGeneralChannel(channel)
    && !isOwnedByCurrentUser(channel, currentUserId);
}

export function canDeleteChannel(
  channel: Channel,
  currentUserId: string | null | undefined,
  hasServerDeletePermission: boolean,
): boolean {
  return isOwnedByCurrentUser(channel, currentUserId)
    && !isGeneralChannel(channel)
    && channel.type !== 'dm'
    && hasServerDeletePermission;
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

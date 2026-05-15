import type { Channel } from '../types';

export type ChannelManagementActionId = 'leave' | 'delete' | 'archive' | 'owner-transfer';

export interface ChannelAllowedActionRule {
  id: ChannelManagementActionId;
  label: string;
  allowed: boolean;
  reason: string;
  destructive?: boolean;
}

export interface ChannelManagementSections {
  created: Channel[];
  joined: Channel[];
}

export interface ChannelAuthorityOptions {
  canDelete?: boolean;
  canArchive?: boolean;
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

export function buildChannelAllowedActionRules(
  channel: Channel,
  currentUserId: string | null | undefined,
  authority: ChannelAuthorityOptions = {},
): ChannelAllowedActionRule[] {
  const isOwner = isOwnedByCurrentUser(channel, currentUserId);
  const isGeneral = isGeneralChannel(channel);
  const member = isJoined(channel);
  const canDeleteByPermission = authority.canDelete ?? true;
  const canArchiveByPermission = authority.canArchive ?? true;

  const leaveReason = (() => {
    if (!currentUserId) return '当前用户未知，不能退出频道';
    if (isGeneral) return '默认频道不能退出';
    if (isOwner) return '创建者不能退出自己创建的频道';
    if (!member) return '未加入频道不能退出';
    return '可退出已加入频道';
  })();

  const deleteReason = (() => {
    if (isGeneral) return '默认频道不能删除';
    if (!isOwner) return '仅创建者可删除频道';
    if (!canDeleteByPermission) return '服务器权限不允许删除频道';
    return '创建者可删除频道';
  })();

  const archiveReason = (() => {
    if (isGeneral) return '默认频道不能归档';
    if (!isOwner) return '仅创建者可归档频道';
    if (!canArchiveByPermission) return '服务器权限不允许归档频道';
    return '创建者可归档频道';
  })();

  return [
    {
      id: 'leave',
      label: '退出',
      allowed: canLeaveChannel(channel, currentUserId),
      reason: leaveReason,
    },
    {
      id: 'delete',
      label: '删除',
      allowed: isOwner && !isGeneral && canDeleteByPermission,
      reason: deleteReason,
      destructive: true,
    },
    {
      id: 'archive',
      label: '归档',
      allowed: isOwner && !isGeneral && canArchiveByPermission,
      reason: archiveReason,
    },
    {
      id: 'owner-transfer',
      label: '转让',
      allowed: false,
      reason: '本轮不支持所有权转让',
    },
  ];
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

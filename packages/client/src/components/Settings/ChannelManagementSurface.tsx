import { useMemo } from 'react';
import { useAppContext } from '../../context/AppContext';
import type { Channel } from '../../types';
import { buildChannelAllowedActionRules, buildChannelManagementSections } from '../../lib/channelManagement';
import { useCan } from '../../hooks/usePermissions';
import ChannelMentionControls from './ChannelMentionControls';

function formatVisibility(channel: Channel): string {
  if (channel.visibility === 'private') return '私有';
  return '公开';
}

function formatMemberCount(channel: Channel): string {
  if (typeof channel.member_count !== 'number') return '成员数未知';
  return `${channel.member_count} 位成员`;
}

function ChannelRow({ channel, currentUserId }: { channel: Channel; currentUserId: string | null | undefined }) {
  const canManageMembers = useCan('channel.manage_members', channel.id);
  const canDelete = useCan('channel.delete', channel.id);
  const canArchive = useCan('channel.manage_visibility', channel.id);
  const actionRules = buildChannelAllowedActionRules(channel, currentUserId, {
    canDelete,
    canArchive,
  });

  return (
    <li className="channel-management-row" data-channel-id={channel.id}>
      <div className="channel-management-row-main">
        <span className="channel-management-name">#{channel.name}</span>
        <span className="channel-management-visibility">{formatVisibility(channel)}</span>
      </div>
      {channel.topic && <p className="channel-management-topic">{channel.topic}</p>}
      <div className="channel-management-meta">
        <span>{formatMemberCount(channel)}</span>
      </div>
      <ChannelMentionControls channelId={channel.id} canManage={canManageMembers} />
      <ul className="channel-management-actions" aria-label={`#${channel.name} 可用操作`}>
        {actionRules.map(rule => (
          <li
            key={rule.id}
            className={`channel-management-action${rule.allowed ? ' allowed' : ' unavailable'}`}
            data-action={rule.id}
            data-allowed={String(rule.allowed)}
            data-destructive={rule.destructive ? 'true' : undefined}
          >
            <span className="channel-management-action-label">{rule.label}</span>
            <span className="channel-management-action-reason">{rule.reason}</span>
          </li>
        ))}
      </ul>
    </li>
  );
}

function ChannelSection({
  title,
  emptyText,
  channels,
  section,
  currentUserId,
}: {
  title: string;
  emptyText: string;
  channels: Channel[];
  section: 'created' | 'joined';
  currentUserId: string | null | undefined;
}) {
  return (
    <section className="channel-management-section" data-section={section}>
      <h2>{title}</h2>
      {channels.length === 0 ? (
        <p className="channel-management-empty">{emptyText}</p>
      ) : (
        <ul className="channel-management-list">
          {channels.map(channel => (
            <ChannelRow key={channel.id} channel={channel} currentUserId={currentUserId} />
          ))}
        </ul>
      )}
    </section>
  );
}

export default function ChannelManagementSurface() {
  const { state } = useAppContext();
  const currentUserId = state.currentUser?.id;
  const sections = useMemo(() => buildChannelManagementSections(
    state.channels,
    currentUserId,
  ), [state.channels, currentUserId]);

  return (
    <div className="channel-management-surface" data-testid="channel-management-surface">
      <header className="channel-management-header">
        <h1>频道管理</h1>
        <p>查看频道成员、提及送达和广播提及行为。成员操作和所有权操作会在后续任务中补齐。</p>
      </header>
      <ChannelSection
        title="我创建的频道"
        emptyText="还没有你创建的频道。"
        channels={sections.created}
        section="created"
        currentUserId={currentUserId}
      />
      <ChannelSection
        title="我加入的频道"
        emptyText="还没有其它已加入频道。"
        channels={sections.joined}
        section="joined"
        currentUserId={currentUserId}
      />
    </div>
  );
}

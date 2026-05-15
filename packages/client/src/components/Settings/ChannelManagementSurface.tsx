import { useMemo } from 'react';
import { useAppContext } from '../../context/AppContext';
import type { Channel } from '../../types';
import { buildChannelManagementSections } from '../../lib/channelManagement';

function formatVisibility(channel: Channel): string {
  if (channel.visibility === 'private') return '私有';
  return '公开';
}

function formatMemberCount(channel: Channel): string {
  if (typeof channel.member_count !== 'number') return '成员数未知';
  return `${channel.member_count} 位成员`;
}

function ChannelRow({ channel }: { channel: Channel }) {
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
    </li>
  );
}

function ChannelSection({
  title,
  emptyText,
  channels,
  section,
}: {
  title: string;
  emptyText: string;
  channels: Channel[];
  section: 'created' | 'joined';
}) {
  return (
    <section className="channel-management-section" data-section={section}>
      <h2>{title}</h2>
      {channels.length === 0 ? (
        <p className="channel-management-empty">{emptyText}</p>
      ) : (
        <ul className="channel-management-list">
          {channels.map(channel => <ChannelRow key={channel.id} channel={channel} />)}
        </ul>
      )}
    </section>
  );
}

export default function ChannelManagementSurface() {
  const { state } = useAppContext();
  const sections = useMemo(() => buildChannelManagementSections(
    state.channels,
    state.currentUser?.id,
  ), [state.channels, state.currentUser?.id]);

  return (
    <div className="channel-management-surface" data-testid="channel-management-surface">
      <header className="channel-management-header">
        <h1>频道管理</h1>
        <p>查看你创建和加入的频道。成员操作和所有权操作会在后续任务中补齐。</p>
      </header>
      <ChannelSection
        title="我创建的频道"
        emptyText="还没有你创建的频道。"
        channels={sections.created}
        section="created"
      />
      <ChannelSection
        title="我加入的频道"
        emptyText="还没有其它已加入频道。"
        channels={sections.joined}
        section="joined"
      />
    </div>
  );
}

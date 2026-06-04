import { useCallback, useEffect, useMemo, useState } from 'react';
import { useAppContext } from '../../context/AppContext';
import { useToast } from '../Toast';
import type { Channel } from '../../types';
import { buildChannelManagementSections, canDeleteChannel } from '../../lib/channelManagement';
import { displayChannelName } from '../../lib/channelDisplay';
import { useCan } from '../../hooks/usePermissions';
import { deleteChannel } from '../../lib/api';
import ChannelMentionControls from './ChannelMentionControls';
import ConfirmDeleteModal from '../ConfirmDeleteModal';

function formatVisibility(channel: Channel): string {
  if (channel.visibility === 'private') return '私有';
  return '公开';
}

function formatMemberCount(channel: Channel): string {
  if (typeof channel.member_count !== 'number') return '成员数未知';
  return `${channel.member_count} 位成员`;
}

function ChannelRow({
  channel,
  currentUserId,
  onRequestDelete,
}: {
  channel: Channel;
  currentUserId: string | null | undefined;
  onRequestDelete: (channel: Channel) => void;
}) {
  const canManageMembers = useCan('channel.manage_members', channel.id);
  const canDeleteServer = useCan('channel.delete', channel.id);
  const deletable = canDeleteChannel(channel, currentUserId, canDeleteServer);

  return (
    <li className="channel-management-row" data-channel-id={channel.id}>
      <div className="channel-management-row-header">
        <div className="channel-management-row-main">
          <span className="channel-management-name">#{displayChannelName(channel)}</span>
          <span className="channel-management-visibility">{formatVisibility(channel)}</span>
          <span className="channel-management-meta">{formatMemberCount(channel)}</span>
        </div>
        {deletable && (
          <button
            type="button"
            className="btn btn-sm btn-danger channel-management-delete"
            data-action="delete"
            data-channel-id={channel.id}
            aria-label={`删除频道 #${displayChannelName(channel)}`}
            onClick={() => onRequestDelete(channel)}
          >
            删除
          </button>
        )}
      </div>
      {channel.topic && <p className="channel-management-topic">{channel.topic}</p>}
      <ChannelMentionControls channelId={channel.id} canManage={canManageMembers} />
    </li>
  );
}

function ChannelSection({
  title,
  emptyText,
  channels,
  section,
  currentUserId,
  onRequestDelete,
}: {
  title: string;
  emptyText: string;
  channels: Channel[];
  section: 'created' | 'joined';
  currentUserId: string | null | undefined;
  onRequestDelete: (channel: Channel) => void;
}) {
  return (
    <section className="channel-management-section" data-section={section}>
      <h2>{title}</h2>
      {channels.length === 0 ? (
        <p className="channel-management-empty">{emptyText}</p>
      ) : (
        <ul className="channel-management-list">
          {channels.map(channel => (
            <ChannelRow
              key={channel.id}
              channel={channel}
              currentUserId={currentUserId}
              onRequestDelete={onRequestDelete}
            />
          ))}
        </ul>
      )}
    </section>
  );
}

export default function ChannelManagementSurface() {
  const { state, dispatch } = useAppContext();
  const { showToast } = useToast();
  const currentUserId = state.currentUser?.id;
  const sections = useMemo(() => buildChannelManagementSections(
    state.channels,
    currentUserId,
  ), [state.channels, currentUserId]);

  const [pendingDelete, setPendingDelete] = useState<Channel | null>(null);
  const [deleting, setDeleting] = useState(false);

  // 如果 pendingDelete 频道被外部 (WS channel_deleted 事件 / 别的 tab) 从 state
  // 里删了, 自动关掉 modal — 否则 confirm 会拿过期 id 调 API 撞 404.
  useEffect(() => {
    if (!pendingDelete) return;
    const stillExists = state.channels.some(c => c.id === pendingDelete.id);
    if (!stillExists) {
      setPendingDelete(null);
      setDeleting(false);
    }
  }, [pendingDelete, state.channels]);

  const handleRequestDelete = useCallback((channel: Channel) => {
    setPendingDelete(channel);
  }, []);

  const handleConfirmDelete = useCallback(async () => {
    if (!pendingDelete) return;
    const target = pendingDelete;
    setDeleting(true);
    try {
      await deleteChannel(target.id);
      const general = state.channels.find(c => c.name === 'general');
      dispatch({ type: 'REMOVE_CHANNEL', channelId: target.id });
      if (general && state.currentChannelId === target.id) {
        dispatch({ type: 'SET_CURRENT_CHANNEL', channelId: general.id });
      }
      showToast(`#${displayChannelName(target)} 已删除`);
      setPendingDelete(null);
    } catch (err) {
      showToast(err instanceof Error ? err.message : '删除失败');
    } finally {
      setDeleting(false);
    }
  }, [pendingDelete, dispatch, showToast, state.channels, state.currentChannelId]);

  const handleCancelDelete = useCallback(() => {
    if (deleting) return;
    setPendingDelete(null);
  }, [deleting]);

  return (
    <div className="channel-management-surface" data-testid="channel-management-surface">
      <header className="channel-management-header">
        <h1>频道管理</h1>
        <p>查看你创建或加入的频道；创建者可以删除自己的频道 (soft delete)。</p>
      </header>
      <ChannelSection
        title="我创建的频道"
        emptyText="还没有你创建的频道。"
        channels={sections.created}
        section="created"
        currentUserId={currentUserId}
        onRequestDelete={handleRequestDelete}
      />
      <ChannelSection
        title="我加入的频道"
        emptyText="还没有其它已加入频道。"
        channels={sections.joined}
        section="joined"
        currentUserId={currentUserId}
        onRequestDelete={handleRequestDelete}
      />
      {pendingDelete && (
        <ConfirmDeleteModal
          channelName={pendingDelete.name}
          onConfirm={handleConfirmDelete}
          onCancel={handleCancelDelete}
          loading={deleting}
        />
      )}
    </div>
  );
}

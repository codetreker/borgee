import React, { useEffect, useState } from 'react';
import { useAppContext } from '../context/AppContext';
import { leaveChannel, joinChannel, fetchChannelPreview, listCommands } from '../lib/api';
import { commandRegistry } from '../commands/registry';
import type { RemoteCommand } from '../commands/registry';
import MessageList from './MessageList';
import MessageInput from './MessageInput';
import ConnectionStatus from './ConnectionStatus';
import ChannelMembersModal from './ChannelMembersModal';
import WorkspacePanel from './WorkspacePanel';
import RemotePanel from './RemotePanel';
import ArtifactPanel from './ArtifactPanel';
import { useVisualViewport } from '../hooks/useVisualViewport';
import type { Message } from '../types';
import { canLeaveChannel } from '../lib/channelManagement';
import { channelDisplayName } from '../lib/channelDisplay';

interface Props {
  channelId: string;
}

// CHN-4 (#374) — URL `?tab=` deep-link 跟 server default_tab="chat" 同源.
// channel.type IN ('private','public') 才生效; DM 视图永不渲染 tab 切换器
// (CHN-2 设计 ② 7 源 byte-identical 锁), 故 DM 路径不走此函数.
//
// 反约束: 'remote' / 'canvas' 仍是 v0 既有 tab (CHN-1 #288 字面承袭),
// 但 deep-link 仅锁 chat / workspace 二态 (CHN-4 文案锁 §1 ① + spec
// brief #375 §0 ② byte-identical) — 其它 tab 走 client state 默认.
type TabKey = 'chat' | 'workspace' | 'remote' | 'canvas';
const VALID_DEEPLINK_TABS: ReadonlyArray<TabKey> = ['chat', 'workspace'];

function initialTabFromURL(): TabKey {
  if (typeof window === 'undefined') return 'chat';
  const params = new URLSearchParams(window.location.search);
  const raw = params.get('tab');
  if (raw && (VALID_DEEPLINK_TABS as readonly string[]).includes(raw)) {
    return raw as TabKey;
  }
  // server default_tab="chat" 字面 byte-identical (#374 设计 ⑥, content-lock §1 ④).
  return 'chat';
}

function syncURLTab(tab: TabKey): void {
  if (typeof window === 'undefined') return;
  const url = new URL(window.location.href);
  if ((VALID_DEEPLINK_TABS as readonly string[]).includes(tab)) {
    url.searchParams.set('tab', tab);
  } else {
    // remote / canvas 不入 URL — 保持 deep-link 二态锁 (反约束 #374 §0 ②).
    url.searchParams.delete('tab');
  }
  window.history.replaceState(null, '', url.toString());
}

export default function ChannelView({ channelId }: Props) {
  const { state, actions, sendWsMessage } = useAppContext();
  const connectionState = state.connectionState;
  const [showMembers, setShowMembers] = useState(false);
  const [previewMessages, setPreviewMessages] = useState<Message[] | null>(null);
  const [joining, setJoining] = useState(false);
  const [activeTab, setActiveTab] = useState<'chat' | 'workspace' | 'remote' | 'canvas'>(
    () => initialTabFromURL(),
  );
  const keyboardHeight = useVisualViewport();

  const channel = state.channels.find(c => c.id === channelId);
  const dmChannel = state.dmChannels.find(dm => dm.id === channelId);
  const isDm = !!dmChannel?.peer;
  const isMember = channel?.is_member !== false;
  const canLeaveCurrentChannel = channel ? canLeaveChannel(channel, state.currentUser?.id) : false;
  const isPublicPreview = !isDm && channel && !isMember && channel.visibility !== 'private';

  useEffect(() => {
    if (isPublicPreview) {
      setPreviewMessages(null);
      fetchChannelPreview(channelId).then(data => {
        setPreviewMessages(data.messages);
      }).catch(() => {
        setPreviewMessages([]);
      });
    } else {
      setPreviewMessages(null);
      actions.loadMessages(channelId);
    }
  }, [channelId, isPublicPreview, actions]);

  useEffect(() => {
    setShowMembers(false);
  }, [channelId]);

  // CHN-4 deep-link sync — activeTab 变化时把 chat|workspace 写回 URL,
  // 让用户分享当前 tab 视图 (跟 spec #374 §1 CHN-4.2 + content-lock §1 ①
  // URL `?tab=` 锁同源). DM 视图无 tab 切换器, isDm 路径下不触发 URL 写.
  useEffect(() => {
    if (isDm) return;
    syncURLTab(activeTab);
  }, [activeTab, isDm]);

  // Load slash commands on mount / channelId change + WS live-reload
  useEffect(() => {
    let cancelled = false;

    const loadCommands = async () => {
      try {
        const data = await listCommands(channelId);
        if (cancelled) return;
        const remoteCommands: RemoteCommand[] = data.agent.flatMap(a =>
          a.commands.map(c => ({ ...c, agentId: a.agent_id, agentName: a.agent_name }))
        );
        commandRegistry.setRemoteCommands(remoteCommands);
      } catch (err) {
        console.warn('[commands] Failed to load commands:', err);
      }
    };

    loadCommands();

    // Debounce WS commands_updated events (300ms) to avoid broadcast storms
    let debounceTimer: ReturnType<typeof setTimeout> | undefined;
    const handleCommandsUpdated = () => {
      if (debounceTimer) clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => {
        loadCommands();
      }, 300);
    };

    window.addEventListener('commands_updated', handleCommandsUpdated);

    return () => {
      cancelled = true;
      window.removeEventListener('commands_updated', handleCommandsUpdated);
      if (debounceTimer) clearTimeout(debounceTimer);
    };
  }, [channelId]);

  if (!channel && !isDm) {
    // A forbidden channel/DM (not in the membership-scoped state lists) and a
    // genuinely non-existent id BOTH land here. Conflating forbidden with
    // not-found is the secure choice — the message must NOT reveal whether the
    // resource exists (no existence leak). `data-channel-not-found` is a stable
    // testability marker so the access-denied state is assertable in e2e; it
    // does not change the (intentionally indistinguishable) user-facing copy.
    return (
      <div className="channel-view" style={keyboardHeight > 0 ? { height: `calc(100% - ${keyboardHeight}px)` } : undefined}>
        <div className="channel-empty" data-channel-not-found="true" role="status">
          频道未找到
        </div>
      </div>
    );
  }

  const headerTitle = isDm
    ? dmChannel.peer.display_name
    : `${channel!.visibility === 'private' ? '🔒 ' : '#'}${channelDisplayName(channel!)}`;
  const headerTopic = isDm ? undefined : channel!.topic;
  const isGeneral = channel?.name === 'general';

  const handleLeave = async () => {
    try {
      await leaveChannel(channelId);
      await actions.loadChannels();
    } catch (err) {
      alert(err instanceof Error ? err.message : '离开失败');
    }
  };

  const handleJoin = async () => {
    if (joining) return;
    setJoining(true);
    try {
      await joinChannel(channelId);
      await actions.loadChannels();
      await actions.loadMessages(channelId);
      setPreviewMessages(null);
      sendWsMessage({ type: 'subscribe', channel_id: channelId });
    } catch (err) {
      alert(err instanceof Error ? err.message : '加入失败');
    } finally {
      setJoining(false);
    }
  };

  return (
    <div className="channel-view" data-channel-type={isDm ? 'dm' : 'channel'} style={keyboardHeight > 0 ? { height: `calc(100% - ${keyboardHeight}px)` } : undefined}>
      <div className="channel-header">
        <div className="channel-header-info">
          <h2 className="channel-title">{headerTitle}</h2>
          {headerTopic && <span className="channel-topic">{headerTopic}</span>}
        </div>
        {!isDm && channel && (
          <>
            {canLeaveCurrentChannel && (
              <button
                className="btn btn-sm leave-btn"
                title="离开频道"
                onClick={handleLeave}
              >
                离开频道
              </button>
            )}
            <button
              className="icon-btn"
              title="成员管理"
              onClick={() => setShowMembers(true)}
            >
              👥{channel.member_count != null ? ` ${channel.member_count}` : ''}
            </button>
          </>
        )}
      </div>
      {!isDm && isMember && !isPublicPreview && (
        <div className="channel-view-tabs">
          <button
            className={`channel-view-tab${activeTab === 'chat' ? ' active' : ''}`}
            data-tab="chat"
            onClick={() => setActiveTab('chat')}
          >
            聊天
          </button>
          <button
            className={`channel-view-tab${activeTab === 'canvas' ? ' active' : ''}`}
            data-tab="canvas"
            onClick={() => setActiveTab('canvas')}
          >
            Canvas
          </button>
          <button
            className={`channel-view-tab${activeTab === 'workspace' ? ' active' : ''}`}
            data-tab="workspace"
            onClick={() => setActiveTab('workspace')}
          >
            工作区
          </button>
          <button
            className={`channel-view-tab${activeTab === 'remote' ? ' active' : ''}`}
            data-tab="remote"
            onClick={() => setActiveTab('remote')}
          >
            Remote
          </button>
        </div>
      )}
      {activeTab === 'workspace' && !isDm && isMember && !isPublicPreview ? (
        <WorkspacePanel channelId={channelId} />
      ) : activeTab === 'remote' && !isDm && isMember && !isPublicPreview ? (
        <RemotePanel channelId={channelId} />
      ) : activeTab === 'canvas' && !isDm && isMember && !isPublicPreview ? (
        <ArtifactPanel channelId={channelId} />
      ) : (
        <>
          {isPublicPreview && (
            <div className="preview-banner">
              你正在预览 <strong>#{channelDisplayName(channel!)}</strong>
            </div>
          )}
          <ConnectionStatus state={connectionState} />
          {isPublicPreview ? (
            <>
              <MessageList channelId={channelId} previewMessages={previewMessages} />
              <div className="preview-join-container">
                <button
                  className="btn btn-primary preview-join-btn"
                  onClick={handleJoin}
                  disabled={joining}
                >
                  {joining ? '加入中...' : '加入频道'}
                </button>
              </div>
            </>
          ) : (
            <>
              <MessageList channelId={channelId} />
              <MessageInput channelId={channelId} />
            </>
          )}
        </>
      )}
      {showMembers && channel && (
        <ChannelMembersModal
          channelId={channel.id}
          onClose={() => setShowMembers(false)}
        />
      )}
    </div>
  );
}

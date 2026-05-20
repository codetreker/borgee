import React, { useState, useEffect, useCallback } from 'react';
import { useAppContext } from '../context/AppContext';
import { useCan } from '../hooks/usePermissions';
import { fetchChannelMembers, addChannelMember, removeChannelMember, updateChannel, deleteChannel, archiveChannel, fetchAgents } from '../lib/api';
import type { ChannelMember, Agent, AgentRuntimeState, AgentRuntimeReason } from '../lib/api';
import ConfirmDeleteModal from './ConfirmDeleteModal';
import { useToast } from './Toast';
import PresenceDot from './PresenceDot';
import { usePresence } from '../hooks/usePresence';

// AL-3.3 (#R3 Phase 2) — agent member presence row.
// Constraint §3.2: 仅 role==='agent' 行带 dot, 人 (member/admin) 行无 [data-presence].
// fallbackState/Reason: 跟 Sidebar.tsx::DmPresence (gh#922) 同套路 — REST
// /api/v1/channels/:id/members 现在 fold 了 server-side state, modal 一开
// (WS presence cache 还冷的窗口) 也能即时显真 state, 不再永远灰头像.
function MemberPresence({
  agentID,
  fallbackState,
  fallbackReason,
}: {
  agentID: string;
  fallbackState?: AgentRuntimeState;
  fallbackReason?: AgentRuntimeReason;
}) {
  const live = usePresence(agentID);
  const state = live?.state ?? fallbackState;
  const reason = live?.reason ?? (state === 'error' ? fallbackReason : undefined);
  return <PresenceDot state={state} reason={reason} />;
}

export default function ChannelMembersModal({ channelId, onClose }: { channelId: string; onClose: () => void }) {
  const { state, actions, dispatch } = useAppContext();
  const { showToast } = useToast();
  const channel = state.channels.find(c => c.id === channelId);
  const [members, setMembers] = useState<ChannelMember[]>([]);
  const [candidateMembers, setCandidateMembers] = useState<ChannelMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [showAddList, setShowAddList] = useState(false);
  const [confirmVisibility, setConfirmVisibility] = useState<'public' | 'private' | null>(null);
  const [switching, setSwitching] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  // CHN-1.3 设计 ⑤: archive UI gate. Owner-only flip; server stamps
  // archived_at and emits the system DM ("channel #{name} 已被 ... 关闭于 ...").
  const [archiving, setArchiving] = useState(false);

  const channelName = channel?.name ?? '';
  const channelCreatedBy = channel?.created_by ?? '';
  const isGeneral = channelName === 'general';
  const isDm = channel?.type === 'dm';
  const currentUser = state.currentUser;
  const isChannelOwner = currentUser?.id === channelCreatedBy;
  const canManageMembers = useCan('channel.manage_members', channelId);
  const canDeleteChannel = useCan('channel.delete', channelId);
  const canManageVisibility = useCan('channel.manage_visibility', channelId);
  const canManage = canManageMembers;
  const canDelete = isChannelOwner && canDeleteChannel && !isGeneral && !isDm;
  const canArchive = isChannelOwner && canManageVisibility && !isGeneral && !isDm;
  const visibility = channel?.visibility ?? 'public';

  const load = useCallback(async () => {
    try {
      const m = await fetchChannelMembers(channelId);
      setMembers(m);
    } finally {
      setLoading(false);
    }
  }, [channelId]);

  useEffect(() => { load(); }, [load]);

  useEffect(() => {
    if (!showAddList) return;
    let cancelled = false;
    // 候选成员有两个来源:
    //   1. #general 频道的人类成员 — 作为"workspace 人员"的代理 (现在没专门的
    //      `/users` endpoint, #general 通常承载所有 human; agents 不在里面).
    //   2. 调用者拥有的 agents — server `handleAddMember` 只允许 agent owner
    //      把它加进 channel, 所以候选范围就限定于自己拥有的 agent.
    // 合并后按 user_id 去重, 排除 channel 已有成员.
    const general = state.channels.find(c => c.name === 'general');
    const generalPromise: Promise<ChannelMember[]> = (general && general.id !== channelId)
      ? fetchChannelMembers(general.id).catch(() => [])
      : Promise.resolve([]);
    const agentsPromise: Promise<Agent[]> = fetchAgents().catch(() => []);
    Promise.all([generalPromise, agentsPromise]).then(([generalMembers, agents]) => {
      if (cancelled) return;
      const agentsAsMembers: ChannelMember[] = agents.map(a => ({
        user_id: a.id,
        display_name: a.display_name,
        role: 'agent',
        avatar_url: a.avatar_url,
        joined_at: a.created_at,
      }));
      const seen = new Set<string>();
      const merged: ChannelMember[] = [];
      for (const m of [...agentsAsMembers, ...generalMembers]) {
        if (seen.has(m.user_id)) continue;
        seen.add(m.user_id);
        merged.push(m);
      }
      setCandidateMembers(merged);
    });
    return () => { cancelled = true; };
  }, [showAddList, state.channels, channelId]);

  const memberIds = new Set(members.map(m => m.user_id));
  const nonMembers = candidateMembers.filter(u => !memberIds.has(u.user_id));

  const handleAdd = async (userId: string) => {
    setAdding(true);
    try {
      await addChannelMember(channelId, userId);
      await load();
      dispatch({ type: 'BUMP_CHANNEL_MEMBERS_VERSION', channelId });
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to add member');
    } finally {
      setAdding(false);
    }
  };

  const handleRemove = async (userId: string) => {
    try {
      await removeChannelMember(channelId, userId);
      await load();
      dispatch({ type: 'BUMP_CHANNEL_MEMBERS_VERSION', channelId });
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to remove member');
    }
  };

  const handleVisibilitySwitch = async () => {
    if (!confirmVisibility) return;
    setSwitching(true);
    try {
      await updateChannel(channelId, { visibility: confirmVisibility });
      await actions.loadChannels();
      setConfirmVisibility(null);
    } catch (err) {
      alert(err instanceof Error ? err.message : '切换失败');
    } finally {
      setSwitching(false);
    }
  };

  const targetVisibility = visibility === 'public' ? 'private' : 'public';

  const handleDelete = async () => {
    setDeleting(true);
    try {
      await deleteChannel(channelId);
      const general = state.channels.find(c => c.name === 'general');
      dispatch({ type: 'REMOVE_CHANNEL', channelId });
      if (general && state.currentChannelId === channelId) {
        dispatch({ type: 'SET_CURRENT_CHANNEL', channelId: general.id });
      }
      onClose();
      showToast('频道已删除');
    } catch (err) {
      showToast(err instanceof Error ? err.message : '删除失败');
      setDeleting(false);
    }
  };

  // CHN-1.3 设计 ⑤: archive flip. Server-stamped timestamp + fanout system DM
  // (channel-model.md §2 不变量 #3 — archive preserves history).
  const isArchived = (channel?.archived_at ?? null) != null;
  const handleArchive = async () => {
    setArchiving(true);
    try {
      await archiveChannel(channelId, !isArchived);
      await actions.loadChannels();
      showToast(isArchived ? '频道已恢复' : '频道已归档');
    } catch (err) {
      showToast(err instanceof Error ? err.message : '归档失败');
    } finally {
      setArchiving(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3>{visibility === 'private' ? '🔒' : '#'}{channelName} 成员</h3>
          <button className="icon-btn" onClick={onClose}>✕</button>
        </div>

        {loading ? (
          <div className="modal-body"><p>加载中...</p></div>
        ) : (
          <div className="modal-body">
            {canManageVisibility && (
              <div className="visibility-section">
                <div className="visibility-current">
                  频道可见性：{visibility === 'public' ? '🌐 公开' : '🔒 私有'}
                </div>
                <button
                  className="btn btn-sm"
                  disabled={isGeneral || switching}
                  onClick={() => setConfirmVisibility(targetVisibility)}
                  title={isGeneral ? '#general 不可设为私有' : undefined}
                >
                  切换为{targetVisibility === 'public' ? '公开' : '私有'}
                </button>
              </div>
            )}

            {confirmVisibility && (
              <div className="confirm-dialog">
                <p>
                  {confirmVisibility === 'private'
                    ? '将频道设为私有？已有成员将保留，新用户不会自动加入。'
                    : '将频道设为公开？所有用户将自动加入此频道。'}
                </p>
                <div className="form-actions">
                  <button
                    className="btn btn-sm btn-primary"
                    onClick={handleVisibilitySwitch}
                    disabled={switching}
                  >
                    {switching ? '切换中...' : '确认'}
                  </button>
                  <button className="btn btn-sm" onClick={() => setConfirmVisibility(null)}>取消</button>
                </div>
              </div>
            )}

            <div className="member-list">
              {members.map(m => (
                <div key={m.user_id} className="member-row" data-role={m.role === 'agent' ? 'agent' : 'user'}>
                  <div className="user-avatar-small">{m.display_name[0]?.toUpperCase()}</div>
                  <span
                    className="member-name"
                    {...(m.role === 'agent'
                      ? {
                          // CM-5.3 client SPA: agent collab hover link.
                          // 设计 ⑤ owner-first 透明协作 — agent 跟人 path
                          // 同源, hover 显示 "正在协作" 提示给 owner 视角
                          // 看见 agent 工作链路. Constraint: 不订阅 push frame
                          // (走 channel members 既有 lookup), 不引 ai_only
                          // visibility scope (蓝图 §185 透明协作字面承袭).
                          'data-cm5-collab-link': '',
                          title: `${m.display_name} 正在协作`,
                        }
                      : {})}
                  >
                    {m.display_name}
                  </span>
                  {m.role === 'agent' && <span className="user-badge">Bot</span>}
                  {m.role === 'agent' && (
                    <MemberPresence
                      agentID={m.user_id}
                      fallbackState={m.state}
                      fallbackReason={m.reason}
                    />
                  )}
                  {m.role === 'agent' && m.silent && (
                    <span className="user-badge user-badge-silent" title="silent: 不计入 unread / mention 计数">🔕 silent</span>
                  )}
                  {m.user_id === channelCreatedBy && <span className="user-badge">创建者</span>}
                  {canManage && !isGeneral && m.user_id !== currentUser?.id && m.user_id !== channelCreatedBy && (
                    <button
                      className="btn btn-sm btn-danger"
                      onClick={() => handleRemove(m.user_id)}
                    >
                      移除
                    </button>
                  )}
                </div>
              ))}
            </div>

            {canManage && !isGeneral && (
              <div className="add-member-section">
                <button
                  className="btn btn-sm btn-primary"
                  onClick={() => setShowAddList(!showAddList)}
                >
                  {showAddList ? '收起' : '添加成员'}
                </button>
                {showAddList && (
                  <div className="member-list add-member-list">
                    {nonMembers.map(u => (
                      <div key={u.user_id} className="member-row">
                        <div className="user-avatar-small">{u.display_name[0]?.toUpperCase()}</div>
                        <span className="member-name">{u.display_name}</span>
                        {u.role === 'agent' && <span className="user-badge">Bot</span>}
                        <button
                          className="btn btn-sm btn-primary"
                          onClick={() => handleAdd(u.user_id)}
                          disabled={adding}
                        >
                          添加
                        </button>
                      </div>
                    ))}
                    {nonMembers.length === 0 && (
                      <div className="member-row">
                        <span className="member-name">暂无可添加成员</span>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}

            {(canDelete || canArchive) && (
              <div className="danger-section">
                <div className="danger-section-label">危险区域</div>
                {canArchive && (
                  <button
                    className="btn btn-sm"
                    onClick={handleArchive}
                    disabled={archiving}
                    title={isArchived ? '恢复后频道将重新出现在列表' : '归档将保留历史记录但隐藏频道'}
                  >
                    {archiving ? '处理中...' : isArchived ? '恢复频道' : '归档频道'}
                  </button>
                )}
                {canDelete && (
                  <button
                    className="btn btn-sm btn-danger"
                    onClick={() => setConfirmingDelete(true)}
                  >
                    删除频道
                  </button>
                )}
              </div>
            )}
          </div>
        )}
      </div>
      {confirmingDelete && (
        <ConfirmDeleteModal
          channelName={channelName}
          onConfirm={handleDelete}
          onCancel={() => setConfirmingDelete(false)}
          loading={deleting}
        />
      )}
    </div>
  );
}

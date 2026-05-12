// InvitationsInbox.tsx — CM-4.2 owner-side inbox for cross-org agent
// invitations. Lists pending invitations for agents the caller owns and
// surfaces 同意 / 拒绝 quick actions that PATCH the CM-4.1 endpoint and
// reload the list. On approve, optionally jumps to the joined channel
// (CM-4.2c — relies on the server-side AddChannelMember side-effect from
// CM-4.1).
//
// Out of scope here: BPP frame, system message DM, offline detection
// (CM-4.3 / CM-4.3b). This is the "inbox card" surface only — paired
// with the simpler bell button in Sidebar.

import React, { useCallback, useEffect, useState } from 'react';
import { useAppContext } from '../context/AppContext';
import {
  listAgentInvitations,
  decideAgentInvitation,
  ApiError,
  type AgentInvitation,
} from '../lib/api';
import { useInvitationFrames } from '../hooks/useWsHubFrames';

interface Props {
  onBack: () => void;
  /**
   * Called when an approval succeeds and the resulting channel is one the
   * caller is already a member of. Lets the host App switch focus to the
   * channel without coupling this component to the AppContext reducer.
   */
  onJumpToChannel?: (channelId: string) => void;
}

type Filter = 'pending' | 'all';

export default function InvitationsInbox({ onBack, onJumpToChannel }: Props) {
  const { actions } = useAppContext();
  const [invitations, setInvitations] = useState<AgentInvitation[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<Filter>('pending');
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setErrorMsg(null);
    try {
      const data = await listAgentInvitations('owner');
      setInvitations(data);
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  // RT-0 (#40): drop-in push listener — when the server pushes an
  // `agent_invitation_pending` or `_decided` frame the inbox re-fetches
  // immediately. Server is authoritative; the frame is just the
  // wake-up. G2.4 latency requirement (≤ 3s) is satisfied because the path
  // is /ws push → CustomEvent → load() (no 60s polling).
  useInvitationFrames({
    onPending: load,
    onDecided: load,
  });

  const handleDecision = useCallback(
    async (inv: AgentInvitation, decision: 'approved' | 'rejected') => {
      if (busyId) return;
      setBusyId(inv.id);
      setErrorMsg(null);
      try {
        const updated = await decideAgentInvitation(inv.id, decision);
        // Optimistic local replace — server is the source of truth, but
        // we want the row to flip state immediately even if the next
        // list reload is delayed.
        setInvitations(prev => prev.map(p => (p.id === inv.id ? updated : p)));

        if (decision === 'approved') {
          // The server's CM-4.1 handler already idempotent-joined the
          // agent. Reload channels so the sidebar reflects the new
          // membership, then jump unconditionally — server authz is
          // the gatekeeper, and `state.channels` here would be the
          // pre-await stale closure (find() races with the reload).
          await actions.loadChannels();
          if (onJumpToChannel) {
            onJumpToChannel(updated.channel_id);
          }
        }
      } catch (err) {
        if (err instanceof ApiError && err.status === 409) {
          setErrorMsg('该邀请已被处理或状态已变更，请刷新');
        } else {
          setErrorMsg(err instanceof Error ? err.message : '操作失败');
        }
      } finally {
        setBusyId(null);
      }
    },
    [busyId, actions, onJumpToChannel],
  );

  const visible = filter === 'pending'
    ? invitations.filter(inv => inv.state === 'pending')
    : invitations;

  return (
    <div className="agent-page">
      <div className="admin-section-header">
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <button className="btn btn-sm" onClick={onBack}>← Back</button>
          <h2>Agent 邀请</h2>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button
            className={`btn btn-sm${filter === 'pending' ? ' btn-primary' : ''}`}
            onClick={() => setFilter('pending')}
          >
            待处理
          </button>
          <button
            className={`btn btn-sm${filter === 'all' ? ' btn-primary' : ''}`}
            onClick={() => setFilter('all')}
          >
            全部
          </button>
          <button className="btn btn-sm" onClick={load} disabled={loading}>
            刷新
          </button>
        </div>
      </div>

      {errorMsg && (
        <div className="admin-error" role="alert" style={{ margin: '8px 0' }}>
          {errorMsg}
        </div>
      )}

      {loading ? (
        <div className="app-loading"><div className="loading-spinner-large" /></div>
      ) : visible.length === 0 ? (
        <p style={{ color: 'var(--text-secondary)', textAlign: 'center', marginTop: 40 }}>
          {filter === 'pending' ? '暂无待处理邀请' : '暂无邀请记录'}
        </p>
      ) : (
        <ul className="invitation-list" style={{ listStyle: 'none', padding: 0, margin: 0 }}>
          {visible.map(inv => (
            <InvitationCard
              key={inv.id}
              invitation={inv}
              busy={busyId === inv.id}
              onApprove={() => handleDecision(inv, 'approved')}
              onReject={() => handleDecision(inv, 'rejected')}
            />
          ))}
        </ul>
      )}
    </div>
  );
}

interface CardProps {
  invitation: AgentInvitation;
  busy: boolean;
  onApprove: () => void;
  onReject: () => void;
}

function InvitationCard({ invitation, busy, onApprove, onReject }: CardProps) {
  const isPending = invitation.state === 'pending';
  const stateLabel = stateToLabel(invitation.state);

  return (
    <li className="invitation-card" style={{
      border: '1px solid var(--border-color)',
      borderRadius: 6,
      padding: 12,
      marginBottom: 8,
      display: 'flex',
      flexDirection: 'column',
      gap: 6,
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8 }}>
        <div>
          {/* Bug-029 P0: prefer human-readable name from server JOIN; raw
              UUID survives only as title hover for a11y/debug. Empty
              string from sanitizer fallback → degrade to ID. */}
          <strong>邀请 agent</strong>{' '}
          <span title={invitation.agent_id}>
            {invitation.agent_name || invitation.agent_id}
          </span>{' '}
          加入 channel{' '}
          <span title={invitation.channel_id}>
            {invitation.channel_name
              ? `#${invitation.channel_name}`
              : invitation.channel_id}
          </span>
        </div>
        <span className={`invitation-state invitation-state-${invitation.state}`}>
          {stateLabel}
        </span>
      </div>
      <div style={{ color: 'var(--text-secondary)', fontSize: 12 }}>
        发起人{' '}
        <span title={invitation.requested_by}>
          {invitation.requester_name || invitation.requested_by}
        </span>{' '}
        · 创建于{' '}
        {formatTs(invitation.created_at)}
        {invitation.decided_at !== undefined && (
          <> · 处理于 {formatTs(invitation.decided_at)}</>
        )}
        {invitation.expires_at !== undefined && (
          <> · 过期于 {formatTs(invitation.expires_at)}</>
        )}
      </div>
      {isPending && (
        <div style={{ display: 'flex', gap: 8, marginTop: 4 }}>
          <button
            className="btn btn-primary btn-sm"
            onClick={onApprove}
            disabled={busy}
          >
            {busy ? '处理中…' : '同意'}
          </button>
          <button
            className="btn btn-sm"
            onClick={onReject}
            disabled={busy}
          >
            拒绝
          </button>
        </div>
      )}
    </li>
  );
}

export function stateToLabel(state: AgentInvitation['state']): string {
  switch (state) {
    case 'pending': return '待处理';
    case 'approved': return '已同意';
    case 'rejected': return '已拒绝';
    case 'expired': return '已过期';
  }
}

function formatTs(ms: number): string {
  if (!ms) return '-';
  const d = new Date(ms);
  if (Number.isNaN(d.getTime())) return '-';
  return d.toLocaleString();
}

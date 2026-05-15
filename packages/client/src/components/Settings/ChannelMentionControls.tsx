import { useCallback, useState } from 'react';
import {
  fetchChannelMembers,
  setChannelMemberRequireMentionPolicy,
} from '../../lib/api';
import type { ChannelMember, RequireMentionPolicy } from '../../lib/api';

const POLICY_LABELS: Record<RequireMentionPolicy, string> = {
  inherit: '继承 agent 全局设置',
  on: '必须明确 @ 提及',
  off: '允许非提及送达',
};

function policyOf(member: ChannelMember): RequireMentionPolicy {
  return member.require_mention_policy ?? 'inherit';
}

function effectiveLabel(member: ChannelMember): string {
  if (typeof member.effective_require_mention !== 'boolean') return '当前状态待服务器返回';
  return member.effective_require_mention ? '当前需要 @ 提及' : '普通消息也会送达';
}

interface Props {
  channelId: string;
  canManage: boolean;
}

export default function ChannelMentionControls({ channelId, canManage }: Props) {
  const [expanded, setExpanded] = useState(false);
  const [members, setMembers] = useState<ChannelMember[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [loading, setLoading] = useState(false);
  const [savingUserId, setSavingUserId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const next = await fetchChannelMembers(channelId);
      setMembers(next);
      setLoaded(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : '提及设置加载失败');
    } finally {
      setLoading(false);
    }
  }, [channelId]);

  const handleToggle = useCallback(() => {
    setExpanded(prev => {
      const next = !prev;
      if (next && !loaded && !loading) void load();
      return next;
    });
  }, [load, loaded, loading]);

  const handlePolicyChange = useCallback(async (member: ChannelMember, policy: RequireMentionPolicy) => {
    setSavingUserId(member.user_id);
    setError(null);
    try {
      const state = await setChannelMemberRequireMentionPolicy(channelId, member.user_id, policy);
      setMembers(prev => prev.map(item => item.user_id === member.user_id
        ? {
            ...item,
            require_mention_policy: state.require_mention_policy,
            effective_require_mention: state.effective_require_mention,
          }
        : item));
    } catch (err) {
      setError(err instanceof Error ? err.message : '提及策略更新失败');
    } finally {
      setSavingUserId(null);
    }
  }, [channelId]);

  const agents = members.filter(member => member.role === 'agent');

  return (
    <div className="channel-mention-controls">
      <button
        type="button"
        className="btn btn-sm"
        data-testid={`mention-controls-toggle-${channelId}`}
        onClick={handleToggle}
      >
        {expanded ? '收起提及设置' : '提及设置'}
      </button>

      {expanded && (
        <div className="channel-mention-panel" data-testid={`mention-controls-panel-${channelId}`}>
          <p className="channel-mention-note" data-testid={`everyone-authority-${channelId}`}>
            @Everyone 由服务器按当前频道成员计算并限流；客户端不能选择或扩大收件人。
          </p>
          <p className="channel-mention-note">
            agent 非提及送达只在 agent 拥有者已允许时生效，服务器会拒绝越权设置。
          </p>

          {loading && <p className="channel-mention-status">加载中...</p>}
          {error && <p className="channel-mention-error" role="alert">{error}</p>}
          {!loading && loaded && agents.length === 0 && (
            <p className="channel-mention-status">没有 agent 成员需要单独设置。</p>
          )}

          {agents.length > 0 && (
            <ul className="channel-mention-agent-list">
              {agents.map(agent => (
                <li
                  key={agent.user_id}
                  className="channel-mention-agent-row"
                  data-agent-id={agent.user_id}
                  data-policy={policyOf(agent)}
                >
                  <div className="channel-mention-agent-copy">
                    <span className="channel-mention-agent-name">{agent.display_name}</span>
                    <span className="channel-mention-effective">{effectiveLabel(agent)}</span>
                  </div>
                  <select
                    aria-label={`${agent.display_name} 提及策略`}
                    value={policyOf(agent)}
                    disabled={!canManage || savingUserId === agent.user_id}
                    onChange={event => void handlePolicyChange(agent, event.currentTarget.value as RequireMentionPolicy)}
                  >
                    <option value="inherit">{POLICY_LABELS.inherit}</option>
                    <option value="on">{POLICY_LABELS.on}</option>
                    <option value="off">{POLICY_LABELS.off}</option>
                  </select>
                </li>
              ))}
            </ul>
          )}

          {!canManage && (
            <p className="channel-mention-status">需要频道成员管理权限才能更改 agent 提及策略。</p>
          )}
        </div>
      )}
    </div>
  );
}

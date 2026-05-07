import React, { useState, useEffect, useCallback } from 'react';
import { useAppContext } from '../context/AppContext';
import { useUnsavedChangesGuard } from '../hooks/useUnsavedChangesGuard';
import {
  fetchAgents,
  fetchAgent,
  createAgent,
  deleteAgent,
  rotateAgentApiKey,
  fetchAgentPermissions,
  updateAgentPermissions,
  addAgentToChannel,
  fetchAgentRuntime,
  type Agent,
  type AgentRuntime,
  type PermissionDetail,
} from '../lib/api';
import { describeAgentState } from '../lib/agent-state';
import PresenceDot from './PresenceDot';
import RuntimeCard from './RuntimeCard';
import { AgentConfigPanel } from './AgentConfigPanel';
import { usePresence } from '../hooks/usePresence';

const KNOWN_PERMISSIONS = [
  'message.send',
  'channel.create',
  'channel.delete',
  'channel.manage_members',
  'channel.manage_visibility',
];

interface Props {
  onBack: () => void;
}

export default function AgentManager({ onBack }: Props) {
  const { state } = useAppContext();
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [selectedAgent, setSelectedAgent] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const data = await fetchAgents();
      setAgents(data);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  return (
    <div className="agent-page">
      <div className="admin-section-header">
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <button className="btn btn-sm" onClick={onBack}>← Back</button>
          <h2>My Agents</h2>
        </div>
        <button className="btn btn-primary btn-sm" onClick={() => setShowCreate(true)}>Create Agent</button>
      </div>

      {loading ? (
        <div className="app-loading"><div className="loading-spinner-large" /></div>
      ) : agents.length === 0 ? (
        <p style={{ color: 'var(--text-secondary)', textAlign: 'center', marginTop: 40 }}>
          No agents yet. Create one to get started.
        </p>
      ) : (
        agents.map(agent => (
          <AgentCard
            key={agent.id}
            agent={agent}
            expanded={selectedAgent === agent.id}
            onToggle={() => setSelectedAgent(selectedAgent === agent.id ? null : agent.id)}
            onDelete={async () => {
              if (!confirm(`Delete agent "${agent.display_name}"?`)) return;
              try {
                await deleteAgent(agent.id);
                await load();
              } catch (err) {
                alert(err instanceof Error ? err.message : 'Failed');
              }
            }}
            onRefresh={load}
            channels={state.channels}
          />
        ))
      )}

      {showCreate && (
        <CreateAgentModal
          onClose={() => setShowCreate(false)}
          onCreated={() => { setShowCreate(false); load(); }}
        />
      )}
    </div>
  );
}

function AgentCard({
  agent,
  expanded,
  onToggle,
  onDelete,
  onRefresh,
  channels,
}: {
  agent: Agent;
  expanded: boolean;
  onToggle: () => void;
  onDelete: () => void;
  onRefresh: () => void;
  channels: { id: string; name: string }[];
}) {
  const [permissions, setPermissions] = useState<PermissionDetail[]>([]);
  const [loadingPerms, setLoadingPerms] = useState(false);
  const [newKey, setNewKey] = useState<string | null>(null);
  const [visibleKey, setVisibleKey] = useState<string | null>(null);
  const [loadingKey, setLoadingKey] = useState(false);
  const [joinChannelId, setJoinChannelId] = useState('');

  // AL-4.3 (#379 §1 拆段): runtime 卡片状态. fetchAgentRuntime 返回
  // null 表示该 agent 还没注册 runtime (graceful degrade — 立场 ①
  // "Borgee 不带 runtime", 不假装有). expanded 时按需拉, 不在 list
  // 视图浪费 N 次请求.
  const { state: appState } = useAppContext();
  const viewerUserID = appState.currentUser?.id ?? null;
  const [runtime, setRuntime] = useState<AgentRuntime | null>(null);
  const [runtimeLoaded, setRuntimeLoaded] = useState(false);
  const loadRuntime = useCallback(async () => {
    try {
      const rt = await fetchAgentRuntime(agent.id);
      setRuntime(rt);
    } catch {
      // 静默失败 (沉默胜于假 loading §11; transient error 不阻 expanded 展开).
      setRuntime(null);
    } finally {
      setRuntimeLoaded(true);
    }
  }, [agent.id]);

  const loadPerms = useCallback(async () => {
    setLoadingPerms(true);
    try {
      const data = await fetchAgentPermissions(agent.id);
      setPermissions(data.details);
    } finally {
      setLoadingPerms(false);
    }
  }, [agent.id]);

  useEffect(() => {
    if (expanded) {
      loadPerms();
      loadRuntime();
    }
  }, [expanded, loadPerms, loadRuntime]);

  const handleShowKey = async () => {
    if (visibleKey) {
      setVisibleKey(null);
      return;
    }
    setLoadingKey(true);
    try {
      const data = await fetchAgent(agent.id);
      setVisibleKey(data.api_key ?? null);
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed');
    } finally {
      setLoadingKey(false);
    }
  };

  const handleRotateKey = async () => {
    try {
      const key = await rotateAgentApiKey(agent.id);
      setNewKey(key);
      setVisibleKey(key);
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed');
    }
  };

  const handleTogglePerm = async (perm: string) => {
    const has = permissions.some(p => p.permission === perm && p.scope === '*');
    let newPerms: { permission: string; scope?: string }[];
    if (has) {
      newPerms = permissions.filter(p => !(p.permission === perm && p.scope === '*')).map(p => ({ permission: p.permission, scope: p.scope }));
    } else {
      newPerms = [...permissions.map(p => ({ permission: p.permission, scope: p.scope })), { permission: perm, scope: '*' }];
    }
    try {
      await updateAgentPermissions(agent.id, newPerms);
      await loadPerms();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed');
    }
  };

  const handleJoinChannel = async () => {
    if (!joinChannelId) return;
    try {
      await addAgentToChannel(joinChannelId, agent.id);
      setJoinChannelId('');
      alert('Agent added to channel');
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed');
    }
  };

  return (
    <div className="agent-card">
      <div className="admin-card-row">
        <div className="admin-card-info">
          <strong>{agent.display_name}</strong>
          {/* AL-1a (#R3): runtime 三态 + 故障原因. 文案锁见 lib/agent-state.ts (野马 #190 §11). */}
          <AgentStateBadge agent={agent} />
          <div style={{ fontSize: 12, color: 'var(--text-secondary)' }}>
            ID: {agent.id.slice(0, 12)}... | Created: {new Date(agent.created_at).toLocaleDateString()}
          </div>
        </div>
        <div className="admin-card-actions">
          <button className="btn btn-sm" onClick={onToggle}>{expanded ? 'Collapse' : 'Manage'}</button>
          <button className="btn btn-sm btn-danger" onClick={onDelete}>Delete</button>
        </div>
      </div>

      {expanded && (
        <div style={{ marginTop: 12, borderTop: '1px solid var(--border)', paddingTop: 12 }}>
          {/* API Key */}
          <div style={{ marginBottom: 12 }}>
            <strong>API Key</strong>
            {(newKey || visibleKey) ? (
              <div className="api-key-box">
                {newKey || visibleKey}
                <button className="btn-icon" onClick={() => navigator.clipboard.writeText((newKey || visibleKey)!)} title="Copy">📋</button>
              </div>
            ) : (
              <p style={{ fontSize: 13, color: 'var(--text-secondary)' }}>
                API key is hidden.
                <button className="btn btn-sm" style={{ marginLeft: 8 }} onClick={handleShowKey} disabled={loadingKey}>
                  {loadingKey ? 'Loading...' : 'Show'}
                </button>
              </p>
            )}
            <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
              {visibleKey && !newKey && (
                <button className="btn btn-sm" onClick={() => setVisibleKey(null)}>Hide</button>
              )}
              <button className="btn btn-sm" onClick={handleRotateKey}>Rotate API Key</button>
            </div>
          </div>

          {/* AL-4.3 (#379 §1 拆段) — Runtime 卡片. fetchAgentRuntime
              null → graceful degrade omit (立场 ① "Borgee 不带 runtime");
              非 owner → owner-only DOM gate 走 RuntimeCard 内部 isOwner
              判断 (反约束: 非 owner 看到 status badge 但看不到 start/stop
              btn, 跟 #321 §2 同源). */}
          {runtimeLoaded && (
            <RuntimeCard
              agent={agent}
              runtime={runtime}
              viewerUserID={viewerUserID}
              onRefresh={loadRuntime}
            />
          )}

          {/* AL-2a.3 (#447 + #480 mount) — agent config SSOT editor (name /
              avatar / prompt / model / capabilities / enabled / memory_ref).
              蓝图 §1.4 SSOT 字段划界, server-side allowedConfigKeys whitelist
              fail-closed; 反约束 立场 ⑤ runtime-only 字段 (api_key /
              temperature / token_limit / retry_policy) 此 form 不渲染. */}
          <div style={{ marginBottom: 12 }}>
            <strong>Config (SSOT)</strong>
            <AgentConfigPanel agentId={agent.id} />
          </div>

          {/* Permissions */}
          <div style={{ marginBottom: 12 }}>
            <strong>Permissions</strong>
            {loadingPerms ? (
              <p>Loading...</p>
            ) : (
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 8 }}>
                {KNOWN_PERMISSIONS.map(perm => {
                  const has = permissions.some(p => p.permission === perm && p.scope === '*');
                  return (
                    <label key={perm} style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 13 }}>
                      <input type="checkbox" checked={has} onChange={() => handleTogglePerm(perm)} />
                      {perm}
                    </label>
                  );
                })}
              </div>
            )}
          </div>

          {/* Join Channel */}
          <div>
            <strong>Add to Channel</strong>
            <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
              <select className="input-field" value={joinChannelId} onChange={e => setJoinChannelId(e.target.value)} style={{ flex: 1 }}>
                <option value="">Select channel...</option>
                {channels.filter(c => c.name !== 'general').map(c => (
                  <option key={c.id} value={c.id}>#{c.name}</option>
                ))}
              </select>
              <button className="btn btn-primary btn-sm" onClick={handleJoinChannel} disabled={!joinChannelId}>Add</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// AL-1a (#R3 Phase 2) — Agent state inline badge.
// 故障态点 reason label 直接给 owner 故障原因 (蓝图 §2.3 "可解释").
// data-state 让 Playwright (REG-AL1A-*) 锁住 selector.
//
// AL-3.3 (#R3 Phase 2): 接 usePresence cache — WS `presence.changed` frame
// 推来的实时态比 fetchAgents() 快照新, 优先用 cache. cache miss (没收到
// frame 或刚连上) 走 agent.state 兜底; 都没有再 describeAgentState 兜回
// "已离线" (野马 §11 不准灰糊弄).
function AgentStateBadge({ agent }: { agent: Agent }) {
  const live = usePresence(agent.id);
  const state = live?.state ?? agent.state;
  const reason = live?.reason ?? agent.reason;
  const label = describeAgentState(state, reason);
  const color = label.tone === 'ok' ? 'var(--success, #1a7f37)'
    : label.tone === 'error' ? 'var(--danger, #cf222e)'
    : 'var(--text-secondary)';
  return (
    <span
      data-testid="agent-state-badge"
      data-state={state ?? 'offline'}
      data-reason={reason ?? ''}
      style={{ marginLeft: 8, fontSize: 12, color, fontWeight: 500, display: 'inline-flex', alignItems: 'center', gap: 6 }}
    >
      <PresenceDot state={state} reason={reason} compact />
      {label.text}
    </span>
  );
}

function CreateAgentModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [displayName, setDisplayName] = useState('');
  const [agentId, setAgentId] = useState('');
  const [selectedPerms, setSelectedPerms] = useState<Set<string>>(new Set(['message.send']));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [createdId, setCreatedId] = useState<string | null>(null);

  // #682: 注册未保存改动守卫. 用户在表单里填了东西但还没提交时,
  // 切换到别的 sidepane 会先弹 confirmation. 已提交 (createdKey 有值) 后
  // 不再算 dirty — 那时表单是结果展示, 不是待保存改动.
  useUnsavedChangesGuard(
    () => createdKey === null && (displayName.trim() !== '' || agentId.trim() !== ''),
    'Create Agent 表单有填写但还没提交, 离开会丢失. 确认离开吗?',
  );

  const togglePerm = (perm: string) => {
    setSelectedPerms(prev => {
      const next = new Set(prev);
      if (next.has(perm)) next.delete(perm); else next.add(perm);
      return next;
    });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!displayName.trim()) return;
    setSaving(true);
    setError('');
    try {
      const trimmedId = agentId.trim() || undefined;
      const agent = await createAgent(displayName.trim(), [...selectedPerms], trimmedId);
      if (agent.api_key) {
        setCreatedKey(agent.api_key);
        setCreatedId(agent.id);
      } else {
        onCreated();
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed');
    } finally {
      setSaving(false);
    }
  };

  if (createdKey) {
    return (
      <div className="admin-modal" onClick={() => { onCreated(); }}>
        <div className="admin-modal-content" onClick={e => e.stopPropagation()}>
          <h3>Agent Created</h3>
          {createdId && <p style={{ fontSize: 13, color: 'var(--text-secondary)' }}>Agent ID: <code>{createdId}</code></p>}
          <p>Copy this API key. You can also view it later from the agent details.</p>
          <div className="api-key-box">
            {createdKey}
            <button className="btn-icon" onClick={() => navigator.clipboard.writeText(createdKey)} title="Copy">📋</button>
          </div>
          <div className="form-actions">
            <button className="btn btn-primary btn-sm" onClick={onCreated}>Done</button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="admin-modal" onClick={onClose}>
      <div className="admin-modal-content" onClick={e => e.stopPropagation()}>
        <h3>Create Agent</h3>
        <form onSubmit={handleSubmit}>
          <label>
            Display Name
            <input className="input-field" value={displayName} onChange={e => setDisplayName(e.target.value)} required autoFocus />
          </label>
          <label style={{ marginTop: 8, display: 'block' }}>
            Agent ID <span style={{ fontSize: 12, color: 'var(--text-secondary)' }}>(optional — auto-generated if empty)</span>
            <input
              className="input-field"
              value={agentId}
              onChange={e => setAgentId(e.target.value)}
              placeholder="e.g. my-bot-01"
              pattern="^[a-zA-Z0-9][\w-]{0,62}[a-zA-Z0-9]$"
              title="2-64 characters: letters, digits, hyphens, underscores"
            />
          </label>
          <div style={{ margin: '12px 0' }}>
            <strong style={{ fontSize: 14 }}>Permissions</strong>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginTop: 8 }}>
              {KNOWN_PERMISSIONS.map(perm => (
                <label key={perm} style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 13 }}>
                  <input type="checkbox" checked={selectedPerms.has(perm)} onChange={() => togglePerm(perm)} />
                  {perm}
                </label>
              ))}
            </div>
          </div>
          {error && <div className="admin-form-error">{error}</div>}
          <div className="form-actions">
            <button type="submit" className="btn btn-primary btn-sm" disabled={saving}>{saving ? 'Creating...' : 'Create'}</button>
            <button type="button" className="btn btn-sm" onClick={onClose}>Cancel</button>
          </div>
        </form>
      </div>
    </div>
  );
}

import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useAppContext } from '../context/AppContext';
import { useUnsavedChangesGuard } from '../hooks/useUnsavedChangesGuard';
import { useToast } from './Toast';
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

// #684 — Mask helper for API keys.
//
// Render `bgr_...{last4}` (prefix + ... + last four chars) to avoid copying
// OpenAI-style `sk-` examples. The last four chars are enough to identify the
// key but not enough to brute-force it.
//
// Constraint: the full plaintext key must not enter the DOM. This helper accepts
// only the last four chars; callers slice after fetch and immediately drop the
// full key so it cannot be kept in React state.
//
// 前缀 `bgr_` 写死 — server-go `GenerateAPIKey()` actual value (queries_phase2b.go:440).
// Matches brief §2.3 + §2.4 grep guards; `sk-` must not appear.
function formatMaskedApiKey(last4: string): string {
  return `bgr_...${last4}`;
}

// #684 — Auto-clear delay: 60 seconds, matching the security design and common
// password-manager behavior.
const API_KEY_AUTO_CLEAR_MS = 60_000;

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
  // #684 — API key display policy. Constraint: the full plaintext key never
  // enters React state or refs. It is held only inside the fetch closure, sliced
  // with `key.slice(-4)`, then discarded.
  //
  // Previous visibleKey / newKey state was removed because it stored the full plaintext key.
  const [last4, setLast4] = useState<string | null>(null);
  const [loadingKey, setLoadingKey] = useState(false);
  const [copying, setCopying] = useState(false);
  // 60s auto-clear timer ID, 用 ref 避免 re-render 干扰 cleanup (跟 #695
  // useUnsavedChangesGuard useRef 思路一致, 反 closure staleness).
  const autoClearTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { showToast } = useToast();
  const [joinChannelId, setJoinChannelId] = useState('');

  // AL-4.3 (#379 §1 split): runtime card state. fetchAgentRuntime returns
  // null when the agent has not registered a runtime yet (graceful degradation,
  // design ① "Borgee 不带 runtime"). Load only when expanded to avoid one
  // request per list row.
  const { state: appState } = useAppContext();
  const viewerUserID = appState.currentUser?.id ?? null;
  const [runtime, setRuntime] = useState<AgentRuntime | null>(null);
  const [runtimeLoaded, setRuntimeLoaded] = useState(false);
  const loadRuntime = useCallback(async () => {
    try {
      const rt = await fetchAgentRuntime(agent.id);
      setRuntime(rt);
    } catch {
      // Silent failure: #190 §11 prefers silence over fake loading; a transient error
      // should not block the expanded panel.
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

  // #684 — Load the API key mask on expand. Fetch the full key, store only the
  // last four chars in state, and do not keep the full key anywhere after the
  // closure exits.
  const loadKeyMask = useCallback(async () => {
    setLoadingKey(true);
    try {
      const data = await fetchAgent(agent.id);
      // Constraint: store only slice(-4), never the full key.
      if (typeof data.api_key === 'string' && data.api_key.length >= 4) {
        setLast4(data.api_key.slice(-4));
      } else {
        setLast4(null);
      }
    } catch {
      // Failure is shown inline only, matching RuntimeCard's no-fake-loading pattern.
      setLast4(null);
    } finally {
      setLoadingKey(false);
    }
  }, [agent.id]);

  useEffect(() => {
    if (expanded) {
      loadPerms();
      loadRuntime();
      loadKeyMask();
    }
  }, [expanded, loadPerms, loadRuntime, loadKeyMask]);

  // #684 — Clean up the auto-clear timer on unmount so it cannot fire after the
  // user leaves the sidepane or refreshes. Cleanup does not write to the
  // clipboard; after unmount, clipboard contents remain under user control.
  useEffect(() => {
    return () => {
      if (autoClearTimerRef.current) {
        clearTimeout(autoClearTimerRef.current);
        autoClearTimerRef.current = null;
      }
    };
  }, []);

  // #684 — Copy + auto-clear after 60s. The full key is held only inside this
  // function closure. After setLast4, it falls out of scope. The timer clears
  // the clipboard only if it still contains this exact key, so user changes made
  // during the 60s window are preserved.
  const handleCopyKey = async () => {
    setCopying(true);
    try {
      const data = await fetchAgent(agent.id);
      const key = data.api_key;
      if (typeof key !== 'string' || key.length === 0) {
        showToast('复制失败, 请手动选择 mask 后的 key 复制片段');
        return;
      }
      await navigator.clipboard.writeText(key);
      // Refresh the mask so last4 matches the freshly fetched key and avoids races.
      setLast4(key.slice(-4));
      showToast('API Key 已复制, 60 秒后自动清空');
      // 启动 60s auto-clear: closure 捕获本次 key, setTimeout 触发时
      // readText 比对 — 只在还是这把 key 时清.
      if (autoClearTimerRef.current) clearTimeout(autoClearTimerRef.current);
      const copiedKey = key;
      autoClearTimerRef.current = setTimeout(async () => {
        autoClearTimerRef.current = null;
        try {
          const current = await navigator.clipboard.readText();
          if (current === copiedKey) {
            await navigator.clipboard.writeText('');
            showToast('剪贴板已清空 (安全保护)');
          }
          // If it differs, the user copied something else; leave the clipboard alone.
        } catch {
          // readText may be denied (Firefox denies by default). In that case,
          // do not clear anything; blindly writing '' could erase user-copied content.
        }
      }, API_KEY_AUTO_CLEAR_MS);
    } catch (err) {
      // Fallback for browsers without clipboard support or non-HTTPS contexts.
      // document.execCommand is deprecated but remains the available fallback
      // here, avoiding an extra third-party dependency.
      try {
        const data = await fetchAgent(agent.id);
        const key = data.api_key ?? '';
        const ta = document.createElement('textarea');
        ta.value = key;
        ta.style.position = 'fixed';
        ta.style.left = '-9999px';
        document.body.appendChild(ta);
        ta.select();
        const ok = document.execCommand('copy');
        document.body.removeChild(ta);
        if (ok) {
          setLast4(key.slice(-4));
          showToast('API Key 已复制, 60 秒后自动清空');
        } else {
          showToast('复制失败, 请手动选择 mask 后的 key 复制片段');
        }
      } catch {
        showToast('复制失败, 请手动选择 mask 后的 key 复制片段');
      }
      void err;
    } finally {
      setCopying(false);
    }
  };

  const handleRotateKey = async () => {
    try {
      const key = await rotateAgentApiKey(agent.id);
      // Do not put the full key in the DOM; write it to the clipboard and update the mask.
      await navigator.clipboard.writeText(key);
      setLast4(key.slice(-4));
      showToast('API Key 已复制, 60 秒后自动清空');
      if (autoClearTimerRef.current) clearTimeout(autoClearTimerRef.current);
      const copiedKey = key;
      autoClearTimerRef.current = setTimeout(async () => {
        autoClearTimerRef.current = null;
        try {
          const current = await navigator.clipboard.readText();
          if (current === copiedKey) {
            await navigator.clipboard.writeText('');
            showToast('剪贴板已清空 (安全保护)');
          }
        } catch {
          // If readText is denied, leave the clipboard untouched.
        }
      }, API_KEY_AUTO_CLEAR_MS);
    } catch (err) {
      showToast(err instanceof Error ? err.message : '复制失败, 请手动选择 mask 后的 key 复制片段');
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
      {/* #684 — Identity card (header): avatar, name, state, ID, Created, and top buttons. */}
      <section className="agent-detail-card agent-detail-card-identity">
        <div className="admin-card-row">
          <div className="admin-card-info" style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            {agent.avatar_url && (
              <img
                src={agent.avatar_url}
                alt=""
                width={32}
                height={32}
                style={{ borderRadius: '50%', flexShrink: 0 }}
              />
            )}
            <div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <strong>{agent.display_name}</strong>
                {/* AL-1a (#R3): runtime 三态 + 故障原因. 文案锁见 lib/agent-state.ts (§11). */}
                <AgentStateBadge agent={agent} />
              </div>
              <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginTop: 2 }}>
                ID: {agent.id.slice(0, 12)}... | Created: {new Date(agent.created_at).toLocaleDateString()}
              </div>
            </div>
          </div>
          <div className="admin-card-actions">
            <button className="btn btn-sm" onClick={onToggle}>{expanded ? 'Collapse' : 'Manage'}</button>
            <button className="btn btn-sm btn-danger" onClick={onDelete}>Delete</button>
          </div>
        </div>
      </section>

      {expanded && (
        <>
          {/* #684 — Credentials card. Show the `bgr_...{last4}` mask by default,
              with no Show button. The full plaintext key never enters the DOM;
              copy fetches on demand, writes to clipboard, auto-clears after 60s,
              and first verifies clipboard contents with readText. */}
          <section className="agent-detail-card agent-detail-card-credentials">
            <strong>API Key</strong>
            <div className="api-key-box" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span data-testid="agent-api-key-mask" style={{ flex: 1 }}>
                {loadingKey ? '加载中...' : (last4 ? formatMaskedApiKey(last4) : '加载中...')}
              </span>
              <button
                className="btn-icon"
                onClick={handleCopyKey}
                disabled={copying || loadingKey}
                aria-label="复制 API Key"
                title="复制完整 API Key 到剪贴板"
              >
                {copying ? '...' : '📋'}
              </button>
            </div>
            <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
              <button className="btn btn-sm" onClick={handleRotateKey}>Rotate API Key</button>
            </div>
          </section>

          {/* #684 — Runtime card. AL-4.3 (#379 §1 split): fetchAgentRuntime null
              means graceful degradation, so omit the card (design ① "Borgee 不带 runtime").
              RuntimeCard handles the owner-only DOM gate: non-owners can see the
              status badge but not start/stop buttons (#321 §2). */}
          {runtimeLoaded && (
            <section className="agent-detail-card agent-detail-card-runtime">
              <RuntimeCard
                agent={agent}
                runtime={runtime}
                viewerUserID={viewerUserID}
                onRefresh={loadRuntime}
              />
            </section>
          )}

          {/* #684 — Config card. AL-2a.3 (#447 + #480 mount): agent config SSOT
              editor (name / avatar / prompt / model / capabilities / enabled /
              memory_ref). Blueprint §1.4 defines the SSOT boundary; server-side
              allowedConfigKeys fail closed. Runtime-only fields (api_key /
              temperature / token_limit / retry_policy) are not rendered here.
              AgentConfigPanel already includes the "Agent 配置" heading. */}
          <section className="agent-detail-card agent-detail-card-config">
            <AgentConfigPanel agentId={agent.id} />
          </section>

          {/* #684 — Permissions 卡 (第 5 卡). */}
          <section className="agent-detail-card agent-detail-card-permissions">
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
          </section>

          {/* #684 — Channels 卡 (第 6 卡, 加入频道). */}
          <section className="agent-detail-card agent-detail-card-channels">
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
          </section>
        </>
      )}
    </div>
  );
}

// AL-1a (#R3 Phase 2) — Agent state inline badge.
// Error-state reason label gives the owner an explainable failure reason
// (blueprint §2.3). data-state keeps the Playwright selector stable (REG-AL1A-*).
//
// AL-3.3 (#R3 Phase 2): usePresence cache receives `presence.changed` WebSocket
// frames, which are fresher than the fetchAgents() snapshot. Prefer the cache;
// on cache miss, fall back to agent.state, then to describeAgentState's "已离线".
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

  // #682: register the unsaved-changes guard. If the user has filled the form
  // but not submitted it, switching sidepanes prompts for confirmation. After
  // submit (createdKey is set), the form is a result view and no longer dirty.
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

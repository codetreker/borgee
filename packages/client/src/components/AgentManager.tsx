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
// 渲染 `bgr_...{last4}` 形式 (前缀 + ... + 末 4), 反 OpenAI `sk-` 误抄
// (yema brief v3 §3 文案锁; heima Sec 设计 1 末 4 位露够认人不够暴破解).
//
// 反约束: 完整明文 key 不进 DOM (Sec by-construction); 这个 helper **只**
// 接末 4 位字符串 (caller 只在 fetch 后 `key.slice(-4)`, 立刻丢完整 key),
// 不接全 key. 反 dev 误传完整 key 进来 mask 后字符串还在 React state.
//
// 前缀 `bgr_` 写死 — server-go `GenerateAPIKey()` 真值 (queries_phase2b.go:440).
// 跟 brief §2.3 + §2.4 grep 守卫一致, 不允许 `sk-` 出现.
function formatMaskedApiKey(last4: string): string {
  return `bgr_...${last4}`;
}

// #684 — Auto-clear delay 60 秒 (heima Sec 设计 3 + 1Password / Bitwarden 行业值).
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
  // #684 — API Key 显示策略. 反约束: 完整 plaintext key 永不进 React state /
  // ref, 只在 fetch closure 临时持有, 走 `key.slice(-4)` 拿 last4 后立即丢
  // (heima Sec 设计 by-construction + yema brief v3 §2.3).
  //
  // 之前的 visibleKey / newKey state 删掉 — 那俩存完整 plaintext, by-construction
  // 反约束.
  const [last4, setLast4] = useState<string | null>(null);
  const [loadingKey, setLoadingKey] = useState(false);
  const [copying, setCopying] = useState(false);
  // 60s auto-clear timer ID, 用 ref 避免 re-render 干扰 cleanup (跟 #695
  // useUnsavedChangesGuard useRef 思路一致, 反 closure staleness).
  const autoClearTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { showToast } = useToast();
  const [joinChannelId, setJoinChannelId] = useState('');

  // AL-4.3 (#379 §1 拆段): runtime 卡片状态. fetchAgentRuntime 返回
  // null 表示该 agent 还没注册 runtime (graceful degrade — 设计 ①
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

  // #684 — Load last4 of API key on expand. yema brief Q1 方案 B + Q2 现有
  // endpoint slice(-4) 立即丢: fetch 完整 key, 取末 4 位放进 state, **不存
  // 完整 key 任何位置** (反 by-construction). const localKey 走出 closure
  // 后被 GC.
  const loadKeyMask = useCallback(async () => {
    setLoadingKey(true);
    try {
      const data = await fetchAgent(agent.id);
      // 反约束: 只取 slice(-4), 不 setState 完整 key.
      if (typeof data.api_key === 'string' && data.api_key.length >= 4) {
        setLast4(data.api_key.slice(-4));
      } else {
        setLast4(null);
      }
    } catch {
      // 失败仅 inline 显示 — 跟 RuntimeCard "沉默胜于假 loading" 同模式.
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

  // #684 — Cleanup auto-clear timer on unmount. 反 dirty timer 在 component
  // 卸载后还触发 (用户切走 sidepane / 刷新页面). cleanup 不主动 writeText
  // 那次 — 用户 unmount 之后剪贴板状态由用户掌控, 不擅自动手, 跟 Sec 设计 3
  // "auto-clear 是 ux 安全提升, 不是强制权限" 一致.
  useEffect(() => {
    return () => {
      if (autoClearTimerRef.current) {
        clearTimeout(autoClearTimerRef.current);
        autoClearTimerRef.current = null;
      }
    };
  }, []);

  // #684 — 复制 + auto-clear 60s. 反约束: 完整 key 仅在本 fn closure 临时持有,
  // setLast4 后局部变量出作用域被 GC. setTimeout 60s 后 readText 比对, 只在
  // 剪贴板里还是这把 key 时才清 (yema brief §2.3 边界 — 用户 60s 内主动改剪
  // 贴板内容时不动他).
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
      // mask 同步刷新 (新 rotate 后 last4 跟 fetch 来的一致, 反竞态).
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
          // 不等 (例如用户已经主动复制别的) → 不动剪贴板, 不 toast 减打扰.
        } catch {
          // readText 可能被 permission 拒 (Firefox 默认拒 readText). 降级:
          // 不读不清, 让用户掌控. 反约束: 不主动 writeText('') 因为可能覆
          // 盖用户手工复制的内容.
        }
      }, API_KEY_AUTO_CLEAR_MS);
    } catch (err) {
      // 浏览器不支持 clipboard / 非 https 走 fallback. document.execCommand
      // 已 deprecated 但仍是 fallback 唯一选项, 反第三方库 (heima Sec 设计 4).
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
      // 不进 DOM, 直接走 clipboard + mask (跟 handleCopyKey 同模式).
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
          // readText 拒, 降级不动剪贴板.
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
      {/* #684 — Identity 卡 (Header). yema brief §2.1 第 1 卡: 头像 + 名 +
          状态 + ID + Created + 顶部按钮. */}
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
                {/* AL-1a (#R3): runtime 三态 + 故障原因. 文案锁见 lib/agent-state.ts (野马 #190 §11). */}
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
          {/* #684 — Credentials 卡 (yema brief §2.1 第 2 卡 + §2.3 重点).
              默认显 mask `bgr_...{last4}`, 没 Show 按钮. 完整 plaintext key 永
              不进 DOM (heima Sec by-construction); 复制按钮按需 fetch + clipboard
              + auto-clear 60s + readText 比对. */}
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

          {/* #684 — Runtime 卡 (第 3 卡). AL-4.3 (#379 §1 拆段) — fetchAgentRuntime
              null → graceful degrade omit (设计 ① "Borgee 不带 runtime");
              非 owner → owner-only DOM gate 走 RuntimeCard 内部 isOwner 判断
              (反约束: 非 owner 看到 status badge 但看不到 start/stop btn,
              跟 #321 §2 同源). */}
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

          {/* #684 — Config 卡 (第 4 卡). AL-2a.3 (#447 + #480 mount) — agent
              config SSOT editor (name / avatar / prompt / model / capabilities
              / enabled / memory_ref). 蓝图 §1.4 SSOT 字段划界, server-side
              allowedConfigKeys whitelist fail-closed; 反约束 设计 ⑤ runtime-only
              字段 (api_key / temperature / token_limit / retry_policy) 此 form
              不渲染. AgentConfigPanel 内部已有 "Agent 配置" 标题, 删外层冗余. */}
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

import { useState, useEffect, useCallback } from 'react';
import { useAppContext } from '../context/AppContext';
import { useUnsavedChangesGuard } from '../hooks/useUnsavedChangesGuard';
import {
  fetchRemoteNodes,
  createRemoteNode,
  deleteRemoteNode,
  fetchRemoteBindings,
  createRemoteBinding,
  deleteRemoteBinding,
  fetchRemoteNodeStatus,
  type RemoteNode,
  type RemoteBinding,
} from '../lib/api';

interface Props {
  onBack: () => void;
}

export default function NodeManager({ onBack }: Props) {
  const { state } = useAppContext();
  const [nodes, setNodes] = useState<RemoteNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [selectedNode, setSelectedNode] = useState<string | null>(null);
  const [onlineStatus, setOnlineStatus] = useState<Record<string, boolean>>({});

  const load = useCallback(async () => {
    try {
      const data = await fetchRemoteNodes();
      setNodes(data);
      const statuses: Record<string, boolean> = {};
      await Promise.all(
        data.map(async (n) => {
          try {
            const s = await fetchRemoteNodeStatus(n.id);
            statuses[n.id] = s.online;
          } catch {
            statuses[n.id] = false;
          }
        }),
      );
      setOnlineStatus(statuses);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleCreate = async (name: string) => {
    const node = await createRemoteNode(name);
    setShowCreate(false);
    setNodes(prev => [node, ...prev]);
    setSelectedNode(node.id);
  };

  const handleDelete = async (id: string) => {
    if (!confirm('确定删除此 Node？所有绑定也会被删除。')) return;
    await deleteRemoteNode(id);
    setNodes(prev => prev.filter(n => n.id !== id));
    if (selectedNode === id) setSelectedNode(null);
  };

  if (loading) {
    return (
      <div className="node-manager">
        <div className="node-manager-header">
          <button className="btn btn-sm" onClick={onBack}>← 返回</button>
          <h2>Remote Nodes</h2>
        </div>
        <div className="remote-empty">加载中...</div>
      </div>
    );
  }

  const selected = nodes.find(n => n.id === selectedNode);

  return (
    <div className="node-manager">
      <div className="node-manager-header">
        <button className="btn btn-sm" onClick={onBack}>← 返回</button>
        <h2>Remote Nodes</h2>
        <button className="btn btn-sm btn-primary" onClick={() => setShowCreate(true)}>+ 添加 Node</button>
      </div>

      {showCreate && <CreateNodeForm onSubmit={handleCreate} onCancel={() => setShowCreate(false)} />}

      <div className="node-manager-content">
        <div className="node-list">
          {nodes.length === 0 ? (
            <div className="remote-empty">
              <p>暂无 Remote Node</p>
              <p style={{ fontSize: '0.85em' }}>点击"+ 添加 Node"创建一个</p>
            </div>
          ) : (
            nodes.map(node => (
              <div
                key={node.id}
                className={`node-list-item${selectedNode === node.id ? ' active' : ''}`}
                onClick={() => setSelectedNode(node.id)}
              >
                <span className={`node-status-dot ${onlineStatus[node.id] ? 'online' : 'offline'}`} />
                <span className="node-list-name">{node.machine_name}</span>
              </div>
            ))
          )}
        </div>

        {selected && (
          <NodeDetail
            node={selected}
            online={onlineStatus[selected.id] ?? false}
            channels={state.channels}
            onDelete={() => handleDelete(selected.id)}
          />
        )}
      </div>
    </div>
  );
}

// gh#703 PR-2/2 — export 给单测直接渲染 (反 NodeManager 拉全 AppContext +
// fetchRemoteNodes mock 重壳测试).
export function CreateNodeForm({ onSubmit, onCancel }: { onSubmit: (name: string) => void; onCancel: () => void }) {
  const [name, setName] = useState('');

  // gh#703 PR-2/2 — sidepane 切换 + beforeunload 守卫 (创建 form 模式).
  // 字段非空 (trim 后) 算 dirty, 反 JSON.stringify byte 比对 (创建 form 没
  // original baseline 不需要顺序敏感 deep equal).
  useUnsavedChangesGuard(
    () => name.trim() !== '',
    'Create Node 表单有填写但还没提交, 离开会丢失. 确认离开吗?',
  );

  return (
    <div className="node-create-form">
      <input
        type="text"
        placeholder="机器名称（如：my-server）"
        value={name}
        onChange={e => setName(e.target.value)}
        onKeyDown={e => { if (e.key === 'Enter' && name.trim()) onSubmit(name.trim()); }}
        autoFocus
      />
      <button className="btn btn-sm btn-primary" onClick={() => name.trim() && onSubmit(name.trim())} disabled={!name.trim()}>创建</button>
      <button className="btn btn-sm" onClick={onCancel}>取消</button>
    </div>
  );
}

// gh#703 PR-2/2 — export 给单测直接渲染 (反 NodeManager 拉全 AppContext +
// fetchRemoteNodes mock 重壳测试).
export function NodeDetail({ node, online, channels, onDelete }: {
  node: RemoteNode;
  online: boolean;
  channels: { id: string; name: string }[];
  onDelete: () => void;
}) {
  const [bindings, setBindings] = useState<RemoteBinding[]>([]);
  const [showToken, setShowToken] = useState(false);
  const [copied, setCopied] = useState(false);
  const [showAddBinding, setShowAddBinding] = useState(false);
  const [bindChannelId, setBindChannelId] = useState('');
  const [bindPath, setBindPath] = useState('');
  const [bindLabel, setBindLabel] = useState('');

  // gh#703 PR-2/2 — sidepane 切换 + beforeunload 守卫 (创建 form 模式).
  // showAddBinding 开 + 任一字段非空才算 dirty (form 没显时不算, 反 form
  // 关闭后旧字段值还残留误触发). bindChannelId 是 select value, 不需要 trim.
  useUnsavedChangesGuard(
    () =>
      showAddBinding &&
      (bindChannelId !== '' ||
        bindPath.trim() !== '' ||
        bindLabel.trim() !== ''),
    '目录绑定表单有填写但还没提交, 离开会丢失. 确认离开吗?',
  );

  const loadBindings = useCallback(async () => {
    try {
      const data = await fetchRemoteBindings(node.id);
      setBindings(data);
    } catch {
      setBindings([]);
    }
  }, [node.id]);

  useEffect(() => {
    loadBindings();
    setShowToken(false);
    setCopied(false);
  }, [loadBindings]);

  // no-hardcoded-domain milestone: server URL injected at build time via
  // VITE_AGENT_WS_SERVER (.env.example documents the var). 0 hardcoded
  // codetrek.cn 字面 in source — fork / staging / testing / on-prem 部署
  // 真生效 (deploy.yml + deploy-test.yml inject per-env value).
  const agentWsServer = (import.meta.env.VITE_AGENT_WS_SERVER as string | undefined) ?? 'wss://localhost:4900';
  const startCmd = `npx @codetreker/borgee-remote-agent --server ${agentWsServer} --token ${showToken ? node.connection_token : '••••••••'} --dirs /path/to/dir`;
  const fullCmd = `npx @codetreker/borgee-remote-agent --server ${agentWsServer} --token ${node.connection_token} --dirs /path/to/dir`;

  const handleCopy = () => {
    navigator.clipboard.writeText(fullCmd);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleAddBinding = async () => {
    if (!bindChannelId || !bindPath.trim()) return;
    await createRemoteBinding(node.id, bindChannelId, bindPath.trim(), bindLabel.trim() || undefined);
    setShowAddBinding(false);
    setBindChannelId('');
    setBindPath('');
    setBindLabel('');
    loadBindings();
  };

  const handleDeleteBinding = async (bindingId: string) => {
    await deleteRemoteBinding(node.id, bindingId);
    loadBindings();
  };

  return (
    <div className="node-detail">
      <div className="node-detail-header">
        <h3>{node.machine_name}</h3>
        <span className={`node-status-badge ${online ? 'online' : 'offline'}`}>
          {online ? '在线' : '离线'}
        </span>
      </div>

      <div className="node-detail-section">
        <h4>连接信息</h4>
        <div className="node-token-area">
          <button className="btn btn-sm" onClick={() => setShowToken(!showToken)}>
            {showToken ? '隐藏 Token' : '显示 Token'}
          </button>
          {showToken && (
            <code className="node-token">{node.connection_token}</code>
          )}
        </div>
        <div className="node-cmd-area">
          <p style={{ fontSize: '0.8em', color: 'var(--text-secondary)', margin: '8px 0 4px' }}>启动命令：</p>
          <div className="node-cmd-box">
            <code>{startCmd}</code>
            <button className="btn btn-sm" onClick={handleCopy}>{copied ? '已复制' : '复制'}</button>
          </div>
        </div>
      </div>

      <div className="node-detail-section">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <h4>目录绑定</h4>
          <button className="btn btn-sm" onClick={() => setShowAddBinding(true)}>+ 绑定</button>
        </div>

        {showAddBinding && (
          <div className="node-bind-form">
            <select value={bindChannelId} onChange={e => setBindChannelId(e.target.value)}>
              <option value="">选择频道...</option>
              {channels.filter(c => (c as any).type !== 'dm').map(c => (
                <option key={c.id} value={c.id}>#{c.name}</option>
              ))}
            </select>
            <input type="text" placeholder="远程路径 (如 /workspace)" value={bindPath} onChange={e => setBindPath(e.target.value)} />
            <input type="text" placeholder="标签 (可选)" value={bindLabel} onChange={e => setBindLabel(e.target.value)} />
            <div style={{ display: 'flex', gap: '6px' }}>
              <button className="btn btn-sm btn-primary" onClick={handleAddBinding} disabled={!bindChannelId || !bindPath.trim()}>确定</button>
              <button className="btn btn-sm" onClick={() => setShowAddBinding(false)}>取消</button>
            </div>
          </div>
        )}

        {bindings.length === 0 ? (
          <p style={{ fontSize: '0.85em', color: 'var(--text-secondary)' }}>暂无绑定</p>
        ) : (
          <div className="node-binding-list">
            {bindings.map(b => {
              const ch = channels.find(c => c.id === b.channel_id);
              return (
                <div key={b.id} className="node-binding-item">
                  <span className="node-binding-path">{b.label || b.path}</span>
                  <span className="node-binding-channel">→ #{ch?.name ?? '未知'}</span>
                  <button className="btn btn-sm btn-danger" onClick={() => handleDeleteBinding(b.id)}>解绑</button>
                </div>
              );
            })}
          </div>
        )}
      </div>

      <div className="node-detail-section">
        <button className="btn btn-sm btn-danger" onClick={onDelete}>Delete</button>
      </div>
    </div>
  );
}

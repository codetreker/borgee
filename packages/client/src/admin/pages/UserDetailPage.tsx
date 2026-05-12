import React, { useEffect, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import {
  fetchUserAgents,
  fetchUsers,
  fetchUserPermissions,
  grantUserPermission,
  revokeUserPermission,
  patchUser,
} from '../api';
import type { AdminUser, UserPermissionDetail } from '../api';
import { CAPABILITY_TOKENS, capabilityLabel, isKnownCapability } from '../../lib/capabilities';

// ADMIN-SPA-UI-COVERAGE: surface 3 server-wired admin endpoints that had
// no UI before (D6 coverage: capability grant + role/disabled/password
// PATCH). No server, endpoint, or schema change; the admin path stays separate
// under ADM-0 §1.3, with only the admin SPA calling /admin-api/*.

export default function UserDetailPage() {
  const { id } = useParams();
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [agents, setAgents] = useState<AdminUser[]>([]);
  const [permissions, setPermissions] = useState<UserPermissionDetail[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [actionMsg, setActionMsg] = useState('');
  const userId = id ? decodeURIComponent(id) : '';

  // edit-form local state
  const [editingPassword, setEditingPassword] = useState('');
  const [editingRole, setEditingRole] = useState<'member' | 'agent' | ''>('');
  const [editingDisabled, setEditingDisabled] = useState<boolean | null>(null);

  // grant-form local state
  const [grantCapability, setGrantCapability] = useState<string>(CAPABILITY_TOKENS[0]);
  const [grantScope, setGrantScope] = useState<string>('*');

  async function reload() {
    setLoading(true);
    try {
      const [nextUsers, nextAgents, nextPerms] = await Promise.all([
        fetchUsers(),
        fetchUserAgents(userId),
        fetchUserPermissions(userId),
      ]);
      setUsers(nextUsers);
      setAgents(nextAgents);
      setPermissions(nextPerms.details);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load user');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    if (!userId) return;
    void reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [userId]);

  const user = useMemo(() => users.find(u => u.id === userId), [users, userId]);

  async function handleResetPassword() {
    if (!editingPassword) return;
    try {
      await patchUser(userId, { password: editingPassword });
      setActionMsg('密码已重置');
      setEditingPassword('');
    } catch (err) {
      setActionMsg(err instanceof Error ? err.message : '重置失败');
    }
  }

  async function handleSetRole() {
    if (!editingRole) return;
    try {
      await patchUser(userId, { role: editingRole });
      setActionMsg(`角色已改为 ${editingRole}`);
      await reload();
    } catch (err) {
      setActionMsg(err instanceof Error ? err.message : '改角色失败');
    }
  }

  async function handleToggleDisabled() {
    const next = !user?.disabled;
    try {
      await patchUser(userId, { disabled: next });
      setActionMsg(next ? '账号已停用' : '账号已启用');
      await reload();
    } catch (err) {
      setActionMsg(err instanceof Error ? err.message : '切换失败');
    }
  }

  async function handleGrant() {
    if (!isKnownCapability(grantCapability)) {
      setActionMsg('未知能力 (反 CAPABILITY-DOT 14 const)');
      return;
    }
    try {
      await grantUserPermission(userId, grantCapability, grantScope || '*');
      setActionMsg(`已授予 ${capabilityLabel(grantCapability)} (${grantScope || '*'})`);
      await reload();
    } catch (err) {
      setActionMsg(err instanceof Error ? err.message : '授权失败');
    }
  }

  async function handleRevoke(perm: UserPermissionDetail) {
    try {
      await revokeUserPermission(userId, perm.permission, perm.scope);
      setActionMsg(`已撤销 ${capabilityLabel(perm.permission)} (${perm.scope})`);
      await reload();
    } catch (err) {
      setActionMsg(err instanceof Error ? err.message : '撤销失败');
    }
  }

  if (loading) return <div className="app-loading"><div className="loading-spinner-large" /></div>;
  if (error) return <div className="admin-error">{error}</div>;
  if (!user) return <div className="admin-error">User not found</div>;

  return (
    <div data-page="admin-user-detail">
      <div className="admin-section-header">
        <h2>{user.display_name}</h2>
        <Link className="btn btn-sm" to="/admin/users">Back</Link>
      </div>
      <div className="admin-card admin-detail-grid">
        <Field label="ID" value={user.id} mono />
        <Field label="Email" value={user.email ?? '-'} />
        <Field label="Role" value={user.role} />
        <Field label="Status" value={user.deleted_at ? 'Deleted' : user.disabled ? 'Disabled' : 'Active'} />
        <Field label="Created" value={formatDate(user.created_at)} />
        <Field label="Last Seen" value={user.last_seen_at ? formatDate(user.last_seen_at) : '-'} />
      </div>

      {actionMsg && (
        <div className="admin-action-msg" data-asuc-action-msg>{actionMsg}</div>
      )}

      <div className="admin-section-header"><h2>账号操作</h2></div>
      <div className="admin-card" data-asuc-account-actions>
        <div className="admin-action-row">
          <label>重置密码：</label>
          <input
            type="password"
            placeholder="新密码"
            value={editingPassword}
            onChange={e => setEditingPassword(e.target.value)}
            data-asuc-password-input
          />
          <button
            className="btn btn-sm"
            onClick={handleResetPassword}
            disabled={!editingPassword}
            data-asuc-reset-password
          >重置密码</button>
        </div>
        <div className="admin-action-row">
          <label>改角色：</label>
          <select
            value={editingRole}
            onChange={e => setEditingRole(e.target.value as 'member' | 'agent' | '')}
            data-asuc-role-select
          >
            <option value="">- 选择角色 -</option>
            <option value="member">member</option>
            <option value="agent">agent</option>
          </select>
          <button
            className="btn btn-sm"
            onClick={handleSetRole}
            disabled={!editingRole}
            data-asuc-set-role
          >设置角色</button>
        </div>
        <div className="admin-action-row">
          <label>账号状态：</label>
          <button
            className="btn btn-sm"
            onClick={handleToggleDisabled}
            data-asuc-toggle-disabled
          >{user.disabled ? '启用账号' : '停用账号'}</button>
        </div>
      </div>

      <div className="admin-section-header"><h2>能力授权</h2></div>
      <div className="admin-card" data-asuc-grant-form>
        <div className="admin-action-row">
          <label>能力：</label>
          <select
            value={grantCapability}
            onChange={e => setGrantCapability(e.target.value)}
            data-asuc-capability-select
          >
            {CAPABILITY_TOKENS.map(tok => (
              <option key={tok} value={tok}>{capabilityLabel(tok)} ({tok})</option>
            ))}
          </select>
          <label>范围：</label>
          <input
            type="text"
            placeholder="* 或 channel:&lt;id&gt; 或 artifact:&lt;id&gt;"
            value={grantScope}
            onChange={e => setGrantScope(e.target.value)}
            data-asuc-scope-input
          />
          <button
            className="btn btn-sm btn-primary"
            onClick={handleGrant}
            data-asuc-grant-button
          >授予</button>
        </div>
      </div>

      <div className="admin-section-header"><h2>当前授权 ({permissions.length})</h2></div>
      <div className="admin-table-wrapper" data-asuc-permissions-list>
        <table className="admin-table">
          <thead>
            <tr><th>能力</th><th>Token</th><th>Scope</th><th>授予时间</th><th>操作</th></tr>
          </thead>
          <tbody>
            {permissions.map((p, i) => (
              <tr key={`${p.permission}-${p.scope}-${i}`} data-asuc-permission-row>
                <td>{capabilityLabel(p.permission)}</td>
                <td><code>{p.permission}</code></td>
                <td><code>{p.scope}</code></td>
                <td>{formatDate(p.granted_at)}</td>
                <td>
                  <button
                    className="btn btn-sm btn-danger"
                    onClick={() => handleRevoke(p)}
                    data-asuc-revoke-button
                  >撤销</button>
                </td>
              </tr>
            ))}
            {permissions.length === 0 && (
              <tr><td colSpan={5} style={{ textAlign: 'center' }}>暂无授权</td></tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="admin-section-header"><h2>Agents</h2></div>
      <div className="admin-table-wrapper">
        <table className="admin-table">
          <thead>
            <tr><th>ID</th><th>Name</th><th>Status</th><th>Created</th></tr>
          </thead>
          <tbody>
            {agents.map(agent => (
              <tr key={agent.id}>
                <td><code className="user-id-cell">{agent.id}</code></td>
                <td>{agent.display_name}</td>
                <td>{agent.deleted_at ? 'Deleted' : agent.disabled ? 'Disabled' : 'Active'}</td>
                <td>{formatDate(agent.created_at)}</td>
              </tr>
            ))}
            {agents.length === 0 && <tr><td colSpan={4} style={{ textAlign: 'center' }}>No agents</td></tr>}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <div className="admin-detail-label">{label}</div>
      <div className={mono ? 'admin-detail-value admin-detail-mono' : 'admin-detail-value'}>{value}</div>
    </div>
  );
}

function formatDate(ts: number): string {
  return new Date(ts).toLocaleString();
}

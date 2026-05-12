// AdminAuditLogPage — ADM-2.2 admin SPA audit-log 页 (#484).
//
// Blueprint: docs/blueprint/current/admin-model.md §1.4 rule: admin write actions
// must be audited and visible across admins.
// Spec: docs/current/admin/README.md §6 admin API GET /admin-api/v1/audit-log
//   - Default query has no WHERE; the three filters
//     (?actor_id / ?action / ?target_user_id) are UI narrowing, not buckets.
//   - Admin-cookie routing is separate from user cookies
//     (user cookie → 401, REG-ADM0-002 baseline).
// Content lock: docs/qa/adm-2-content-lock.md §5 (admin SPA cross-surface literals,
//   admin English enum values stay aligned with user-side Chinese verbs).
//
// DOM 锚: `[data-page="admin-audit-log"]` + 每行 `[data-action-row]` + 每
// filter input `[data-filter="{actor|action|target}"]`.
//
// Cross-surface literal lock (constraint summary):
//   - This page uses English enum action literals (delete_channel/suspend_user/...).
//   - 用户端 Settings/AdminActionsList 走中文动词字面 (ACTION_VERBS map
//     in the user SPA; admin SPA must not import it. See
//     adm-2-admin-spa-cross-end.test.ts for the reverse check.
//   - 改 enum = 改 server CHECK constraint + 此 admin SPA + 用户端 SPA 三处.
//   - Constraint: admin must not render Chinese verbs; admin SPA displays the
//     English enum directly, while Chinese verbs are user-facing.
//   - Constraint: admin SPA renders actor_id (admins can see each other);
//     the user side does not render raw actor_id and uses admin
//     lookup to show admin_username instead.

import React, { useEffect, useState } from 'react';
import { fetchAdminAuditLog, type AdminActionRow, type AuditLogFilters } from '../api';

// ACTION_ENUM — English enum literals match the server CHECK constraint
// (admin_actions 表 CHECK (action IN ('delete_channel', 'suspend_user', ...))).
// 改这里 = 改 server migration v=22 + user SPA AdminActionsList ACTION_VERBS.
const ACTION_ENUM = [
  'delete_channel',
  'suspend_user',
  'change_role',
  'reset_password',
  'start_impersonation',
] as const;

function formatTs(ms: number): string {
  const d = new Date(ms);
  const pad = (n: number) => n.toString().padStart(2, '0');
  // Server-side `time.Format("2006-01-02 15:04")` uses the same literal format.
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export default function AdminAuditLogPage() {
  const [rows, setRows] = useState<AdminActionRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<AuditLogFilters>({});
  const [busy, setBusy] = useState(false);

  const load = (f: AuditLogFilters) => {
    setBusy(true);
    setError(null);
    fetchAdminAuditLog(f)
      .then((data) => setRows(data))
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setBusy(false));
  };

  useEffect(() => {
    load(filters);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleFilter = (e: React.FormEvent) => {
    e.preventDefault();
    load(filters);
  };

  const handleReset = () => {
    setFilters({});
    load({});
  };

  return (
    <div data-page="admin-audit-log" data-adm2-audit-list="true">
      {/* ADM-2-FOLLOWUP REG-011 — red banner active when impersonate session is in effect.
          Literal text follows content-lock §1 + admin-2-followup-stance §1. */}
      <div data-adm2-red-banner="active" className="admin-impersonate-banner" role="alert">
        当前以业主身份操作 — 该会话受 24h 时限
      </div>
      <div className="admin-section-header">
        <h2>审计日志</h2>
      </div>

      <form
        className="admin-audit-filters"
        onSubmit={handleFilter}
        aria-label="Audit log filters"
      >
        <label>
          Actor ID
          <input
            type="text"
            data-filter="actor"
            value={filters.actor_id ?? ''}
            onChange={(e) => setFilters({ ...filters, actor_id: e.target.value || undefined })}
            placeholder="UUID"
          />
        </label>
        <label>
          Action
          <select
            data-filter="action"
            value={filters.action ?? ''}
            onChange={(e) => setFilters({ ...filters, action: e.target.value || undefined })}
          >
            <option value="">(any)</option>
            {ACTION_ENUM.map((a) => (
              <option key={a} value={a}>
                {a}
              </option>
            ))}
          </select>
        </label>
        <label>
          Target User ID
          <input
            type="text"
            data-filter="target"
            value={filters.target_user_id ?? ''}
            onChange={(e) => setFilters({ ...filters, target_user_id: e.target.value || undefined })}
            placeholder="UUID"
          />
        </label>
        {/* ADMIN-SPA-ARCHIVED-UI-FOLLOWUP: AL-8 §0 archived tri-state
            filter toggle. 默认 "active" (empty value maps to server active default).
            content-lock §1: 3 label "Active" / "Archived" / "All" 字面单源. */}
        <label>
          View
          <select
            data-filter="archived"
            value={filters.archived ?? 'active'}
            onChange={(e) =>
              setFilters({
                ...filters,
                archived: (e.target.value as 'active' | 'archived' | 'all'),
              })
            }
          >
            <option value="active">Active</option>
            <option value="archived">Archived</option>
            <option value="all">All</option>
          </select>
        </label>
        <button type="submit" className="btn btn-sm" disabled={busy}>
          Filter
        </button>
        <button type="button" className="btn btn-sm" onClick={handleReset} disabled={busy}>
          Reset
        </button>
      </form>

      {error !== null && (
        <p className="admin-error" role="alert">
          Failed to load: {error}
        </p>
      )}

      {rows === null && error === null && <div className="app-loading">Loading...</div>}

      {rows !== null && rows.length === 0 && error === null && (
        <p className="admin-audit-empty">暂无审计记录</p>
      )}

      {rows !== null && rows.length > 0 && (
        <table className="admin-audit-table" data-section="admin-audit-log-table">
          <thead>
            <tr>
              <th scope="col">Time</th>
              <th scope="col">Actor</th>
              <th scope="col">Action</th>
              <th scope="col">Target</th>
              <th scope="col">Metadata</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr
                key={row.id}
                data-action-row
                data-action={row.action}
                data-adm2-actor-kind="admin"
                /* ADMIN-SPA-SHAPE-FIX D4: AL-8 §0 archived tri-state row class.
                   server sanitizeAdminAction nil-safe surface archived_at —
                   null/缺 = active, non-null = archived. */
                data-archived-state={row.archived_at != null ? 'archived' : 'active'}
                className={row.archived_at != null ? 'admin-audit-row-archived' : 'admin-audit-row-active'}
              >
                <td>{formatTs(row.created_at)}</td>
                <td className="admin-audit-actor">
                  <code>{row.actor_id}</code>
                </td>
                <td className="admin-audit-action">{row.action}</td>
                <td className="admin-audit-target">
                  <code>{row.target_user_id}</code>
                </td>
                <td className="admin-audit-metadata">
                  <code>{row.metadata}</code>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

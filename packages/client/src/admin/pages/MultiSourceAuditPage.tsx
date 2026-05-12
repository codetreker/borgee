// MultiSourceAuditPage — ADM-3 multi-source audit merged-query admin SPA page.
//
// Blueprint: docs/blueprint/current/admin-model.md §1.4 来源透明 (人/agent/admin/混合).
// Spec: docs/implementation/modules/adm-3-spec.md §1 ADM3.2.
//
// Intent:
//   - 4 source enum literals (server/plugin/host_bridge/agent)
//     must match server-side AuditSources; changes require
//     updating the server const, this page, and the i18n entries together.
//   - Admin path stays separate under ADM-0 §1.3: only
//     /admin-api/v1/audit/multi-source is exposed, with no user API path added.
//   - DOM anchor: `[data-page="admin-audit-multi-source"]` + each row
//     `[data-source-row="{source}"]`.

import React, { useEffect, useState } from 'react';
import {
  AUDIT_SOURCES,
  AuditSource,
  fetchMultiSourceAudit,
  MultiSourceAuditRow,
} from '../api';

// SOURCE_LABEL — 4 sources and 4 i18n keys. Keep aligned with the
// server const and content-lock §1.
const SOURCE_LABEL: Record<AuditSource, string> = {
  server: 'Server',
  plugin: 'Plugin',
  host_bridge: 'Host Bridge',
  agent: 'Agent',
};

const SOURCE_BADGE_CLASS: Record<AuditSource, string> = {
  server: 'audit-source-badge audit-source-server',
  plugin: 'audit-source-badge audit-source-plugin',
  host_bridge: 'audit-source-badge audit-source-host-bridge',
  agent: 'audit-source-badge audit-source-agent',
};

function formatTs(ms: number): string {
  if (!ms) return '';
  const d = new Date(ms);
  const pad = (n: number) => n.toString().padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export default function MultiSourceAuditPage() {
  const [rows, setRows] = useState<MultiSourceAuditRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [source, setSource] = useState<AuditSource | ''>('');
  const [busy, setBusy] = useState(false);

  const load = (src: AuditSource | '') => {
    setBusy(true);
    setError(null);
    fetchMultiSourceAudit(src ? { source: src } : {})
      .then((data) => setRows(data))
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setBusy(false));
  };

  useEffect(() => {
    load(source);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleSource = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const v = e.target.value as AuditSource | '';
    setSource(v);
    load(v);
  };

  return (
    <div data-page="admin-audit-multi-source">
      <div className="admin-section-header">
        <h2>Multi-source Audit</h2>
        <p className="admin-section-hint">
          蓝图 §1.4 来源透明: server / plugin / host_bridge / agent 4 源合并查询.
        </p>
      </div>
      <div className="admin-filter-bar">
        <label htmlFor="audit-source-filter">Source:</label>
        <select
          id="audit-source-filter"
          data-filter="source"
          value={source}
          onChange={handleSource}
        >
          <option value="">All sources</option>
          {AUDIT_SOURCES.map((s) => (
            <option key={s} value={s}>{SOURCE_LABEL[s]}</option>
          ))}
        </select>
        {busy && <span className="admin-busy-indicator">Loading…</span>}
      </div>

      {error && <div className="admin-error" role="alert">{error}</div>}

      {rows && rows.length === 0 && (
        <div className="admin-empty-state">No audit rows.</div>
      )}

      {rows && rows.length > 0 && (
        <table className="admin-table" data-testid="multi-source-audit-table">
          <thead>
            <tr>
              <th>Source</th>
              <th>When</th>
              <th>Actor</th>
              <th>Action</th>
              <th>Payload</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row, i) => (
              <tr key={`${row.source}-${row.ts}-${i}`} data-source-row={row.source}>
                <td>
                  <span className={SOURCE_BADGE_CLASS[row.source]}>
                    {SOURCE_LABEL[row.source]}
                  </span>
                </td>
                <td>{formatTs(row.ts)}</td>
                <td className="audit-actor">{row.actor}</td>
                <td className="audit-action">{row.action}</td>
                <td className="audit-payload">
                  <code>{row.payload}</code>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

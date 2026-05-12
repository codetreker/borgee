import React, { useCallback, useEffect, useState } from 'react';
import { fetchAdminRuntimes } from '../api';
import type { AdminRuntime } from '../api';

// ADMIN-SPA-UI-COVERAGE-WAVE2: GET /admin-api/v1/runtimes admin god-mode
// metadata read (AL-4.2 #398). Readonly full agent_runtimes view;
// last_error_reason server-side OMITTED (ADM-0 §1.3 隐私). 0 server 改.

export default function RuntimesPage() {
  const [runtimes, setRuntimes] = useState<AdminRuntime[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setRuntimes(await fetchAdminRuntimes());
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load runtimes');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  if (loading) return <div className="app-loading"><div className="loading-spinner-large" /></div>;
  if (error) return <div className="admin-error">{error}</div>;

  return (
    <div data-page="admin-runtimes">
      <div className="admin-section-header">
        <h2>运行时 ({runtimes.length})</h2>
        <button className="btn btn-sm" onClick={load} data-asuc2-runtimes-refresh>刷新</button>
      </div>
      <div className="admin-table-wrapper" data-asuc2-runtimes-list>
        <table className="admin-table">
          <thead>
            <tr>
              <th>Agent ID</th>
              <th>Endpoint</th>
              <th>Process</th>
              <th>Status</th>
              <th>Last Heartbeat</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            {runtimes.map(rt => (
              <tr key={rt.id} data-asuc2-runtime-row>
                <td><code className="user-id-cell">{rt.agent_id}</code></td>
                <td><code>{rt.endpoint_url}</code></td>
                <td>{rt.process_kind}</td>
                <td>{rt.status}</td>
                <td>{rt.last_heartbeat_at ? formatDate(rt.last_heartbeat_at) : '-'}</td>
                <td>{formatDate(rt.created_at)}</td>
              </tr>
            ))}
            {runtimes.length === 0 && (
              <tr><td colSpan={6} style={{ textAlign: 'center' }}>暂无运行时</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function formatDate(ts: number): string {
  return new Date(ts).toLocaleString();
}

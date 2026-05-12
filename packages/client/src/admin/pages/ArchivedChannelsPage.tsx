import React, { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { fetchAdminArchivedChannels } from '../api';
import type { AdminArchivedChannel } from '../api';

// ADMIN-SPA-UI-COVERAGE-WAVE2: GET /admin-api/v1/channels/archived (CHN-5 #189).
// Intent: admin read-only full-org archived view; do not mount an unarchive
// entry point. Admin routes do not mutate archived state directly; owners use
// the user API path. No server change.

export default function ArchivedChannelsPage() {
  const [channels, setChannels] = useState<AdminArchivedChannel[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setChannels(await fetchAdminArchivedChannels());
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load archived channels');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  if (loading) return <div className="app-loading"><div className="loading-spinner-large" /></div>;
  if (error) return <div className="admin-error">{error}</div>;

  return (
    <div data-page="admin-archived-channels">
      <div className="admin-section-header">
        <h2>已归档频道 ({channels.length})</h2>
        <button className="btn btn-sm" onClick={load} data-asuc2-archived-refresh>刷新</button>
      </div>
      <div className="admin-table-wrapper" data-asuc2-archived-list>
        <table className="admin-table">
          <thead>
            <tr>
              <th>名称</th>
              <th>类型</th>
              <th>可见性</th>
              <th>成员数</th>
              <th>归档时间</th>
              <th>描述历史</th>
            </tr>
          </thead>
          <tbody>
            {channels.map(c => (
              <tr key={c.id} data-asuc2-archived-row>
                <td>{c.name}</td>
                <td>{c.type}</td>
                <td>{c.visibility}</td>
                <td>{c.member_count}</td>
                <td>{c.archived_at ? formatDate(c.archived_at) : '-'}</td>
                <td>
                  <Link
                    className="btn btn-sm"
                    to={`/admin/channels/${encodeURIComponent(c.id)}/description-history`}
                    data-asuc2-history-link
                  >查看</Link>
                </td>
              </tr>
            ))}
            {channels.length === 0 && (
              <tr><td colSpan={6} style={{ textAlign: 'center' }}>暂无归档频道</td></tr>
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

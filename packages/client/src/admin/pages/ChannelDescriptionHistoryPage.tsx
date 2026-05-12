import React, { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { fetchAdminChannelDescriptionHistory } from '../api';
import type { ChannelDescriptionHistoryEntry } from '../api';

// ADMIN-SPA-UI-COVERAGE-WAVE2: GET /admin-api/v1/channels/{id}/description/history
// (CHN-14 #429). Readonly description-history viewer; entry shape
// `{old_content, ts, reason}`. 3 fields match server queries.go. No server changes.

export default function ChannelDescriptionHistoryPage() {
  const { id } = useParams();
  const channelId = id ? decodeURIComponent(id) : '';
  const [history, setHistory] = useState<ChannelDescriptionHistoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!channelId) return;
    let cancelled = false;
    (async () => {
      try {
        const data = await fetchAdminChannelDescriptionHistory(channelId);
        if (!cancelled) setHistory(data);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load history');
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [channelId]);

  if (loading) return <div className="app-loading"><div className="loading-spinner-large" /></div>;
  if (error) return <div className="admin-error">{error}</div>;

  return (
    <div data-page="admin-channel-description-history">
      <div className="admin-section-header">
        <h2>描述变更历史</h2>
        <Link className="btn btn-sm" to="/admin/channels-archived">返回</Link>
      </div>
      <div className="admin-card admin-detail-grid">
        <div>
          <div className="admin-detail-label">频道 ID</div>
          <div className="admin-detail-value admin-detail-mono">{channelId}</div>
        </div>
        <div>
          <div className="admin-detail-label">变更条数</div>
          <div className="admin-detail-value">{history.length}</div>
        </div>
      </div>

      <div className="admin-table-wrapper" data-asuc2-history-list>
        <table className="admin-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>原内容</th>
              <th>原因</th>
            </tr>
          </thead>
          <tbody>
            {history.map((h, i) => (
              <tr key={`${h.ts}-${i}`} data-asuc2-history-row>
                <td>{formatDate(h.ts)}</td>
                <td><code>{h.old_content || '(空)'}</code></td>
                <td>{h.reason}</td>
              </tr>
            ))}
            {history.length === 0 && (
              <tr><td colSpan={3} style={{ textAlign: 'center' }}>暂无变更历史</td></tr>
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

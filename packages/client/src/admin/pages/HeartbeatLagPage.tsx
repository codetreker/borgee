import React, { useCallback, useEffect, useState } from 'react';
import { fetchAdminHeartbeatLag } from '../api';
import type { LagSnapshot } from '../api';

// ADMIN-SPA-UI-COVERAGE-WAVE2: GET /admin-api/v1/heartbeat-lag (HB-5 #408).
// Readonly 30s rolling window LagSnapshot; 9 fields match the server shape.
// No server changes.

export default function HeartbeatLagPage() {
  const [snap, setSnap] = useState<LagSnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setSnap(await fetchAdminHeartbeatLag());
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load heartbeat lag');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  if (loading) return <div className="app-loading"><div className="loading-spinner-large" /></div>;
  if (error) return <div className="admin-error">{error}</div>;
  if (!snap) return <div className="admin-error">暂无数据</div>;

  return (
    <div data-page="admin-heartbeat-lag">
      <div className="admin-section-header">
        <h2>心跳滞后</h2>
        <button className="btn btn-sm" onClick={load} data-asuc2-lag-refresh>刷新</button>
      </div>
      <div className="admin-card admin-detail-grid" data-asuc2-lag-card>
        <Field label="样本数" value={String(snap.count)} />
        <Field label="窗口" value={`${snap.window_seconds} 秒`} />
        <Field label="阈值" value={`${snap.threshold_ms} ms`} />
        <Field label="P50" value={`${snap.p50_ms} ms`} />
        <Field label="P95" value={`${snap.p95_ms} ms`} />
        <Field label="P99" value={`${snap.p99_ms} ms`} />
        <Field label="采样时间" value={formatDate(snap.sampled_at)} />
        <Field
          label="状态"
          value={snap.at_risk ? '⚠️ 超阈值' : '✅ 正常'}
        />
      </div>
      {snap.at_risk && snap.reason_if_at_risk && (
        <div className="admin-error" data-asuc2-lag-reason>
          原因: {snap.reason_if_at_risk}
        </div>
      )}
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="admin-detail-label">{label}</div>
      <div className="admin-detail-value">{value}</div>
    </div>
  );
}

function formatDate(ts: number): string {
  return new Date(ts).toLocaleString();
}

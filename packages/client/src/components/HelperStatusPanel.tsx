import React, { useCallback, useEffect, useState } from 'react';

import { fetchHelperEnrollments, type HelperEnrollmentStatusView } from '../lib/api';

interface Props {
  onBack: () => void;
  fetchEnrollments?: () => Promise<HelperEnrollmentStatusView[]>;
}

const statusLabels: Record<string, string> = {
  connected: 'Helper connected',
  offline: 'Helper offline',
  revoked: 'Helper revoked',
  uninstalled: 'Helper uninstalled',
  pending: 'Waiting for local Helper',
};

const statusDescriptions: Record<string, string> = {
  connected: 'This Helper enrollment was recently seen by the server.',
  offline: 'This Helper enrollment is not fresh right now.',
  revoked: 'This Helper enrollment is revoked on the server.',
  uninstalled: 'This Helper enrollment has reported uninstall.',
  pending: 'This enrollment is waiting for a local Helper claim.',
};

const categoryLabels: Record<string, string> = {
  openclaw_lifecycle: 'OpenClaw lifecycle',
  openclaw_config: 'OpenClaw config',
  helper_lifecycle: 'Helper lifecycle',
  status_collect: 'Status collection',
};

function statusLabel(status: string): string {
  return statusLabels[status] ?? `Unknown Helper status: ${status}`;
}

function statusDescription(status: string): string {
  return statusDescriptions[status] ?? 'The server returned a Helper status this client does not recognize yet.';
}

function formatTimestamp(value?: number): string {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) {
    return 'No last seen yet';
  }
  return new Date(value).toLocaleString();
}

export default function HelperStatusPanel({ onBack, fetchEnrollments = fetchHelperEnrollments }: Props): React.ReactElement {
  const [rows, setRows] = useState<HelperEnrollmentStatusView[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const next = await fetchEnrollments();
      setRows(next);
      setSelectedId((prev) => {
        if (prev && next.some((row) => row.enrollment_id === prev)) return prev;
        return next[0]?.enrollment_id ?? null;
      });
    } catch {
      setError('Helper status unavailable');
    } finally {
      setLoading(false);
    }
  }, [fetchEnrollments]);

  useEffect(() => {
    void load();
  }, [load]);

  const selected = rows.find((row) => row.enrollment_id === selectedId) ?? rows[0];

  return (
    <div className="helper-status-panel" data-page="helper-status">
      <header className="helper-status-header">
        <button className="helper-status-back-btn" onClick={onBack} aria-label="Back to workspace">←</button>
        <h2 className="helper-status-title">Helper Status</h2>
        <button className="btn btn-sm" onClick={() => void load()} disabled={loading}>Refresh</button>
      </header>

      {loading && rows.length === 0 ? (
        <div className="helper-status-empty">Loading Helper status...</div>
      ) : error ? (
        <div className="helper-status-empty helper-status-error">{error}</div>
      ) : rows.length === 0 ? (
        <div className="helper-status-empty">No Helper enrollments</div>
      ) : (
        <div className="helper-status-content">
          <div className="helper-status-list" aria-label="Helper enrollments">
            {rows.map((row) => (
              <button
                key={row.enrollment_id}
                className={`helper-status-list-item${selected?.enrollment_id === row.enrollment_id ? ' active' : ''}`}
                data-helper-status={row.status}
                onClick={() => setSelectedId(row.enrollment_id)}
              >
                <span className={`helper-status-dot helper-status-dot-${row.status}`} />
                <span className="helper-status-host">
                  {row.host_label || row.enrollment_id}
                  <span className="helper-status-list-categories">
                    {row.allowed_categories.map((category) => (
                      <span key={category} data-helper-category={category}>{categoryLabels[category] ?? category}</span>
                    ))}
                  </span>
                  <span className="helper-status-list-seen">{formatTimestamp(row.last_seen_at)}</span>
                </span>
                <span className="helper-status-list-label">{statusLabel(row.status)}</span>
              </button>
            ))}
          </div>

          {selected && (
            <section className="helper-status-detail" aria-label="Selected Helper enrollment">
              <div className="helper-status-detail-head">
                <h3>{selected.host_label || selected.enrollment_id}</h3>
                <span className={`helper-status-badge helper-status-badge-${selected.status}`} data-helper-status={selected.status}>
                  {statusLabel(selected.status)}
                </span>
              </div>
              <p className="helper-status-description">{statusDescription(selected.status)}</p>

              <dl className="helper-status-facts">
                <div>
                  <dt>Last seen</dt>
                  <dd>{formatTimestamp(selected.last_seen_at)}</dd>
                </div>
                <div>
                  <dt>Fresh</dt>
                  <dd>{selected.fresh ? 'Yes' : 'No'}</dd>
                </div>
                {selected.helper_device_id && (
                  <div>
                    <dt>Helper device</dt>
                    <dd>{selected.helper_device_id}</dd>
                  </div>
                )}
                {selected.revoked_at && (
                  <div>
                    <dt>Revoked</dt>
                    <dd>{formatTimestamp(selected.revoked_at)}</dd>
                  </div>
                )}
                {selected.uninstalled_at && (
                  <div>
                    <dt>Uninstalled</dt>
                    <dd>{formatTimestamp(selected.uninstalled_at)}</dd>
                  </div>
                )}
              </dl>

              <div className="helper-status-categories" aria-label="Allowed categories">
                <h4>Allowed categories</h4>
                <div className="helper-status-category-list">
                  {selected.allowed_categories.length === 0 ? (
                    <span className="helper-status-category-empty">No categories</span>
                  ) : selected.allowed_categories.map((category) => (
                    <span key={category} className="helper-status-category" data-helper-category={category}>
                      {categoryLabels[category] ?? category}
                    </span>
                  ))}
                </div>
              </div>
            </section>
          )}
        </div>
      )}
    </div>
  );
}

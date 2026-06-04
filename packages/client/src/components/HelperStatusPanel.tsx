import React, { useCallback, useEffect, useMemo, useState } from 'react';

import {
  HELPER_ALLOWED_CATEGORIES,
  createHelperEnrollment as defaultCreateHelperEnrollment,
  fetchHelperEnrollments,
  installOpenClawOnHelper as defaultInstallOpenClaw,
  openClawInstallInFlight,
  openClawInstallSucceeded,
  type CreateHelperEnrollmentInput,
  type CreateHelperEnrollmentResponse,
  type HelperEnrollmentStatusView,
  type InstallOpenClawJob,
} from '../lib/api';
import PageHeader from './common/PageHeader';

interface Props {
  fetchEnrollments?: () => Promise<HelperEnrollmentStatusView[]>;
  createEnrollment?: (
    input: CreateHelperEnrollmentInput,
  ) => Promise<CreateHelperEnrollmentResponse>;
  installOpenClaw?: (enrollmentId: string) => Promise<InstallOpenClawJob>;
}

// OpenClaw install metadata shown in the confirmation modal. These are
// informational only — the server is the source of truth for the actual
// `manifest_url`, `pubkey_base64`, and `target_path` (see
// packages/server-go/internal/api/helper_manifest.go). The literals below
// mirror the daemon's expected install target (`RequiredPathID =
// openclaw_install`, root `/usr/local/lib/borgee/openclaw`) so the
// operator sees the same path the executor will write.
const OPENCLAW_INSTALL_TARGET_PATH = '/usr/local/lib/borgee/openclaw';
const OPENCLAW_INSTALL_PLUGIN_ID = 'openclaw';
const OPENCLAW_INSTALL_MANIFEST_NOTE =
  'Manifest URL and signing key are resolved server-side from the signed canonical helper-policy manifest.';

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
  return (
    statusDescriptions[status] ??
    'The server returned a Helper status this client does not recognize yet.'
  );
}

function formatTimestamp(value?: number): string {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) {
    return 'No last seen yet';
  }
  return new Date(value).toLocaleString();
}

function configureStateClass(state: string): string {
  return state.replace(/[^a-z0-9_-]/gi, '-').toLowerCase();
}

function configureStepLabel(jobType: string): string {
  switch (jobType) {
    case 'openclaw.install_from_manifest':
      return 'OpenClaw install';
    case 'openclaw.configure_agent':
      return 'Agent config';
    case 'borgee_plugin.configure_connection':
      return 'Borgee plugin channel';
    case 'service.lifecycle':
      return 'Service lifecycle';
    default:
      return jobType;
  }
}

export default function HelperStatusPanel({
  fetchEnrollments = fetchHelperEnrollments,
  createEnrollment = defaultCreateHelperEnrollment,
  installOpenClaw = defaultInstallOpenClaw,
}: Props): React.ReactElement {
  const [rows, setRows] = useState<HelperEnrollmentStatusView[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [installOpenForId, setInstallOpenForId] = useState<string | null>(null);
  const [pendingInstallJob, setPendingInstallJob] =
    useState<{ enrollment_id: string; status: string } | null>(null);

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
      <PageHeader
        title="Helper Status"
        actions={
          <>
            <button
              className="btn btn-sm btn-primary"
              data-action="add-helper-host"
              onClick={() => setCreateOpen(true)}
              disabled={loading}
            >
              Add host
            </button>
            <button className="btn btn-sm" onClick={() => void load()} disabled={loading}>
              Refresh
            </button>
          </>
        }
      />

      {createOpen && (
        <CreateHelperEnrollmentModal
          createEnrollment={createEnrollment}
          onClose={() => setCreateOpen(false)}
          onCreated={() => {
            void load();
          }}
        />
      )}

      {installOpenForId && (() => {
        const target = rows.find((row) => row.enrollment_id === installOpenForId);
        if (!target) {
          return null;
        }
        return (
          <InstallOpenClawModal
            enrollment={target}
            installOpenClaw={installOpenClaw}
            onClose={() => setInstallOpenForId(null)}
            onEnqueued={(job) => {
              setPendingInstallJob({
                enrollment_id: job.enrollment_id || target.enrollment_id,
                status: job.status,
              });
              setInstallOpenForId(null);
              // Refresh the list so the configure_openclaw aggregate
              // includes the new in-flight install step. Progress is
              // refresh-driven (post-POST cache + the Refresh button +
              // any subsequent navigation back to this page that re-runs
              // `load()`); there is no WS subscription on this panel.
              // Acceptance OUT-3 permits this page-refresh fallback. A
              // true helper-job WS push hookup is tracked as follow-up
              // to PR-4 (#1042); see acceptance-criteria.md.
              void load();
            }}
          />
        );
      })()}

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
                      <span key={category} data-helper-category={category}>
                        {categoryLabels[category] ?? category}
                      </span>
                    ))}
                  </span>
                  {row.configure_openclaw && (
                    <span
                      className={`helper-configure-list helper-configure-list-${configureStateClass(row.configure_openclaw.state)}`}
                      data-configure-openclaw-state={row.configure_openclaw.state}
                    >
                      {row.configure_openclaw.label}
                    </span>
                  )}
                  <span className="helper-status-list-seen">
                    {formatTimestamp(row.last_seen_at)}
                  </span>
                </span>
                <span className="helper-status-list-label">{statusLabel(row.status)}</span>
              </button>
            ))}
          </div>

          {selected && (
            <section className="helper-status-detail" aria-label="Selected Helper enrollment">
              <div className="helper-status-detail-head">
                <h3>{selected.host_label || selected.enrollment_id}</h3>
                <span
                  className={`helper-status-badge helper-status-badge-${selected.status}`}
                  data-helper-status={selected.status}
                >
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
                  ) : (
                    selected.allowed_categories.map((category) => (
                      <span
                        key={category}
                        className="helper-status-category"
                        data-helper-category={category}
                      >
                        {categoryLabels[category] ?? category}
                      </span>
                    ))
                  )}
                </div>
              </div>

              {(() => {
                const installSucceeded = openClawInstallSucceeded(selected);
                const installInFlight = openClawInstallInFlight(selected);
                const helperAlive = selected.status === 'connected' && selected.fresh;
                const pendingForThis =
                  pendingInstallJob && pendingInstallJob.enrollment_id === selected.enrollment_id
                    ? pendingInstallJob
                    : null;
                const showInstallSurface =
                  helperAlive && !installSucceeded && selected.allowed_categories.includes('openclaw_lifecycle');
                if (!showInstallSurface) {
                  if (installSucceeded) {
                    return (
                      <div
                        className="helper-install-openclaw-installed"
                        data-helper-openclaw-state="installed"
                        aria-label="OpenClaw install state"
                      >
                        <span
                          className="helper-install-openclaw-badge helper-install-openclaw-badge-installed"
                          data-helper-openclaw-badge="installed"
                        >
                          OpenClaw installed
                        </span>
                      </div>
                    );
                  }
                  return null;
                }
                const liveJobStatus = pendingForThis?.status ?? '';
                const showProgressBadge =
                  installInFlight ||
                  (liveJobStatus !== '' && liveJobStatus !== 'succeeded' && liveJobStatus !== 'failed');
                return (
                  <div
                    className="helper-install-openclaw"
                    data-helper-openclaw-state={
                      showProgressBadge ? 'in_flight' : 'not_installed'
                    }
                    aria-label="OpenClaw install action"
                  >
                    {showProgressBadge ? (
                      <span
                        className="helper-install-openclaw-badge helper-install-openclaw-badge-progress"
                        data-helper-openclaw-badge="progress"
                        aria-live="polite"
                      >
                        Installing OpenClaw{liveJobStatus ? ` (${liveJobStatus})` : '…'}
                      </span>
                    ) : (
                      <button
                        className="btn btn-sm btn-primary"
                        type="button"
                        data-action="install-openclaw"
                        aria-label="Install OpenClaw on this host"
                        onClick={() => setInstallOpenForId(selected.enrollment_id)}
                      >
                        Install OpenClaw
                      </button>
                    )}
                  </div>
                );
              })()}

              {selected.configure_openclaw && (
                <section
                  className="helper-configure-openclaw"
                  aria-label="Configure OpenClaw status"
                  data-configure-openclaw-state={selected.configure_openclaw.state}
                >
                  <div className="helper-configure-openclaw-head">
                    <h4>Configure OpenClaw</h4>
                    <span
                      className={`helper-configure-badge helper-configure-badge-${configureStateClass(selected.configure_openclaw.state)}`}
                    >
                      {selected.configure_openclaw.label}
                    </span>
                  </div>

                  {(selected.configure_openclaw.failure_code ||
                    selected.configure_openclaw.failure_message) && (
                    <dl className="helper-configure-reason">
                      {selected.configure_openclaw.failure_code && (
                        <div>
                          <dt>Reason</dt>
                          <dd>{selected.configure_openclaw.failure_code}</dd>
                        </div>
                      )}
                      {selected.configure_openclaw.failure_message && (
                        <div>
                          <dt>Detail</dt>
                          <dd>{selected.configure_openclaw.failure_message}</dd>
                        </div>
                      )}
                    </dl>
                  )}

                  {(selected.configure_openclaw.audit_refs.length > 0 ||
                    selected.configure_openclaw.log_refs.length > 0) && (
                    <div
                      className="helper-configure-refs"
                      aria-label="Configure OpenClaw evidence refs"
                    >
                      {selected.configure_openclaw.audit_refs.map((ref) => (
                        <span key={`audit-${ref}`}>Audit {ref}</span>
                      ))}
                      {selected.configure_openclaw.log_refs.map((ref) => (
                        <span key={`log-${ref}`}>Log {ref}</span>
                      ))}
                    </div>
                  )}

                  {selected.configure_openclaw.steps.length > 0 && (
                    <ol
                      className="helper-configure-steps"
                      aria-label="Configure OpenClaw closure steps"
                    >
                      {selected.configure_openclaw.steps.map((step) => (
                        <li key={`${step.job_type}-${step.created_at ?? step.status}`}>
                          <span>{configureStepLabel(step.job_type)}</span>
                          <strong>{step.status}</strong>
                          {step.failure_code && <em>{step.failure_code}</em>}
                        </li>
                      ))}
                    </ol>
                  )}
                </section>
              )}
            </section>
          )}
        </div>
      )}
    </div>
  );
}

interface CreateHelperEnrollmentModalProps {
  createEnrollment: (
    input: CreateHelperEnrollmentInput,
  ) => Promise<CreateHelperEnrollmentResponse>;
  onClose: () => void;
  onCreated: () => void;
}

// CreateHelperEnrollmentModal is the operator-facing UI surface that mints a
// one-shot enrollment token + install command. Two phases:
//   form:    operator picks host label + allowed categories, clicks Create
//   reveal:  the server response (token + install_command) is shown ONCE.
//            Closing the modal drops the response from React state — there
//            is no second display path and no console / network log of the
//            token (the only sink is the input value of the visible textarea
//            for copy + Copy buttons that write to the clipboard).
function CreateHelperEnrollmentModal({
  createEnrollment,
  onClose,
  onCreated,
}: CreateHelperEnrollmentModalProps): React.ReactElement {
  const [hostLabel, setHostLabel] = useState('');
  const [categories, setCategories] = useState<string[]>(['openclaw_config', 'status_collect']);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [revealed, setRevealed] = useState<CreateHelperEnrollmentResponse | null>(null);

  const trimmedLabel = useMemo(() => hostLabel.trim(), [hostLabel]);
  const canSubmit = trimmedLabel.length > 0 && categories.length > 0 && !submitting;

  const toggleCategory = useCallback((category: string) => {
    setCategories((prev) =>
      prev.includes(category) ? prev.filter((c) => c !== category) : [...prev, category],
    );
  }, []);

  const submit = useCallback(async () => {
    if (!canSubmit) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      const out = await createEnrollment({
        host_label: trimmedLabel,
        allowed_categories: categories,
      });
      setRevealed(out);
    } catch (err) {
      setSubmitError(
        err instanceof Error && err.message ? err.message : 'Failed to create enrollment',
      );
    } finally {
      setSubmitting(false);
    }
  }, [canSubmit, categories, createEnrollment, trimmedLabel]);

  const handleDone = useCallback(() => {
    setRevealed(null);
    onCreated();
    onClose();
  }, [onClose, onCreated]);

  // Copy-to-clipboard with a no-op fallback when the clipboard API is
  // unavailable (older browsers, insecure context). The token textarea
  // is also selectable so manual copy works regardless.
  const copyToClipboard = useCallback(async (value: string) => {
    if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
      try {
        await navigator.clipboard.writeText(value);
      } catch {
        /* ignore — operator can still hand-select */
      }
    }
  }, []);

  return (
    <div
      className="helper-status-modal-backdrop"
      data-helper-create-modal
      role="dialog"
      aria-modal="true"
      aria-label="Create helper enrollment"
    >
      <div className="helper-status-modal">
        {revealed ? (
          <>
            <h3>Helper enrollment ready</h3>
            <p
              className="helper-status-warning"
              data-helper-create-warning
              style={{
                border: '2px solid #c0392b',
                background: '#fdecea',
                color: '#a93226',
                padding: '0.75rem 1rem',
                borderRadius: '0.375rem',
                marginBottom: '1rem',
              }}
            >
              This token is shown ONCE. Copy now; you cannot retrieve it later.
              If you lose it, revoke this enrollment and create a new one.
            </p>

            <label className="helper-status-modal-label">
              Install command (paste on the host VM)
              <textarea
                data-helper-install-command
                readOnly
                rows={3}
                value={revealed.install_command}
                onFocus={(e) => e.currentTarget.select()}
                style={{ width: '100%', fontFamily: 'monospace' }}
              />
            </label>
            <button
              className="btn btn-sm"
              type="button"
              data-action="copy-install-command"
              onClick={() => void copyToClipboard(revealed.install_command)}
            >
              Copy install command
            </button>

            <label className="helper-status-modal-label" style={{ marginTop: '1rem' }}>
              Enrollment token (only the part after <code>--token</code>)
              <textarea
                data-helper-enrollment-token
                readOnly
                rows={2}
                value={revealed.enrollment_token}
                onFocus={(e) => e.currentTarget.select()}
                style={{ width: '100%', fontFamily: 'monospace' }}
              />
            </label>
            <button
              className="btn btn-sm"
              type="button"
              data-action="copy-enrollment-token"
              onClick={() => void copyToClipboard(revealed.enrollment_token)}
            >
              Copy token
            </button>

            <div className="helper-status-modal-actions" style={{ marginTop: '1rem' }}>
              <button
                className="btn btn-primary"
                type="button"
                data-action="close-helper-create-modal"
                onClick={handleDone}
              >
                Done
              </button>
            </div>
          </>
        ) : (
          <>
            <h3>Add host</h3>
            <p className="helper-status-modal-description">
              Mint a one-time enrollment token. Paste the install command on the host VM —
              no SSH or curl required.
            </p>

            <label className="helper-status-modal-label">
              Host label
              <input
                type="text"
                data-helper-host-label
                value={hostLabel}
                maxLength={100}
                placeholder="e.g. stage-2-test-host"
                onChange={(e) => setHostLabel(e.target.value)}
                disabled={submitting}
              />
            </label>

            <fieldset className="helper-status-modal-categories">
              <legend>Allowed categories</legend>
              {HELPER_ALLOWED_CATEGORIES.map((category) => (
                <label key={category} data-helper-category-checkbox={category}>
                  <input
                    type="checkbox"
                    checked={categories.includes(category)}
                    onChange={() => toggleCategory(category)}
                    disabled={submitting}
                  />
                  {categoryLabels[category] ?? category}
                </label>
              ))}
            </fieldset>

            {submitError && (
              <p className="helper-status-error" data-helper-create-error>
                {submitError}
              </p>
            )}

            <div className="helper-status-modal-actions">
              <button
                className="btn"
                type="button"
                data-action="cancel-helper-create"
                onClick={onClose}
                disabled={submitting}
              >
                Cancel
              </button>
              <button
                className="btn btn-primary"
                type="button"
                data-action="submit-helper-create"
                onClick={() => void submit()}
                disabled={!canSubmit}
              >
                {submitting ? 'Creating…' : 'Create'}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

interface InstallOpenClawModalProps {
  enrollment: HelperEnrollmentStatusView;
  installOpenClaw: (enrollmentId: string) => Promise<InstallOpenClawJob>;
  onClose: () => void;
  onEnqueued: (job: InstallOpenClawJob) => void;
}

// InstallOpenClawModal is the owner-facing confirmation for issue #1050.
// Shows the operator a summary of what will happen (target path, plugin
// id, server-resolved manifest), accepts a single Confirm, and POSTs to
// the existing helper-jobs enqueue endpoint.
//
// Server contract (see packages/server-go/internal/api/helper_jobs.go):
//   - Owner-gated. Non-owner returns 403 / forbidden — surfaced as an
//     inline error inside the modal so the operator can close + escalate
//     rather than thinking the click did nothing.
//   - Idempotent on `idempotency_key="install-openclaw-<enrollment_id>"`.
//     A second click while the prior job is still queued/running returns
//     the same `job_id` with status 200; the refresh-driven badge then
//     continues to reflect the existing job.
//
// Accessibility:
//   - role=dialog, aria-modal, aria-labelledby pinned to the heading.
//   - ESC closes (unless a POST is in flight, where ESC is no-op so the
//     user does not race the network).
//   - Focus auto-targets the Cancel button on mount.
function InstallOpenClawModal({
  enrollment,
  installOpenClaw,
  onClose,
  onEnqueued,
}: InstallOpenClawModalProps): React.ReactElement {
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const headingId = `install-openclaw-heading-${enrollment.enrollment_id}`;
  const cancelButtonRef = React.useRef<HTMLButtonElement | null>(null);

  React.useEffect(() => {
    // Focus the Cancel button so keyboard operators can ESC / TAB
    // without the modal stealing focus into an interactive control.
    cancelButtonRef.current?.focus();
  }, []);

  React.useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape' && !submitting) {
        onClose();
      }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose, submitting]);

  const submit = useCallback(async () => {
    if (submitting) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      const job = await installOpenClaw(enrollment.enrollment_id);
      onEnqueued(job);
    } catch (err) {
      const msg =
        err instanceof Error && err.message
          ? err.message
          : 'Failed to enqueue OpenClaw install';
      setSubmitError(msg);
    } finally {
      setSubmitting(false);
    }
  }, [enrollment.enrollment_id, installOpenClaw, onEnqueued, submitting]);

  return (
    <div
      className="helper-status-modal-backdrop"
      data-helper-install-openclaw-modal
      role="dialog"
      aria-modal="true"
      aria-labelledby={headingId}
    >
      <div className="helper-status-modal">
        <h3 id={headingId}>
          Install OpenClaw on {enrollment.host_label || enrollment.enrollment_id}
        </h3>
        <p className="helper-status-modal-description">
          The Borgee helper on this host will download and install OpenClaw
          from the signed canonical manifest. The operator does not need to
          SSH into the machine.
        </p>

        <dl className="helper-install-openclaw-facts" data-helper-install-openclaw-facts>
          <div>
            <dt>Host</dt>
            <dd data-helper-install-openclaw-host>
              {enrollment.host_label || enrollment.enrollment_id}
            </dd>
          </div>
          <div>
            <dt>Plugin ID</dt>
            <dd data-helper-install-openclaw-plugin-id>{OPENCLAW_INSTALL_PLUGIN_ID}</dd>
          </div>
          <div>
            <dt>Target path</dt>
            <dd data-helper-install-openclaw-target-path>
              <code>{OPENCLAW_INSTALL_TARGET_PATH}</code>
            </dd>
          </div>
          <div>
            <dt>Manifest</dt>
            <dd data-helper-install-openclaw-manifest-note>
              {OPENCLAW_INSTALL_MANIFEST_NOTE}
            </dd>
          </div>
        </dl>

        {submitError && (
          <p
            className="helper-status-error"
            data-helper-install-openclaw-error
            role="alert"
          >
            {submitError}
          </p>
        )}

        <div className="helper-status-modal-actions">
          <button
            ref={cancelButtonRef}
            className="btn"
            type="button"
            data-action="cancel-install-openclaw"
            aria-label="Cancel OpenClaw install"
            onClick={onClose}
            disabled={submitting}
          >
            Cancel
          </button>
          <button
            className="btn btn-primary"
            type="button"
            data-action="confirm-install-openclaw"
            aria-label="Confirm OpenClaw install"
            onClick={() => void submit()}
            disabled={submitting}
          >
            {submitting ? 'Installing…' : 'Confirm install'}
          </button>
        </div>
      </div>
    </div>
  );
}

// PluginConnectionsPanel.tsx — #1049 owner UI for borgee plugin connections.
//
// Provides Add / Edit / Delete affordances for plugin connections on a
// helper enrollment. The state is derived server-side from the
// helper_jobs job stream (configure_connection + remove_connection);
// this component renders the list, lets the owner enqueue new configure
// jobs, and lets the owner enqueue a remove job for a row.
//
// Notes:
//   - `connection_id` is ALWAYS server-derived (= digest of
//     org_id|agent_id|channel_id). The add form only takes `channel_id`
//     — there is no user-facing connection_id input, because anything
//     the user typed would be silently discarded by the server. The
//     list view still shows the server-derived id per row (read-only).
//   - "Edit" simply re-issues a `configure_connection` with the new
//     `channel_id`. The server's idempotency on (org|agent|channel)
//     means same-channel reconfigure overwrites in place; switching to
//     a NEW channel derives a NEW connection_id (the old row would
//     orphan — out-of-scope cleanup for this PR, see
//     acceptance-criteria.md). This avoids the remove+configure
//     non-atomic failure mode where remove succeeds and configure fails.
//   - Confirm-delete dialog (run_4 fix): focus moves to Cancel on
//     open, Escape dismisses, Tab/Shift+Tab cycle stays inside the
//     dialog (so `aria-modal="true"` no longer lies), and focus
//     returns to the Delete button that opened the dialog when it
//     closes (so keyboard users don't lose context to <body>).
//   - States: loading / error (with retry) / empty / populated.

import { useCallback, useEffect, useRef, useState } from 'react';
import {
  configurePluginConnection,
  fetchPluginConnections,
  removePluginConnection,
  type PluginConnectionView,
} from '../lib/api';

interface PluginConnectionsPanelProps {
  enrollmentId: string;
  agentId: string;
}

type FormMode =
  | { kind: 'closed' }
  | { kind: 'add' }
  | { kind: 'edit'; connectionId: string; channelId: string };

type ConfirmDelete = { connectionId: string } | null;

export function PluginConnectionsPanel({
  enrollmentId,
  agentId,
}: PluginConnectionsPanelProps) {
  const [rows, setRows] = useState<PluginConnectionView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<FormMode>({ kind: 'closed' });
  const [formChannelId, setFormChannelId] = useState('');
  const [pending, setPending] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState<ConfirmDelete>(null);
  // Tracks the element that opened the confirm-delete dialog so we
  // can restore focus to it on close (run_4 a11y fix). Captured at
  // openDelete() call site; consumed on dialog unmount.
  const deleteOpenerRef = useRef<HTMLElement | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await fetchPluginConnections(enrollmentId);
      setRows(list.filter(r => r.agent_id === agentId));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load plugin connections');
    } finally {
      setLoading(false);
    }
  }, [enrollmentId, agentId]);

  useEffect(() => {
    void load();
  }, [load]);

  const channelIdValid = formChannelId.trim().length > 0;
  const submitDisabled = pending || !channelIdValid;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (submitDisabled) return;
    setPending(true);
    try {
      // Edit and Add both just re-issue configure — the server is
      // idempotent on (org|agent|channel) so same-channel save
      // overwrites in place (no remove needed); switching to a new
      // channel derives a new connection_id. Avoids the remove-then-
      // configure non-atomic failure mode (CRIT-5).
      await configurePluginConnection(enrollmentId, agentId, formChannelId.trim());
      setForm({ kind: 'closed' });
      setFormChannelId('');
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save');
    } finally {
      setPending(false);
    }
  }

  async function handleConfirmDelete() {
    if (!confirmDelete) return;
    setPending(true);
    try {
      await removePluginConnection(enrollmentId, agentId, confirmDelete.connectionId);
      setConfirmDelete(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete');
    } finally {
      setPending(false);
    }
  }

  function openDelete(connectionId: string, opener: HTMLElement | null) {
    deleteOpenerRef.current = opener;
    setConfirmDelete({ connectionId });
  }

  function closeDelete() {
    setConfirmDelete(null);
  }

  function openAdd() {
    setForm({ kind: 'add' });
    setFormChannelId('');
    setError(null);
  }
  function openEdit(row: PluginConnectionView) {
    setForm({ kind: 'edit', connectionId: row.connection_id, channelId: row.channel_id });
    setFormChannelId(row.channel_id);
    setError(null);
  }
  function closeForm() {
    setForm({ kind: 'closed' });
    setFormChannelId('');
  }

  return (
    <section
      data-testid="plugin-connections-section"
      style={{ marginTop: 16, padding: 12, border: '1px solid var(--border-color)', borderRadius: 4 }}
    >
      <header style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 8 }}>
        <strong>Plugin connections</strong>
        <button
          type="button"
          data-testid="plugin-connection-add-btn"
          onClick={openAdd}
          disabled={pending || form.kind !== 'closed'}
        >
          Add connection
        </button>
      </header>

      {loading && (
        <p data-testid="plugin-connections-loading" style={{ marginTop: 8 }}>
          Loading...
        </p>
      )}

      {error && (
        <div
          data-testid="plugin-connections-error"
          role="alert"
          style={{ marginTop: 8, color: 'var(--danger)' }}
        >
          {error}{' '}
          <button
            type="button"
            data-testid="plugin-connections-retry-btn"
            onClick={() => void load()}
          >
            Retry
          </button>
        </div>
      )}

      {!loading && !error && rows.length === 0 && form.kind === 'closed' && (
        <p data-testid="plugin-connections-empty" style={{ marginTop: 8 }}>
          No plugin connections
        </p>
      )}

      {!loading && rows.length > 0 && (
        <div style={{ marginTop: 8, overflowX: 'auto' }}>
          <table
            data-testid="plugin-connections-table"
            style={{ width: '100%', borderCollapse: 'collapse' }}
          >
            <thead>
              <tr>
                <th style={{ textAlign: 'left' }}>Connection ID</th>
                <th style={{ textAlign: 'left' }}>Channel</th>
                <th style={{ textAlign: 'left' }}>Last configured</th>
                <th style={{ textAlign: 'right' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {rows.map(row => (
                <tr key={row.connection_id} data-testid="plugin-connection-row" data-connection-id={row.connection_id}>
                  <td style={{ fontFamily: 'monospace', fontSize: 12 }}>{row.connection_id}</td>
                  <td>{row.channel_id}</td>
                  <td>
                    {row.last_configured_at > 0
                      ? new Date(row.last_configured_at).toISOString()
                      : ''}
                  </td>
                  <td style={{ textAlign: 'right' }}>
                    <button
                      type="button"
                      data-testid={`plugin-connection-edit-btn-${row.connection_id}`}
                      aria-label={`Edit connection ${row.connection_id}`}
                      onClick={() => openEdit(row)}
                      disabled={pending || form.kind !== 'closed'}
                    >
                      Edit
                    </button>{' '}
                    <button
                      type="button"
                      data-testid={`plugin-connection-delete-btn-${row.connection_id}`}
                      aria-label={`Delete connection ${row.connection_id}`}
                      onClick={e => openDelete(row.connection_id, e.currentTarget)}
                      disabled={pending}
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {form.kind !== 'closed' && (
        <form
          data-testid="plugin-connection-form"
          onSubmit={handleSubmit}
          style={{ marginTop: 12, padding: 8, border: '1px solid var(--border-color)', borderRadius: 4 }}
        >
          <label style={{ display: 'block', marginTop: 8 }}>
            Channel ID
            <input
              type="text"
              data-testid="plugin-connection-form-channel-id"
              value={formChannelId}
              onChange={e => setFormChannelId(e.target.value)}
              style={{ display: 'block', width: '100%', boxSizing: 'border-box' }}
              required
            />
          </label>
          <div style={{ marginTop: 8, display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
            <button type="button" onClick={closeForm} disabled={pending}>
              Cancel
            </button>
            <button
              type="submit"
              data-testid="plugin-connection-form-submit"
              disabled={submitDisabled}
            >
              {pending ? 'Saving...' : form.kind === 'edit' ? 'Save' : 'Add'}
            </button>
          </div>
        </form>
      )}

      {confirmDelete && (
        <ConfirmDeleteDialog
          connectionId={confirmDelete.connectionId}
          pending={pending}
          openerRef={deleteOpenerRef}
          onCancel={closeDelete}
          onConfirm={() => void handleConfirmDelete()}
        />
      )}
    </section>
  );
}

// ConfirmDeleteDialog — small subcomponent so the focus / Escape effect
// has a stable mount lifecycle. Mirrors RollbackConfirmModal in
// ArtifactPanel.tsx (autofocus Cancel for destructive ops, Esc dismiss).
//
// run_4 a11y: also (a) traps Tab + Shift+Tab inside the dialog so
// `aria-modal="true"` is not a lie for keyboard users, and (b) restores
// focus to `openerRef.current` (= the Delete button that opened the
// dialog) on unmount so keyboard users don't lose context to <body>.
function ConfirmDeleteDialog({
  connectionId,
  pending,
  openerRef,
  onCancel,
  onConfirm,
}: {
  connectionId: string;
  pending: boolean;
  openerRef: React.MutableRefObject<HTMLElement | null>;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const cancelBtnRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    // a11y: focus Cancel by default for destructive op — prevents
    // an accidental Enter press from confirming the delete.
    cancelBtnRef.current?.focus();
  }, []);

  // Focus return on unmount: restore focus to the opener (the Delete
  // button that triggered the dialog). Captured by parent as ref.
  useEffect(() => {
    const opener = openerRef.current;
    return () => {
      // Some openers may have been re-rendered / removed (row deleted
      // after confirm); guard for that. document.body is the fallback,
      // which is what the browser would do anyway.
      if (opener && typeof opener.focus === 'function' && document.body.contains(opener)) {
        opener.focus();
      }
    };
  }, [openerRef]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !pending) {
        onCancel();
        return;
      }
      if (e.key !== 'Tab') return;
      // Trap Tab + Shift+Tab inside the dialog. Pull all currently-
      // focusable descendants on every Tab press so dynamic content
      // (e.g. button enabling after pending flips) is handled.
      const root = dialogRef.current;
      if (!root) return;
      const focusables = Array.from(
        root.querySelectorAll<HTMLElement>(
          'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
        ),
      ).filter(el => !el.hasAttribute('aria-hidden'));
      if (focusables.length === 0) return;
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      const active = document.activeElement as HTMLElement | null;
      if (e.shiftKey) {
        if (active === first || !root.contains(active)) {
          e.preventDefault();
          last.focus();
        }
      } else {
        if (active === last || !root.contains(active)) {
          e.preventDefault();
          first.focus();
        }
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onCancel, pending]);

  return (
    <div
      ref={dialogRef}
      data-testid="plugin-connection-confirm-dialog"
      role="dialog"
      aria-modal="true"
      aria-label="Confirm delete"
      style={{
        marginTop: 12,
        padding: 12,
        background: 'var(--bg-secondary)',
        border: '1px solid var(--border-color)',
        borderRadius: 4,
      }}
    >
      <p>Delete connection {connectionId}?</p>
      <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
        <button
          ref={cancelBtnRef}
          type="button"
          data-testid="plugin-connection-cancel-delete-btn"
          onClick={onCancel}
          disabled={pending}
        >
          Cancel
        </button>
        <button
          type="button"
          data-testid="plugin-connection-confirm-delete-btn"
          onClick={onConfirm}
          disabled={pending}
        >
          {pending ? 'Deleting...' : 'Delete'}
        </button>
      </div>
    </div>
  );
}

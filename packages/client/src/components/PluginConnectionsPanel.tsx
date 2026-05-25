// PluginConnectionsPanel.tsx — #1049 owner UI for borgee plugin connections.
//
// Provides Add / Edit / Delete affordances for plugin connections on a
// helper enrollment. The state is derived server-side from the
// helper_jobs job stream (configure_connection + remove_connection);
// this component renders the list, lets the owner enqueue new configure
// jobs, and lets the owner enqueue a remove job for a row.
//
// Notes:
//   - `connection_id` shown in the list is server-derived (digest of
//     org_id|agent_id|channel_id). The "Add connection" form lets the
//     owner type a free-text connection_id which is validated against
//     `^borgee-plugin:[A-Za-z0-9_.-]{1,200}$` client-side; the server
//     still derives the canonical id for the configure path. For remove,
//     the client round-trips the server-derived id from the list.
//   - "Edit" is modeled as remove(old) + configure(new) since the
//     server-derived id changes when (agent, channel) changes.
//   - States: loading / error (with retry) / empty / populated.

import { useCallback, useEffect, useState } from 'react';
import {
  configurePluginConnection,
  fetchPluginConnections,
  PLUGIN_CONNECTION_ID_RE,
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

const FORM_INITIAL_CONN_ID = '';

export function PluginConnectionsPanel({
  enrollmentId,
  agentId,
}: PluginConnectionsPanelProps) {
  const [rows, setRows] = useState<PluginConnectionView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<FormMode>({ kind: 'closed' });
  const [formConnectionId, setFormConnectionId] = useState(FORM_INITIAL_CONN_ID);
  const [formChannelId, setFormChannelId] = useState('');
  const [pending, setPending] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState<ConfirmDelete>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await fetchPluginConnections(enrollmentId);
      setRows(list.filter(r => r.agent_id === agentId));
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, [enrollmentId, agentId]);

  useEffect(() => {
    void load();
  }, [load]);

  const connectionIdValid =
    form.kind === 'add' ? PLUGIN_CONNECTION_ID_RE.test(formConnectionId) : true;
  const channelIdValid = formChannelId.trim().length > 0;
  const submitDisabled = pending || !channelIdValid || (form.kind === 'add' && !connectionIdValid);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (submitDisabled) return;
    setPending(true);
    try {
      if (form.kind === 'edit') {
        await removePluginConnection(enrollmentId, agentId, form.connectionId);
      }
      await configurePluginConnection(enrollmentId, agentId, formChannelId.trim());
      setForm({ kind: 'closed' });
      setFormConnectionId(FORM_INITIAL_CONN_ID);
      setFormChannelId('');
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
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
      setError(err instanceof Error ? err.message : '删除失败');
    } finally {
      setPending(false);
    }
  }

  function openAdd() {
    setForm({ kind: 'add' });
    setFormConnectionId(FORM_INITIAL_CONN_ID);
    setFormChannelId('');
    setError(null);
  }
  function openEdit(row: PluginConnectionView) {
    setForm({ kind: 'edit', connectionId: row.connection_id, channelId: row.channel_id });
    setFormConnectionId(row.connection_id);
    setFormChannelId(row.channel_id);
    setError(null);
  }
  function closeForm() {
    setForm({ kind: 'closed' });
    setFormConnectionId(FORM_INITIAL_CONN_ID);
    setFormChannelId('');
  }

  return (
    <section
      data-testid="plugin-connections-section"
      style={{ marginTop: 16, padding: 12, border: '1px solid var(--color-border, #ddd)', borderRadius: 4 }}
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
          加载中...
        </p>
      )}

      {error && (
        <div
          data-testid="plugin-connections-error"
          role="alert"
          style={{ marginTop: 8, color: 'var(--color-error, #c00)' }}
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
        <table
          data-testid="plugin-connections-table"
          style={{ marginTop: 8, width: '100%', borderCollapse: 'collapse' }}
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
                    onClick={() => openEdit(row)}
                    disabled={pending || form.kind !== 'closed'}
                  >
                    Edit
                  </button>{' '}
                  <button
                    type="button"
                    data-testid={`plugin-connection-delete-btn-${row.connection_id}`}
                    onClick={() => setConfirmDelete({ connectionId: row.connection_id })}
                    disabled={pending}
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {form.kind !== 'closed' && (
        <form
          data-testid="plugin-connection-form"
          onSubmit={handleSubmit}
          style={{ marginTop: 12, padding: 8, border: '1px solid var(--color-border, #ddd)', borderRadius: 4 }}
        >
          <label style={{ display: 'block', marginTop: 4 }}>
            Connection ID
            <input
              type="text"
              data-testid="plugin-connection-form-connection-id"
              value={formConnectionId}
              onChange={e => setFormConnectionId(e.target.value)}
              placeholder="borgee-plugin:my-connection"
              disabled={form.kind === 'edit'}
              style={{ display: 'block', width: '100%', boxSizing: 'border-box' }}
            />
            {form.kind === 'add' && formConnectionId !== '' && !connectionIdValid && (
              <span
                data-testid="plugin-connection-form-connection-id-error"
                style={{ color: 'var(--color-error, #c00)', fontSize: 12 }}
              >
                connection_id must match {String(PLUGIN_CONNECTION_ID_RE)}
              </span>
            )}
          </label>
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
        <div
          data-testid="plugin-connection-confirm-dialog"
          role="dialog"
          aria-label="Confirm delete"
          style={{
            marginTop: 12,
            padding: 12,
            background: 'var(--color-surface-alt, #fafafa)',
            border: '1px solid var(--color-border, #ddd)',
            borderRadius: 4,
          }}
        >
          <p>Delete connection {confirmDelete.connectionId}?</p>
          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
            <button
              type="button"
              data-testid="plugin-connection-cancel-delete-btn"
              onClick={() => setConfirmDelete(null)}
              disabled={pending}
            >
              Cancel
            </button>
            <button
              type="button"
              data-testid="plugin-connection-confirm-delete-btn"
              onClick={() => void handleConfirmDelete()}
              disabled={pending}
            >
              {pending ? 'Deleting...' : 'Delete'}
            </button>
          </div>
        </div>
      )}
    </section>
  );
}

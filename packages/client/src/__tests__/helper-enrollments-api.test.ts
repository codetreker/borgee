import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  createHelperEnrollment,
  fetchHelperEnrollment,
  fetchHelperEnrollments,
} from '../lib/api';

afterEach(() => {
  vi.unstubAllGlobals();
});

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('Helper enrollment user-rail API', () => {
  it('fetches the user enrollment list without credential endpoints or secret fields', async () => {
    const fetchMock = vi.fn(async (url: RequestInfo | URL) => {
      expect(String(url)).toBe('/api/v1/helper/enrollments');
      return jsonResponse({
        enrollments: [
          {
            enrollment_id: 'enr-1',
            host_label: 'Dev Mac',
            helper_device_id: 'device-1',
            allowed_categories: ['openclaw_config'],
            status: 'connected',
            fresh: true,
            last_seen_at: 1778840000000,
            created_at: 1778839900000,
            configure_openclaw: {
              state: 'denied',
              label: 'Configure OpenClaw denied',
              failure_code: 'policy_denied',
              failure_message: 'policy handoff denied',
              audit_refs: ['audit-1', '../audit-secret', 'a'.repeat(129)],
              log_refs: ['log-1', 'log/path', 'l'.repeat(129)],
              steps: [
                {
                  job_type: 'openclaw.configure_agent',
                  status: 'failed',
                  failure_code: 'policy_denied',
                  failure_message: 'policy handoff denied',
                  audit_refs: ['step-audit-1', 'step/audit-secret'],
                  log_refs: ['step-log-1', 'step-log\nsecret'],
                  raw_logs: 'must-not-leak',
                },
              ],
              payload_hash: 'must-not-leak',
              manifest_digest: 'must-not-leak',
              result_summary_json: 'must-not-leak',
            },
            helper_credential: 'must-not-leak',
            enrollment_secret: 'must-not-leak',
            org_id: 'org-private',
            connection_token: 'remote-token',
          },
        ],
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const rows = await fetchHelperEnrollments();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(JSON.stringify(rows)).not.toContain('must-not-leak');
    expect(JSON.stringify(rows)).not.toContain('org-private');
    expect(JSON.stringify(rows)).not.toContain('remote-token');
    expect(rows[0]).toEqual({
      enrollment_id: 'enr-1',
      host_label: 'Dev Mac',
      helper_device_id: 'device-1',
      allowed_categories: ['openclaw_config'],
      status: 'connected',
      fresh: true,
      last_seen_at: 1778840000000,
      created_at: 1778839900000,
      configure_openclaw: {
        state: 'denied',
        label: 'Configure OpenClaw denied',
            failure_code: 'policy_denied',
            failure_message: 'policy handoff denied',
            audit_refs: ['audit-1'],
            log_refs: ['log-1'],
        steps: [
          {
            job_type: 'openclaw.configure_agent',
            status: 'failed',
              failure_code: 'policy_denied',
              failure_message: 'policy handoff denied',
              audit_refs: ['step-audit-1'],
              log_refs: ['step-log-1'],
            },
          ],
        },
    });
  });

  it('fetches one enrollment through the user detail route only', async () => {
    const urls: string[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: RequestInfo | URL) => {
        urls.push(String(url));
        return jsonResponse({
          enrollment: {
            enrollment_id: 'enr-2',
            host_label: 'Linux Host',
            allowed_categories: ['status_collect'],
            status: 'offline',
            fresh: false,
            created_at: 1778839900000,
          },
        });
      }),
    );

    const row = await fetchHelperEnrollment('enr-2');

    expect(row.enrollment_id).toBe('enr-2');
    expect(urls).toEqual(['/api/v1/helper/enrollments/enr-2']);
    expect(urls.join(' ')).not.toMatch(/\/claim|\/status|\/uninstall/);
  });
});

describe('createHelperEnrollment (operator UI mint)', () => {
  it('POSTs host_label + allowed_categories and returns the one-shot token + install_command', async () => {
    let captured: { url: string; init: RequestInit } | null = null;
    const fetchMock = vi.fn(async (url: RequestInfo | URL, init?: RequestInit) => {
      captured = { url: String(url), init: init ?? {} };
      return jsonResponse({
        enrollment: {
          enrollment_id: 'enr-new-1',
          host_label: 'Stage 2 Test Host',
          allowed_categories: ['openclaw_config', 'status_collect'],
          status: 'pending',
          created_at: 1778839900000,
        },
        enrollment_secret: 'super-secret-shown-once',
        enrollment_secret_expires_at: 1778839900000 + 15 * 60 * 1000,
        enrollment_token: 'enr-new-1.super-secret-shown-once',
        install_command:
          'npx @codetreker/borgee-remote-agent install --server wss://borgee.example.com --token enr-new-1.super-secret-shown-once',
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const out = await createHelperEnrollment({
      host_label: 'Stage 2 Test Host',
      allowed_categories: ['openclaw_config', 'status_collect'],
    });

    expect(captured!.url).toBe('/api/v1/helper/enrollments');
    expect(captured!.init.method).toBe('POST');
    expect(JSON.parse(String(captured!.init.body))).toEqual({
      host_label: 'Stage 2 Test Host',
      allowed_categories: ['openclaw_config', 'status_collect'],
    });
    expect(out.enrollment_id).toBe('enr-new-1');
    expect(out.enrollment_token).toBe('enr-new-1.super-secret-shown-once');
    expect(out.enrollment_secret).toBe('super-secret-shown-once');
    expect(out.install_command).toContain('--server wss://borgee.example.com');
    expect(out.install_command).toContain('--token enr-new-1.super-secret-shown-once');
    expect(out.allowed_categories).toEqual(['openclaw_config', 'status_collect']);
  });

  it('trims host_label and drops non-string allowed_categories before POSTing', async () => {
    let bodyJSON: unknown = null;
    vi.stubGlobal(
      'fetch',
      vi.fn(async (_url: RequestInfo | URL, init?: RequestInit) => {
        bodyJSON = JSON.parse(String(init?.body));
        return jsonResponse({
          enrollment: { enrollment_id: 'enr-2', host_label: 'X', allowed_categories: [] },
          enrollment_secret: 's',
          enrollment_token: 'enr-2.s',
          install_command: 'sudo npx ... --server ws://localhost:4901 --token enr-2.s',
        });
      }),
    );

    await createHelperEnrollment({
      host_label: '   Padded Host   ',
      // @ts-expect-error — deliberately exercise the runtime filter
      allowed_categories: ['openclaw_config', 42, null, ''],
    });

    expect(bodyJSON).toEqual({
      host_label: 'Padded Host',
      allowed_categories: ['openclaw_config'],
    });
  });
});

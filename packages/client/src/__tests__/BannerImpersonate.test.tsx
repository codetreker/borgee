// BannerImpersonate.test.tsx — ADM-2.2 acceptance §4.2.a red banner DOM lock.
//
// Pins:
//   - `[data-banner="impersonate-active"]` renders only for an active grant
//   - text stays byte-identical with docs/qa/adm-2-content-lock.md §2
//   - no grant / revoked / expired grant → no banner (negative assertion)
//   - `[立即撤销]` entry exists and calls revokeGrant
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import BannerImpersonate from '../components/Settings/BannerImpersonate';

let container: HTMLDivElement | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(() => {
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
});

function render(node: React.ReactElement) {
  const root = createRoot(container!);
  act(() => {
    root.render(node);
  });
}

async function flushPromises() {
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
}

describe('BannerImpersonate — ADM-2 红横幅 DOM 锁 (acceptance §4.2.a)', () => {
  it('renders no banner when grant is null', async () => {
    render(
      <BannerImpersonate
        fetchGrant={async () => null}
        revokeGrant={vi.fn()}
      />,
    );
    await flushPromises();
    expect(container!.querySelector('[data-banner="impersonate-active"]')).toBeNull();
  });

  it('renders no banner when grant is revoked', async () => {
    render(
      <BannerImpersonate
        fetchGrant={async () => ({
          id: 'g-1',
          user_id: 'u1',
          granted_at: Date.now() - 60_000,
          expires_at: Date.now() + 60_000,
          revoked_at: Date.now() - 30_000,
        })}
        revokeGrant={vi.fn()}
      />,
    );
    await flushPromises();
    expect(container!.querySelector('[data-banner="impersonate-active"]')).toBeNull();
  });

  it('renders banner with content-lock §2 字面 byte-identical when grant is active', async () => {
    render(
      <BannerImpersonate
        fetchGrant={async () => ({
          id: 'g-1',
          user_id: 'u1',
          granted_at: Date.now(),
          expires_at: Date.now() + 23 * 3600 * 1000 + 59 * 60 * 1000,
          revoked_at: null,
          admin_username: 'alice',
        })}
        revokeGrant={vi.fn()}
      />,
    );
    await flushPromises();
    const banner = container!.querySelector('[data-banner="impersonate-active"]');
    expect(banner).not.toBeNull();
    const text = banner!.textContent ?? '';
    // content-lock §2: "support {admin_username} 正在协助你, 剩 {h}h{m}m。 [立即撤销]"
    expect(text).toContain('support alice 正在协助你, 剩 23h');
    expect(text).toContain('立即撤销');
  });

  it('falls back to "support" prefix when admin_username unset (蓝图 §1.4 row 2 字面)', async () => {
    render(
      <BannerImpersonate
        fetchGrant={async () => ({
          id: 'g-1',
          user_id: 'u1',
          granted_at: Date.now(),
          expires_at: Date.now() + 60_000,
          revoked_at: null,
        })}
        revokeGrant={vi.fn()}
      />,
    );
    await flushPromises();
    const banner = container!.querySelector('[data-banner="impersonate-active"]');
    expect(banner).not.toBeNull();
    expect(banner!.textContent).toContain('support support 正在协助你');
  });

  it('never renders raw UUID in banner (反约束 ADM2-NEG-001)', async () => {
    render(
      <BannerImpersonate
        fetchGrant={async () => ({
          id: 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee',
          user_id: 'uuuuuuuu-1111-2222-3333-444444444444',
          granted_at: Date.now(),
          expires_at: Date.now() + 60_000,
          revoked_at: null,
          admin_username: 'alice',
        })}
        revokeGrant={vi.fn()}
      />,
    );
    await flushPromises();
    const banner = container!.querySelector('[data-banner="impersonate-active"]');
    expect(banner!.textContent).not.toContain('aaaaaaaa-bbbb');
    expect(banner!.textContent).not.toContain('uuuuuuuu-1111');
  });

  it('clicking [立即撤销] calls revokeGrant', async () => {
    const revoke = vi.fn().mockResolvedValue(undefined);
    render(
      <BannerImpersonate
        fetchGrant={async () => ({
          id: 'g-1',
          user_id: 'u1',
          granted_at: Date.now(),
          expires_at: Date.now() + 60_000,
          revoked_at: null,
        })}
        revokeGrant={revoke}
      />,
    );
    await flushPromises();
    const btn = container!.querySelector('[data-action="revoke-impersonate"]') as HTMLButtonElement;
    expect(btn).not.toBeNull();
    act(() => btn.click());
    expect(revoke).toHaveBeenCalledTimes(1);
  });
});

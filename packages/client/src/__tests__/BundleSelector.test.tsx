// AP-2 client — BundleSelector vitest (acceptance §3.1, 4 case).
//
// 4 case (acceptance §3.1):
//   1. bundle click 展开 capability checkbox + DOM `data-bundle-name`
//   2. 反向不自动 submit (用户主权, 跟 DM-9 同精神)
//   3. 用户必显式 confirm (onConfirm called only on click)
//   4. content-lock 反 RBAC role name in component body (grep 检查)
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import { BundleSelector } from '../components/BundleSelector';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
});

async function render(node: React.ReactElement) {
  root = createRoot(container!);
  await act(async () => {
    root!.render(node);
  });
}

function fire(target: Element, type: string) {
  const ev = new Event(type, { bubbles: true });
  act(() => {
    target.dispatchEvent(ev);
  });
}

describe('AP-2 ⭐ BundleSelector — bundle 展开 + 用户主权 + content-lock', () => {
  it('§3.1.1 bundle click 展开 capability checkbox + data-bundle-name 锚', async () => {
    let confirmed: string[] | null = null;
    await render(<BundleSelector onConfirm={(c) => { confirmed = c.slice(); }} />);
    // Initial: 3 bundle rows + 0 capability checkbox.
    const rows = container!.querySelectorAll('[data-ap2-bundle-row]');
    expect(rows.length).toBe(3);
    expect(container!.querySelectorAll('[data-ap2-bundle-checkbox]').length).toBe(0);

    // Click workspace bundle expand.
    const expandBtn = container!.querySelector(
      '[data-ap2-bundle-expand][data-bundle-name="workspace"]',
    ) as HTMLButtonElement;
    expect(expandBtn).not.toBeNull();
    await act(async () => {
      expandBtn.click();
    });
    // Workspace bundle has 3 capabilities — expanded checkbox count == 3.
    const checks = container!.querySelectorAll('[data-ap2-bundle-checkbox]');
    expect(checks.length).toBe(3);
    // data-bundle-name anchor on row marks expanded state.
    const ws = container!.querySelector(
      '[data-ap2-bundle-row][data-bundle-name="workspace"]',
    );
    expect(ws?.getAttribute('data-ap2-expanded')).toBe('true');
    // 反向断言: confirmed 没被偷调 (反自动 submit).
    expect(confirmed).toBeNull();
  });

  it('§3.1.2 反向不自动 submit — 仅 expand 不调 onConfirm', async () => {
    let calls = 0;
    await render(<BundleSelector onConfirm={() => { calls++; }} />);
    const expandBtn = container!.querySelector(
      '[data-ap2-bundle-expand][data-bundle-name="reader"]',
    ) as HTMLButtonElement;
    await act(async () => {
      expandBtn.click();
    });
    expect(calls).toBe(0); // 反偷自动 submit
  });

  it('§3.1.3 用户必显式 confirm — click confirm 后 onConfirm called with bundle capabilities', async () => {
    let captured: string[] | null = null;
    await render(<BundleSelector onConfirm={(c) => { captured = c.slice(); }} />);
    const expandBtn = container!.querySelector(
      '[data-ap2-bundle-expand][data-bundle-name="mention"]',
    ) as HTMLButtonElement;
    await act(async () => {
      expandBtn.click();
    });
    // Mention bundle has 2 capabilities (mention_user + send_dm), default checked.
    const confirmBtn = container!.querySelector(
      '[data-ap2-bundle-confirm]',
    ) as HTMLButtonElement;
    expect(confirmBtn).not.toBeNull();
    await act(async () => {
      confirmBtn.click();
    });
    // After tick (Promise resolves microtask).
    await act(async () => {
      await Promise.resolve();
    });
    expect(captured).not.toBeNull();
    expect((captured as unknown as string[]).sort()).toEqual(['dm.send', 'user.mention']);
  });

  it('§3.1.4 content-lock — DOM body 反 RBAC role name (英 4 + 中 3) 0 hit', async () => {
    await render(<BundleSelector onConfirm={() => {}} />);
    // Expand all 3 bundles in turn, snapshot DOM body.
    for (const id of ['workspace', 'reader', 'mention']) {
      const b = container!.querySelector(
        `[data-ap2-bundle-expand][data-bundle-name="${id}"]`,
      ) as HTMLButtonElement;
      await act(async () => { b.click(); });
    }
    const body = (container!.textContent ?? '').toLowerCase();
    for (const bad of ['admin', 'editor', 'viewer', 'owner', 'moderator']) {
      expect(body.includes(bad)).toBe(false);
    }
    for (const bad of ['管理员', '编辑者', '查看者']) {
      expect(body.includes(bad)).toBe(false);
    }
  });
});

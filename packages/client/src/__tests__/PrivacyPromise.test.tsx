// PrivacyPromise.test.tsx — ADM-1 acceptance §1 + §2 Vitest lock.
//
// Anchor: docs/qa/adm-1-implementation-spec.md §4 five negative assertions + acceptance §1/§2.
// Design: admin-model.md §4.1 copy is locked 1:1 (also declared by the drift test).
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import PrivacyPromise, {
  PRIVACY_PROMISES,
  PRIVACY_TABLE_ROWS,
} from '../components/Settings/PrivacyPromise';

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

describe('PrivacyPromise — §1 三条承诺字面 1:1 (admin-model §4.1)', () => {
  it('PRIVACY_PROMISES 三元组 byte-identical 跟 admin-model §4.1 R3 同源', () => {
    expect(PRIVACY_PROMISES).toHaveLength(3);
    expect(PRIVACY_PROMISES[0]).toBe(
      '**Admin 是平台运维, 不是协作者** — 永不出现在 channel / DM / 团队列表里。',
    );
    expect(PRIVACY_PROMISES[1]).toBe(
      '**Admin 看不到消息 / 文件 / artifact 内容** — 除非你主动授权 impersonate (24h 时窗, 顶部红色横幅常驻, 可随时撤销)。',
    );
    expect(PRIVACY_PROMISES[2]).toBe(
      '**Admin 能看的是元数据** (用户名 / channel 名 / 条数 / 登录时间), **看不到正文**。',
    );
  });

  it('renders 3 promises literally as <li> children', () => {
    render(<PrivacyPromise />);
    const items = container!.querySelectorAll('.privacy-promise-item');
    expect(items).toHaveLength(3);
    // marked + DOMPurify renders **bold** as <strong>; textContent, including
    // strong child nodes, must contain the locked literals.
    expect(items[0]!.textContent).toContain('Admin 是平台运维, 不是协作者');
    expect(items[0]!.textContent).toContain('永不出现在 channel / DM / 团队列表里');
    expect(items[1]!.textContent).toContain('Admin 看不到消息 / 文件 / artifact 内容');
    expect(items[1]!.textContent).toContain('24h 时窗, 顶部红色横幅常驻');
    expect(items[2]!.textContent).toContain('Admin 能看的是元数据');
    expect(items[2]!.textContent).toContain('看不到正文');
  });

  it('default expanded — no <details> wrapper (野马 R3 反约束)', () => {
    render(<PrivacyPromise />);
    expect(container!.querySelectorAll('details')).toHaveLength(0);
  });
});

describe('PrivacyPromise — §3 八行 ✅/❌ 表格 byte-identical', () => {
  it('PRIVACY_TABLE_ROWS 八行顺序不变 byte-identical', () => {
    expect(PRIVACY_TABLE_ROWS).toHaveLength(8);
    expect(PRIVACY_TABLE_ROWS[0]).toEqual({ category: '用户名 / 邮箱', mark: '✅', kind: 'allow' });
    expect(PRIVACY_TABLE_ROWS[1]).toEqual({ category: 'channel 名 / 列表', mark: '✅', kind: 'allow' });
    expect(PRIVACY_TABLE_ROWS[2]).toEqual({ category: '消息条数 / 登录时间', mark: '✅', kind: 'allow' });
    expect(PRIVACY_TABLE_ROWS[3]).toEqual({ category: '消息正文 (channel / DM)', mark: '❌', kind: 'deny' });
    expect(PRIVACY_TABLE_ROWS[4]).toEqual({ category: 'artifact / 文件内容', mark: '❌', kind: 'deny' });
    expect(PRIVACY_TABLE_ROWS[5]).toEqual({ category: '你和 owner-agent 内置 DM', mark: '❌', kind: 'deny' });
    expect(PRIVACY_TABLE_ROWS[6]).toEqual({ category: 'API key 原值', mark: '❌', kind: 'deny' });
    expect(PRIVACY_TABLE_ROWS[7]).toEqual({ category: '授权 impersonate 后 24h 实时入站', mark: '✅ (临时)', kind: 'impersonate' });
  });

  it('row class names match policy — 三色锁 (allow/deny/impersonate)', () => {
    render(<PrivacyPromise />);
    const rows = container!.querySelectorAll('.privacy-promise-table tbody tr');
    expect(rows).toHaveLength(8);
    // First 3 rows: allow (gray default).
    expect(rows[0]!.className).toBe('privacy-row-allow');
    expect(rows[0]!.getAttribute('data-row-kind')).toBe('allow');
    expect(rows[1]!.className).toBe('privacy-row-allow');
    expect(rows[2]!.className).toBe('privacy-row-allow');
    // Rows 4-7 (idx 3-6): deny (#d33).
    expect(rows[3]!.className).toBe('privacy-row-deny');
    expect(rows[3]!.getAttribute('data-row-kind')).toBe('deny');
    expect(rows[4]!.className).toBe('privacy-row-deny');
    expect(rows[5]!.className).toBe('privacy-row-deny');
    expect(rows[6]!.className).toBe('privacy-row-deny');
    // Row 8 (idx 7): impersonate amber (#d97706).
    expect(rows[7]!.className).toBe('privacy-row-impersonate');
    expect(rows[7]!.getAttribute('data-row-kind')).toBe('impersonate');
  });

  it('table content shows category + mark byte-identical', () => {
    render(<PrivacyPromise />);
    const rows = container!.querySelectorAll('.privacy-promise-table tbody tr');
    expect(rows[0]!.querySelectorAll('td')[0]!.textContent).toBe('用户名 / 邮箱');
    expect(rows[0]!.querySelectorAll('td')[1]!.textContent).toBe('✅');
    expect(rows[7]!.querySelectorAll('td')[1]!.textContent).toBe('✅ (临时)');
    expect(rows[3]!.querySelectorAll('td')[1]!.textContent).toBe('❌');
  });
});

describe('PrivacyPromise — 反向断言 (acceptance §2 反 grep)', () => {
  it('reject 折叠/collapse/展开/收起 同义词漂移 (源码层)', async () => {
    // @ts-ignore vite ?raw import lacks built-in TS module decl
    const mod = await import('../components/Settings/PrivacyPromise.tsx?raw');
    const src = (mod as { default: string }).default;
    // Allow comments mentioning "默认展开不可折叠" as anti-constraint anchor;
    // reject any actual <details> wrapper or 折叠 button literal.
    expect(src).not.toContain('<details');
    expect(src).not.toContain('collapse');
    expect(src).not.toContain('展开/收起');
  });
});

// CodeRenderer.test.tsx — CV-3.3 acceptance §2.2 §2.3 vitest 锁.
//
// 锚: docs/qa/cv-3-content-lock.md §1 ②③ + acceptance §2.2 §2.3 +
//     spec §0 设计 ① 11 项语言白名单字面.
//
// table-driven 12 项白名单 exact-match (跟 server ValidCodeLanguages
// 同源) + 反向 reject 全名同义词 'golang'/'typescript'/'python'/
// 'shell'/'bash'/'plaintext'.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import CodeRenderer, {
  CODE_LANGUAGES,
  LANG_LABEL,
  normalizeLang,
} from '../components/CodeRenderer';
import { ToastProvider } from '../components/Toast';

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
    root.render(<ToastProvider>{node}</ToastProvider>);
  });
  return root;
}

describe('CodeRenderer — 11 项白名单 + text fallback (12 项)', () => {
  it('CODE_LANGUAGES is exact-match 跟 server ValidCodeLanguages 同源', () => {
    expect(CODE_LANGUAGES).toEqual([
      'go', 'ts', 'js', 'py', 'md', 'sh',
      'sql', 'yaml', 'json', 'html', 'css',
      'text',
    ]);
    expect(CODE_LANGUAGES).toHaveLength(12);
  });

  it.each(CODE_LANGUAGES)('LANG_LABEL[%s] = uppercase', (lang) => {
    expect(LANG_LABEL[lang]).toBe(lang.toUpperCase());
  });

  it('normalizeLang accepts 12 short codes', () => {
    for (const lang of CODE_LANGUAGES) {
      expect(normalizeLang(lang)).toBe(lang);
    }
  });

  it.each(['golang', 'typescript', 'python', 'shell', 'bash', 'plaintext'])(
    'normalizeLang rejects 全名同义词 %s → text fallback',
    (full) => {
      expect(normalizeLang(full)).toBe('text');
    },
  );

  it('normalizeLang null/undefined/empty → text', () => {
    expect(normalizeLang(undefined)).toBe('text');
    expect(normalizeLang(null)).toBe('text');
    expect(normalizeLang('')).toBe('text');
  });
});

describe('CodeRenderer DOM exact-match', () => {
  it('renders <span class="code-lang-badge" data-lang> with uppercase label', () => {
    render(<CodeRenderer body="package main" language="go" />);
    const badge = container!.querySelector('.code-lang-badge');
    expect(badge).toBeTruthy();
    expect(badge!.getAttribute('data-lang')).toBe('go');
    expect(badge!.textContent).toBe('GO');
  });

  it('renders 复制按钮 with title=aria-label="复制代码" + 📋 icon', () => {
    render(<CodeRenderer body="x" language="ts" />);
    const btn = container!.querySelector('.code-copy-btn');
    expect(btn).toBeTruthy();
    expect(btn!.getAttribute('title')).toBe('复制代码');
    expect(btn!.getAttribute('aria-label')).toBe('复制代码');
    expect(btn!.textContent).toBe('📋');
  });

  it('unknown language → text fallback', () => {
    render(<CodeRenderer body="x" language="brainfuck" />);
    const badge = container!.querySelector('.code-lang-badge');
    expect(badge!.getAttribute('data-lang')).toBe('text');
    expect(badge!.textContent).toBe('TEXT');
  });
});

describe('CodeRenderer 复制按钮', () => {
  it('clipboard.writeText called + toast "已复制"', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    render(<CodeRenderer body="hello world" language="js" />);
    const btn = container!.querySelector('.code-copy-btn') as HTMLButtonElement;
    await act(async () => {
      btn.click();
      await Promise.resolve();
    });
    expect(writeText).toHaveBeenCalledWith('hello world');
    // toast item rendered with 已复制 文案锁 exact-match.
    const toastTexts = Array.from(container!.querySelectorAll('.toast-item')).map((el) => el.textContent);
    expect(toastTexts).toContain('已复制');
  });
});

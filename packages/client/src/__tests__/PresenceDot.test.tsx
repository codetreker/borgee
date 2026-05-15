// PresenceDot.test.tsx — AL-3.3 (#R3 Phase 2) DOM 字面锁单测.
//
// al-3.md §3.1 acceptance: online → data-presence="online" + 文本 "在线";
// offline → data-presence="offline" + 文本 "已离线"; error → data-presence=
// "error" + 文本 "故障 (REASON)" (跟 #249 6 reason codes exact-match).
//
// §5.4 reverse constraint: 每命中 .presence-dot 必带 sibling 文本 — 这里通过组件渲染
// 保证 dot + text 永远成对出现, 测两个 sibling 都在 DOM 里.
//
// §5.1 reverse constraint: 文本不准是 "busy" / "idle" / "忙" / "空闲" — 这里穷举状态
// 反查输出文本, 守 phase 2 仅 3 态.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import PresenceDot from '../components/PresenceDot';

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

function render(node: React.ReactElement): HTMLElement {
  const root = createRoot(container!);
  act(() => {
    root.render(node);
  });
  return container!;
}

describe('PresenceDot DOM 字面锁 (al-3.md §3.1)', () => {
  it('online → data-presence="online" + 绿点 class + 文本 "在线"', () => {
    const c = render(<PresenceDot state="online" reason={undefined} />);
    const wrap = c.querySelector('[data-presence]')!;
    expect(wrap.getAttribute('data-presence')).toBe('online');
    expect(c.querySelector('.presence-dot.presence-online')).toBeTruthy();
    expect(c.textContent).toBe('在线');
  });

  it('offline → data-presence="offline" + 文本 "已离线" (野马 §11 不准 idle 灰糊弄)', () => {
    const c = render(<PresenceDot state="offline" reason={undefined} />);
    const wrap = c.querySelector('[data-presence]')!;
    expect(wrap.getAttribute('data-presence')).toBe('offline');
    expect(c.querySelector('.presence-dot.presence-offline')).toBeTruthy();
    expect(c.textContent).toBe('已离线');
  });

  it('error api_key_invalid → data-presence="error" + 文本 "故障 (API key 失效)"', () => {
    const c = render(<PresenceDot state="error" reason="api_key_invalid" />);
    const wrap = c.querySelector('[data-presence]')!;
    expect(wrap.getAttribute('data-presence')).toBe('error');
    expect(wrap.getAttribute('data-reason')).toBe('api_key_invalid');
    expect(c.textContent).toBe('故障 (API key 失效)');
  });

  it('error 6 reason codes exact-match 文案 (跟 #249 lib/agent-state.ts 字面绑定)', () => {
    const cases: Array<[Parameters<typeof PresenceDot>[0]['reason'], string]> = [
      ['api_key_invalid',     '故障 (API key 失效)'],
      ['quota_exceeded',      '故障 (已超出配额)'],
      ['network_unreachable', '故障 (网络不可达)'],
      ['runtime_crashed',     '故障 (Runtime 崩溃)'],
      ['runtime_timeout',     '故障 (Runtime 超时)'],
      ['unknown',             '故障 (未知错误)'],
    ];
    for (const [reason, text] of cases) {
      const c = render(<PresenceDot state="error" reason={reason} />);
      expect(c.textContent).toBe(text);
    }
  });

  it('undefined state 兜底 offline (server 没回 state 时不糊弄)', () => {
    const c = render(<PresenceDot state={undefined} reason={undefined} />);
    expect(c.querySelector('[data-presence]')!.getAttribute('data-presence')).toBe('offline');
    expect(c.textContent).toBe('已离线');
  });

  it('compact mode 把文案放 title, 不渲染 visible text 旁标 (sidebar 密集列表用)', () => {
    const c = render(<PresenceDot state="online" reason={undefined} compact />);
    const wrap = c.querySelector('[data-presence]')! as HTMLElement;
    expect(wrap.getAttribute('title')).toBe('在线');
    // sr-only 仍含文本 (a11y), 但视觉上不显. Reverse constraint §5.4 仍通过 sr-only 满足.
    expect(c.textContent).toContain('在线');
  });

  it('reverse constraint AL-3 §5.1 (Phase 2 仅 3 态): online/offline/error 状态的 text 不准误漏 busy/idle 字面 (busy/idle 走独立 path AL-1b)', () => {
    for (const state of ['online', 'offline', 'error'] as const) {
      const c = render(<PresenceDot state={state} reason={state === 'error' ? 'unknown' : undefined} />);
      const text = c.textContent ?? '';
      // AL-1a 三态 不应吐 busy/idle 字面 (busy/idle 由 AL-1b 独立 state 触发).
      expect(text).not.toMatch(/busy|idle|忙|空闲/i);
    }
  });
});

describe('PresenceDot — AL-1b (#R3 Phase 4) busy/idle DOM 字面锁', () => {
  // acceptance al-1b.md §3.1 — busy → data-task-state="busy" + 蓝点 + "在工作".
  // data-presence 仍是 'online' (busy/idle = 连着), data-task-state 是独立 attr.
  it('busy → data-task-state="busy" + data-presence="online" + 文本 "在工作" + presence-task-busy class', () => {
    const c = render(<PresenceDot state="busy" reason={undefined} />);
    const wrap = c.querySelector('[data-presence]')!;
    expect(wrap.getAttribute('data-presence')).toBe('online');
    expect(wrap.getAttribute('data-task-state')).toBe('busy');
    expect(c.querySelector('.presence-dot.presence-task-busy')).toBeTruthy();
    expect(c.textContent).toBe('在工作');
  });

  // acceptance §3.2 — idle → data-task-state="idle" + 灰点 + "空闲".
  it('idle → data-task-state="idle" + data-presence="online" + 文本 "空闲" + presence-task-idle class', () => {
    const c = render(<PresenceDot state="idle" reason={undefined} />);
    const wrap = c.querySelector('[data-presence]')!;
    expect(wrap.getAttribute('data-presence')).toBe('online');
    expect(wrap.getAttribute('data-task-state')).toBe('idle');
    expect(c.querySelector('.presence-dot.presence-task-idle')).toBeTruthy();
    expect(c.textContent).toBe('空闲');
  });

  // acceptance §3.3 — AL-1a 三态 data-task-state 应为空 string (DOM 不渲染 attr 或填空).
  it('AL-1a 三态 (online/offline/error) data-task-state 为空 string (回归不破)', () => {
    for (const state of ['online', 'offline', 'error'] as const) {
      const c = render(<PresenceDot state={state} reason={state === 'error' ? 'unknown' : undefined} />);
      const wrap = c.querySelector('[data-presence]')!;
      expect(wrap.getAttribute('data-task-state')).toBe('');
    }
  });

  // acceptance §3.4 — 文案锁 reverse constraint: 不准 "活跃" / "running" / "Standing by" / "等待中" 模糊词.
  it('reverse constraint §3.4: busy/idle 文案不准用模糊词 ("活跃"/"running"/"Standing by"/"等待中")', () => {
    for (const state of ['busy', 'idle'] as const) {
      const c = render(<PresenceDot state={state} reason={undefined} />);
      const text = c.textContent ?? '';
      expect(text).not.toMatch(/活跃|running|standing\s*by|等待中/i);
    }
  });

  // 设计 ① 拆三路径 — busy/idle compact mode 同样套 title 文案锁.
  it('busy compact mode → title="在工作" + sr-only text', () => {
    const c = render(<PresenceDot state="busy" reason={undefined} compact />);
    const wrap = c.querySelector('[data-presence]')! as HTMLElement;
    expect(wrap.getAttribute('title')).toBe('在工作');
    expect(wrap.getAttribute('data-task-state')).toBe('busy');
    expect(c.textContent).toContain('在工作');
  });
});

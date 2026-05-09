// dnd_position.test.ts — CHN-12.3 computeReorderPosition unit test.
//
// Spec: docs/implementation/modules/chn-12-spec.md §1 CHN-12.3.
// Content lock: docs/qa/chn-12-content-lock.md §3 单调小数算法.
// Mirror: CHN-3.3 #415 acceptance §2.4 (REAL 单调小数) byte-identical.

import { describe, it, expect } from 'vitest';
import { computeReorderPosition } from '../lib/dnd_position';

describe('CHN-12.3 computeReorderPosition (单调小数算法 byte-identical 跟 CHN-3.3 #415)', () => {
  it('① 都 null (列表唯一行 fallback) → 1.0', () => {
    expect(computeReorderPosition(null, null)).toBe(1.0);
  });

  it('② prev=null (拖到首位) → next-1.0', () => {
    expect(computeReorderPosition(null, 5.0)).toBe(4.0);
    expect(computeReorderPosition(null, 0.5)).toBe(-0.5); // 负 position 允许 (CHN-3 设计 ④)
  });

  it('③ next=null (拖到末尾) → prev+1.0', () => {
    expect(computeReorderPosition(3.0, null)).toBe(4.0);
    expect(computeReorderPosition(-2.5, null)).toBe(-1.5);
  });

  it('④ 中间插入 → 两邻 position 中点 (REAL 单调小数)', () => {
    expect(computeReorderPosition(1.0, 3.0)).toBe(2.0);
    expect(computeReorderPosition(2.0, 2.5)).toBe(2.25);
    // 单调性保留 — 浮点足够分辨力 v0.
    expect(computeReorderPosition(1.0, 1.0001)).toBeCloseTo(1.00005, 5);
  });

  it('⑤ 跟 CHN-3.3 设计 ④ 一致: 接受负数 position (REAL 含负, MIN-1.0 client 算 — pin 走 ChannelContextMenu 既有 path 不漂)', () => {
    expect(computeReorderPosition(-100.5, -99.0)).toBe(-99.75);
    expect(computeReorderPosition(-1e6, 1e6)).toBe(0);
  });
});

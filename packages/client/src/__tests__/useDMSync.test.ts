// useDMSync.test.ts — 5 vitest cases pin DM-3.2 设计 ④ (cursor round-trip
// 跟 lastSeenCursor 同模式).
//
// Cases:
//   ① cold-start — fresh sessionStorage → loadDMCursor returns 0
//   ② monotonic — persistDMCursor regression rejected
//   ③ page-reload — sessionStorage survives module re-import
//   ④ corrupt-clamp — non-finite / negative input → falls back to 0
//   ⑤ multi-device — two distinct dmIDs use independent storage keys

import { describe, it, expect, beforeEach } from 'vitest';
import {
  loadDMCursor,
  persistDMCursor,
  __resetDMCursorForTests,
} from '../hooks/useDMSync';

describe('useDMSync cursor lib (lastSeenCursor 同模式)', () => {
  const dmA = 'dm-aaa';
  const dmB = 'dm-bbb';

  beforeEach(() => {
    __resetDMCursorForTests(dmA);
    __resetDMCursorForTests(dmB);
  });

  it('① cold-start returns 0 with fresh sessionStorage', () => {
    expect(loadDMCursor(dmA)).toBe(0);
    expect(loadDMCursor('any-other-id')).toBe(0);
  });

  it('② monotonic advance only — smaller cursor does not regress', () => {
    expect(persistDMCursor(dmA, 100)).toBe(100);
    expect(loadDMCursor(dmA)).toBe(100);
    // Smaller cursor must NOT regress.
    expect(persistDMCursor(dmA, 50)).toBe(100);
    expect(loadDMCursor(dmA)).toBe(100);
    // Equal cursor is a no-op (still 100).
    expect(persistDMCursor(dmA, 100)).toBe(100);
    // Larger cursor advances.
    expect(persistDMCursor(dmA, 200)).toBe(200);
    expect(loadDMCursor(dmA)).toBe(200);
  });

  it('③ page-reload — persisted value survives a fresh load', () => {
    persistDMCursor(dmA, 42);
    // Simulate page reload by re-reading sessionStorage from scratch.
    expect(loadDMCursor(dmA)).toBe(42);
    // Verify storage key is the documented prefix.
    expect(window.sessionStorage.getItem('borgee.dm3.cursor:' + dmA)).toBe('42');
  });

  it('④ corrupt-clamp — non-finite / negative input falls back to 0', () => {
    // Manually inject corrupt value into sessionStorage.
    window.sessionStorage.setItem('borgee.dm3.cursor:' + dmA, 'not-a-number');
    expect(loadDMCursor(dmA)).toBe(0);
    window.sessionStorage.setItem('borgee.dm3.cursor:' + dmA, '-5');
    expect(loadDMCursor(dmA)).toBe(0);
    // persistDMCursor with negative is a no-op (returns current).
    __resetDMCursorForTests(dmA);
    expect(persistDMCursor(dmA, -1)).toBe(0);
    expect(persistDMCursor(dmA, NaN)).toBe(0);
    expect(persistDMCursor(dmA, Infinity)).toBe(0);
    expect(persistDMCursor('', 100)).toBe(0); // empty channelID rejected
  });

  it('⑤ multi-device — two DM channels use independent storage keys', () => {
    persistDMCursor(dmA, 100);
    persistDMCursor(dmB, 200);
    expect(loadDMCursor(dmA)).toBe(100);
    expect(loadDMCursor(dmB)).toBe(200);
    // Mutating A does not affect B (multi-device cursor isolation).
    persistDMCursor(dmA, 150);
    expect(loadDMCursor(dmA)).toBe(150);
    expect(loadDMCursor(dmB)).toBe(200);
    // Distinct prefixed keys in storage.
    expect(window.sessionStorage.getItem('borgee.dm3.cursor:' + dmA)).toBe('150');
    expect(window.sessionStorage.getItem('borgee.dm3.cursor:' + dmB)).toBe('200');
  });
});

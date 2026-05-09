// CS-2.1 — 故障三态 SSOT 单测 (cs-2-stance-checklist 设计 ① + content-lock §1).
import { describe, it, expect } from 'vitest';
import { FAILURE_TRI_STATE, IsFailureState } from '../lib/cs2-failure-state';

describe('CS-2.1 — FAILURE_TRI_STATE byte-identical (蓝图 §1.3 三态拆死)', () => {
  it('TestCS21_TriStateByteIdentical — 顺序 byte-identical 跟蓝图 §1.3 表', () => {
    expect([...FAILURE_TRI_STATE]).toEqual(['online', 'error', 'offline']);
  });

  it('TestCS21_TriStateLength3 — 严锁 3 项不漂第 4 态', () => {
    expect(FAILURE_TRI_STATE.length).toBe(3);
  });

  it('TestCS21_IsFailureState_TruthTable — 3 true + 5 false', () => {
    // 3 true (三态拆死)
    expect(IsFailureState('online')).toBe(true);
    expect(IsFailureState('error')).toBe(true);
    expect(IsFailureState('offline')).toBe(true);
    // 5 false: AL-1b busy/idle 不漂入 + 同义词漂禁
    expect(IsFailureState('busy')).toBe(false);
    expect(IsFailureState('idle')).toBe(false);
    expect(IsFailureState('standby')).toBe(false);
    expect(IsFailureState('connected')).toBe(false);
    expect(IsFailureState('')).toBe(false);
  });

  it('TestCS21_NoBusyIdleStandbyDrift — busy/idle/standby 反向断 (AL-1b §2.3 拆死)', () => {
    const stateSet = new Set<string>(FAILURE_TRI_STATE);
    expect(stateSet.has('busy')).toBe(false);
    expect(stateSet.has('idle')).toBe(false);
    expect(stateSet.has('standby')).toBe(false);
  });
});

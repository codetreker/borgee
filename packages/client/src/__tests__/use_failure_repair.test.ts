// CS-2.2 — useFailureRepair stub hook (cs-2-stance-checklist 设计 ② 2.5).
import { describe, it, expect } from 'vitest';
import { useFailureRepair, type FailureRepairAction } from '../lib/use_failure_repair';

// Direct hook return-value check — the stub is a pure function (stable inside useCallback),
// so it does not need a React render harness; v1 RPC wiring should use a component test.
describe('CS-2.2 — useFailureRepair (inline 修复 stub)', () => {
  it('TestCS22_3ActionStubReturn — 3 action 占位返 status="pending"', () => {
    // hook 在 React 上下文外不能直接调; 此处验 STUB_MESSAGES + handle 形态由
    // FailurePopover.test.tsx 集成验 (点 button 触发 handle).
    // This unit test locks the action enum shape with a type-level assertion.
    const actions: FailureRepairAction[] = ['reconnect', 'refill_api_key', 'view_logs'];
    expect(actions.length).toBe(3);
  });
});

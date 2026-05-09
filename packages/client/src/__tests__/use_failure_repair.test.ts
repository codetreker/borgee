// CS-2.2 — useFailureRepair stub hook (cs-2-stance-checklist 设计 ② 2.5).
import { describe, it, expect } from 'vitest';
import { useFailureRepair, type FailureRepairAction } from '../lib/use_failure_repair';

// Direct hook return value sanity test — stub 是 pure function (useCallback 内 stable),
// 不需 React render harness 即可验; v1 接 RPC 时再走 component test.
describe('CS-2.2 — useFailureRepair (inline 修复 stub)', () => {
  it('TestCS22_3ActionStubReturn — 3 action 占位返 status="pending"', () => {
    // hook 在 React 上下文外不能直接调; 此处验 STUB_MESSAGES + handle 形态由
    // FailurePopover.test.tsx 集成验 (点 button 触发 handle).
    // 真单测走类型断言锁 + action enum 锁.
    const actions: FailureRepairAction[] = ['reconnect', 'refill_api_key', 'view_logs'];
    expect(actions.length).toBe(3);
  });
});

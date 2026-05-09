// CS-2 — inline repair action handler stub (蓝图 client-shape.md §1.3
// "inline 修复, 不跳设置页"). v0 hook 占位, v1 接 plugin SDK + AL-2a /
// HB-3 真路径.
//
// 反约束 (cs-2-stance-checklist 设计 ② + content-lock §2):
//   - 浮层 3 inline button 不跳设置页 (grep 检查 `navigate.*\/settings`
//     在 Failure*.tsx count==0).
//   - hook return value 是占位 string, 真实施时改 RPC call.

import { useCallback } from 'react';

export type FailureRepairAction = 'reconnect' | 'refill_api_key' | 'view_logs';

export interface FailureRepairResult {
  action: FailureRepairAction;
  /** v0 stub: 'pending' (handler 未真接); v1: 'ok' / 'failed' / 'pending'. */
  status: 'pending';
  /** Stub message — v1 改 RPC error message. */
  message: string;
}

const STUB_MESSAGES: Record<FailureRepairAction, string> = {
  reconnect: '正在重连…',
  refill_api_key: '请填写新的 API key',
  view_logs: '正在打开日志…',
};

/**
 * useFailureRepair — inline 修复 action handler (v0 stub).
 *
 * 真实施 v1: reconnect 接 BPP-3 force-reconnect frame; refill_api_key 接
 * AL-2a config update PATCH; view_logs 接 plugin SDK log stream.
 */
export function useFailureRepair() {
  const handle = useCallback((action: FailureRepairAction): FailureRepairResult => {
    return {
      action,
      status: 'pending',
      message: STUB_MESSAGES[action],
    };
  }, []);
  return { handle };
}

// CS-2 — six-entry plain-language reason dictionary (blueprint client-shape.md §1.3).
//
// The labels stay byte-identical with blueprint §1.3 literals ("DevAgent 跟
// OpenClaw 失联" / "API key 已失效, 需要重新填写"), reasons.IsValid #496, and
// the AL-4 #321 system DM wording lock. Any wording change must update all three:
//   - server: packages/server-go/internal/agent/reasons/reasons.go (literal keys)
//   - client: this file's FAILURE_REASON_LABELS (user-facing labels)
//   - content-lock: docs/qa/cs-2-content-lock.md §3 (literal audit)
//
// Constraints (cs-2-content-lock §3):
//   - synonym drift is forbidden: "故障了" / "挂了" / "不可用" / "服务异常" /
//     "崩了" / "掉线" must have 0 hits in this file.
//   - backend wire error details such as "401 Unauthorized" or
//     "connection refused" must not appear in user-visible text.

import type { AgentRuntimeReason } from './api';

/**
 * FAILURE_REASON_LABELS — six-entry reason dictionary → plain-language template.
 *
 * Template placeholder `{agent_name}` is replaced by formatFailureLabel(). Keep
 * the key order aligned with reasons.IsValid #496 ALL (api_key_invalid /
 * quota_exceeded / network_unreachable / runtime_crashed / runtime_timeout /
 * unknown).
 */
export const FAILURE_REASON_LABELS: Record<AgentRuntimeReason, string> = {
  api_key_invalid: 'API key 已失效, 需要重新填写',
  quota_exceeded: '{agent_name} 的配额已用完',
  network_unreachable: '{agent_name} 跟 OpenClaw 失联',
  runtime_crashed: '{agent_name} 进程崩溃, 请重启',
  runtime_timeout: '{agent_name} 响应超时',
  unknown: '{agent_name} 出错, 请查日志',
};

/**
 * formatFailureLabel returns the plain-language label string with {agent_name} replaced.
 *
 * @param reason - any key in the six-entry dictionary (out-of-dict → fallback 'unknown' template)
 * @param agentName - agent display name (empty string falls back to "agent")
 */
export function formatFailureLabel(
  reason: AgentRuntimeReason | undefined,
  agentName: string,
): string {
  const safeName = agentName && agentName.trim() ? agentName : 'agent';
  const tpl = (reason && FAILURE_REASON_LABELS[reason]) || FAILURE_REASON_LABELS.unknown;
  return tpl.replace(/\{agent_name\}/g, safeName);
}

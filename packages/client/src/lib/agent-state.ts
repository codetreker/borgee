// AL-1a (#R3 Phase 2) — agent runtime state wording lock.
//
// Issue #190 §11 hard requirement: Phase 2 Sidebar must not show an unexplained
// gray dot. State labels must be explicit ("已离线", not vague idle gray), and
// error states must explain the reason (blueprint agent-lifecycle §2.3).
//
// AL-1b (#R3 Phase 4) adds busy/idle. Server-side 5-state merge priority is
// defined in al-1b-spec.md §1 (error > busy > idle > online > offline).
// describeAgentState() only maps one state to a label; server resolveStatus5State()
// owns priority merging, so the client does not re-merge statuses.
//
// Changing these labels also requires updating the server agent.Reason* constant
// strings. Tests bind this file byte-for-byte with internal/agent/state.go and
// internal/api/al_1b_2_status.go.
import type { AgentRuntimeReason, AgentRuntimeState } from './api';

export interface AgentStateLabel {
  text: string;
  tone: 'ok' | 'muted' | 'error';
}

export const REASON_LABELS: Record<AgentRuntimeReason, string> = {
  api_key_invalid: 'API key 失效',
  quota_exceeded: '已超出配额',
  network_unreachable: '网络不可达',
  runtime_crashed: 'Runtime 崩溃',
  runtime_timeout: 'Runtime 超时',
  unknown: '未知错误',
};

export function describeAgentState(
  state: AgentRuntimeState | undefined,
  reason: AgentRuntimeReason | undefined,
): AgentStateLabel {
  if (state === 'online') return { text: '在线', tone: 'ok' };
  if (state === 'error') {
    const reasonText = reason ? REASON_LABELS[reason] ?? reason : '未知错误';
    return { text: `故障 (${reasonText})`, tone: 'error' };
  }
  // AL-1b (#R3 Phase 4) — busy/idle wording lock (acceptance al-1b.md §3.1 + §3.2).
  // Constraint: avoid vague labels such as "活跃" / "running" / "Standing by" /
  // "等待中" (acceptance §3.4 grep guard expects zero hits).
  if (state === 'busy') return { text: '在工作', tone: 'ok' };
  if (state === 'idle') return { text: '空闲', tone: 'muted' };
  // Default + 'offline' bucket: blueprint §2.3 requires an explicit offline label.
  return { text: '已离线', tone: 'muted' };
}

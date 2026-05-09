// AL-1a (#R3 Phase 2) — Agent runtime state 文案锁.
//
// 野马 #190 §11 硬条件: Phase 2 Sidebar 不准 "灰点 + 不说原因" 糊弄. 状态
// 必须明确文案 ("已离线" 而不是模糊 idle 灰), 故障态必须可解释 (蓝图
// agent-lifecycle §2.3 "故障态的关键设计").
//
// AL-1b (#R3 Phase 4) 扩 busy/idle 两态 — server-side 5-state 合并优先级
// 见 al-1b-spec.md §1 (error > busy > idle > online > offline). 客户端
// describeAgentState() 只负责把单个 state 翻成文案 — 优先级合并由 server
// 端 resolveStatus5State() 做 (设计 ① 拆三路径 — client 不重做合并).
//
// 改这里 = 改 server 的 agent.Reason* 常量字符串. 单测 agent-state.test.ts
// 跟 internal/agent/state.go + internal/api/al_1b_2_status.go 字面绑定.
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
  // AL-1b (#R3 Phase 4) — busy/idle 文案锁 (acceptance al-1b.md §3.1 + §3.2).
  // 反约束: 不准 "活跃" / "running" / "Standing by" / "等待中" 模糊 (acceptance
  // §3.4 — grep -nE "活跃|standing by|running" count==0 反查锚).
  if (state === 'busy') return { text: '在工作', tone: 'ok' };
  if (state === 'idle') return { text: '空闲', tone: 'muted' };
  // Default + 'offline' bucket — 蓝图 §2.3 守: 不准糊弄 idle 灰.
  return { text: '已离线', tone: 'muted' };
}

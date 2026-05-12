// AL-3.3 (#R3 Phase 2) — Agent runtime PresenceDot component.
//
// 蓝图 docs/blueprint/current/agent-lifecycle.md §2.3 + §11 文案守:
// Phase 2 仅承诺 online / offline + error 旁路 (busy/idle 留 BPP-1 同期).
// Sidebar 严禁 "灰点不说原因" — 离线必须明确 "已离线", 故障必须显 reason.
//
// AL-1b (#R3 Phase 4) 扩 busy/idle 两态 — DOM 加 data-task-state 槽位
// (acceptance al-1b.md §3.* + spec §1 拆段 AL-1b.3). 字面锁:
//   data-presence="online" / "offline" / "error"     (AL-3 既有 — 不动)
//   data-task-state="busy" / "idle"                  (AL-1b 新增, busy/idle 态填)
//
// DOM 字面锁 (AL-3 acceptance §3.1 + AL-1b acceptance §3.*):
//   data-presence="online"  + 绿点 + 文本 "在线"
//   data-presence="offline" + 文本 "已离线"
//   data-presence="error"   + 文本 "故障 (REASON_LABEL)"
//   data-task-state="busy"  + 蓝点 + 文本 "在工作"  (AL-1b)
//   data-task-state="idle"  + 灰点 + 文本 "空闲"    (AL-1b)
//
// 反约束 (acceptance §3.2): 仅 agent 行带 dot, 人 (role='user'/'admin') 行无槽位.
// 调用方决定是否渲染 PresenceDot — 这个组件本身不判 role, 由 caller 控制.
//
// 反约束 (acceptance §5.1 / 5.4 + al-1b §3.4): 每 presence-dot 必带 sibling
// text — 这里通过 inline 渲染保证 dot + text 永远成对出现 (反"灰点糊弄").
import React from 'react';
import type { AgentRuntimeState, AgentRuntimeReason } from '../lib/api';
import { describeAgentState } from '../lib/agent-state';

export interface PresenceDotProps {
  state: AgentRuntimeState | undefined;
  reason: AgentRuntimeReason | undefined;
  /** Compact mode: 仅 dot, 文案放 title (用于密集列表如 sidebar DM 行). */
  compact?: boolean;
}

// normalizeState — undefined / 未知值兜底为 'offline' (§11 requires explicit offline wording).
// AL-1b: busy/idle 走独立路径 (data-task-state), normalizeState 不返 busy/idle —
// data-presence 仍是 AL-3 三态; 调用方决定 task-state attr 是否填.
function normalizeState(s: AgentRuntimeState | undefined): 'online' | 'offline' | 'error' {
  if (s === 'online' || s === 'error') return s;
  // busy/idle: AL-1b 5-state 合并语义 = "连着但不闲/连着且闲", 跟 AL-3 角度仍是
  // online (有 hub session); data-presence 槽位填 'online' (绿点表示连接活)
  // + data-task-state 填 'busy'/'idle' (任务态独立 DOM attr).
  if (s === 'busy' || s === 'idle') return 'online';
  return 'offline';
}

// taskStateAttr — AL-1b: busy/idle 时返字面值, 其他态返空 string (DOM
// 不渲染 attr 或填空). 跟 data-reason 同模式 (空 string DOM 不显著).
function taskStateAttr(s: AgentRuntimeState | undefined): '' | 'busy' | 'idle' {
  if (s === 'busy' || s === 'idle') return s;
  return '';
}

export default function PresenceDot({ state, reason, compact = false }: PresenceDotProps) {
  const normalized = normalizeState(state);
  const taskState = taskStateAttr(state);
  const label = describeAgentState(state, reason);
  // dot 颜色全部从 CSS 类来 — 改色不许在这里硬写, 防止 a11y 退化.
  // AL-1b: busy/idle 时优先用 task-state 类 (蓝/灰点), 否则走 AL-3 presence 类.
  const dotClass = taskState
    ? `presence-dot presence-task-${taskState}`
    : `presence-dot presence-${normalized}`;

  if (compact) {
    return (
      <span
        className="presence-inline presence-inline-compact"
        data-presence={normalized}
        data-task-state={taskState}
        data-reason={reason ?? ''}
        data-failure-badge={normalized === 'error' ? 'true' : ''}
        title={label.text}
      >
        <span className={dotClass} aria-hidden="true" />
        <span className="sr-only">{label.text}</span>
      </span>
    );
  }
  return (
    <span
      className="presence-inline"
      data-presence={normalized}
      data-task-state={taskState}
      data-reason={reason ?? ''}
      data-failure-badge={normalized === 'error' ? 'true' : ''}
    >
      <span className={dotClass} aria-hidden="true" />
      <span className="presence-text">{label.text}</span>
    </span>
  );
}

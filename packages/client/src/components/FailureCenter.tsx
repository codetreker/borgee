// CS-2 — FailureCenter 团队栏聚合按钮 (蓝图 client-shape.md §1.3 第 4 层).
//
// 触发: ≥2 故障 agent 时渲染按钮 + 展开列表; 单 agent 故障时 return null
// (浮层已够, 不需聚合).
//
// DOM 字面锁 (cs-2-content-lock §2):
//   <button data-cs2-failure-center-toggle>故障中心 (N)</button>
//   <ul data-cs2-failure-center-list>...</ul>
//
// Negative constraint: 不挂 admin-only path (ADM-0 §1.3 constraint — admin 看 audit 不看实时
// 故障 UX); grep 检查 `admin.*failure-ux|admin.*FailureCenter` count==0.
import React, { useState } from 'react';
import type { AgentRuntimeReason } from '../lib/api';
import { formatFailureLabel } from '../lib/cs2-failure-labels';

export interface FailureCenterAgent {
  id: string;
  name: string;
  reason: AgentRuntimeReason;
}

export interface FailureCenterProps {
  failedAgents: ReadonlyArray<FailureCenterAgent>;
}

export default function FailureCenter({ failedAgents }: FailureCenterProps) {
  const [open, setOpen] = useState(false);
  // ≥2 故障 agent 时才渲染聚合按钮 (单 agent 走浮层)
  if (failedAgents.length < 2) return null;
  return (
    <div className="cs2-failure-center" data-cs2-failure-center>
      <button
        type="button"
        className="cs2-failure-center-toggle"
        data-cs2-failure-center-toggle
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
      >
        故障中心 ({failedAgents.length})
      </button>
      {open && (
        <ul className="cs2-failure-center-list" data-cs2-failure-center-list>
          {failedAgents.map((a) => (
            <li key={a.id} data-cs2-failure-center-item={a.id}>
              <span className="cs2-failure-center-agent-name">{a.name}</span>
              <span className="cs2-failure-center-agent-reason">
                {formatFailureLabel(a.reason, a.name)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

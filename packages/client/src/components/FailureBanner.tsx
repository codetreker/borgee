// CS-2 — FailureBanner 顶部 banner (蓝图 client-shape.md §1.3 第 3 层 UX).
//
// 触发: "全部故障" (allAgents.every(failure)) OR "核心 agent 故障 > 5min"
// (>= CORE_AGENT_FAILURE_THRESHOLD_MS).
//
// DOM 字面锁 (cs-2-content-lock §2):
//   <div data-cs2-failure-banner="visible" role="alert">
//     <span>{banner body 文案 byte-identical}</span>
//     <button data-cs2-failure-banner-dismiss>关闭</button>
//   </div>
import React, { useState } from 'react';

/** 核心 agent 故障阈值 — 5 min byte-identical 跟 cs-2-spec.md §0 设计 ②. */
export const CORE_AGENT_FAILURE_THRESHOLD_MS = 5 * 60 * 1000;

export interface FailureBannerAgent {
  id: string;
  name: string;
  isFailed: boolean;
  /** Core agent: 用户 self-marked critical (v0 由 caller 决定). */
  isCore?: boolean;
  /** Failure 起始时间 (ms epoch). */
  failedAt?: number;
}

export interface FailureBannerProps {
  agents: ReadonlyArray<FailureBannerAgent>;
  /** Now (ms) — 注入便于测试; default Date.now(). */
  now?: number;
}

function shouldShow(
  agents: ReadonlyArray<FailureBannerAgent>,
  now: number,
): { show: boolean; body: string } {
  if (agents.length === 0) return { show: false, body: '' };
  // 全部故障 (≥2 agent 全 failed; 单 agent 走浮层 / 故障中心走 ≥2 路径)
  if (agents.length >= 2 && agents.every((a) => a.isFailed)) {
    return { show: true, body: '全部 agent 故障, 请检查' };
  }
  // 核心 agent 故障 > 5min
  const longCore = agents.find(
    (a) => a.isCore && a.isFailed && a.failedAt !== undefined && now - a.failedAt > CORE_AGENT_FAILURE_THRESHOLD_MS,
  );
  if (longCore) {
    return { show: true, body: `${longCore.name} 已故障 5 分钟以上` };
  }
  return { show: false, body: '' };
}

export default function FailureBanner({ agents, now = Date.now() }: FailureBannerProps) {
  const [dismissed, setDismissed] = useState(false);
  const { show, body } = shouldShow(agents, now);
  if (!show || dismissed) return null;
  return (
    <div className="cs2-failure-banner" data-cs2-failure-banner="visible" role="alert">
      <span className="cs2-failure-banner-body">{body}</span>
      <button
        type="button"
        className="cs2-failure-banner-dismiss"
        data-cs2-failure-banner-dismiss
        onClick={() => setDismissed(true)}
        aria-label="关闭"
      >
        关闭
      </button>
    </div>
  );
}

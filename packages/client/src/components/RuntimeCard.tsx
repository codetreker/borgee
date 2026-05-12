// RuntimeCard.tsx — AL-4.3 (#379 v2 §1 拆段) agent runtime 启停 UI
// owner-only DOM gate + 4 态 badge labels matching AL-1a #249 +
// AL-3 #305 + DM-2 #314 同源.
//
// Blueprint refs: docs/blueprint/current/agent-lifecycle.md §2.2 (默认 remote-agent
// + power user 直配 plugin 双路径) + §2.3 (故障可解释) + §11 (沉默胜
// 于 synthetic progress); README.md §1 设计 #7 (Borgee 不带 runtime, plugin
// process descriptor only).
//
// Spec: docs/implementation/modules/al-4-spec.md (#313 v0 → #379
// v2, merged 962fec7) §0 design ①②③ + §1 AL-4.3 split. Checklist: PR #387
// v0.1. Acceptance: PR #318 §3 — agent settings card with owner-only
// start/stop buttons, four-state badge, and reason_label.
//
// 设计反查 (acceptance §3.1-§3.4):
//   ② owner-only — 非 owner 视图 DOM 直接 omit start/stop 按钮 (跟
//     CV-1 ⑦ rollback owner-only DOM gate 同模式, 不仅是 disabled).
//     Constraint: disabled.*owner_id must have 0 hits (grep check + unit test);
//     a disabled button would leak owner information.
//   ③ runtime status ≠ presence — `data-runtime-status` 锁 4 态严闭
//     ('registered','running','stopped','error'), v0 不开 'starting'
//     / 'stopping' / 'restarting' intermediate states (#321 §2). The synchronous
//     API directly UPDATEs status, with no async pending period.
//   reason 复用 AL-1a #249 6 reason — REASON_LABELS 跟 lib/agent-state.ts
//     同源 (改 = 改三处 — server agent/state.go + 此 const + AL-3 PresenceDot;
//     design ④ requires these labels to stay unified).
//
// Rules (#321 §2 grep 检查 + #379 §3):
//   - ❌ 不显示 endpoint_url / last_heartbeat_at 原始时间戳 (#321 §2
//     constraint: avoid false precision and do not expose runtime process internals).
//   - ❌ 不发 toast / 浏览器通知 (#321 §1 general constraint — 走 system DM
//     do not add a parallel UI notification path; §11 prefers silence over synthetic progress).
//   - ❌ data-runtime-status 不准出现 'starting' / 'stopping' /
//     'restarting' (#321 §2 constraint).
//   - ❌ start/stop button 非 owner DOM 直接 omit, 不是 disabled
//     (#321 §2 constraint — disabled.*owner 0 hit).

import React, { useState, useCallback } from 'react';
import {
  type Agent,
  type AgentRuntime,
  type AgentRuntimeStatus,
  startAgentRuntime,
  stopAgentRuntime,
  ApiError,
} from '../lib/api';
import { REASON_LABELS } from '../lib/agent-state';

// STATUS_LABELS — four-state labels kept exact with #321 + spec §0
// design ③. 'registered' means the server has registered the runtime but it has
// not started; do not show owners "已启动" because registered !== running.
const STATUS_LABELS: Record<AgentRuntimeStatus, string> = {
  registered: '未启动',
  running: '运行中',
  stopped: '已停止',
  error: '故障',
};

// STATUS_TONES — color tokens align with the AL-1a #249 PresenceDot palette
// (change both this const and PresenceDot.tsx together; design ④ keeps them unified).
const STATUS_TONES: Record<AgentRuntimeStatus, 'ok' | 'muted' | 'error'> = {
  registered: 'muted',
  running: 'ok',
  stopped: 'muted',
  error: 'error',
};

interface Props {
  agent: Agent;
  runtime: AgentRuntime | null;
  // viewerUserID — null = unauthenticated / loading; non-null = the
  // logged-in user. 设计 ② owner-only DOM gate 走严格相等
  // viewerUserID === agent.owner_id (constraint: undefined / null 都不渲染
  // start/stop 按钮, 防 leak).
  viewerUserID: string | null;
  onRefresh: () => void;
}

export default function RuntimeCard({ agent, runtime, viewerUserID, onRefresh }: Props) {
  const [busy, setBusy] = useState<'start' | 'stop' | null>(null);
  const [error, setError] = useState<string | null>(null);

  const isOwner = viewerUserID !== null && viewerUserID === agent.owner_id;

  const handleStart = useCallback(async () => {
    if (busy) return;
    setBusy('start');
    setError(null);
    try {
      await startAgentRuntime(agent.id);
      onRefresh();
    } catch (err) {
      // Design ⑤: prefer silence over synthetic progress. Show errors inline only,
      // with no toast (#321 §1); runtime status changes use system DMs, while
      // this owner-initiated action gets local inline feedback.
      setError(err instanceof ApiError ? err.message : '启动失败');
    } finally {
      setBusy(null);
    }
  }, [agent.id, busy, onRefresh]);

  const handleStop = useCallback(async () => {
    if (busy) return;
    setBusy('stop');
    setError(null);
    try {
      await stopAgentRuntime(agent.id);
      onRefresh();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '停止失败');
    } finally {
      setBusy(null);
    }
  }, [agent.id, busy, onRefresh]);

  // No runtime registered yet: hide the card entirely. Design ① "Borgee 不带 runtime"
  // means an unregistered agent should not pretend to have a runtime.
  if (!runtime) {
    return null;
  }

  const status = runtime.status;
  const reason = runtime.last_error_reason;
  const reasonLabel = reason ? REASON_LABELS[reason] ?? '未知错误' : null;

  return (
    <div className="runtime-card" data-runtime-status={status}>
      <div className="runtime-card-header">
        <strong>Runtime</strong>
        <span
          className={`runtime-status-badge runtime-status-${STATUS_TONES[status]}`}
          data-status={status}
        >
          {STATUS_LABELS[status]}
        </span>
        {/* error 态 reason badge — 跟 AL-3 PresenceDot 故障文案
            exact text matches the server labels (改 = 改三处 — server state.go + 此 +
            PresenceDot). */}
        {status === 'error' && reason && (
          <span className="runtime-error-reason" data-error-reason={reason}>
            {reasonLabel}
          </span>
        )}
      </div>

      <div className="runtime-card-body">
        <div className="runtime-card-meta">
          {/* Show process_kind; v1 exposes only 'openclaw' (blueprint §2.2). Constraint:
              endpoint_url / last_heartbeat_at 原始时间戳 NOT shown
              (#321 §2 constraint). */}
          <span className="runtime-meta-process" data-process-kind={runtime.process_kind}>
            {runtime.process_kind}
          </span>
        </div>

        {/* 设计 ② owner-only DOM gate — 非 owner 直接 omit (constraint: 不
            disabled, 不 leak owner 信息). isOwner 严格 viewerUserID ===
            agent.owner_id, undefined / null viewerUserID 都不渲染. */}
        {isOwner && (
          <div className="runtime-card-actions" data-runtime-actions="owner">
            {(status === 'registered' || status === 'stopped' || status === 'error') && (
              <button
                className="btn btn-sm btn-primary"
                data-runtime-action="start"
                onClick={handleStart}
                disabled={busy !== null}
              >
                {busy === 'start' ? '...' : '启动'}
              </button>
            )}
            {status === 'running' && (
              <button
                className="btn btn-sm"
                data-runtime-action="stop"
                onClick={handleStop}
                disabled={busy !== null}
              >
                {busy === 'stop' ? '...' : '停止'}
              </button>
            )}
          </div>
        )}

        {error && (
          <div className="runtime-card-error" role="alert">
            {error}
          </div>
        )}
      </div>
    </div>
  );
}

// Exported for test access: file-local consts that keep labels exact
// with #321 §2 + AL-1a #249.
export const RUNTIME_STATUS_LABELS = STATUS_LABELS;
export const RUNTIME_STATUS_TONES = STATUS_TONES;

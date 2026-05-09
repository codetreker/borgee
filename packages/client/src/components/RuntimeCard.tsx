// RuntimeCard.tsx — AL-4.3 (#379 v2 §1 拆段) agent runtime 启停 UI
// owner-only DOM gate + 4 态 badge byte-identical 跟 AL-1a #249 +
// AL-3 #305 + DM-2 #314 同源.
//
// Blueprint锚: docs/blueprint/current/agent-lifecycle.md §2.2 (默认 remote-agent
// + power user 直配 plugin 双路径) + §2.3 (故障可解释) + §11 (沉默胜
// 于假 loading); README.md §1 设计 #7 (Borgee 不带 runtime, plugin
// process descriptor only).
//
// Spec: docs/implementation/modules/al-4-spec.md (飞马 #313 v0 → #379
// v2, merged 962fec7) §0 设计 ①②③ + §1 拆段 AL-4.3. Stance: PR #387
// v0.1 (野马). Acceptance: PR #318 (烈马) §3 — agent settings 卡片
// owner-only 启停按钮 + 4 态 badge + reason_label.
//
// 设计反查 (acceptance §3.1-§3.4):
//   ② owner-only — 非 owner 视图 DOM 直接 omit start/stop 按钮 (跟
//     CV-1 ⑦ rollback owner-only DOM gate 同模式, 不仅是 disabled).
//     反约束: disabled.*owner_id 0 hit (grep 检查 + 单测 belt; disabled
//     leak owner 信息 = 设计漂).
//   ③ runtime status ≠ presence — `data-runtime-status` 锁 4 态严闭
//     ('registered','running','stopped','error'), v0 不开 'starting'
//     / 'stopping' / 'restarting' 中间态 (跟野马 #321 §2 同源 — 同步
//     API 直接 UPDATE, 无异步 pending 期).
//   reason 复用 AL-1a #249 6 reason — REASON_LABELS 跟 lib/agent-state.ts
//     同源 (改 = 改三处 — server agent/state.go + 此 const + AL-3 PresenceDot;
//     设计 ④ 字面禁分裂).
//
// 反约束 (#321 §2 grep 检查 + #379 §3):
//   - ❌ 不显示 endpoint_url / last_heartbeat_at 原始时间戳 (#321 §2
//     反约束 — 沉默胜于假精确, runtime 进程内部细节不外暴; 设计 ① 同精神).
//   - ❌ 不发 toast / 浏览器通知 (#321 §1 通用反约束 — 走 system DM
//     不走 UI 旁路, §11 沉默胜于假 loading).
//   - ❌ data-runtime-status 不准出现 'starting' / 'stopping' /
//     'restarting' (#321 §2 反约束).
//   - ❌ start/stop button 非 owner DOM 直接 omit, 不是 disabled
//     (#321 §2 反约束 — disabled.*owner 0 hit).

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

// STATUS_LABELS — 4 态字面 byte-identical 跟野马 #321 + spec §0 设计 ③
// 同源. 'registered' 是 AL-4.2 server 注册后未启动的态 — UI 显示但不
// 给 owner 展示 "已启动" 误导 (反约束: registered ≠ running, 蓝图
// §2.3 拆死).
const STATUS_LABELS: Record<AgentRuntimeStatus, string> = {
  registered: '未启动',
  running: '运行中',
  stopped: '已停止',
  error: '故障',
};

// STATUS_TONES — 颜色 token 跟 AL-1a #249 PresenceDot 三态调色板对齐
// (改 = 改两处 — 此 const + PresenceDot.tsx; 设计 ④ 字面禁分裂).
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
  // viewerUserID === agent.owner_id (反约束: undefined / null 都不渲染
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
      // 设计 ⑤ 沉默胜于假 loading — error 仅 inline, 不发 toast (#321
      // §1 通用反约束 — runtime 状态变化只走 system DM, UI 是 owner 自
      // 主操作的 inline feedback 例外).
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

  // No runtime registered yet — graceful degrade. Hide entirely (设计
  // ① "Borgee 不带 runtime" — 没注册的 agent 不假装有 runtime).
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
            byte-identical 同源 (改 = 改三处 — server state.go + 此 +
            PresenceDot). */}
        {status === 'error' && reason && (
          <span className="runtime-error-reason" data-error-reason={reason}>
            {reasonLabel}
          </span>
        )}
      </div>

      <div className="runtime-card-body">
        <div className="runtime-card-meta">
          {/* process_kind 显 — v1 仅 'openclaw' (蓝图 §2.2). 反约束:
              endpoint_url / last_heartbeat_at 原始时间戳 NOT shown
              (#321 §2 反约束). */}
          <span className="runtime-meta-process" data-process-kind={runtime.process_kind}>
            {runtime.process_kind}
          </span>
        </div>

        {/* 设计 ② owner-only DOM gate — 非 owner 直接 omit (反约束: 不
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

// Exported for test access — file-local consts that pin文案锁 byte-identical
// 跟 #321 §2 + AL-1a #249 同源.
export const RUNTIME_STATUS_LABELS = STATUS_LABELS;
export const RUNTIME_STATUS_TONES = STATUS_TONES;

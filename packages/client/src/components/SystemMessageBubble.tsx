// SystemMessageBubble.tsx — BPP-3.2.2 + AL-5.2 system DM render path.
//
// Splits the BPP-3.2 grant flow's three-button render OUT of MessageItem.tsx
// so it can be unit-tested without an AppContext provider (MessageItem.tsx
// uses useAppContext for non-system message paths).
//
// 锚: docs/qa/bpp-3.2-content-lock.md §3 — DOM 字面 byte-identical:
//   <button data-bpp32-button="primary" data-action="grant">授权</button>
//   <button data-bpp32-button="danger"  data-action="reject">拒绝</button>
//   <button data-bpp32-button="ghost"   data-action="snooze">稍后</button>
//
// AL-5.2 锚: docs/implementation/modules/al-5-spec.md §1 AL-5.2 —
//   <button data-al5-button="recover" data-action="recover">重连</button>
//
// 反约束: 12 同义词禁词grep 检查 (批准/授予/同意/许可 / 驳回/拒接/否决/
// 不允许 / 稍候/延后/推迟/暂缓/过会儿 + AL-5: 重启/reset/restart/
// 重新启动) — 守 future drift.

import React from 'react';
import { postMeGrant, postAgentRecover } from '../lib/api';

export interface BPP32GrantPayload {
  action: 'grant' | 'reject' | 'snooze';
  agent_id: string;
  capability: string;
  scope: string;
  request_id: string;
}

/**
 * AL5RecoverPayload — quick_action JSON shape for AL-5 owner recovery.
 * Server reads last error reason from agent_state_log; client only sends
 * agent_id + request_id.
 */
export interface AL5RecoverPayload {
  action: 'recover';
  agent_id: string;
  reason: string;
  request_id: string;
}

export interface SystemMessageBubbleProps {
  /** Pre-rendered HTML body (e.g. markdown rendered upstream). */
  bodyHTML: string;
  /** BPP-3.2 quick_action payload (parsed). null = no buttons. */
  bpp32?: BPP32GrantPayload | null;
  /** AL-5 recovery quick_action payload (parsed). null = no recover button. */
  al5?: AL5RecoverPayload | null;
  /** CM-onboarding fallback (parsed). null = no fallback button. */
  fallback?: { kind?: string; label?: string; action?: string } | null;
  /** Test seam — defaults to real postMeGrant API call. */
  onGrant?: (payload: BPP32GrantPayload) => Promise<void>;
  /** Test seam — defaults to real postAgentRecover API call. */
  onRecover?: (payload: AL5RecoverPayload) => Promise<void>;
}

/**
 * isBPP32GrantPayload narrows an arbitrary parsed quick_action JSON to
 * the BPP-3.2 shape (action ∈ 3-enum + 4 required string fields).
 */
export function isBPP32GrantPayload(qa: unknown): qa is BPP32GrantPayload {
  if (!qa || typeof qa !== 'object') return false;
  const o = qa as Record<string, unknown>;
  return (o.action === 'grant' || o.action === 'reject' || o.action === 'snooze')
    && typeof o.agent_id === 'string' && o.agent_id.length > 0
    && typeof o.capability === 'string' && o.capability.length > 0
    && typeof o.scope === 'string' && o.scope.length > 0
    && typeof o.request_id === 'string' && o.request_id.length > 0;
}

/**
 * isAL5RecoverPayload narrows quick_action JSON to AL-5 recovery shape
 * (action='recover' + agent_id + reason + request_id required).
 */
export function isAL5RecoverPayload(qa: unknown): qa is AL5RecoverPayload {
  if (!qa || typeof qa !== 'object') return false;
  const o = qa as Record<string, unknown>;
  return o.action === 'recover'
    && typeof o.agent_id === 'string' && o.agent_id.length > 0
    && typeof o.reason === 'string' && o.reason.length > 0
    && typeof o.request_id === 'string' && o.request_id.length > 0;
}

const SystemMessageBubble: React.FC<SystemMessageBubbleProps> = ({ bodyHTML, bpp32, al5, fallback, onGrant, onRecover }) => {
  const handleClick = (action: 'grant' | 'reject' | 'snooze') => async () => {
    if (!bpp32) return;
    const payload: BPP32GrantPayload = { ...bpp32, action };
    if (onGrant) {
      await onGrant(payload);
    } else {
      await postMeGrant(payload).catch(() => { /* swallow; caller would show toast */ });
    }
  };
  const handleRecover = async () => {
    if (!al5) return;
    if (onRecover) {
      await onRecover(al5);
    } else {
      await postAgentRecover(al5).catch(() => { /* swallow; caller would show toast */ });
    }
  };
  return (
    <div className="message-item message-system">
      <div className="message-system-content">
        <div className="message-text" dangerouslySetInnerHTML={{ __html: bodyHTML }} />
        {bpp32 && (
          <div className="message-system-bpp32-grant" data-bpp32-grant="true">
            <button
              type="button"
              className="message-system-quick-action"
              data-bpp32-button="primary"
              data-action="grant"
              onClick={handleClick('grant')}
            >授权</button>
            <button
              type="button"
              className="message-system-quick-action"
              data-bpp32-button="danger"
              data-action="reject"
              onClick={handleClick('reject')}
            >拒绝</button>
            <button
              type="button"
              className="message-system-quick-action"
              data-bpp32-button="ghost"
              data-action="snooze"
              onClick={handleClick('snooze')}
            >稍后</button>
          </div>
        )}
        {al5 && !bpp32 && (
          <div className="message-system-al5-recover" data-al5-recover="true">
            <button
              type="button"
              className="message-system-quick-action"
              data-al5-button="recover"
              data-action="recover"
              onClick={handleRecover}
            >重连</button>
          </div>
        )}
        {!bpp32 && !al5 && fallback && fallback.kind === 'button' && fallback.label && fallback.action && (
          <button
            type="button"
            className="message-system-quick-action"
            data-action={fallback.action}
            onClick={() => {
              window.dispatchEvent(new CustomEvent('borgee:quick-action', {
                detail: { action: fallback.action },
              }));
            }}
          >{fallback.label}</button>
        )}
      </div>
    </div>
  );
};

export default SystemMessageBubble;

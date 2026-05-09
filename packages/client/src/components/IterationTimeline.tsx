// IterationTimeline.tsx — CV-4 v2: artifact iteration history timeline.
//
// 设计 (跟 cv-4-v2-stance-checklist.md §1+§2+§3):
//   ① iteration history 复用 v1 GET endpoint (listIterations + ?limit
//      query, 不另起 history endpoint / sequence).
//   ② thumbnail history 不存 — 直接显 row.preview_url 字段值 (来自父组件
//      传入的 artifact_versions row map; 不另缓存 thumbnail snapshot).
//   ③ cursor 复用 RT-1.1 useArtifactUpdated — 此组件本身不订阅 RT-1
//      cursor (父组件持有 useArtifactUpdated, 重渲时把最新 iteration
//      list 透传进来). **不写** sessionStorage cursor (跟 DM-4
//      useDMEdit DoesNotWriteOwnCursor 同精神).
//
// API:
//   <IterationTimeline
//     iterations={iterationRows}
//     versionPreviewMap={{ [versionId]: 'https://...' }}
//     onJump={(versionId) => navigate(...)}
//   />
//
// state badge 4 态文案锁 byte-identical 跟 CV-4 v1 #380 content-lock:
// pending / running / completed / failed (英文 enum 字面, 国际化由父组件).

import React from 'react';
import type { ArtifactIteration, IterationState } from '../lib/api';

export interface IterationTimelineProps {
  /** Server-fetched iteration rows (DESC by created_at). */
  iterations: ArtifactIteration[];
  /** Map artifact_version_id → preview_url (CV-3 v2 #517 字段复用,
   *  设计 ② thumbnail history 不存 — 由父组件从 artifact_versions list
   *  反查后传入, 不在此组件缓存). */
  versionPreviewMap?: Record<string, string | null>;
  /** Click callback — jumps to artifact version detail view. */
  onJump?: (versionId: string) => void;
}

const STATE_BADGE_CLASS: Record<IterationState, string> = {
  pending: 'iteration-badge iteration-badge-pending',
  running: 'iteration-badge iteration-badge-running',
  completed: 'iteration-badge iteration-badge-completed',
  failed: 'iteration-badge iteration-badge-failed',
};

const IterationTimeline: React.FC<IterationTimelineProps> = ({
  iterations,
  versionPreviewMap,
  onJump,
}) => {
  if (!iterations || iterations.length === 0) {
    return (
      <div className="iteration-timeline iteration-timeline-empty" data-cv4v2-timeline="empty">
        <p>暂无 iteration 历史</p>
      </div>
    );
  }
  return (
    <ol className="iteration-timeline" data-cv4v2-timeline="list">
      {iterations.map((it) => {
        const versionID = it.created_artifact_version_id ?? null;
        const preview = versionID && versionPreviewMap ? versionPreviewMap[versionID] : null;
        const handleClick = () => {
          if (versionID && onJump) onJump(String(versionID));
        };
        return (
          <li
            key={it.id}
            className="iteration-row"
            data-cv4v2-iteration={it.id}
            data-cv4v2-state={it.state}
          >
            <span
              className={STATE_BADGE_CLASS[it.state]}
              data-cv4v2-badge={it.state}
            >
              {it.state}
            </span>
            <span className="iteration-intent">{it.intent_text}</span>
            {preview && (
              <img
                className="iteration-thumbnail"
                src={preview}
                alt=""
                loading="lazy"
                data-cv4v2-thumbnail="true"
              />
            )}
            {versionID && (
              <button
                type="button"
                className="iteration-jump"
                data-cv4v2-jump="true"
                onClick={handleClick}
              >
                查看版本
              </button>
            )}
          </li>
        );
      })}
    </ol>
  );
};

export default IterationTimeline;

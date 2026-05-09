// ArtifactDrawer — CS-1.2 right-column drawer container (380px slide-in).
//
// 设计 (spec §0):
//   ② drawer → split 升级走 onPromoteToSplit (drag handle OR explicit btn);
//      反向: closed → split 直接 reject (走 useArtifactPanel.promoteToSplit guard)
//   ③ 复用 ArtifactPanel byte-identical (仅加 mode prop wrap, 不改内部渲染)
//
// DOM 锚 (改 = 改两处: 此组件 + acceptance template):
//   - div[data-testid="artifact-drawer"][data-mode="drawer|split|fullscreen"]
//   - button[data-testid="artifact-drawer-close"] aria-label="关闭"
//   - div[data-testid="artifact-drawer-drag-handle"] (drawer→split 触发)

import React from 'react';
import type { ArtifactPanelMode } from '../lib/use_artifact_panel';

interface Props {
  mode: ArtifactPanelMode; // 'drawer' | 'split' | 'fullscreen' (closed 不渲染)
  artifactId: string | null;
  onClose: () => void;
  onPromoteToSplit: () => void;
  children: React.ReactNode; // ArtifactPanel 既有内容
}

export default function ArtifactDrawer({
  mode,
  artifactId,
  onClose,
  onPromoteToSplit,
  children,
}: Props) {
  if (mode === 'closed') return null;

  return (
    <div
      className={`artifact-drawer artifact-drawer-${mode}`}
      data-testid="artifact-drawer"
      data-mode={mode}
      data-artifact-id={artifactId ?? ''}
    >
      <header className="artifact-drawer-header">
        {mode === 'drawer' && (
          <button
            type="button"
            className="artifact-drawer-promote"
            data-testid="artifact-drawer-promote"
            onClick={onPromoteToSplit}
            title="展开为 split 视图"
            aria-label="展开"
          >
            ⇔
          </button>
        )}
        <button
          type="button"
          className="artifact-drawer-close"
          data-testid="artifact-drawer-close"
          onClick={onClose}
          aria-label="关闭"
        >
          ×
        </button>
      </header>
      {mode === 'drawer' && (
        <div
          className="artifact-drawer-drag-handle"
          data-testid="artifact-drawer-drag-handle"
          role="separator"
          aria-orientation="vertical"
          onMouseUp={onPromoteToSplit}
        />
      )}
      <div className="artifact-drawer-body">{children}</div>
    </div>
  );
}

// AppShell — CS-1.1 三栏布局 container (蓝图 client-shape.md §1.2 byte-identical).
//
// 三 grid columns: 240px Sidebar / 1fr 主区 (children) / 380px Artifact 抽屉
// (mode==='drawer' / 'split' / 'fullscreen'; closed 时第三栏不渲染).
//
// 移动 (≤768px) 降级:
//   - Sidebar → drawer (overlay)
//   - 主区 → 单栏 full width
//   - Artifact split → 全屏 modal (mode='fullscreen')
//
// 设计 (spec §0):
//   ① 三栏 byte-identical 跟蓝图 §1.2 ASCII (240px / 1fr / 380px / 768px breakpoint)
//   ② Artifact 4 态 state machine 单源 (useArtifactPanel hook)
//   ③ 复用既有 Sidebar / ChannelView / ArtifactPanel byte-identical (Wrapper milestone)

import React from 'react';
import type { ArtifactPanelMode } from '../lib/use_artifact_panel';

interface AppShellProps {
  sidebar: React.ReactNode;
  main: React.ReactNode;
  artifactPanel?: React.ReactNode;
  artifactMode: ArtifactPanelMode;
  isMobile: boolean;
  sidebarOpen: boolean;
  onSidebarClose: () => void;
}

export const APP_SHELL_DESKTOP_SIDEBAR = 240; // px — 蓝图 §1.2 字面
export const APP_SHELL_DESKTOP_DRAWER = 380; // px — 蓝图 §1.2 字面
export const APP_SHELL_MOBILE_BREAKPOINT = 768; // px — 蓝图 §1.2 字面

/**
 * Compute grid-template-columns based on artifact mode + viewport.
 * Desktop:
 *   closed     → '240px 1fr'
 *   drawer     → '240px 1fr 380px'
 *   split      → '240px 1fr 1fr'
 *   fullscreen → '240px 1fr' (artifact overlays)
 * Mobile:
 *   any → '1fr' (sidebar+artifact 走 overlay/modal)
 */
export function computeGridColumns(mode: ArtifactPanelMode, isMobile: boolean): string {
  if (isMobile) return '1fr';
  switch (mode) {
    case 'closed':
      return '240px 1fr';
    case 'drawer':
      return '240px 1fr 380px';
    case 'split':
      return '240px 1fr 1fr';
    case 'fullscreen':
      return '240px 1fr';
    default:
      return '240px 1fr';
  }
}

export default function AppShell({
  sidebar,
  main,
  artifactPanel,
  artifactMode,
  isMobile,
  sidebarOpen,
  onSidebarClose,
}: AppShellProps) {
  const gridTemplateColumns = computeGridColumns(artifactMode, isMobile);
  const showArtifactColumn =
    !isMobile && (artifactMode === 'drawer' || artifactMode === 'split');
  const showFullscreenOverlay = artifactMode === 'fullscreen';

  return (
    <div
      className="app-shell"
      data-testid="app-shell"
      data-artifact-mode={artifactMode}
      data-mobile={isMobile ? 'true' : 'false'}
      style={{
        display: 'grid',
        gridTemplateColumns,
        height: '100vh',
        width: '100vw',
        overflow: 'hidden',
      }}
    >
      {isMobile && sidebarOpen && (
        <div
          className="app-shell-sidebar-overlay"
          data-testid="app-shell-sidebar-overlay"
          onClick={onSidebarClose}
        />
      )}
      <div
        className={`app-shell-sidebar${isMobile && !sidebarOpen ? ' app-shell-sidebar-closed' : ''}`}
        data-testid="app-shell-sidebar"
      >
        {sidebar}
      </div>
      <div className="app-shell-main" data-testid="app-shell-main">
        {main}
      </div>
      {showArtifactColumn && (
        <div
          className={`app-shell-artifact app-shell-artifact-${artifactMode}`}
          data-testid="app-shell-artifact-column"
        >
          {artifactPanel}
        </div>
      )}
      {showFullscreenOverlay && (
        <div
          className="app-shell-artifact-fullscreen"
          data-testid="app-shell-artifact-fullscreen"
          role="dialog"
          aria-modal="true"
        >
          {artifactPanel}
        </div>
      )}
    </div>
  );
}

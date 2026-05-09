// useArtifactPanel — CS-1.1 4-state state machine for AppShell right column.
//
// 4 状态 (蓝图 client-shape.md §1.2 byte-identical):
//   - 'closed'     无 artifact 引用, 右栏不渲染
//   - 'drawer'     首次点击 artifact 引用 → 380px 右侧抽屉 (轻量预览)
//   - 'split'      显式动作 (拖拽 OR 二次点击) → 主区 + artifact 50/50
//   - 'fullscreen' mobile (≤768px) 降级 → 全屏 modal
//
// 反约束 (spec §0 设计 ②): closed → split 直接 reject (必先经 drawer);
// state 单源不另起多 state.
//
// AST/grep 锚: grep 检查 `SplitView.*directOpen|artifact.*autoSplit|setMode\("split"\)`
// 仅命中 ArtifactDrawer drag handler 一处.

import { useCallback, useState } from 'react';

export type ArtifactPanelMode = 'closed' | 'drawer' | 'split' | 'fullscreen';

export interface ArtifactPanelState {
  mode: ArtifactPanelMode;
  artifactId: string | null;
}

export function useArtifactPanel(initial: ArtifactPanelMode = 'closed') {
  const [state, setState] = useState<ArtifactPanelState>({
    mode: initial,
    artifactId: null,
  });

  // open(artifactId) — 首次点击 artifact 引用 → drawer.
  // closed → drawer 允许; drawer/split/fullscreen → 复用既有 mode (仅切 artifactId).
  const open = useCallback((artifactId: string) => {
    setState((prev) => ({
      mode: prev.mode === 'closed' ? 'drawer' : prev.mode,
      artifactId,
    }));
  }, []);

  // promoteToSplit() — 拖拽 OR 二次点击 → drawer → split.
  // 反约束: 仅 drawer → split 允许; closed → split 直接 reject (返回 false).
  const promoteToSplit = useCallback((): boolean => {
    let promoted = false;
    setState((prev) => {
      if (prev.mode === 'drawer') {
        promoted = true;
        return { ...prev, mode: 'split' };
      }
      // closed → split direct reject; split/fullscreen → no-op
      return prev;
    });
    return promoted;
  }, []);

  // demoteToDrawer() — split → drawer (允许).
  const demoteToDrawer = useCallback(() => {
    setState((prev) =>
      prev.mode === 'split' ? { ...prev, mode: 'drawer' } : prev,
    );
  }, []);

  // close() — 任何状态 → closed (清 artifactId).
  const close = useCallback(() => {
    setState({ mode: 'closed', artifactId: null });
  }, []);

  // setFullscreen(on) — mobile (≤768px) 降级 trigger.
  // closed 状态保持 closed; 否则切 fullscreen / 退回 drawer.
  const setFullscreen = useCallback((on: boolean) => {
    setState((prev) => {
      if (prev.mode === 'closed') return prev;
      return { ...prev, mode: on ? 'fullscreen' : 'drawer' };
    });
  }, []);

  return { state, open, promoteToSplit, demoteToDrawer, close, setFullscreen };
}

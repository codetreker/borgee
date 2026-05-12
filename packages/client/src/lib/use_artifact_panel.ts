// useArtifactPanel — CS-1.1 4-state state machine for AppShell right column.
//
// 4 states, byte-identical with blueprint client-shape.md §1.2:
//   - 'closed'     no artifact reference, right column not rendered
//   - 'drawer'     first artifact-reference click → 380px right drawer (light preview)
//   - 'split'      explicit action (drag OR second click) → main area + artifact 50/50
//   - 'fullscreen' mobile (≤768px) fallback → fullscreen modal
//
// Required transition (spec §0 design ②): closed → split is rejected directly;
// callers must pass through drawer first. Keep one state source rather than
// adding parallel state.
//
// AST/grep anchor: grep check `SplitView.*directOpen|artifact.*autoSplit|setMode\("split"\)`
// should only hit the ArtifactDrawer drag handler.

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

  // open(artifactId) — first artifact-reference click → drawer.
  // closed → drawer is allowed; drawer/split/fullscreen reuse the current mode
  // and only switch artifactId.
  const open = useCallback((artifactId: string) => {
    setState((prev) => ({
      mode: prev.mode === 'closed' ? 'drawer' : prev.mode,
      artifactId,
    }));
  }, []);

  // promoteToSplit() — drag OR second click → drawer → split.
  // Only drawer → split is allowed; closed → split rejects directly and returns false.
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

  // demoteToDrawer() — split → drawer is allowed.
  const demoteToDrawer = useCallback(() => {
    setState((prev) =>
      prev.mode === 'split' ? { ...prev, mode: 'drawer' } : prev,
    );
  }, []);

  // close() — any state → closed and clears artifactId.
  const close = useCallback(() => {
    setState({ mode: 'closed', artifactId: null });
  }, []);

  // setFullscreen(on) — mobile (≤768px) fallback trigger.
  // closed stays closed; other states switch to fullscreen or back to drawer.
  const setFullscreen = useCallback((on: boolean) => {
    setState((prev) => {
      if (prev.mode === 'closed') return prev;
      return { ...prev, mode: on ? 'fullscreen' : 'drawer' };
    });
  }, []);

  return { state, open, promoteToSplit, demoteToDrawer, close, setFullscreen };
}

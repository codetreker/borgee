// useArtifactPanel.test.tsx — CS-1.1 4-state state machine acceptance.
// Uses vanilla createRoot pattern (no @testing-library/react dependency).

import React, { useImperativeHandle, forwardRef } from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import { useArtifactPanel, type ArtifactPanelMode } from '../lib/use_artifact_panel';

type Hook = ReturnType<typeof useArtifactPanel>;

const HookProbe = forwardRef<Hook, { initial?: ArtifactPanelMode }>(
  function HookProbe(props, ref) {
    const hook = useArtifactPanel(props.initial ?? 'closed');
    useImperativeHandle(ref, () => hook, [hook]);
    return null;
  },
);

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
  vi.restoreAllMocks();
});

function mountHook(initial?: ArtifactPanelMode): React.RefObject<Hook> {
  const ref = React.createRef<Hook>();
  act(() => {
    root!.render(<HookProbe ref={ref} initial={initial} />);
  });
  return ref as React.RefObject<Hook>;
}

describe('useArtifactPanel — CS-1.1 4-state machine', () => {
  it('initial state is closed with null artifactId', () => {
    const ref = mountHook();
    expect(ref.current!.state.mode).toBe('closed');
    expect(ref.current!.state.artifactId).toBeNull();
  });

  it('open(id): closed → drawer with artifactId set', () => {
    const ref = mountHook();
    act(() => {
      ref.current!.open('art-1');
    });
    expect(ref.current!.state.mode).toBe('drawer');
    expect(ref.current!.state.artifactId).toBe('art-1');
  });

  it('open(id) when already drawer: artifactId switches, mode stays drawer', () => {
    const ref = mountHook('drawer');
    act(() => {
      ref.current!.open('art-2');
    });
    expect(ref.current!.state.mode).toBe('drawer');
    expect(ref.current!.state.artifactId).toBe('art-2');
  });

  it('promoteToSplit: drawer → split returns true', () => {
    const ref = mountHook('drawer');
    let promoted = false;
    act(() => {
      promoted = ref.current!.promoteToSplit();
    });
    expect(promoted).toBe(true);
    expect(ref.current!.state.mode).toBe('split');
  });

  // ⭐ 设计 ② 反向断言: closed → split 直接 reject
  it('promoteToSplit: closed → no-op returns false (反向 spec §0 设计 ②)', () => {
    const ref = mountHook('closed');
    let promoted = false;
    act(() => {
      promoted = ref.current!.promoteToSplit();
    });
    expect(promoted).toBe(false);
    expect(ref.current!.state.mode).toBe('closed');
  });

  it('demoteToDrawer: split → drawer', () => {
    const ref = mountHook('split');
    act(() => {
      ref.current!.demoteToDrawer();
    });
    expect(ref.current!.state.mode).toBe('drawer');
  });

  it('close: any → closed with artifactId cleared', () => {
    const ref = mountHook();
    act(() => {
      ref.current!.open('art-1');
    });
    expect(ref.current!.state.mode).toBe('drawer');
    act(() => {
      ref.current!.close();
    });
    expect(ref.current!.state.mode).toBe('closed');
    expect(ref.current!.state.artifactId).toBeNull();
  });

  it('setFullscreen(true): drawer → fullscreen; setFullscreen(false) → drawer', () => {
    const ref = mountHook('drawer');
    act(() => {
      ref.current!.open('art-1');
    });
    act(() => {
      ref.current!.setFullscreen(true);
    });
    expect(ref.current!.state.mode).toBe('fullscreen');
    act(() => {
      ref.current!.setFullscreen(false);
    });
    expect(ref.current!.state.mode).toBe('drawer');
  });

  it('setFullscreen on closed: stays closed (反向)', () => {
    const ref = mountHook('closed');
    act(() => {
      ref.current!.setFullscreen(true);
    });
    expect(ref.current!.state.mode).toBe('closed');
  });
});

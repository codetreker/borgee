// ArtifactDrawer.test.tsx — CS-1.2 drawer container DOM contract.

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import ArtifactDrawer from '../components/ArtifactDrawer';

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

function render(node: React.ReactElement) {
  act(() => {
    root!.render(node);
  });
}

describe('ArtifactDrawer — CS-1.2 drawer/split/fullscreen contract', () => {
  it('mode="closed" renders nothing', () => {
    render(
      <ArtifactDrawer
        mode="closed"
        artifactId={null}
        onClose={() => {}}
        onPromoteToSplit={() => {}}
      >
        <div>body</div>
      </ArtifactDrawer>,
    );
    expect(container!.querySelector('[data-testid="artifact-drawer"]')).toBeNull();
  });

  it('mode="drawer" renders close + promote + drag-handle DOM anchors', () => {
    render(
      <ArtifactDrawer
        mode="drawer"
        artifactId="art-1"
        onClose={() => {}}
        onPromoteToSplit={() => {}}
      >
        <div data-testid="art-body">body</div>
      </ArtifactDrawer>,
    );
    const drawer = container!.querySelector('[data-testid="artifact-drawer"]');
    expect(drawer).toBeTruthy();
    expect(drawer?.getAttribute('data-mode')).toBe('drawer');
    expect(drawer?.getAttribute('data-artifact-id')).toBe('art-1');
    expect(container!.querySelector('[data-testid="artifact-drawer-close"]')).toBeTruthy();
    expect(container!.querySelector('[data-testid="artifact-drawer-promote"]')).toBeTruthy();
    expect(container!.querySelector('[data-testid="artifact-drawer-drag-handle"]')).toBeTruthy();
    expect(container!.querySelector('[data-testid="art-body"]')).toBeTruthy();
  });

  it('close button fires onClose', () => {
    const onClose = vi.fn();
    render(
      <ArtifactDrawer
        mode="drawer"
        artifactId="art-1"
        onClose={onClose}
        onPromoteToSplit={() => {}}
      >
        <div>body</div>
      </ArtifactDrawer>,
    );
    const btn = container!.querySelector('[data-testid="artifact-drawer-close"]') as HTMLButtonElement;
    act(() => {
      btn.click();
    });
    expect(onClose).toHaveBeenCalled();
  });

  it('drag-handle mouseUp fires onPromoteToSplit (drawer → split 触发)', () => {
    const onPromote = vi.fn();
    render(
      <ArtifactDrawer
        mode="drawer"
        artifactId="art-1"
        onClose={() => {}}
        onPromoteToSplit={onPromote}
      >
        <div>body</div>
      </ArtifactDrawer>,
    );
    const handle = container!.querySelector('[data-testid="artifact-drawer-drag-handle"]') as HTMLElement;
    act(() => {
      handle.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
    });
    expect(onPromote).toHaveBeenCalled();
  });

  it('mode="split" hides promote+drag-handle (already split)', () => {
    render(
      <ArtifactDrawer
        mode="split"
        artifactId="art-1"
        onClose={() => {}}
        onPromoteToSplit={() => {}}
      >
        <div>body</div>
      </ArtifactDrawer>,
    );
    expect(container!.querySelector('[data-testid="artifact-drawer-promote"]')).toBeNull();
    expect(container!.querySelector('[data-testid="artifact-drawer-drag-handle"]')).toBeNull();
    expect(container!.querySelector('[data-testid="artifact-drawer-close"]')).toBeTruthy();
  });
});

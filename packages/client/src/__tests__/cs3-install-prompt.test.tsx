// CS-3.1 — useInstallPrompt hook 单测.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import { useInstallPrompt, type InstallState } from '../lib/cs3-install-prompt';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
});

function HookHarness({ onState }: { onState: (s: InstallState) => void }) {
  const { state } = useInstallPrompt();
  React.useEffect(() => {
    onState(state);
  }, [state, onState]);
  return null;
}

async function render(node: React.ReactElement) {
  root = createRoot(container!);
  await act(async () => {
    root!.render(node);
  });
}

describe('CS-3.1 — useInstallPrompt (蓝图 §1.1 PWA install)', () => {
  it('TestCS31_InstallStateEnum — 三态 byte-identical', () => {
    const valid: InstallState[] = ['installable', 'installed', 'unavailable'];
    expect(valid.length).toBe(3);
  });

  it('TestCS31_InitialStateUnavailable — 无 event 时 unavailable', async () => {
    let captured: InstallState = 'installable';
    await render(<HookHarness onState={(s) => (captured = s)} />);
    expect(captured).toBe('unavailable');
  });

  it('TestCS31_InstallPromptHookCachesEvent — beforeinstallprompt → installable', async () => {
    let captured: InstallState = 'unavailable';
    await render(<HookHarness onState={(s) => (captured = s)} />);

    // simulate browser firing beforeinstallprompt
    const fakeEvent = new Event('beforeinstallprompt');
    Object.assign(fakeEvent, {
      prompt: () => Promise.resolve(),
      userChoice: Promise.resolve({ outcome: 'accepted', platform: 'web' }),
      platforms: ['web'],
    });
    await act(async () => {
      window.dispatchEvent(fakeEvent);
    });
    expect(captured).toBe('installable');
  });

  it('TestCS31_AppinstalledEvent → installed', async () => {
    let captured: InstallState = 'unavailable';
    await render(<HookHarness onState={(s) => (captured = s)} />);
    await act(async () => {
      window.dispatchEvent(new Event('appinstalled'));
    });
    expect(captured).toBe('installed');
  });
});

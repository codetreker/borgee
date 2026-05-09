// CS-3.1 — PWA install prompt SSOT (蓝图 client-shape.md §1.1 PWA 主战场).
//
// 设计 ① (cs-3-stance-checklist):
//   - `beforeinstallprompt` event 拦截 (preventDefault) + cache deferredPrompt
//   - `prompt()` 必由 user click handler 触发 (Chrome/Edge 防滥用红线)
//   - 三态 enum: 'installable' / 'installed' / 'unavailable'
//
// 反约束:
//   - mount/effect 内调 prompt() — grep 检查 `prompt\(\)\.then` 0 hit
//   - auto/silent install — grep 检查 `auto.*install|silent.*install` 0 hit

import { useEffect, useState, useCallback } from 'react';

export type InstallState = 'installable' | 'installed' | 'unavailable';

/** Browser-emitted event (Chrome/Edge); not in lib.dom.d.ts default types. */
interface BeforeInstallPromptEvent extends Event {
  readonly platforms: ReadonlyArray<string>;
  readonly userChoice: Promise<{ outcome: 'accepted' | 'dismissed'; platform: string }>;
  prompt(): Promise<void>;
}

function detectInstalled(): boolean {
  if (typeof window === 'undefined') return false;
  // display-mode: standalone — PWA 已安装且作为 standalone window 运行
  return window.matchMedia?.('(display-mode: standalone)').matches ?? false;
}

/**
 * useInstallPrompt — React hook returning {state, prompt}.
 *
 * `prompt` MUST be called from a user click handler (Chrome rejects
 * programmatic / mount-time prompt with TypeError).
 */
export function useInstallPrompt(): {
  state: InstallState;
  prompt: () => Promise<'accepted' | 'dismissed' | 'unavailable'>;
} {
  const [deferred, setDeferred] = useState<BeforeInstallPromptEvent | null>(null);
  const [installed, setInstalled] = useState<boolean>(detectInstalled());

  useEffect(() => {
    if (typeof window === 'undefined') return;
    const onBeforeInstall = (e: Event) => {
      e.preventDefault();
      setDeferred(e as BeforeInstallPromptEvent);
    };
    const onAppInstalled = () => {
      setDeferred(null);
      setInstalled(true);
    };
    window.addEventListener('beforeinstallprompt', onBeforeInstall as EventListener);
    window.addEventListener('appinstalled', onAppInstalled);
    return () => {
      window.removeEventListener('beforeinstallprompt', onBeforeInstall as EventListener);
      window.removeEventListener('appinstalled', onAppInstalled);
    };
  }, []);

  const prompt = useCallback(async (): Promise<'accepted' | 'dismissed' | 'unavailable'> => {
    if (!deferred) return 'unavailable';
    await deferred.prompt();
    const choice = await deferred.userChoice;
    setDeferred(null);
    return choice.outcome;
  }, [deferred]);

  let state: InstallState;
  if (installed) state = 'installed';
  else if (deferred) state = 'installable';
  else state = 'unavailable';

  return { state, prompt };
}

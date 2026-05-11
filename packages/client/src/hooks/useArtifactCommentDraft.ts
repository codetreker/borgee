// useArtifactCommentDraft — CV-10.1 client hook for unsaved comment draft
// persistence across page reloads. Pure localStorage (反约束 0 server code).
//
// Spec: docs/implementation/modules/cv-10-spec.md §0 立场 ①.
// Stance: docs/qa/cv-10-stance-checklist.md §1.
// Content-lock: docs/qa/cv-10-content-lock.md §3 (key namespace).
//
// 立场反查:
//   - ① localStorage 单源, key namespace `borgee.cv10.comment-draft:<artifactId>`
//     byte-identical (跟 DM-4 既有 `borgee.dm.draft:` 同模式).
//   - ② save is debounced (500ms), avoiding a localStorage write on every keystroke; clear()
//     是 submit 后调用 (移除 key, getItem 返回 null).
//
// 反约束:
//   - 不用 sessionStorage (要跨 reload)
//   - No server fetch (反向 grep 见 cv-10-content-lock §4)

import { useCallback, useEffect, useRef, useState } from 'react';

const KEY_PREFIX = 'borgee.cv10.comment-draft:';
const SAVE_DEBOUNCE_MS = 500;

function keyFor(artifactId: string): string {
  return KEY_PREFIX + artifactId;
}

export interface UseArtifactCommentDraftResult {
  /** Current draft text (initial value loaded from localStorage). */
  draft: string;
  /** Update draft (also schedules debounced localStorage write). */
  setDraft: (value: string) => void;
  /** Remove the localStorage entry (call after successful submit). */
  clear: () => void;
  /** True if and only if a draft existed in localStorage at mount. */
  restored: boolean;
}

export function useArtifactCommentDraft(artifactId: string): UseArtifactCommentDraftResult {
  const initial = (() => {
    try {
      return localStorage.getItem(keyFor(artifactId)) ?? '';
    } catch {
      return '';
    }
  })();
  const [draft, setDraftState] = useState(initial);
  const [restored] = useState(initial !== '');
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Debounced write: avoid writing to localStorage on every keystroke.
  const setDraft = useCallback(
    (value: string) => {
      setDraftState(value);
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
      timerRef.current = setTimeout(() => {
        try {
          if (value === '') {
            localStorage.removeItem(keyFor(artifactId));
          } else {
            localStorage.setItem(keyFor(artifactId), value);
          }
        } catch {
          // localStorage may be disabled; silent fallback.
        }
      }, SAVE_DEBOUNCE_MS);
    },
    [artifactId],
  );

  const clear = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    try {
      localStorage.removeItem(keyFor(artifactId));
    } catch {
      // ignore
    }
    setDraftState('');
  }, [artifactId]);

  // Cleanup pending timer on unmount.
  useEffect(() => {
    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
    };
  }, []);

  return { draft, setDraft, clear, restored };
}

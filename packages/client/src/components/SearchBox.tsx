// SearchBox — CV-6 client SPA artifact search input (#cv-6).
//
// Spec: docs/implementation/modules/cv-6-spec.md.
// Content lock: docs/qa/cv-6-content-lock.md §2 (DOM 字面锁) + §4 (5 错码
// 文案 byte-identical 跟 server const + SEARCH_ERR_TOAST map 同源).
//
// 设计反查:
//   - ① server-side SSOT (复用 SQLite FTS5) — 不引入 fuse.js / minisearch
//     / fuzzysort / flexsearch (grep 检查 package.json count==0).
//   - ⑨ debounce 300ms — prevent one HTTP request per keypress; useEffect cleanup 模式.
//   - kbd `/` focus + `Esc` clear — UX 标尺 (跟既有 mention `@` 同精神).
//
// DOM 字面锁 (content-lock §2):
//   - <input type="search" placeholder="搜索 artifact (按 / 聚焦)"
//     maxlength="256" data-testid="artifact-search-input"
//     aria-label="搜索 artifact">
import { useEffect, useRef, useState } from 'react';
import { searchArtifacts, SEARCH_ERR_TOAST, type SearchResult } from '../lib/api';

const DEBOUNCE_MS = 300;
const MAXLENGTH = 256;

interface Props {
  channelId: string;
  onResults: (results: SearchResult[]) => void;
  onError?: (toastMessage: string) => void;
}

export default function SearchBox({ channelId, onResults, onError }: Props) {
  const [query, setQuery] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // kbd `/` focus + `Esc` clear (跟既有 mention `@` 同精神).
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      const target = e.target as HTMLElement | null;
      const inEditable =
        target &&
        (target.tagName === 'INPUT' ||
          target.tagName === 'TEXTAREA' ||
          target.isContentEditable);
      if (e.key === '/' && !inEditable) {
        e.preventDefault();
        inputRef.current?.focus();
      } else if (e.key === 'Escape' && document.activeElement === inputRef.current) {
        setQuery('');
        onResults([]);
      }
    }
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onResults]);

  // debounce 300ms to avoid one HTTP request per keypress.
  useEffect(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    const trimmed = query.trim();
    if (trimmed === '') {
      onResults([]);
      return;
    }
    timerRef.current = setTimeout(() => {
      void (async () => {
        try {
          const r = await searchArtifacts(trimmed, channelId);
          onResults(r.results);
        } catch (e) {
          const code = e instanceof Error ? e.message : 'unknown';
          // 设计 ④ 错码 → toast 文案 byte-identical 跟 SEARCH_ERR_TOAST.
          const matchKey = Object.keys(SEARCH_ERR_TOAST).find(k => code.includes(k));
          const toast = matchKey ? SEARCH_ERR_TOAST[matchKey] : '搜索请求失败, 请重试';
          onError?.(toast);
        }
      })();
    }, DEBOUNCE_MS);
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [query, channelId, onResults, onError]);

  return (
    <input
      ref={inputRef}
      type="search"
      placeholder="搜索 artifact (按 / 聚焦)"
      maxLength={MAXLENGTH}
      data-testid="artifact-search-input"
      aria-label="搜索 artifact"
      className="artifact-search-input"
      value={query}
      onChange={(e) => setQuery(e.target.value)}
    />
  );
}

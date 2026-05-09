// ChannelSearchInput.tsx — CHN-13.3 channel sidebar search input.
//
// Spec: docs/implementation/modules/chn-13-spec.md §1 CHN-13.3.
// Content lock: docs/qa/chn-13-content-lock.md §1+§5 (placeholder
// `搜索频道` byte-identical + debounce 200ms).
// 反约束: 同义词 reject (find/lookup/locate 中文 3 同义词反向).
//
// Behavior:
//   - controlled input — value/onChange 走 props
//   - debounce 200ms — query 变化后 200ms 静默期才触发 onSearch
//   - 空 q 也触发 onSearch("") — 立即恢复全列表 (grep 检查项)

import React, { useEffect, useState } from 'react';

interface Props {
  /** Initial value (默认空 string). */
  initialQuery?: string;
  /** 调用方 fetchChannels(q) 重 fetch. */
  onSearch: (q: string) => void;
}

const DEBOUNCE_MS = 200;

export function ChannelSearchInput({ initialQuery = '', onSearch }: Props) {
  const [query, setQuery] = useState(initialQuery);

  useEffect(() => {
    const t = setTimeout(() => {
      onSearch(query);
    }, DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [query, onSearch]);

  return (
    <div className="channel-search-input" data-testid="channel-search-input">
      <input
        type="text"
        placeholder="搜索频道"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        data-testid="channel-search-input-field"
        aria-label="搜索频道"
      />
    </div>
  );
}

// useUserLayout.ts — CHN-3.3 client SPA personal layout hook.
//
// Spec: docs/implementation/modules/chn-3-spec.md §1 CHN-3.3 段 + §0
// 立场 ⑥ "ordering 是 client 端事" + #366 立场 ⑥ "GET-PUT loading is separate from push".
// Server: packages/server-go/internal/api/layout.go (#412, stacked off
// CHN-3.1 schema v=19).
// Content lock: docs/qa/chn-3-content-lock.md §1 ④ (failure toast 文案
// "侧栏顺序保存失败, 请重试" 5 源 byte-identical) + ⑥ (GET pull only, no
// push frame).
//
// Behavior:
//   1. On mount, GET /me/layout once — populate local layout map keyed
//      by channel_id. 偏好缺失 → fallback 作者侧顺序 (立场 ② 同 #366
//      "偏好缺失 = fallback 作者顺序").
//   2. setCollapsed(channelId, collapsed) / pinChannel(channelId) /
//      reorder(channelId, newPosition) write to local state immediately
//      (optimistic) and queue a debounced PUT (200ms, 跟 #366 立场 ⑥
//      "拖拽完成立即 PUT debounce 200ms" + acceptance §3.5 同源).
//   3. PUT failure → toast "侧栏顺序保存失败, 请重试" byte-identical
//      (#371 / acceptance §3.5 / #402 ④ / #412 server const 5 源).
//      Layout state rolled back to last server-confirmed snapshot.
//
// 反约束:
//   - Do not subscribe to a push frame (#366 立场 ⑥ + #371 立场
//     ③ + 文案锁 ⑥; reverse grep frame name in ws/ count==0).
//   - 不缓存到 IndexedDB (v3+ 留账; #366 立场 ⑥ + 文案锁 ⑥).
//   - pin is computed client-side: position = MIN(已有 position) - 1.0, using
//     a smaller numeric position each time (立场 ③ + 文案锁 ③; server 不算 MIN-1.0 反约束 #412 注释).

import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiError, type LayoutRow, getMyLayout, putMyLayout } from '../lib/api';
import { useToast } from '../components/Toast';

// 文案锁 byte-identical 跟 #371 / acceptance §3.5 / #402 ④ / #412 server
// 5 源. Changing this string breaks the reverse grep guard (anchor-content-lock 同模式).
export const LAYOUT_SAVE_TOAST = '侧栏顺序保存失败, 请重试';

const PUT_DEBOUNCE_MS = 200;

export interface UserLayout {
  /** Map keyed by channel_id; an absent key falls back to 作者顺序 (立场 ②). */
  byChannel: Map<string, LayoutRow>;
  loaded: boolean;
}

export function useUserLayout() {
  const { showToast } = useToast();
  const [layout, setLayout] = useState<UserLayout>(() => ({
    byChannel: new Map(),
    loaded: false,
  }));
  // Last server-confirmed snapshot for rollback on PUT failure.
  const confirmedRef = useRef<Map<string, LayoutRow>>(new Map());
  // Pending dirty rows queued for next PUT.
  const dirtyRef = useRef<Map<string, LayoutRow>>(new Map());
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const { layout: rows } = await getMyLayout();
        if (cancelled) return;
        const m = new Map<string, LayoutRow>();
        for (const r of rows) m.set(r.channel_id, r);
        confirmedRef.current = new Map(m);
        setLayout({ byChannel: m, loaded: true });
      } catch (err) {
        // 401 / network: no toast; initial load silently falls back to 作者侧顺序.
        if (cancelled) return;
        setLayout({ byChannel: new Map(), loaded: true });
      }
    })();
    return () => {
      cancelled = true;
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, []);

  const flushDirty = useCallback(async () => {
    const dirty = Array.from(dirtyRef.current.values());
    if (dirty.length === 0) return;
    dirtyRef.current = new Map();
    try {
      await putMyLayout(dirty);
      // Persist to confirmed snapshot.
      for (const r of dirty) confirmedRef.current.set(r.channel_id, r);
    } catch (err) {
      // 立场 ⑥: failure toast must stay byte-identical, then roll back state.
      // ApiError carries status — we don't show raw error.message (隐私 +
      // UX 反约束 文案锁 ④).
      const _ = err instanceof ApiError ? err.status : 0;
      showToast(LAYOUT_SAVE_TOAST);
      // Rollback dirty rows to confirmed snapshot.
      setLayout(prev => {
        const next = new Map(prev.byChannel);
        for (const r of dirty) {
          const conf = confirmedRef.current.get(r.channel_id);
          if (conf) next.set(r.channel_id, conf);
          else next.delete(r.channel_id);
        }
        return { byChannel: next, loaded: prev.loaded };
      });
    }
  }, [showToast]);

  const queuePut = useCallback(
    (rows: LayoutRow[]) => {
      for (const r of rows) dirtyRef.current.set(r.channel_id, r);
      if (debounceRef.current) clearTimeout(debounceRef.current);
      debounceRef.current = setTimeout(() => {
        void flushDirty();
      }, PUT_DEBOUNCE_MS);
    },
    [flushDirty],
  );

  const setCollapsed = useCallback(
    (channelId: string, collapsed: boolean) => {
      setLayout(prev => {
        const next = new Map(prev.byChannel);
        const existing = next.get(channelId);
        const row: LayoutRow = {
          channel_id: channelId,
          collapsed: collapsed ? 1 : 0,
          // Default position = 0 if no prior row (server still accepts the UPSERT;
          // 作者侧 fallback ordering 由 channel_groups.position 决定).
          position: existing?.position ?? 0,
        };
        next.set(channelId, row);
        queuePut([row]);
        return { byChannel: next, loaded: prev.loaded };
      });
    },
    [queuePut],
  );

  /**
   * pinChannel — 立场 ③ + 文案锁 ③: position = MIN(已有 position) - 1.0,
   * using a smaller numeric position each time. This moves the channel to the front of the current layout.
   * Multiple pins are allowed (#366 立场 ③ "个人 pin 数量不限"). 反约束: 不裂 pinned BOOL 双源
   * 排序 (反向 grep `pinned\s+BOOL` 0 hit).
   */
  const pinChannel = useCallback(
    (channelId: string) => {
      setLayout(prev => {
        const next = new Map(prev.byChannel);
        let minPos = 0;
        for (const r of next.values()) {
          if (r.position < minPos) minPos = r.position;
        }
        const newPos = minPos - 1.0;
        const existing = next.get(channelId);
        const row: LayoutRow = {
          channel_id: channelId,
          collapsed: existing?.collapsed ?? 0,
          position: newPos,
        };
        next.set(channelId, row);
        queuePut([row]);
        return { byChannel: next, loaded: prev.loaded };
      });
    },
    [queuePut],
  );

  /**
   * unpinChannel — reverse pin: reset position to current MAX + 1.0 (move to the end,
   * 作者侧 fallback applies again). 文案锁 ③ "取消置顶" 字面 byte-identical.
   */
  const unpinChannel = useCallback(
    (channelId: string) => {
      setLayout(prev => {
        const next = new Map(prev.byChannel);
        let maxPos = 0;
        for (const r of next.values()) {
          if (r.position > maxPos) maxPos = r.position;
        }
        const newPos = maxPos + 1.0;
        const existing = next.get(channelId);
        const row: LayoutRow = {
          channel_id: channelId,
          collapsed: existing?.collapsed ?? 0,
          position: newPos,
        };
        next.set(channelId, row);
        queuePut([row]);
        return { byChannel: next, loaded: prev.loaded };
      });
    },
    [queuePut],
  );

  const isPinned = useCallback(
    (channelId: string): boolean => {
      const row = layout.byChannel.get(channelId);
      return row != null && row.position < 0;
    },
    [layout],
  );

  const isCollapsed = useCallback(
    (channelId: string): boolean => {
      const row = layout.byChannel.get(channelId);
      return row?.collapsed === 1;
    },
    [layout],
  );

  return { layout, setCollapsed, pinChannel, unpinChannel, isPinned, isCollapsed };
}

// ChannelContextMenu.tsx — CHN-3.3 channel-row right-click pin menu.
//
// Spec: docs/implementation/modules/chn-3-spec.md §1 CHN-3.3 段
// Content lock: docs/qa/chn-3-content-lock.md §1 ③ ("置顶" / "取消置顶"
// byte-identical 2 字面锁) + ⑤ (DM 行不弹 — 反约束 5 源 byte-identical
// #366 ④ + #364 + #371 ② + #376 §3.4 + #382 ⑤).
//
// 设计 ③ pin = position MIN-1.0 单调小数 (server 不算, useUserLayout 算).
// 设计 ④ DM 行 reverse-DOM omit — 不在此组件 mount; 调用方 (Sidebar)
// 必须按 channel.type==='dm' 判断, 保留 defense-in-depth 约束.
//
// DOM lock byte-identical (cv-3-content-lock.md ③ + reverse grep ≥2):
//   <menu role="menu" data-context="channel-pin">
//     <button>{置顶 | 取消置顶}</button>
//   </menu>

import { useEffect, useRef } from 'react';

interface Props {
  x: number;
  y: number;
  pinned: boolean;
  onPin: () => void;
  onUnpin: () => void;
  onClose: () => void;
}

// 字面锁 byte-identical 跟 chn-3-content-lock.md ③ 同源. Drift here →
// vitest chn-3-content-lock.test.ts grep 检查 fail.
export const PIN_LITERAL = '置顶';
export const UNPIN_LITERAL = '取消置顶';

export default function ChannelContextMenu({
  x,
  y,
  pinned,
  onPin,
  onUnpin,
  onClose,
}: Props) {
  const ref = useRef<HTMLMenuElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        onClose();
      }
    };
    const escHandler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    // RAF defer so the right-click that opened us doesn't immediately close.
    const id = requestAnimationFrame(() => {
      document.addEventListener('mousedown', handler);
      document.addEventListener('keydown', escHandler);
    });
    return () => {
      cancelAnimationFrame(id);
      document.removeEventListener('mousedown', handler);
      document.removeEventListener('keydown', escHandler);
    };
  }, [onClose]);

  return (
    <menu
      ref={ref}
      style={{ position: 'fixed', left: x, top: y, zIndex: 1000 }}
      role="menu"
      data-context="channel-pin"
      className="channel-context-menu"
    >
      {pinned ? (
        <button
          className="channel-context-menu-item"
          onClick={() => {
            onUnpin();
            onClose();
          }}
        >
          {UNPIN_LITERAL}
        </button>
      ) : (
        <button
          className="channel-context-menu-item"
          onClick={() => {
            onPin();
            onClose();
          }}
        >
          {PIN_LITERAL}
        </button>
      )}
    </menu>
  );
}

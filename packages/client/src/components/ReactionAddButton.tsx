// ReactionAddButton — 添加表情的按钮 + emoji 选择面板, 同一份逻辑两处复用:
//   1. 消息已经有 reaction 时: 出现在 ReactionBar 末尾, 用 reaction-pill 样式
//   2. 消息没 reaction 时:    出现在 .message-actions 工具栏, 用 message-action-btn 样式
//
// Design reference: docs/implementation/design/686-message-spacing-reaction-position.md
// - §4 #11 optimistic render + failure toast
// - §4 #11b REMOVE by user_id, not only emoji (race fix)
// - §4 #12 双击防御 (busy state)
// - §4 #15 键盘 a11y (aria-label/aria-haspopup/aria-expanded)
// - §6.1 text lock: failure toast text "添加 reaction 失败, 请重试" must match exactly
//
// Constraints:
//   - 不复制 Picker 组件 (只一份)
//   - 不另起 fetch (调既有 lib/api.addReaction)
//   - 不 dangerouslySetInnerHTML (security constraint)
//   - 不引 floating-ui / popper.js / react-focus-lock 第三方 (本次只是 bug fix)
import React, { useState, useRef, useEffect, useLayoutEffect } from 'react';
import Picker from '@emoji-mart/react';
import emojiData from '@emoji-mart/data';
import * as api from '../lib/api';
import { useAppContext } from '../context/AppContext';
import { useToast } from './Toast';

interface Props {
  channelId: string;
  messageId: string;
  /** 当前用户 id, 用于乐观 dispatch + 失败按 user_id 撤回，避免误删别人的 reaction. */
  currentUserId: string;
  /**
   * 决定渲染样式. inline-pill = 跟 reaction 列那行同款圆角药丸;
   * toolbar-btn = 跟 .message-actions 里 edit/delete 同款无边框小按钮.
   */
  variant: 'inline-pill' | 'toolbar-btn';
}

// Design §6.1 text lock: failure toast must match exactly; change it in three places
// (此 const + design doc + 单元测试断言).
const ADD_REACTION_FAILED_TOAST = '添加 reaction 失败, 请重试';

// emoji-mart Picker 默认宽度 (px). 用常量做翻转判断, 不依赖 popover 首帧 layout —
// 避免异步渲染时 width=0 导致误判. 跟 emoji-mart 默认 perLine=9 + 36px 单格相符,
// 留 8px 视口余量.
const EMOJI_PICKER_WIDTH = 352;
const VIEWPORT_PADDING = 8;
const MOBILE_BREAKPOINT = 768;

export default function ReactionAddButton({ channelId, messageId, currentUserId, variant }: Props) {
  const { dispatch } = useAppContext();
  const { showToast } = useToast();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  // 翻转: 当 anchor 靠右导致 popover 沿默认 left:0 会溢出右视口时, 改成 right:0.
  // null = 还未测量首帧, 先 visibility:hidden 隐藏避免闪烁;
  // 'left' / 'right' = 桌面端测好后用对应对齐;
  // 'mobile' = 视口 < 768px, 不应用 JS 对齐 (让 CSS sheet 模式 left:0 right:0 接管).
  const [align, setAlign] = useState<'left' | 'right' | 'mobile' | null>(null);
  const btnRef = useRef<HTMLButtonElement>(null);
  const pickerRef = useRef<HTMLDivElement>(null);

  // Design §4 #15 keyboard a11y: Escape closes picker (consistent with ReactionBar).
  // outside-click 关 picker.
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (pickerRef.current?.contains(e.target as Node)) return;
      if (btnRef.current?.contains(e.target as Node)) return;
      setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  // gh#bug-reaction-overflow: 视口翻转. 当 + 按钮在消息右侧 (.message-actions
  // 工具栏 right:16px 紧贴消息右边) 时, popover 沿 CSS 默认 left:0 展开会被
  // 视口右边裁切. 在 open 切 true 那一刻 + 视口跨 768 边界时测 anchor 位置 +
  // 已知 picker 宽度, 决定 left / right / mobile 对齐. 移动端 (<=768px) 走 CSS
  // sheet 模式 (position:fixed left:0 right:0), 标 'mobile' 不写 inline 让
  // CSS media query 接管 — 但若 open 时桌面→移动 resize, 必须重测, 否则
  // 残留的 inline left:auto / right:auto 会胜过 media query 的 left:0 / right:0
  // (media query 不加 specificity), 破坏 sheet 模式 (review 双签 §1 §2).
  // 反约束: 不引 floating-ui / popper.js (沿用原作者 §4 #14 决定).
  useLayoutEffect(() => {
    if (!open) {
      setAlign(null);
      return;
    }
    if (typeof window === 'undefined') return;

    const measure = () => {
      // 移动端 sheet 模式: CSS 接管, JS 翻转无意义. 标 'mobile' 让渲染端不写
      // 任何 left/right inline style, CSS @media (max-width:768px) 的
      // position:fixed; left:0; right:0 起效.
      // NB: 用 <= 跟 CSS @media (max-width:768px) 同语义, 不留 768px 缝.
      if (window.innerWidth <= MOBILE_BREAKPOINT) {
        setAlign('mobile');
        return;
      }
      const btn = btnRef.current;
      if (!btn) return;
      const rect = btn.getBoundingClientRect();
      // D1 silent-fallback guard: hidden-by-ancestor / detached node returns
      // {0,0,0,0}. 不要拿零矩形当真值算 flip — 保留前一帧 align (或 null 维持
      // visibility:hidden), 等下次 measure 再决定.
      if (rect.width === 0 && rect.height === 0) return;
      // anchor.left + popover 宽度 > 视口宽 - 余量 → 翻成 right 对齐.
      const overflowsRight =
        rect.left + EMOJI_PICKER_WIDTH > window.innerWidth - VIEWPORT_PADDING;
      setAlign(overflowsRight ? 'right' : 'left');
    };

    measure();
    // 跨 768 边界 (旋转 / resize) 时重测, 防止 desktop 残留 inline left/right:auto
    // 胜过移动 media query.
    window.addEventListener('resize', measure);
    return () => window.removeEventListener('resize', measure);
  }, [open]);

  // Design §4 #11 optimistic render + #11b race fix: dispatch optimistic add,
  // then await api.addReaction; on failure, remove by user_id instead of
  // deleting the whole emoji row, then showToast.
  const handlePickerSelect = async (emoji: { native: string }) => {
    if (busy) return; // §4 #12 双击防御
    setOpen(false);
    setBusy(true);
    // ① Optimistic dispatch: show the new pill immediately.
    dispatch({
      type: 'ADD_REACTION_OPTIMISTIC',
      channelId,
      messageId,
      emoji: emoji.native,
      userId: currentUserId,
    });
    try {
      // ② Await server confirmation.
      await api.addReaction(messageId, emoji.native);
      // ③ Success: no local change. Server ack triggers WS UPDATE_REACTIONS
      // with the full row (server dedupes by user_id + emoji), so no duplicate pill is added.
    } catch {
      // ④ Failure: remove by user_id to avoid deleting another user's reaction, then toast.
      dispatch({
        type: 'REMOVE_REACTION_OPTIMISTIC',
        channelId,
        messageId,
        emoji: emoji.native,
        userId: currentUserId,
      });
      showToast(ADD_REACTION_FAILED_TOAST);
    } finally {
      setBusy(false);
    }
  };

  const handleClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (busy) return; // §4 #12 防双击
    setOpen(v => !v);
  };

  const btnClass =
    variant === 'inline-pill'
      ? 'reaction-pill reaction-add'
      : 'message-action-btn message-action-react';

  return (
    <span className="reaction-add-button-root">
      <button
        ref={btnRef}
        type="button"
        className={btnClass}
        onClick={handleClick}
        title="添加表情"
        aria-label="添加表情"
        aria-haspopup="dialog"
        aria-expanded={open}
        disabled={busy}
        data-reaction-add-variant={variant}
      >
        ➕
      </button>
      {open && (
        <div
          className="reaction-picker-popover"
          ref={pickerRef}
          data-reaction-picker-align={align ?? 'measuring'}
          style={
            align === 'right'
              ? { left: 'auto', right: 0, visibility: 'visible' }
              : align === 'left'
                ? { left: 0, right: 'auto', visibility: 'visible' }
                : align === 'mobile'
                  ? { visibility: 'visible' }
                  : { visibility: 'hidden' }
          }
        >
          <Picker
            data={emojiData}
            onEmojiSelect={handlePickerSelect}
            locale="zh"
            previewPosition="none"
          />
        </div>
      )}
    </span>
  );
}

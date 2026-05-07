// ReactionAddButton — 添加表情的按钮 + emoji 选择面板, 同一份逻辑两处复用:
//   1. 消息已经有 reaction 时: 出现在 ReactionBar 末尾, 用 reaction-pill 样式
//   2. 消息没 reaction 时:    出现在 .message-actions 工具栏, 用 message-action-btn 样式
//
// design 锚: docs/implementation/design/686-message-spacing-reaction-position.md
// - §4 #11 乐观渲染 + 失败 toast (yema 拍 X 方案)
// - §4 #11b REMOVE 按 user_id 不按 emoji (feima R2 race 修法)
// - §4 #12 双击防御 (busy state)
// - §4 #15 键盘 a11y (aria-label/aria-haspopup/aria-expanded)
// - §6.1 文案锁: 失败 toast 字面 "添加 reaction 失败, 请重试" byte-identical
//
// 反约束:
//   - 不复制 Picker 组件 (只一份)
//   - 不另起 fetch (调既有 lib/api.addReaction)
//   - 不 dangerouslySetInnerHTML (heima Security 锁)
//   - 不引 floating-ui / popper.js / react-focus-lock 第三方 (本次只是 bug fix)
import React, { useState, useRef, useEffect } from 'react';
import Picker from '@emoji-mart/react';
import emojiData from '@emoji-mart/data';
import * as api from '../lib/api';
import { useAppContext } from '../context/AppContext';
import { useToast } from './Toast';

interface Props {
  channelId: string;
  messageId: string;
  /** 当前用户 id, 用于乐观 dispatch + 失败按 user_id 撤回 (反 race 误删别人 reaction). */
  currentUserId: string;
  /**
   * 决定渲染样式. inline-pill = 跟 reaction 列那行同款圆角药丸;
   * toolbar-btn = 跟 .message-actions 里 edit/delete 同款无边框小按钮.
   */
  variant: 'inline-pill' | 'toolbar-btn';
}

// design §6.1 文案锁: 失败 toast 字面 byte-identical, 改 = 改三处
// (此 const + design doc + 单元测试断言).
const ADD_REACTION_FAILED_TOAST = '添加 reaction 失败, 请重试';

export default function ReactionAddButton({ channelId, messageId, currentUserId, variant }: Props) {
  const { dispatch } = useAppContext();
  const { showToast } = useToast();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const btnRef = useRef<HTMLButtonElement>(null);
  const pickerRef = useRef<HTMLDivElement>(null);

  // design §4 #15 键盘 a11y: Escape 关 picker (跟既有 ReactionBar 一致).
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

  // design §4 #11 乐观渲染 + #11b race 修法: dispatch optimistic add, 然后
  // await api.addReaction; 失败时按 user_id 撤回 (不按 emoji 删整条) +
  // showToast.
  const handlePickerSelect = async (emoji: { native: string }) => {
    if (busy) return; // §4 #12 双击防御
    setOpen(false);
    setBusy(true);
    // ① 乐观 dispatch — 立刻在 UI 显新 pill
    dispatch({
      type: 'ADD_REACTION_OPTIMISTIC',
      channelId,
      messageId,
      emoji: emoji.native,
      userId: currentUserId,
    });
    try {
      // ② await 服务器
      await api.addReaction(messageId, emoji.native);
      // ③ 成功 — 不动. 服务器 ack 后 WS 会推 UPDATE_REACTIONS 整列替换
      // (服务器端按 user_id + emoji dedupe), 不会重复加 pill.
    } catch {
      // ④ 失败 — 按 user_id 撤回 (反 race 误删别人 reaction) + toast.
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
        <div className="reaction-picker-popover" ref={pickerRef}>
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

import React from "react";
import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import type { Channel } from "../types";

interface Props {
  channel: Channel;
  active: boolean;
  isOwner: boolean;
  onClick: () => void;
  groupId?: string | null;
  /** CHN-3.3 — personal pin context menu trigger (DM rows must not pass this). */
  onContextMenu?: (e: React.MouseEvent) => void;
  /** CHN-3.3 — personal pinned indicator (pin = position < 0 单调小数). */
  pinned?: boolean;
}

export default function SortableChannelItem({ channel, active, isOwner, onClick, groupId, onContextMenu, pinned }: Props) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
    isOver,
  } = useSortable({ id: channel.id, disabled: !isOwner, data: { type: 'channel' as const, groupId: groupId ?? null } });

  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.4 : 1,
    position: "relative",
  };

  const unread = channel.unread_count ?? 0;
  const isPrivate = channel.visibility === "private";
  const isMember = channel.is_member !== false;
  // CHN-1.3 设计 ⑤: archived channels render with a dimmed style + 📦 marker
  // so members see closures inline. Server-side they are filtered out of the
  // public discovery list for non-members; current member rows still see them
  // (history preservation, channel-model.md §2 不变量 #3).
  const isArchived = channel.archived_at != null;

  return (
    <button
      ref={setNodeRef}
      style={style}
      className={`channel-item ${active ? "channel-item-active" : ""} ${!isMember ? "channel-item-preview" : ""} ${isArchived ? "channel-item-archived" : ""} ${isDragging ? "channel-item-dragging" : ""} ${pinned ? "channel-item-pinned" : ""}`}
      onClick={onClick}
      onContextMenu={onContextMenu}
      data-archived={isArchived ? "true" : undefined}
      data-pinned={pinned ? "true" : undefined}
    >
      {isOver && !isDragging && (
        <span className="drop-indicator" />
      )}
      {/* CHN-3.3 personal sortable handle (byte-identical 跟 chn-3-content-lock.md
          §1 ① 字面锁 + #371 spec §1 CHN-3.3 同源).
          DOM 锁: <button class="sortable-handle" data-sortable-handle=""
                    aria-label="拖拽调整顺序">⋮⋮</button>.
          反约束: DM 行不渲染 (Sidebar.tsx DMItem 绕过此组件; 此 component
          只服务 channel rows). isOwner 走作者侧 ≡ handle (CHN-1 #288); 非
          owner 也可 reorder 自己侧栏 (CHN-3 设计 ① 物理拆死作者侧 vs 个人) —
          但 dnd-kit useSortable 的 ordering 影响只在本人 SPA 内, 写 PUT
          /me/layout (CHN-3.2). */}
      {!isArchived && (
        <button
          type="button"
          className="sortable-handle"
          data-sortable-handle=""
          aria-label="拖拽调整顺序"
          {...attributes}
          {...listeners}
          onClick={e => e.stopPropagation()}
        >
          ⋮⋮
        </button>
      )}
      <span className="channel-hash">{isArchived ? "📦" : isPrivate ? "🔒" : "#"}</span>
      <span className="channel-name">{channel.name}</span>
      {pinned && <span className="channel-pinned-indicator" title="已置顶">📌</span>}
      {isArchived && <span className="archived-badge" title="已归档">已归档</span>}
      {!isMember && !isPrivate && !isArchived && (
        <span className="preview-badge">预览</span>
      )}
      {unread > 0 && isMember && !isArchived && (
        <span className="unread-badge">{unread > 99 ? "99+" : unread}</span>
      )}
    </button>
  );
}

/** Non-sortable version for preview (non-member) channels */
export function ChannelItemStatic({ channel, active, onClick }: Omit<Props, "isOwner">) {
  const unread = channel.unread_count ?? 0;
  const isPrivate = channel.visibility === "private";
  const isMember = channel.is_member !== false;
  const isArchived = channel.archived_at != null;

  return (
    <button
      className={`channel-item ${active ? "channel-item-active" : ""} ${!isMember ? "channel-item-preview" : ""} ${isArchived ? "channel-item-archived" : ""}`}
      onClick={onClick}
      data-archived={isArchived ? "true" : undefined}
    >
      <span className="channel-hash">{isArchived ? "📦" : isPrivate ? "🔒" : "#"}</span>
      <span className="channel-name">{channel.name}</span>
      {isArchived && <span className="archived-badge" title="已归档">已归档</span>}
      {!isMember && !isPrivate && !isArchived && (
        <span className="preview-badge">预览</span>
      )}
      {unread > 0 && isMember && !isArchived && (
        <span className="unread-badge">{unread > 99 ? "99+" : unread}</span>
      )}
    </button>
  );
}

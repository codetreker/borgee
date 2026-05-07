import React, { useState, useEffect, useRef } from 'react';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { ChannelGroup } from '../types';

interface Props {
  group: ChannelGroup;
  collapsed: boolean;
  onToggle: () => void;
  onContextMenu?: (e: React.MouseEvent) => void;
  isOwner?: boolean;
  renaming?: boolean;
  onRenameSubmit?: (name: string) => void;
  onRenameCancel?: () => void;
}

export default function GroupHeader({ group, collapsed, onToggle, onContextMenu, isOwner, renaming, onRenameSubmit, onRenameCancel }: Props) {
  const [editName, setEditName] = useState(group.name);
  const inputRef = useRef<HTMLInputElement>(null);

  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: `group:${group.id}`, disabled: !isOwner, data: { type: 'group' as const } });

  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.4 : 1,
  };

  useEffect(() => {
    if (renaming) {
      setEditName(group.name);
      inputRef.current?.focus();
      inputRef.current?.select();
    }
  }, [renaming, group.name]);

  const handleRenameKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (editName.trim()) onRenameSubmit?.(editName.trim());
    } else if (e.key === 'Escape') {
      onRenameCancel?.();
    }
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className="group-header"
      data-collapsed={collapsed ? "true" : "false"}
      aria-label="折叠分组"
      onClick={onToggle}
      onContextMenu={e => { e.preventDefault(); onContextMenu?.(e); }}
      {...attributes}
    >
      <span className={`group-header-arrow${collapsed ? ' collapsed' : ''}`} aria-hidden="true">{collapsed ? '▶' : '▼'}</span>
      {renaming ? (
        <input
          ref={inputRef}
          className="group-header-rename-input"
          value={editName}
          onChange={e => setEditName(e.target.value)}
          onKeyDown={handleRenameKeyDown}
          onBlur={() => onRenameCancel?.()}
          onClick={e => e.stopPropagation()}
        />
      ) : (
        <span className="group-header-name">{group.name}</span>
      )}
      {isOwner && !renaming && (
        <span className="group-header-actions">
          <span
            className="drag-handle group-header-drag-handle"
            aria-label="拖动分组"
            {...listeners}
            onClick={e => e.stopPropagation()}
          >
            ≡
          </span>
          <button
            type="button"
            className="icon-btn group-header-menu-btn"
            aria-label="分组菜单"
            onClick={e => { e.stopPropagation(); onContextMenu?.(e); }}
          >
            ⋯
          </button>
        </span>
      )}
    </div>
  );
}

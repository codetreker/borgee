import React, { useMemo, useCallback, useState } from "react";
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  useDroppable,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
  arrayMove,
} from "@dnd-kit/sortable";
import SortableChannelItem, { ChannelItemStatic } from "./SortableChannelItem";
import ChannelGroupComponent from "./ChannelGroupComponent";
import ChannelContextMenu from "./ChannelContextMenu";
import { useUserLayout } from "../hooks/useUserLayout";
import { useAppContext } from "../context/AppContext";
import * as api from "../lib/api";
import type { Channel } from "../types";

const STORAGE_KEY = 'borgee:collapsed-groups';

function loadCollapsedGroups(): Record<string, boolean> {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}');
  } catch { return {}; }
}

function saveCollapsedGroups(collapsed: Record<string, boolean>) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(collapsed));
}

function UngroupedDroppable({ children }: { children: React.ReactNode }) {
  const { setNodeRef } = useDroppable({ id: 'droppable:ungrouped' });
  return <div ref={setNodeRef}>{children}</div>;
}

interface Props {
  channels: Channel[];
  currentChannelId: string | null;
  onSelectChannel: (channelId: string) => void;
}

export default function ChannelList({ channels, currentChannelId, onSelectChannel }: Props) {
  const { state, dispatch } = useAppContext();
  const currentUser = state.currentUser;
  // CHN-3.3 personal layout (拖拽 reorder + pin via position MIN-1.0).
  const { pinChannel, unpinChannel, isPinned } = useUserLayout();
  const [pinMenu, setPinMenu] = useState<{ x: number; y: number; channelId: string } | null>(null);
  const channelGroups = state.groups;

  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>(loadCollapsedGroups);

  const toggleGroup = useCallback((groupId: string) => {
    setCollapsedGroups(prev => {
      const next = { ...prev, [groupId]: !prev[groupId] };
      saveCollapsedGroups(next);
      return next;
    });
  }, []);

  // Mobile: disable touch drag to avoid click conflicts (BUG-023)
  // Desktop only: pointer + keyboard drag
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const { memberChannels, previewChannels } = useMemo(() => {
    const member: Channel[] = [];
    const preview: Channel[] = [];
    for (const ch of channels) {
      if (ch.is_member === false) {
        preview.push(ch);
      } else {
        member.push(ch);
      }
    }
    return { memberChannels: member, previewChannels: preview };
  }, [channels]);

  const { ungroupedChannels, groupedChannels } = useMemo(() => {
    const ungrouped: Channel[] = [];
    const grouped: Record<string, Channel[]> = {};
    for (const ch of memberChannels) {
      if (!ch.group_id) {
        ungrouped.push(ch);
      } else {
        if (!grouped[ch.group_id]) grouped[ch.group_id] = [];
        grouped[ch.group_id].push(ch);
      }
    }
    return { ungroupedChannels: ungrouped, groupedChannels: grouped };
  }, [memberChannels]);

  // CHN-3.3 — personal layout position overrides author-side sort:
  // pinned (position < 0) bubbles to top, then fall through to author
  // ordering (chs.position localeCompare). 立场 ⑥ ordering client 端事.
  const sortChannels = (chs: Channel[]) => {
    return [...chs].sort((a, b) => {
      const ap = (state.currentUser && isPinned(a.id)) ? -1 : 0;
      const bp = (state.currentUser && isPinned(b.id)) ? -1 : 0;
      if (ap !== bp) return ap - bp; // pinned ahead
      if (a.position && b.position) return a.position.localeCompare(b.position);
      if (a.position && !b.position) return -1;
      if (!a.position && b.position) return 1;
      const aTime = a.last_message_at ?? a.created_at;
      const bTime = b.last_message_at ?? b.created_at;
      return bTime - aTime;
    });
  };

  const sortedUngrouped = useMemo(() => sortChannels(ungroupedChannels), [ungroupedChannels, isPinned, state.currentUser]);

  const sortedGroups = useMemo(() => {
    // 防御 (#678): 任何 group 没 position 时, 直接
    // a.position.localeCompare(b.position) 炸 TypeError, 整个 SPA 白屏.
    // 服务器代码 (channels.go::handleCreateGroup + models.go ChannelGroup
    // schema not null) 不会发 undefined, 但旧版本 localStorage / IndexedDB
    // 缓存 / 测试数据 / 手工加的行可能漏 position — 客户端兜底防止
    // single bad row 拖死全部. 跟同文件 sortChannels (L104-116) +
    // ChannelGroupComponent.tsx L44-46 同模式: 两边都有 position 才用
    // localeCompare, 单边或两边都缺时用稳定退路 (有 position 的排前面 /
    // 都没就保持原顺序).
    return [...channelGroups].sort((a, b) => {
      if (a.position && b.position) return a.position.localeCompare(b.position);
      if (a.position && !b.position) return -1;
      if (!a.position && b.position) return 1;
      return 0;
    });
  }, [channelGroups]);

  const sortedPreviewChannels = useMemo(() => {
    return [...previewChannels].sort((a, b) => {
      if (a.position && b.position) return a.position.localeCompare(b.position);
      const aTime = a.last_message_at ?? a.created_at;
      const bTime = b.last_message_at ?? b.created_at;
      return bTime - aTime;
    });
  }, [previewChannels]);

  const ungroupedIds = useMemo(
    () => sortedUngrouped.map(ch => ch.id),
    [sortedUngrouped],
  );

  const groupSortableIds = useMemo(
    // 防御 (#678): 同 sortedGroups, partial state 时 g.id 也可能 undefined,
    // SortableContext items 收 "group:undefined" 字面会让 dnd-kit 内部
    // 算 unique key 出意外. 过滤掉再 push.
    () => sortedGroups.filter(g => g.id).map(g => `group:${g.id}`),
    [sortedGroups],
  );

  const channelGroupLookup = useMemo(() => {
    const map: Record<string, string | null> = {};
    for (const ch of memberChannels) {
      map[ch.id] = ch.group_id ?? null;
    }
    return map;
  }, [memberChannels]);

  const handleDragEnd = useCallback((event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;

    const activeId = String(active.id);
    const overId = String(over.id);

    if (active.data.current?.type === 'group') {
      handleGroupReorder(activeId, overId);
    } else {
      handleChannelReorder(activeId, overId, over.data.current);
    }
  }, [channels, sortedUngrouped, sortedGroups, channelGroups, groupedChannels, channelGroupLookup, dispatch]);

  const handleGroupReorder = useCallback((activeId: string, overId: string) => {
    const srcGroupId = activeId.replace('group:', '');
    const tgtGroupId = overId.replace('group:', '');

    const oldIndex = sortedGroups.findIndex(g => g.id === srcGroupId);
    const newIndex = sortedGroups.findIndex(g => g.id === tgtGroupId);
    if (oldIndex === -1 || newIndex === -1) return;

    const reordered = arrayMove(sortedGroups, oldIndex, newIndex);
    const movedIdx = reordered.findIndex(g => g.id === srcGroupId);
    const afterId = movedIdx === 0 ? null : reordered[movedIdx - 1].id;

    const snapshot = [...channelGroups];
    const updatedGroups = reordered.map((g, i) => ({ ...g, position: String(i).padStart(6, '0') }));
    dispatch({ type: 'SET_GROUPS', groups: updatedGroups });

    api.reorderChannelGroup(srcGroupId, afterId).catch(() => {
      dispatch({ type: 'SET_GROUPS', groups: snapshot });
    });
  }, [sortedGroups, channelGroups, dispatch]);

  const handleChannelReorder = useCallback((activeId: string, overId: string, overData: Record<string, unknown> | undefined) => {
    const sourceGroupId = channelGroupLookup[activeId] ?? null;

    let targetGroupId: string | null;
    if (overId.startsWith('droppable:group:')) {
      targetGroupId = overId.replace('droppable:group:', '');
    } else if (overId === 'droppable:ungrouped') {
      targetGroupId = null;
    } else if (overId.startsWith('group:')) {
      targetGroupId = overId.replace('group:', '');
    } else {
      targetGroupId = (overData?.groupId as string) ?? channelGroupLookup[overId] ?? null;
    }

    const targetList = targetGroupId
      ? sortChannels(groupedChannels[targetGroupId] ?? [])
      : sortedUngrouped;

    const isCrossGroup = sourceGroupId !== targetGroupId;

    let afterId: string | null;
    if (overId.startsWith('droppable:') || overId.startsWith('group:')) {
      afterId = targetList.length > 0 ? targetList[targetList.length - 1].id : null;
    } else {
      const overIndex = targetList.findIndex(ch => ch.id === overId);
      if (overIndex === -1) {
        afterId = targetList.length > 0 ? targetList[targetList.length - 1].id : null;
      } else if (isCrossGroup) {
        afterId = overIndex === 0 ? null : targetList[overIndex - 1].id;
      } else {
        const sourceList = sourceGroupId
          ? sortChannels(groupedChannels[sourceGroupId] ?? [])
          : sortedUngrouped;
        const oldIndex = sourceList.findIndex(ch => ch.id === activeId);
        const newIndex = overIndex;
        const reordered = arrayMove(sourceList, oldIndex, newIndex);
        const movedIdx = reordered.findIndex(ch => ch.id === activeId);
        afterId = movedIdx === 0 ? null : reordered[movedIdx - 1].id;
      }
    }

    const snapshot = [...channels];
    const updatedChannels = channels.map(ch => {
      if (ch.id === activeId) {
        return { ...ch, group_id: targetGroupId };
      }
      return ch;
    });
    dispatch({ type: "SET_CHANNELS", channels: updatedChannels });

    api.reorderChannel(activeId, afterId, targetGroupId).catch(() => {
      dispatch({ type: "SET_CHANNELS", channels: snapshot });
    });
  }, [channels, sortedUngrouped, groupedChannels, channelGroupLookup, dispatch]);

  return (
    <div className="channel-list" data-kind="channel">
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragEnd={handleDragEnd}
      >
        <UngroupedDroppable>
          <SortableContext items={ungroupedIds} strategy={verticalListSortingStrategy}>
            {sortedUngrouped.map(channel => (
              <SortableChannelItem
                key={channel.id}
                channel={channel}
                active={channel.id === currentChannelId}
                isOwner={currentUser ? channel.created_by === currentUser.id : false}
                onClick={() => onSelectChannel(channel.id)}
                groupId={null}
                pinned={isPinned(channel.id)}
                onContextMenu={(e) => {
                  e.preventDefault();
                  setPinMenu({ x: e.clientX, y: e.clientY, channelId: channel.id });
                }}
              />
            ))}
          </SortableContext>
        </UngroupedDroppable>

        <SortableContext items={groupSortableIds} strategy={verticalListSortingStrategy}>
          {sortedGroups.map(group => (
            <ChannelGroupComponent
              key={group.id}
              group={group}
              channels={groupedChannels[group.id] ?? []}
              collapsed={!!collapsedGroups[group.id]}
              onToggle={() => toggleGroup(group.id)}
              currentChannelId={currentChannelId}
              currentUser={currentUser}
              onSelectChannel={onSelectChannel}
            />
          ))}
        </SortableContext>
      </DndContext>

      {sortedPreviewChannels.length > 0 && (
        <>
          <div className="channel-group-label">公开频道</div>
          {sortedPreviewChannels.map(channel => (
            <ChannelItemStatic
              key={channel.id}
              channel={channel}
              active={channel.id === currentChannelId}
              onClick={() => onSelectChannel(channel.id)}
            />
          ))}
        </>
      )}

      {channels.length === 0 && (
        <div className="sidebar-empty">暂无频道</div>
      )}
      {/* CHN-3.3 right-click pin/unpin menu (DM rows are mounted via
          MergedDmList in Sidebar.tsx, not this list, so DM 反约束 自然
          满足 — 5 源 byte-identical: chn-3-content-lock §1 ⑤ + #366 ④ +
          #364 + #371 ② + #376 §3.4 + #382 ⑤). */}
      {pinMenu && (
        <ChannelContextMenu
          x={pinMenu.x}
          y={pinMenu.y}
          pinned={isPinned(pinMenu.channelId)}
          onPin={() => pinChannel(pinMenu.channelId)}
          onUnpin={() => unpinChannel(pinMenu.channelId)}
          onClose={() => setPinMenu(null)}
        />
      )}
    </div>
  );
}

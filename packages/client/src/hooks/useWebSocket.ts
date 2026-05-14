import { useEffect, useRef, useCallback } from 'react';
import { useAppContext } from '../context/AppContext';
import { useToast } from '../components/Toast';
import { getDevUserId, fetchMessages, fetchEventsBackfill } from '../lib/api';
import { loadLastSeenCursor, persistLastSeenCursor } from '../lib/lastSeenCursor';
import type { ConnectionState, Message, Channel, ChannelGroup, PendingMessage } from '../types';
import {
  dispatchInvitationPending,
  dispatchInvitationDecided,
  dispatchArtifactUpdated,
  dispatchMentionPushed,
  dispatchAnchorCommentAdded,
  dispatchIterationStateChanged,
  dispatchArtifactCommentAdded,
} from './useWsHubFrames';
import type {
  AgentInvitationPendingFrame,
  AgentInvitationDecidedFrame,
  ArtifactUpdatedFrame,
  MentionPushedFrame,
  AnchorCommentAddedFrame,
  IterationStateChangedFrame,
  ArtifactCommentAddedFrame,
} from '../types/ws-frames';
import { markPresence } from './usePresence';
import type { AgentRuntimeReason, AgentRuntimeState } from '../lib/api';

const PING_INTERVAL = 25_000;
const RECONNECT_DELAYS = [1000, 2000, 4000, 8000, 16000, 30000];
const AUTH_FAILURE_CODES = new Set([4001, 4003]);

/**
 * The server emits two WebSocket frame shapes, while client handlers read the
 * flattened shape:
 *
 *   1. `BroadcastEventToAll` (hub.go:284-289): `{type, data: payload}`
 *      where payload can be `{group: {...}}`; these payload fields are nested
 *      under `data`.
 *   2. Direct push frame (for example PushArtifactUpdated, see cursor_test.go:175):
 *      `{type, cursor, ...fields}`; these payload fields are already top-level.
 *
 * `handleMessage` reads top-level fields such as data.group / data.channel /
 * data.cursor. That matches the backfill path shape: `{type, ...ev.payload}`.
 *
 * This helper normalizes both shapes so handlers do not need to know whether
 * BroadcastEventToAll wrapped the payload. This fixed #678: `group_created`
 * used BroadcastEventToAll, so before flattening `data.group` was undefined.
 *
 * Exported so tests can validate the contract without starting a full
 * WebSocket mock.
 */
export function flattenWsFrame(frame: unknown): { type: string; [key: string]: unknown } {
  if (!frame || typeof frame !== 'object') return { type: '' };
  const f = frame as Record<string, unknown>;
  const type = typeof f.type === 'string' ? f.type : '';
  const { data: payload, ...rest } = f;
  const flatPayload = payload && typeof payload === 'object' ? (payload as Record<string, unknown>) : {};
  // Merge order: rest (including original type) → flatPayload (overriding
  // same-name payload fields) → explicit type. The final type write ensures
  // type is always a string from the top-level frame, even if payload also
  // contains type.
  return { ...rest, ...flatPayload, type };
}

export function useWebSocket() {
  const { state, dispatch } = useAppContext();
  const { showToast } = useToast();
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectAttempt = useRef(0);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout>>();
  const pingTimer = useRef<ReturnType<typeof setInterval>>();
  const subscribedChannels = useRef<Set<string>>(new Set());
  const mountedRef = useRef(true);
  const lastMessageTimestamp = useRef<Map<string, number>>(new Map());
  const scheduleReconnectRef = useRef<() => void>();
  const ackTimers = useRef<Map<string, () => void>>(new Map());
  const handleMessageRef = useRef<(data: { type: string; [key: string]: unknown }) => void>(() => {});
  const currentChannelIdRef = useRef(state.currentChannelId);
  currentChannelIdRef.current = state.currentChannelId;

  const findPendingChannelId = useCallback((clientMessageId: string): string | null => {
    for (const [channelId, pending] of state.pendingMessages) {
      if (pending.some(p => p.clientMessageId === clientMessageId)) return channelId;
    }
    return null;
  }, [state.pendingMessages]);

  const setConnectionState = useCallback((cs: ConnectionState) => {
    dispatch({ type: 'SET_CONNECTION_STATE', state: cs });
  }, [dispatch]);

  const reconcilePendingMessages = useCallback((channelId: string, fetchedMessages: Message[]) => {
    const pending = state.pendingMessages.get(channelId);
    if (!pending || pending.length === 0) return;
    const fetchedContents = new Set(fetchedMessages.map(m => `${m.sender_id}:${m.content}`));
    for (const p of pending) {
      if (fetchedContents.has(`${p.senderId}:${p.content}`)) {
        dispatch({ type: 'REMOVE_PENDING_MESSAGE', clientMessageId: p.clientMessageId, channelId });
        ackTimers.current.get(p.clientMessageId)?.();
        ackTimers.current.delete(p.clientMessageId);
      }
    }
  }, [state.pendingMessages, dispatch]);

  const reconcilePendingRef = useRef(reconcilePendingMessages);
  reconcilePendingRef.current = reconcilePendingMessages;

  const connect = useCallback(() => {
    if (wsRef.current) {
      const rs = wsRef.current.readyState;
      if (rs === WebSocket.OPEN || rs === WebSocket.CONNECTING) return;
      wsRef.current.onclose = null;
      wsRef.current.onerror = null;
      wsRef.current.close();
      wsRef.current = null;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const params = new URLSearchParams();
    const userId = getDevUserId();
    if (userId) params.set('user_id', userId);
    const url = `${protocol}//${host}/ws${params.toString() ? `?${params}` : ''}`;

    setConnectionState(reconnectAttempt.current > 0 ? 'reconnecting' : 'connecting');

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      if (!mountedRef.current) return;
      const wasReconnect = reconnectAttempt.current > 0;
      reconnectAttempt.current = 0;
      setConnectionState('connected');

      // Re-subscribe to all channels
      for (const channelId of subscribedChannels.current) {
        ws.send(JSON.stringify({ type: 'subscribe', channel_id: channelId }));
      }

      // RT-1.2 (#290 follow): on reconnect, ask the server for the
      // events the WS missed during the disconnect window via the
      // `?since=last_seen_cursor` backfill endpoint. Do this BEFORE
      // the per-channel fetchMessages fan-out so the cursor-based
      // dedup gate is primed (we won't double-render frames the WS
      // delivers right after backfill — handleMessage tracks the
      // running high-water mark and persistLastSeenCursor is
      // monotonic).
      //
      // RT-1 spec §1.2 constraint: we do NOT default to full history.
      // If `loadLastSeenCursor()` returns 0 (cold start), skip the
      // backfill — full reconciliation is the per-channel
      // fetchMessages path below. This is the line that distinguishes
      // RT-1.2 from RT-1.3 agent `session.resume` (the latter is
      // explicitly allowed to ask for `replay_mode=full`; humans
      // never default to it).
      if (wasReconnect) {
        const since = loadLastSeenCursor();
        if (since > 0) {
          fetchEventsBackfill(since)
            .then(({ cursor, events }) => {
              for (const ev of events) {
                handleMessageRef.current({
                  type: ev.kind,
                  ...((ev.payload ?? {}) as Record<string, unknown>),
                });
              }
              if (cursor > since) persistLastSeenCursor(cursor);
            })
            .catch((err: unknown) => console.warn('[ws] backfill failed:', err));
        }

        // Per-channel message reconcile is independent of the event
        // stream backfill (messages may not all flow through `events`
        // depending on kind filters), so keep both.
        for (const channelId of subscribedChannels.current) {
          const lastTs = lastMessageTimestamp.current.get(channelId);
          if (lastTs) {
            fetchMessages(channelId, { after: lastTs, limit: 50 })
              .then(({ messages }) => {
                for (const msg of messages) {
                  dispatch({ type: 'ADD_MESSAGE', channelId: msg.channel_id, message: msg });
                }
                reconcilePendingRef.current(channelId, messages);
              })
              .catch((err: unknown) => console.warn('[ws] Failed to fetch missed messages:', err));
          }
        }
      }

      // Start heartbeat
      if (pingTimer.current) clearInterval(pingTimer.current);
      pingTimer.current = setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'ping' }));
        }
      }, PING_INTERVAL);
    };

    ws.onmessage = (event) => {
      if (!mountedRef.current) return;
      try {
        const frame = JSON.parse(event.data);
        // RT-1.2 cursor tracking: any frame that carries a numeric
        // `cursor` (RT-1.1 ArtifactUpdatedFrame is the first; backfill
        // events forwarded through handleMessageRef also carry one)
        // bumps the persisted high-water mark. persistLastSeenCursor
        // is monotonic so out-of-order arrivals are a no-op. We do
        // this BEFORE the handler dispatch so reconnect-mid-handler
        // is still correct.
        if (typeof frame?.cursor === 'number') {
          persistLastSeenCursor(frame.cursor);
        }
        // Flatten the envelope (#680 fix); see the flattenWsFrame comment above.
        handleMessageRef.current(flattenWsFrame(frame));
      } catch {
        // Invalid JSON
      }
    };

    ws.onclose = (event) => {
      if (!mountedRef.current) return;
      cleanup();

      if (AUTH_FAILURE_CODES.has(event.code)) {
        setConnectionState('disconnected');
        console.warn('[ws] Auth failure (code %d), not reconnecting:', event.code, event.reason);
        return;
      }

      console.info('[ws] Closed (code %d), scheduling reconnect', event.code);
      scheduleReconnectRef.current?.();
    };

    ws.onerror = () => {
      // onclose will fire after onerror
    };
  }, [setConnectionState, dispatch]);

  const cleanup = useCallback(() => {
    if (pingTimer.current) {
      clearInterval(pingTimer.current);
      pingTimer.current = undefined;
    }
  }, []);

  const scheduleReconnect = useCallback(() => {
    if (!mountedRef.current) return;
    setConnectionState('reconnecting');
    const delay = RECONNECT_DELAYS[Math.min(reconnectAttempt.current, RECONNECT_DELAYS.length - 1)]!;
    reconnectAttempt.current++;
    reconnectTimer.current = setTimeout(() => {
      if (mountedRef.current) connect();
    }, delay);
  }, [connect, setConnectionState]);

  scheduleReconnectRef.current = scheduleReconnect;

  const handleMessage = useCallback((data: { type: string; [key: string]: unknown }) => {
    switch (data.type) {
      case 'new_message': {
        const message = data.message as Message;
        dispatch({ type: 'ADD_MESSAGE', channelId: message.channel_id, message });
        const prev = lastMessageTimestamp.current.get(message.channel_id) ?? 0;
        if (message.created_at > prev) {
          lastMessageTimestamp.current.set(message.channel_id, message.created_at);
        }
        break;
      }
      case 'presence': {
        const userId = data.user_id as string;
        const status = data.status as string;
        if (status === 'online') {
          dispatch({ type: 'USER_ONLINE', userId });
        } else {
          dispatch({ type: 'USER_OFFLINE', userId });
        }
        break;
      }
      // AL-3.3 (#R3 Phase 2) — agent runtime presence frame.
      // Field allowlist (al-3.md §2.5): {agent_id, status, reason?}. Do not
      // accept IP / last_heartbeat_at / connection_count; the server strips
      // them. 5s throttling is enforced on server egress and again at the
      // client markPresence entry point.
      case 'presence.changed': {
        const agentID = data.agent_id as string | undefined;
        const status = data.status as AgentRuntimeState | undefined;
        const reason = data.reason as AgentRuntimeReason | undefined;
        if (agentID && (status === 'online' || status === 'offline' || status === 'error')) {
          markPresence(agentID, status, reason);
        }
        break;
      }
      case 'channel_created': {
        const channel = data.channel as Channel | undefined;
        if (channel && channel.id) {
          dispatch({ type: 'ADD_CHANNEL', channel });
        }
        break;
      }
      case 'channel_added': {
        // CHN-1.3 hardening: server may send {channel} (preferred) or
        // {channel_id} (legacy fallback). Guard against undefined so the
        // reducer's c.id deref never crashes AppProvider.
        const channel = data.channel as Channel | undefined;
        if (channel && channel.id) {
          dispatch({ type: 'ADD_CHANNEL', channel });
        }
        break;
      }
      case 'channel_removed': {
        const removedChannelId = data.channel_id as string;
        dispatch({ type: 'REMOVE_CHANNEL', channelId: removedChannelId });
        subscribedChannels.current.delete(removedChannelId);
        if (wsRef.current?.readyState === WebSocket.OPEN) {
          wsRef.current.send(JSON.stringify({ type: 'unsubscribe', channel_id: removedChannelId }));
        }
        break;
      }
      case 'channel_deleted': {
        const deletedChannelId = data.channel_id as string;
        const deletedName = data.name as string | undefined;
        dispatch({ type: 'REMOVE_CHANNEL', channelId: deletedChannelId });
        subscribedChannels.current.delete(deletedChannelId);
        if (wsRef.current?.readyState === WebSocket.OPEN) {
          wsRef.current.send(JSON.stringify({ type: 'unsubscribe', channel_id: deletedChannelId }));
        }
        if (currentChannelIdRef.current === deletedChannelId) {
          showToast(deletedName ? `频道 #${deletedName} 已被删除` : '频道已被删除');
        }
        break;
      }
      case 'visibility_changed': {
        const chId = data.channel_id as string;
        const vis = data.visibility as 'public' | 'private';
        dispatch({ type: 'UPDATE_CHANNEL', channelId: chId, updates: { visibility: vis } });
        break;
      }
      case 'channel_updated': {
        const updatedChId = data.channel_id as string;
        const updates: Record<string, unknown> = {};
        if (typeof data.topic === 'string') updates.topic = data.topic;
        if (Object.keys(updates).length > 0) {
          dispatch({ type: 'UPDATE_CHANNEL', channelId: updatedChId, updates });
        }
        break;
      }
      case 'channels_reordered': {
        const reorderedChId = data.channel_id as string;
        const reorderUpdates: Partial<Channel> = {};
        if (typeof data.position === 'string') reorderUpdates.position = data.position;
        if (data.group_id !== undefined) reorderUpdates.group_id = data.group_id as string | null;
        dispatch({ type: 'UPDATE_CHANNEL', channelId: reorderedChId, updates: reorderUpdates });
        break;
      }
      case 'user_joined': {
        const joinedChannelId = data.channel_id as string;
        const joinedUserId = data.user_id as string;
        if (joinedUserId) {
          dispatch({ type: 'USER_ONLINE', userId: joinedUserId });
        }
        if (joinedChannelId) {
          if (typeof data.member_count === 'number') {
            dispatch({ type: 'UPDATE_CHANNEL', channelId: joinedChannelId, updates: { member_count: data.member_count as number } });
          }
          dispatch({ type: 'BUMP_CHANNEL_MEMBERS_VERSION', channelId: joinedChannelId });
        }
        break;
      }
      case 'user_left': {
        const leftChannelId = data.channel_id as string;
        if (leftChannelId) {
          if (typeof data.member_count === 'number') {
            dispatch({ type: 'UPDATE_CHANNEL', channelId: leftChannelId, updates: { member_count: data.member_count as number } });
          }
          dispatch({ type: 'BUMP_CHANNEL_MEMBERS_VERSION', channelId: leftChannelId });
        }
        break;
      }
      case 'pong':
        break;
      case 'typing': {
        const channelId = data.channel_id as string;
        const userId = data.user_id as string;
        const displayName = data.display_name as string;
        dispatch({ type: 'SET_TYPING', channelId, userId, displayName });
        break;
      }
      case 'reaction_update': {
        dispatch({
          type: 'UPDATE_REACTIONS',
          messageId: data.message_id as string,
          channelId: data.channel_id as string,
          reactions: data.reactions as { emoji: string; count: number; user_ids: string[] }[],
        });
        break;
      }
      case 'message_ack': {
        const clientMessageId = data.client_message_id as string | null;
        const serverMessage = data.message as Message;
        if (clientMessageId) {
          dispatch({
            type: 'ACK_PENDING_MESSAGE',
            clientMessageId,
            channelId: serverMessage.channel_id,
            serverMessage,
          });
          ackTimers.current.get(clientMessageId)?.();
          ackTimers.current.delete(clientMessageId);
        }
        break;
      }
      case 'message_nack': {
        const nackClientId = data.client_message_id as string | null;
        if (nackClientId) {
          const channelId = findPendingChannelId(nackClientId);
          if (channelId) {
            dispatch({ type: 'FAIL_PENDING_MESSAGE', clientMessageId: nackClientId, channelId });
          }
          ackTimers.current.get(nackClientId)?.();
          ackTimers.current.delete(nackClientId);
        }
        break;
      }
      case 'message_sent':
        break;
      case 'message_edited': {
        const editedMsg = data.message as Message;
        dispatch({
          type: 'EDIT_MESSAGE',
          channelId: editedMsg.channel_id,
          messageId: editedMsg.id,
          content: editedMsg.content,
          editedAt: editedMsg.edited_at!,
        });
        break;
      }
      case 'message_deleted': {
        const deletedChannelId = data.channel_id as string;
        const deletedMessageId = data.message_id as string;
        const deletedAt = data.deleted_at as number;
        dispatch({
          type: 'DELETE_MESSAGE',
          channelId: deletedChannelId,
          messageId: deletedMessageId,
          deletedAt,
        });
        break;
      }
      case 'group_created': {
        const group = data.group as ChannelGroup;
        dispatch({ type: 'ADD_GROUP', group });
        break;
      }
      case 'group_updated': {
        const group = data.group as ChannelGroup;
        dispatch({ type: 'UPDATE_GROUP', groupId: group.id, updates: group });
        break;
      }
      case 'group_reordered': {
        const groupId = data.group_id as string;
        const position = data.position as string;
        dispatch({ type: 'UPDATE_GROUP', groupId, updates: { position } });
        break;
      }
      case 'channel_groups_reordered': {
        const groupId = data.group_id as string;
        const position = data.position as string;
        dispatch({ type: 'UPDATE_GROUP', groupId, updates: { position } });
        break;
      }
      case 'group_deleted': {
        const groupId = data.group_id as string;
        const ungroupedChannelIds = (data.ungrouped_channel_ids ?? []) as string[];
        dispatch({ type: 'REMOVE_GROUP', groupId, ungroupedChannelIds });
        break;
      }
      case 'commands_updated': {
        window.dispatchEvent(new CustomEvent('commands_updated'));
        break;
      }
      case 'agent_invitation_pending': {
        // RT-0 (#40): owner-side push — replaces the 60s bell-badge
        // poll. Bridge to a window CustomEvent so InvitationsInbox /
        // Sidebar can subscribe without coupling to this hook.
        // Schema lock: docs/blueprint/current/realtime.md §2.3 (exact BPP schema match).
        dispatchInvitationPending(data as unknown as AgentInvitationPendingFrame);
        break;
      }
      case 'agent_invitation_decided': {
        // RT-0 (#40): cross-client sync of approve/reject/expire.
        dispatchInvitationDecided(data as unknown as AgentInvitationDecidedFrame);
        break;
      }
      case 'artifact_updated': {
        // CV-1.2 (#342): server → client signal that an artifact's head
        // moved (commit or rollback). Envelope is signal-only (rule ⑤),
        // ArtifactPanel re-fetches GET /artifacts/:id for body+committer.
        // Schema lock: docs/blueprint/current/realtime.md §2.3 + cursor.go::ArtifactUpdatedFrame
        // (BPP-1 #304 envelope CI lint enforces the exact wire shape).
        dispatchArtifactUpdated(data as unknown as ArtifactUpdatedFrame);
        break;
      }
      case 'mention_pushed': {
        // DM-2.2 (#372): server → client signal that the current user has
        // been @-mentioned in a message. Envelope is signal-only (rule ②);
        // MessageList listens via useMentionPushed and refreshes the
        // channel via actions.loadMessages. body_preview is server-truncated
        // to 80 runes; the client must not re-parse it (privacy §13 constraint,
        // full body arrives via the existing message backfill path).
        // Schema lock: docs/implementation/modules/dm-2.3-spec.md §0 rule
        // ② + mention_pushed_frame.go::MentionPushedFrame (BPP-1 #304
        // envelope CI lint enforces the exact wire shape).
        dispatchMentionPushed(data as unknown as MentionPushedFrame);
        break;
      }
      case 'anchor_comment_added': {
        // CV-2.2 (#360): server → client signal that a comment landed
        // on an anchor thread. Envelope is signal-only, matching the
        // rule ③ / ⑤ no-body pattern; AnchorThreadPanel listens via
        // useAnchorCommentAdded and refetches anchors via
        // GET /api/v1/artifacts/:id/anchors + GET /anchors/:id/comments.
        // Schema lock: docs/implementation/modules/cv-2-spec.md §0 rule ③
        // + anchor_comment_frame.go::AnchorCommentAddedFrame (10-field
        // schema match, BPP-1 #304 envelope CI lint).
        dispatchAnchorCommentAdded(data as unknown as AnchorCommentAddedFrame);
        break;
      }
      case 'iteration_state_changed': {
        // CV-4.2 (#409): server → client signal that an artifact_iterations
        // row transitioned state (pending → running → completed|failed).
        // Envelope is signal-only (rules ② + ⑦ — frame has no intent_text);
        // IteratePanel listens via useIterationStateChanged and refetches
        // GET /artifacts/:id/iterations/:iid for full body + intent_text.
        // Schema lock: docs/implementation/modules/cv-4-spec.md §1 CV-4.2
        // + iteration_state_frame.go::IterationStateChangedFrame (9 字段
        // schema match, BPP-1 #304 envelope CI lint).
        dispatchIterationStateChanged(data as unknown as IterationStateChangedFrame);
        break;
      }
      case 'artifact_comment_added': {
        // CV-5: server → client signal that a comment landed on an
        // artifact's `artifact:<id>` namespace channel. Envelope is
        // signal-only, matching rule ② — body_preview is server-truncated
        // to 80 runes (隐私 §13); ArtifactComments listens via
        // useArtifactCommentAdded and refetches GET
        // /api/v1/artifacts/:id/comments for full body.
        // Schema lock: docs/implementation/modules/cv-5-spec.md §0 rule ②
        // + artifact_comment_added_frame.go::ArtifactCommentAddedFrame
        // (9-field schema match, aligned with RT-3 cursor ordering).
        dispatchArtifactCommentAdded(data as unknown as ArtifactCommentAddedFrame);
        break;
      }
      case 'error':
        console.warn('[ws] Server error:', data.message);
        break;
    }
  }, [dispatch, showToast, findPendingChannelId]);

  handleMessageRef.current = handleMessage;

  const subscribe = useCallback((channelId: string) => {
    if (subscribedChannels.current.has(channelId)) return;
    subscribedChannels.current.add(channelId);
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'subscribe', channel_id: channelId }));
    }
  }, []);

  const unsubscribe = useCallback((channelId: string) => {
    subscribedChannels.current.delete(channelId);
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'unsubscribe', channel_id: channelId }));
    }
  }, []);

  // Connect on mount
  useEffect(() => {
    mountedRef.current = true;
    connect();

    return () => {
      mountedRef.current = false;
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
      cleanup();
      if (wsRef.current) {
        wsRef.current.onclose = null;
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [connect, cleanup]);

  // Auto-subscribe to DM channels
  useEffect(() => {
    for (const dm of state.dmChannels) {
      if (!subscribedChannels.current.has(dm.id)) {
        subscribe(dm.id);
      }
    }
  }, [state.dmChannels, subscribe]);

  const sendWsMessage = useCallback((payload: Record<string, unknown>) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(payload));
    }
  }, []);

  const registerAckTimer = useCallback((clientMessageId: string, cancel: () => void) => {
    ackTimers.current.set(clientMessageId, cancel);
  }, []);

  return {
    subscribe,
    unsubscribe,
    sendWsMessage,
    registerAckTimer,
    connectionState: state.connectionState,
  };
}

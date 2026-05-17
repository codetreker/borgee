// ─── REST API client ─────────────────────────────────────

import type { Channel, ChannelGroup, Message, User, DmChannel, WorkspaceFile } from '../types';
import type { NotifPref } from './notif_pref';

const BASE = ''; // Same origin via Vite proxy in dev, or same server in prod

let currentUserId: string | null = null;

export function setDevUserId(userId: string | null): void {
  currentUserId = userId;
}

export function getDevUserId(): string | null {
  return currentUserId;
}

async function request<T>(url: string, opts: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    ...((opts.headers as Record<string, string>) ?? {}),
  };

  if (import.meta.env.DEV && currentUserId) {
    headers['X-Dev-User-Id'] = currentUserId;
  }

  // Don't set Content-Type for FormData (browser sets boundary automatically)
  // Only set JSON content-type when there's actually a body to send
  if (opts.body && !(opts.body instanceof FormData) && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json';
  }

  const res = await fetch(`${BASE}${url}`, {
    ...opts,
    headers,
    credentials: 'include',
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new ApiError(res.status, body.error ?? 'Request failed');
  }

  return res.json() as Promise<T>;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

// ─── Auth ──────────────────────────────────────────────

export async function login(email: string, password: string): Promise<User> {
  const data = await request<{ user: User }>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
  return data.user;
}

export async function logout(): Promise<void> {
  await request<{ ok: boolean }>('/api/v1/auth/logout', {
    method: 'POST',
  });
}

// ─── Channels ───────────────────────────────────────────

export async function fetchChannels(
  q?: string,
): Promise<{ channels: Channel[]; groups: ChannelGroup[] }> {
  // CHN-13 design ②: optional `q` substring filter. Empty q
  // (undefined / "") keeps the existing `/api/v1/channels` URL
  // byte-identical so grep checks do not see a stray `?q=`. Special
  // characters are encoded before they reach the server's prepared LIKE query.
  const url = q ? `/api/v1/channels?q=${encodeURIComponent(q)}` : '/api/v1/channels';
  return request<{ channels: Channel[]; groups: ChannelGroup[] }>(url);
}

export async function createChannel(
  name: string,
  topic?: string,
  memberIds?: string[],
  visibility?: 'public' | 'private',
): Promise<Channel> {
  const data = await request<{ channel: Channel }>('/api/v1/channels', {
    method: 'POST',
    body: JSON.stringify({ name, topic, member_ids: memberIds, visibility }),
  });
  return data.channel;
}

export async function updateChannel(
  channelId: string,
  updates: {
    name?: string;
    topic?: string;
    visibility?: 'public' | 'private' | 'creator_only';
    archived?: boolean;
  },
): Promise<Channel> {
  if (
    updates.topic !== undefined &&
    !updates.name &&
    !updates.visibility &&
    updates.archived === undefined
  ) {
    const data = await request<{ channel: Channel }>(`/api/v1/channels/${channelId}/topic`, {
      method: 'PUT',
      body: JSON.stringify({ topic: updates.topic }),
    });
    return data.channel;
  }
  const data = await request<{ channel: Channel }>(`/api/v1/channels/${channelId}`, {
    method: 'PUT',
    body: JSON.stringify(updates),
  });
  return data.channel;
}

// CHN-1.2 设计 ⑤: archive flip is server-stamped. Pass `true` to retire,
// `false` to un-archive. Server emits a system DM to channel members on the
// `false → true` transition so that everyone observes the closure.
export async function archiveChannel(channelId: string, archived: boolean): Promise<Channel> {
  return updateChannel(channelId, { archived });
}

// CHN-7: mute / unmute a channel for the current user. User API only;
// no admin bypass route (ADM-0 §1.3 separation constraint + design ②). Server toggles bit 1
// of user_channel_layout.collapsed and preserves bit 0 for CHN-3 collapse.
export async function muteChannel(
  channelId: string,
  muted: boolean,
): Promise<{ collapsed: number; muted: boolean }> {
  const method = muted ? 'POST' : 'DELETE';
  return request<{ collapsed: number; muted: boolean }>(`/api/v1/channels/${channelId}/mute`, {
    method,
  });
}

// CHN-9: set channel visibility (one of 'creator_only' / 'private' /
// 'public'). User API only, owner-only via the existing
// channel.manage_visibility permission. Keep the CHN-1.2 ACL byte-identical;
// there is no admin bypass route (ADM-0 §1.3 separation constraint). Reuse updateChannel.
export async function setChannelVisibility(
  channelId: string,
  visibility: import('./visibility').ChannelVisibility,
): Promise<Channel> {
  return updateChannel(channelId, { visibility });
}

// CHN-5: list the user's archived channels (owner-only — server filters
// to the caller's membership). Single-source API client; components
// must NOT inline fetch this path (grep 检查守, content-lock §3 设计 ⑤).
export async function listArchivedChannels(): Promise<Channel[]> {
  const data = await request<{ channels: Channel[] }>('/api/v1/me/archived-channels');
  return data.channels;
}

// CHN-6: pin / unpin a channel for the current user. User API only;
// no admin bypass route (ADM-0 §1.3 separation constraint + design ②). Server stamps
// position = -(nowMs) on pin and max(positive)+1.0 on unpin, complementing
// the CHN-3.3 MIN-1.0 monotonic decimal ordering model.
export async function pinChannel(
  channelId: string,
  pinned: boolean,
): Promise<{ position: number; pinned: boolean }> {
  const method = pinned ? 'POST' : 'DELETE';
  return request<{ position: number; pinned: boolean }>(`/api/v1/channels/${channelId}/pin`, {
    method,
  });
}

// CHN-8: set notification preference for a channel. User API only;
// no admin bypass route (ADM-0 §1.3 separation constraint + design ②). pref ∈ {'all', 'mention', 'none'}.
export async function setNotificationPref(
  channelId: string,
  pref: NotifPref,
): Promise<{ collapsed: number; pref: NotifPref }> {
  return request<{ collapsed: number; pref: NotifPref }>(
    `/api/v1/channels/${channelId}/notification-pref`,
    {
      method: 'PUT',
      body: JSON.stringify({ pref }),
    },
  );
}

export async function joinChannel(channelId: string): Promise<void> {
  await request<{ ok: boolean }>(`/api/v1/channels/${channelId}/join`, {
    method: 'POST',
  });
}

export async function fetchChannelPreview(
  channelId: string,
): Promise<{ messages: Message[]; channel: Channel }> {
  return request<{ messages: Message[]; channel: Channel }>(
    `/api/v1/channels/${channelId}/preview`,
  );
}

export async function leaveChannel(channelId: string): Promise<void> {
  await request<{ ok: boolean }>(`/api/v1/channels/${channelId}/leave`, {
    method: 'POST',
  });
}

export async function deleteChannel(channelId: string): Promise<void> {
  await request<{ ok: boolean }>(`/api/v1/channels/${channelId}`, {
    method: 'DELETE',
  });
}

export async function reorderChannel(
  channelId: string,
  afterId: string | null,
  groupId?: string | null,
): Promise<void> {
  const body: Record<string, unknown> = { channel_id: channelId, after_id: afterId };
  if (groupId !== undefined) body.group_id = groupId;
  await request<{ ok: boolean }>(`/api/v1/channels/reorder`, {
    method: 'PUT',
    body: JSON.stringify(body),
  });
}

// ─── Channel Groups ────────────────────────────────────

export async function createChannelGroup(name: string): Promise<ChannelGroup> {
  const data = await request<{ group: ChannelGroup }>('/api/v1/channel-groups', {
    method: 'POST',
    body: JSON.stringify({ name }),
  });
  return data.group;
}

export async function updateChannelGroup(groupId: string, name: string): Promise<ChannelGroup> {
  const data = await request<{ group: ChannelGroup }>(`/api/v1/channel-groups/${groupId}`, {
    method: 'PUT',
    body: JSON.stringify({ name }),
  });
  return data.group;
}

export async function deleteChannelGroup(groupId: string): Promise<void> {
  await request<{ ok: boolean }>(`/api/v1/channel-groups/${groupId}`, {
    method: 'DELETE',
  });
}

export async function reorderChannelGroup(groupId: string, afterId: string | null): Promise<void> {
  await request<{ ok: boolean }>('/api/v1/channel-groups/reorder', {
    method: 'PUT',
    body: JSON.stringify({ group_id: groupId, after_id: afterId }),
  });
}

// ─── Channel Members ────────────────────────────────────

export type RequireMentionPolicy = 'inherit' | 'on' | 'off';

export interface ChannelMember {
  user_id: string;
  display_name: string;
  role: string;
  avatar_url?: string | null;
  joined_at: number;
  // CHN-1.2 设计 ⑥ (concept-model §1.4): when true, this member does not
  // auto-broadcast on lifecycle events. Default false for humans; backfilled
  // to true for agents — UI renders a "silent" badge so peers know the agent
  // listens but won't chime in unprompted.
  silent?: boolean;
  require_mention_policy?: RequireMentionPolicy;
  effective_require_mention?: boolean;
}

export interface ChannelMemberRequireMentionState {
  channel_id: string;
  user_id: string;
  require_mention_policy: RequireMentionPolicy;
  effective_require_mention: boolean;
}

export async function fetchChannelMembers(channelId: string): Promise<ChannelMember[]> {
  const data = await request<{ members: ChannelMember[] }>(`/api/v1/channels/${channelId}/members`);
  return data.members;
}

export async function addChannelMember(channelId: string, userId: string): Promise<void> {
  await request<{ ok: boolean }>(`/api/v1/channels/${channelId}/members`, {
    method: 'POST',
    body: JSON.stringify({ user_id: userId }),
  });
}

export async function removeChannelMember(channelId: string, userId: string): Promise<void> {
  await request<{ ok: boolean }>(`/api/v1/channels/${channelId}/members/${userId}`, {
    method: 'DELETE',
  });
}

export async function setChannelMemberRequireMentionPolicy(
  channelId: string,
  userId: string,
  policy: RequireMentionPolicy,
): Promise<ChannelMemberRequireMentionState> {
  return request<ChannelMemberRequireMentionState>(
    `/api/v1/channels/${channelId}/members/${userId}/require-mention`,
    {
      method: 'PUT',
      body: JSON.stringify({ policy }),
    },
  );
}

// ─── Messages ───────────────────────────────────────────

export async function fetchMessages(
  channelId: string,
  opts?: { before?: number; after?: number; limit?: number },
): Promise<{ messages: Message[]; has_more: boolean }> {
  const params = new URLSearchParams();
  if (opts?.before) params.set('before', String(opts.before));
  if (opts?.after) params.set('after', String(opts.after));
  if (opts?.limit) params.set('limit', String(opts.limit));
  const qs = params.toString();
  return request<{ messages: Message[]; has_more: boolean }>(
    `/api/v1/channels/${channelId}/messages${qs ? `?${qs}` : ''}`,
  );
}

// RT-1.2 (#290 follow): backfill the events the WS missed during a
// disconnect window. Server contract (server-go internal/api/poll.go
// handleEventsBackfill): returns ONLY events with `cursor > since`,
// scoped to the user's channel membership, in cursor-ASC order. The
// reverse constraint (RT-1 spec §1.2) is "do NOT default to full history" —
// callers MUST pass an explicit `since` (= last_seen_cursor); the
// server treats `since=0` as "give me everything you have for this
// user from cursor 1" but the client's own gating (only call after a
// dropped WS reconnect, only with a stored cursor) keeps the load
// bounded. `limit` defaults server-side to 200, max 500.
export interface BackfillEvent {
  cursor: number;
  kind: string;
  channel_id: string;
  payload: unknown;
  created_at: number;
}

export async function fetchEventsBackfill(
  since: number,
  opts?: { limit?: number },
): Promise<{ cursor: number; events: BackfillEvent[] }> {
  const params = new URLSearchParams();
  params.set('since', String(since));
  if (opts?.limit) params.set('limit', String(opts.limit));
  return request<{ cursor: number; events: BackfillEvent[] }>(
    `/api/v1/events?${params.toString()}`,
  );
}

export async function sendMessage(
  channelId: string,
  content: string,
  contentType: 'text' | 'image' = 'text',
  _mentions?: string[],
): Promise<Message> {
  const data = await request<{ message: Message }>(`/api/v1/channels/${channelId}/messages`, {
    method: 'POST',
    body: JSON.stringify({ content, content_type: contentType }),
  });
  return data.message;
}

export async function editMessage(messageId: string, content: string): Promise<Message> {
  const data = await request<{ message: Message }>(`/api/v1/messages/${messageId}`, {
    method: 'PUT',
    body: JSON.stringify({ content }),
  });
  return data.message;
}

export async function deleteMessage(messageId: string): Promise<void> {
  const headers: Record<string, string> = {};
  if (import.meta.env.DEV && currentUserId) {
    headers['X-Dev-User-Id'] = currentUserId;
  }
  const res = await fetch(`${BASE}/api/v1/messages/${messageId}`, {
    method: 'DELETE',
    headers,
    credentials: 'include',
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new ApiError(res.status, body.error ?? 'Request failed');
  }
}

// ─── Users ──────────────────────────────────────────────

export async function fetchMe(): Promise<User> {
  const data = await request<{ user: User }>('/api/v1/users/me');
  return data.user;
}

export async function getChannel(channelId: string): Promise<{ channel: Channel }> {
  return request<{ channel: Channel }>(`/api/v1/channels/${channelId}`);
}

export async function updateProfile(data: { display_name?: string }): Promise<User> {
  const res = await request<{ user: User }>('/api/v1/users/me', {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
  return res.user;
}

// ─── Online ─────────────────────────────────────────────

export async function fetchOnlineUsers(): Promise<string[]> {
  const data = await request<{ user_ids: string[] }>('/api/v1/online');
  return data.user_ids;
}

// ─── Upload ─────────────────────────────────────────────

export async function uploadImage(file: File): Promise<{ url: string; content_type: string }> {
  const form = new FormData();
  form.append('file', file);
  return request<{ url: string; content_type: string }>('/api/v1/upload', {
    method: 'POST',
    body: form,
  });
}

// ─── Channel read ───────────────────────────────────────

export async function markChannelRead(channelId: string): Promise<void> {
  await request<{ ok: boolean }>(`/api/v1/channels/${channelId}/read`, {
    method: 'PUT',
  });
}

// ─── DM ─────────────────────────────────────────────────

export async function createOrGetDm(userId: string): Promise<DmChannel> {
  const data = await request<{
    channel: DmChannel['id'] extends string
      ? { id: string; name: string; type: 'dm'; created_at: number; created_by: string }
      : never;
    peer: DmChannel['peer'];
  }>(`/api/v1/dm/${userId}`, { method: 'POST' });
  return {
    id: data.channel.id,
    name: data.channel.name,
    type: 'dm',
    created_at: data.channel.created_at,
    peer: data.peer,
    unread_count: 0,
    last_message: null,
  };
}

export async function fetchDmChannels(): Promise<DmChannel[]> {
  const data = await request<{ channels: DmChannel[] }>('/api/v1/dm');
  return data.channels;
}

// ─── Permissions ────────────────────────────────────────

export interface PermissionDetail {
  id: number;
  permission: string;
  scope: string;
  granted_by: string | null;
  granted_at: number;
}

export interface MyPermissionsResponse {
  user_id: string;
  role: string;
  permissions: string[];
  details: PermissionDetail[];
}

export async function fetchMyPermissions(): Promise<MyPermissionsResponse> {
  return request<MyPermissionsResponse>('/api/v1/me/permissions');
}

// ─── Auth Register ─────────────────────────────────────

export async function register(
  inviteCode: string,
  email: string,
  password: string,
  displayName: string,
): Promise<User> {
  const data = await request<{ user: User }>('/api/v1/auth/register', {
    method: 'POST',
    body: JSON.stringify({ invite_code: inviteCode, email, password, display_name: displayName }),
  });
  return data.user;
}

// ─── Reactions ─────────────────────────────────────────

export async function addReaction(messageId: string, emoji: string): Promise<void> {
  await request<{ ok: boolean }>(`/api/v1/messages/${messageId}/reactions`, {
    method: 'PUT',
    body: JSON.stringify({ emoji }),
  });
}

export async function removeReaction(messageId: string, emoji: string): Promise<void> {
  await request<{ ok: boolean }>(`/api/v1/messages/${messageId}/reactions`, {
    method: 'DELETE',
    body: JSON.stringify({ emoji }),
  });
}

// DM-5: aggregated reaction summary (CV-7 既有 GET endpoint, 0 server code).
// Response shape mirrors server `store.AggregatedReaction`.
export interface AggregatedReaction {
  emoji: string;
  count: number;
  user_ids: string[];
}

export async function getMessageReactions(
  messageId: string,
): Promise<{ reactions: AggregatedReaction[] }> {
  return request<{ reactions: AggregatedReaction[] }>(`/api/v1/messages/${messageId}/reactions`);
}

// ─── Agents ────────────────────────────────────────────

// AL-1a (#R3 Phase 2) — runtime tri-state plus failure reason code.
// Locked labels live in packages/client/src/lib/agent-state.ts (#190 §11).
// The server always emits state (online/offline/error); reason is present
// only for the error state.
// AL-1b (#R3 Phase 4) 扩 'busy' / 'idle' — server GET /agents/:id/status 5-state
// 合并优先级 (error > busy > idle > online > offline), 见 al-1b-spec.md §1.
export type AgentRuntimeState = 'online' | 'offline' | 'error' | 'busy' | 'idle';
export type AgentRuntimeReason =
  | 'api_key_invalid'
  | 'quota_exceeded'
  | 'network_unreachable'
  | 'runtime_crashed'
  | 'runtime_timeout'
  | 'unknown';

export interface Agent {
  id: string;
  display_name: string;
  role: string;
  avatar_url: string | null;
  owner_id: string | null;
  created_at: number;
  api_key?: string;
  disabled?: number;
  state?: AgentRuntimeState;
  reason?: AgentRuntimeReason;
  state_updated_at?: number;
  // AL-1b (#R3 Phase 4) — busy/idle 态时 server 填的 task 元数据.
  last_task_id?: string;
  last_task_started_at?: number;
  last_task_finished_at?: number;
}

// AL-4 (#313 v0 / #379 v2) — agent_runtimes registry. Schema source:
// PR #398 migration v=16 — `agent_runtimes(id PK,
// agent_id NOT NULL UNIQUE, endpoint_url, process_kind, status,
// last_error_reason, last_heartbeat_at, created_at, updated_at)`.
// Server API: AL-4.2 — POST /agents/:id/runtime/start
// + /stop + GET surfaces a single row per agent (蓝图 §2.2 v1 边界
// "不优化多 runtime 并行" + UNIQUE(agent_id) 字面).
//
// AL-4.3 client UI consumes this shape; runtime-not-yet-registered
// agents → server 404 → UI hides the start/stop card surface entirely
// (graceful degradation, design ① "Borgee 不带 runtime": unregistered
// agents should not pretend to have a runtime).

// AgentRuntimeStatus is the four-state process-level enum (v=16 schema
// CHECK). Constraint (#321 §2): v0 does not expose 'starting' / 'stopping' /
// 'restarting' intermediate states; start/stop use the synchronous API and
// directly UPDATE status.
export type AgentRuntimeStatus = 'registered' | 'running' | 'stopped' | 'error';

// AgentRuntimeProcessKind — v1 仅 'openclaw' (蓝图 §2.2 边界字面),
// 'hermes' 占号 v2+ (CHECK 已含, schema 早就支持新值不需 v2 改 CHECK).
export type AgentRuntimeProcessKind = 'openclaw' | 'hermes';

export interface AgentRuntime {
  id: string;
  agent_id: string;
  endpoint_url: string;
  process_kind: AgentRuntimeProcessKind;
  status: AgentRuntimeStatus;
  last_error_reason: AgentRuntimeReason | null;
  last_heartbeat_at: number | null;
  created_at: number;
  updated_at: number;
}

// fetchAgentRuntime returns the runtime row for the agent, or null if
// no runtime is registered (server 404). Design ① "Borgee 不带 runtime":
// unregistered agents should not pretend to have a runtime, so the UI omits
// the card.
//
// Returns null on 404 specifically (not other errors) so the UI can
// distinguish "no runtime registered yet" (show Register CTA stub) from
// transient network failure (show retry).
export async function fetchAgentRuntime(agentId: string): Promise<AgentRuntime | null> {
  try {
    const data = await request<{ runtime: AgentRuntime }>(`/api/v1/agents/${agentId}/runtime`);
    return data.runtime;
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) return null;
    throw err;
  }
}

// startAgentRuntime / stopAgentRuntime — owner-only writes
// (RequirePermission 'agent.runtime.control', AL-4.2 server). Non-owner
// → 403; admin → 401 (ADM-0 §1.4 no admin bypass writes rule). Both endpoints
// return the updated runtime row (#379 v2 §1 拆段 AL-4.2).
export async function startAgentRuntime(agentId: string): Promise<AgentRuntime> {
  const data = await request<{ runtime: AgentRuntime }>(`/api/v1/agents/${agentId}/runtime/start`, {
    method: 'POST',
  });
  return data.runtime;
}

export async function stopAgentRuntime(agentId: string): Promise<AgentRuntime> {
  const data = await request<{ runtime: AgentRuntime }>(`/api/v1/agents/${agentId}/runtime/stop`, {
    method: 'POST',
  });
  return data.runtime;
}

export async function fetchAgents(): Promise<Agent[]> {
  const data = await request<{ agents: Agent[] }>('/api/v1/agents');
  return data.agents;
}

export async function createAgent(
  displayName: string,
  permissions?: string[],
  id?: string,
): Promise<Agent> {
  const data = await request<{ agent: Agent }>('/api/v1/agents', {
    method: 'POST',
    body: JSON.stringify({ display_name: displayName, permissions, ...(id ? { id } : {}) }),
  });
  return data.agent;
}

export async function fetchAgent(id: string): Promise<Agent> {
  const data = await request<{ agent: Agent }>(`/api/v1/agents/${id}`);
  return data.agent;
}

export async function deleteAgent(id: string): Promise<void> {
  await request<{ ok: boolean }>(`/api/v1/agents/${id}`, {
    method: 'DELETE',
  });
}

export async function rotateAgentApiKey(id: string): Promise<string> {
  const data = await request<{ api_key: string }>(`/api/v1/agents/${id}/rotate-api-key`, {
    method: 'POST',
  });
  return data.api_key;
}

export async function fetchAgentPermissions(
  id: string,
): Promise<{ permissions: string[]; details: PermissionDetail[] }> {
  return request<{ permissions: string[]; details: PermissionDetail[] }>(
    `/api/v1/agents/${id}/permissions`,
  );
}

export async function updateAgentPermissions(
  id: string,
  permissions: { permission: string; scope?: string }[],
): Promise<void> {
  await request<{ agent_id: string }>(`/api/v1/agents/${id}/permissions`, {
    method: 'PUT',
    body: JSON.stringify({ permissions }),
  });
}

// AL-2a.2 agent_configs SSOT API (#264 acceptance §4.1.a-d).
// Blueprint: agent-lifecycle.md §2.1 + plugin-protocol.md §1.4 (Borgee=SSOT
// 字段划界, blob 仅含 Borgee 管字段) + §1.5 (热更新分级 — AL-2a 走轮询
// reload, BPP frame agent_config_update 留 AL-2b + BPP-3 同合).
export interface AgentConfig {
  schema_version: number;
  blob: AgentConfigBlob;
  updated_at?: number;
}

// allowedConfigKeys whitelist 跟 server-go internal/api/agent_config.go
// 同源 byte-identical (蓝图 §1.4 SSOT 字段划界).
export interface AgentConfigBlob {
  name?: string;
  avatar?: string;
  prompt?: string;
  model?: string;
  capabilities?: string[];
  enabled?: boolean;
  memory_ref?: string;
}

export async function fetchAgentConfig(id: string): Promise<AgentConfig> {
  return request<AgentConfig>(`/api/v1/agents/${id}/config`);
}

// PATCH atomic blob 整体替换 + schema_version 严格递增 (server-stamp).
// Failure surface (跟 server-go agent_config.go 同源):
//   - 400 agent_config.invalid_payload (空 body / 非 JSON / blob 缺)
//   - 400 agent_config.runtime_field_rejected (runtime-only field, fail-closed)
//   - 403 (cross-owner)
//   - 500 with msg "agent 配置保存失败, 请重试" byte-identical
export async function updateAgentConfig(id: string, blob: AgentConfigBlob): Promise<AgentConfig> {
  return request<AgentConfig>(`/api/v1/agents/${id}/config`, {
    method: 'PATCH',
    body: JSON.stringify({ blob }),
  });
}

export async function addAgentToChannel(channelId: string, agentId: string): Promise<void> {
  await request<{ ok: boolean }>(`/api/v1/channels/${channelId}/members`, {
    method: 'POST',
    body: JSON.stringify({ user_id: agentId }),
  });
}

// ─── Agent Files ──────────────────────────────────────────

export interface AgentFileResponse {
  content: string;
  size: number;
  mime_type: string;
  error?: undefined;
}

export async function getAgentFile(agentId: string, path: string): Promise<AgentFileResponse> {
  return request<AgentFileResponse>(
    `/api/v1/agents/${agentId}/files?path=${encodeURIComponent(path)}`,
  );
}

// ─── Agent invitations (CM-4.2) ───────────────────────
// Mirrors the hand-built sanitizer on the server (CM-4.1, see
// docs/current/server/data-model.md §Agent invitations API). `decided_at`
// and `expires_at` are omitted by the server when nil — declared optional
// here so existing pending rows decode cleanly.
export type AgentInvitationState = 'pending' | 'approved' | 'rejected' | 'expired';

export interface AgentInvitation {
  id: string;
  channel_id: string;
  agent_id: string;
  requested_by: string;
  state: AgentInvitationState;
  created_at: number;
  decided_at?: number;
  expires_at?: number;
  // Bug-029 P0: server JOIN-resolved labels. Empty string when the lookup
  // misses (agent/channel/user removed). Client falls back to raw ID.
  agent_name?: string;
  channel_name?: string;
  requester_name?: string;
}

export type AgentInvitationListRole = 'owner' | 'requester';

export async function createAgentInvitation(
  channelId: string,
  agentId: string,
  expiresAt?: number,
): Promise<AgentInvitation> {
  const data = await request<{ invitation: AgentInvitation }>('/api/v1/agent_invitations', {
    method: 'POST',
    body: JSON.stringify({
      channel_id: channelId,
      agent_id: agentId,
      ...(expiresAt !== undefined ? { expires_at: expiresAt } : {}),
    }),
  });
  return data.invitation;
}

export async function listAgentInvitations(
  role: AgentInvitationListRole = 'owner',
): Promise<AgentInvitation[]> {
  const data = await request<{ invitations: AgentInvitation[] }>(
    `/api/v1/agent_invitations?role=${role}`,
  );
  return data.invitations;
}

export async function fetchAgentInvitation(id: string): Promise<AgentInvitation> {
  const data = await request<{ invitation: AgentInvitation }>(`/api/v1/agent_invitations/${id}`);
  return data.invitation;
}

export async function decideAgentInvitation(
  id: string,
  state: 'approved' | 'rejected',
): Promise<AgentInvitation> {
  const data = await request<{ invitation: AgentInvitation }>(`/api/v1/agent_invitations/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({ state }),
  });
  return data.invitation;
}

// ─── Workspace ────────────────────────────────────────

export async function listWorkspaceFiles(
  channelId: string,
  parentId?: string,
): Promise<WorkspaceFile[]> {
  const qs = parentId ? `?parentId=${parentId}` : '';
  const data = await request<{ files: WorkspaceFile[] }>(
    `/api/v1/channels/${channelId}/workspace${qs}`,
  );
  return data.files;
}

export async function uploadWorkspaceFile(
  channelId: string,
  file: File,
  parentId?: string,
): Promise<WorkspaceFile> {
  const form = new FormData();
  form.append('file', file);
  const qs = parentId ? `?parentId=${parentId}` : '';
  const data = await request<{ file: WorkspaceFile }>(
    `/api/v1/channels/${channelId}/workspace/upload${qs}`,
    {
      method: 'POST',
      body: form,
    },
  );
  return data.file;
}

export async function downloadWorkspaceFile(channelId: string, fileId: string): Promise<Response> {
  const headers: Record<string, string> = {};
  if (import.meta.env.DEV && currentUserId) {
    headers['X-Dev-User-Id'] = currentUserId;
  }
  const res = await fetch(`/api/v1/channels/${channelId}/workspace/files/${fileId}`, {
    headers,
    credentials: 'include',
  });
  if (!res.ok) throw new ApiError(res.status, 'Download failed');
  return res;
}

export async function updateWorkspaceFile(
  channelId: string,
  fileId: string,
  content: string,
): Promise<WorkspaceFile> {
  const data = await request<{ file: WorkspaceFile }>(
    `/api/v1/channels/${channelId}/workspace/files/${fileId}`,
    {
      method: 'PUT',
      body: JSON.stringify({ content }),
    },
  );
  return data.file;
}

export async function deleteWorkspaceFile(channelId: string, fileId: string): Promise<void> {
  const headers: Record<string, string> = {};
  if (import.meta.env.DEV && currentUserId) {
    headers['X-Dev-User-Id'] = currentUserId;
  }
  const res = await fetch(`/api/v1/channels/${channelId}/workspace/files/${fileId}`, {
    method: 'DELETE',
    headers,
    credentials: 'include',
  });
  if (!res.ok) throw new ApiError(res.status, 'Delete failed');
}

export async function mkdirWorkspace(
  channelId: string,
  name: string,
  parentId?: string,
): Promise<WorkspaceFile> {
  const data = await request<{ file: WorkspaceFile }>(
    `/api/v1/channels/${channelId}/workspace/mkdir`,
    {
      method: 'POST',
      body: JSON.stringify({ name, parentId }),
    },
  );
  return data.file;
}

export async function moveWorkspaceFile(
  channelId: string,
  fileId: string,
  parentId: string | null,
): Promise<WorkspaceFile> {
  const data = await request<{ file: WorkspaceFile }>(
    `/api/v1/channels/${channelId}/workspace/files/${fileId}/move`,
    {
      method: 'POST',
      body: JSON.stringify({ parentId }),
    },
  );
  return data.file;
}

export async function renameWorkspaceFile(
  channelId: string,
  fileId: string,
  name: string,
): Promise<WorkspaceFile> {
  const data = await request<{ file: WorkspaceFile }>(
    `/api/v1/channels/${channelId}/workspace/files/${fileId}`,
    {
      method: 'PATCH',
      body: JSON.stringify({ name }),
    },
  );
  return data.file;
}

export async function fetchAllWorkspaces(): Promise<WorkspaceFile[]> {
  const data = await request<{ files: WorkspaceFile[] }>('/api/v1/workspaces');
  return data.files;
}

// ─── Remote Nodes ─────────────────────────────────────

export interface RemoteNode {
  id: string;
  user_id: string;
  machine_name: string;
  connection_token: string;
  last_seen_at: string | null;
  created_at: string;
}

export interface RemoteBinding {
  id: string;
  node_id: string;
  channel_id: string;
  path: string;
  label: string | null;
  created_at: string;
  machine_name?: string;
  node_user_id?: string;
}

export interface RemoteDirEntry {
  name: string;
  isDirectory: boolean;
  size: number;
  mtime: string;
}

export async function fetchRemoteNodes(): Promise<RemoteNode[]> {
  const data = await request<{ nodes: RemoteNode[] }>('/api/v1/remote/nodes');
  return data.nodes;
}

export async function createRemoteNode(machineName: string): Promise<RemoteNode> {
  const data = await request<{ node: RemoteNode }>('/api/v1/remote/nodes', {
    method: 'POST',
    body: JSON.stringify({ machine_name: machineName }),
  });
  return data.node;
}

export async function deleteRemoteNode(nodeId: string): Promise<void> {
  await request<{ ok: boolean }>(`/api/v1/remote/nodes/${nodeId}`, {
    method: 'DELETE',
  });
}

export async function fetchRemoteBindings(nodeId: string): Promise<RemoteBinding[]> {
  const data = await request<{ bindings: RemoteBinding[] }>(
    `/api/v1/remote/nodes/${nodeId}/bindings`,
  );
  return data.bindings;
}

export async function createRemoteBinding(
  nodeId: string,
  channelId: string,
  path: string,
  label?: string,
): Promise<RemoteBinding> {
  const data = await request<{ binding: RemoteBinding }>(
    `/api/v1/remote/nodes/${nodeId}/bindings`,
    {
      method: 'POST',
      body: JSON.stringify({ channel_id: channelId, path, label }),
    },
  );
  return data.binding;
}

export async function deleteRemoteBinding(nodeId: string, bindingId: string): Promise<void> {
  await request<{ ok: boolean }>(`/api/v1/remote/nodes/${nodeId}/bindings/${bindingId}`, {
    method: 'DELETE',
  });
}

export async function fetchChannelRemoteBindings(channelId: string): Promise<RemoteBinding[]> {
  const data = await request<{ bindings: RemoteBinding[] }>(
    `/api/v1/channels/${channelId}/remote-bindings`,
  );
  return data.bindings;
}

export async function remoteLs(
  nodeId: string,
  path: string,
): Promise<{ entries: RemoteDirEntry[] }> {
  return request<{ entries: RemoteDirEntry[] }>(
    `/api/v1/remote/nodes/${nodeId}/ls?path=${encodeURIComponent(path)}`,
  );
}

export async function remoteReadFile(
  nodeId: string,
  path: string,
): Promise<{ content: string; mimeType: string; size: number }> {
  return request<{ content: string; mimeType: string; size: number }>(
    `/api/v1/remote/nodes/${nodeId}/read?path=${encodeURIComponent(path)}`,
  );
}

export async function fetchRemoteNodeStatus(nodeId: string): Promise<{ online: boolean }> {
  return request<{ online: boolean }>(`/api/v1/remote/nodes/${nodeId}/status`);
}

// ─── Helper Enrollments ─────────────────────────────────

export type HelperEnrollmentStatus =
  | 'pending'
  | 'connected'
  | 'offline'
  | 'revoked'
  | 'uninstalled';

export type HelperConfigureOpenClawState =
  | 'queued'
  | 'running'
  | 'succeeded'
  | 'failed'
  | 'denied'
  | 'revoked'
  | 'manual_debug'
  | string;

export interface HelperConfigureOpenClawStep {
  job_type: string;
  status: string;
  created_at?: number;
  completed_at?: number;
  failure_code?: string;
  failure_message?: string;
  audit_refs: string[];
  log_refs: string[];
}

export interface HelperConfigureOpenClawStatusView {
  state: HelperConfigureOpenClawState;
  label: string;
  failure_code?: string;
  failure_message?: string;
  audit_refs: string[];
  log_refs: string[];
  steps: HelperConfigureOpenClawStep[];
}

export interface HelperEnrollmentStatusView {
  enrollment_id: string;
  host_label: string;
  helper_device_id?: string;
  allowed_categories: string[];
  status: HelperEnrollmentStatus | string;
  fresh: boolean;
  last_seen_at?: number;
  created_at: number;
  claimed_at?: number;
  revoked_at?: number;
  uninstalled_at?: number;
  configure_openclaw?: HelperConfigureOpenClawStatusView;
}

function sanitizeBoundedRefs(input: unknown): string[] {
  if (!Array.isArray(input)) return [];
  const refs: string[] = [];
  for (const value of input) {
    if (typeof value !== 'string') continue;
    const ref = value.trim();
    if (!ref || ref.length > 128 || /[\\/\0\n\r]/.test(ref)) continue;
    refs.push(ref);
    if (refs.length === 16) break;
  }
  return refs;
}

function sanitizeConfigureOpenClawStep(input: any): HelperConfigureOpenClawStep {
  return {
    job_type: String(input?.job_type ?? ''),
    status: String(input?.status ?? 'unknown'),
    ...(typeof input?.created_at === 'number' ? { created_at: input.created_at } : {}),
    ...(typeof input?.completed_at === 'number' ? { completed_at: input.completed_at } : {}),
    ...(typeof input?.failure_code === 'string' ? { failure_code: input.failure_code } : {}),
    ...(typeof input?.failure_message === 'string'
      ? { failure_message: input.failure_message }
      : {}),
    audit_refs: sanitizeBoundedRefs(input?.audit_refs),
    log_refs: sanitizeBoundedRefs(input?.log_refs),
  };
}

function sanitizeConfigureOpenClaw(input: any): HelperConfigureOpenClawStatusView | undefined {
  if (!input || typeof input !== 'object') return undefined;
  return {
    state: typeof input?.state === 'string' ? input.state : 'manual_debug',
    label: typeof input?.label === 'string' ? input.label : 'Manual debug required',
    ...(typeof input?.failure_code === 'string' ? { failure_code: input.failure_code } : {}),
    ...(typeof input?.failure_message === 'string'
      ? { failure_message: input.failure_message }
      : {}),
    audit_refs: sanitizeBoundedRefs(input?.audit_refs),
    log_refs: sanitizeBoundedRefs(input?.log_refs),
    steps: Array.isArray(input?.steps) ? input.steps.map(sanitizeConfigureOpenClawStep) : [],
  };
}

function sanitizeHelperEnrollment(input: any): HelperEnrollmentStatusView {
  return {
    enrollment_id: String(input?.enrollment_id ?? ''),
    host_label: String(input?.host_label ?? ''),
    ...(typeof input?.helper_device_id === 'string'
      ? { helper_device_id: input.helper_device_id }
      : {}),
    allowed_categories: Array.isArray(input?.allowed_categories)
      ? input.allowed_categories.filter((v: unknown): v is string => typeof v === 'string')
      : [],
    status: typeof input?.status === 'string' ? input.status : 'unknown',
    fresh: Boolean(input?.fresh),
    ...(typeof input?.last_seen_at === 'number' ? { last_seen_at: input.last_seen_at } : {}),
    created_at: typeof input?.created_at === 'number' ? input.created_at : 0,
    ...(typeof input?.claimed_at === 'number' ? { claimed_at: input.claimed_at } : {}),
    ...(typeof input?.revoked_at === 'number' ? { revoked_at: input.revoked_at } : {}),
    ...(typeof input?.uninstalled_at === 'number' ? { uninstalled_at: input.uninstalled_at } : {}),
    ...(sanitizeConfigureOpenClaw(input?.configure_openclaw)
      ? { configure_openclaw: sanitizeConfigureOpenClaw(input.configure_openclaw) }
      : {}),
  };
}

export async function fetchHelperEnrollments(): Promise<HelperEnrollmentStatusView[]> {
  const data = await request<{ enrollments: unknown[] }>('/api/v1/helper/enrollments');
  return Array.isArray(data.enrollments) ? data.enrollments.map(sanitizeHelperEnrollment) : [];
}

export async function fetchHelperEnrollment(
  enrollmentId: string,
): Promise<HelperEnrollmentStatusView> {
  const data = await request<{ enrollment: unknown }>(
    `/api/v1/helper/enrollments/${encodeURIComponent(enrollmentId)}`,
  );
  return sanitizeHelperEnrollment(data.enrollment);
}

// ─── Commands ──────────────────────────────────────────

export interface AgentCommandInfo {
  agent_id: string;
  agent_name: string;
  commands: Array<{
    name: string;
    description: string;
    usage: string;
    params: Array<{ name: string; type: string; required?: boolean; placeholder?: string }>;
  }>;
}

export interface CommandsResponse {
  builtin: Array<{ name: string; description: string; usage: string }>;
  agent: AgentCommandInfo[];
}

export async function listCommands(channelId?: string): Promise<CommandsResponse> {
  const params = new URLSearchParams();
  if (channelId) params.set('channelId', channelId);
  const qs = params.toString();
  return request<CommandsResponse>(`/api/v1/commands${qs ? `?${qs}` : ''}`);
}

// ─── Artifacts (CV-1.2 server / CV-1.3 client) ─────────────
//
// Spec: docs/implementation/modules/cv-1-spec.md §3 (CV-1.3 段) +
// docs/qa/acceptance-templates/cv-1.md §3 + docs/qa/cv-1-stance-checklist.md
// (7 条原则) + cv-1-stance-v1-supplement.md (②③⑤⑦ v1 字段).
//
// Pull-side契约: server's PushArtifactUpdated frame is signal-only (no
// body, no committer). 当 ArtifactUpdated frame 到达 client, 必须再走
// GET /api/v1/artifacts/:id 拿 body + committer (设计 ⑤ envelope 仅信号).

/**
 * CV-3.1/3.2 (#396 / #400): artifact kind enum extended from the v1
 * 'markdown' lock to three kinds. CV-2 v2 (#cv-2-v2): extends to 5 项 with
 * 'video_link' / 'pdf_link' (byte-identical 跟 cv_2_v2_media_preview.go
 * schema CHECK + cv_3_2_artifact_validation.go ValidArtifactKinds 同源).
 */
export type ArtifactKind = 'markdown' | 'code' | 'image_link' | 'video_link' | 'pdf_link';

export interface Artifact {
  id: string;
  channel_id: string;
  /** CV-3.1 + CV-2 v2: 5 态 enum (was 'markdown'-only in CV-1). */
  type: ArtifactKind;
  title: string;
  body: string;
  current_version: number;
  created_at: number;
  archived_at?: number;
  /** 设计 ⑥ committer_kind 'agent'|'human' (head version 的). */
  committer_kind: 'agent' | 'human';
  committer_id: string;
  /** 设计 ② 单文档锁 30s TTL — nil = 无人持锁可写. */
  lock_holder_user_id?: string;
  lock_acquired_at?: number;
  /** CV-2 v2 (#cv-2-v2): server-recorded thumbnail / poster URL (https only). */
  preview_url?: string;
}

export interface ArtifactVersion {
  version: number;
  body: string;
  committer_kind: 'agent' | 'human';
  committer_id: string;
  created_at: number;
  /** 设计 ⑦ rollback 路径 — 非 NULL = 该 row 是 rollback 触发的新 commit. */
  rolled_back_from_version?: number;
}

export interface CommitArtifactResponse {
  artifact_id: string;
  version: number;
  committer_id: string;
  committer_kind: 'agent' | 'human';
  updated_at: number;
}

export interface RollbackArtifactResponse {
  artifact_id: string;
  version: number;
  rolled_back_from_version: number;
  updated_at: number;
}

/** Create an artifact in a channel (CV-1.2 §2.1). */
export async function createArtifact(
  channelId: string,
  payload: { title: string; body?: string },
): Promise<Artifact> {
  return request<Artifact>(`/api/v1/channels/${encodeURIComponent(channelId)}/artifacts`, {
    method: 'POST',
    body: JSON.stringify({ title: payload.title, body: payload.body ?? '' }),
  });
}

/** GET head body + committer (设计 ⑤ pull 路径). */
export async function getArtifact(artifactId: string): Promise<Artifact> {
  return request<Artifact>(`/api/v1/artifacts/${encodeURIComponent(artifactId)}`);
}

/** GET version sidebar list (设计 ③ 线性版本号). */
export async function listArtifactVersions(
  artifactId: string,
): Promise<{ versions: ArtifactVersion[] }> {
  return request<{ versions: ArtifactVersion[] }>(
    `/api/v1/artifacts/${encodeURIComponent(artifactId)}/versions`,
  );
}

/**
 * Commit a new version. expected_version 来自 client 编辑时的 head;
 * server 端 mismatch → 409 (设计 ② lock conflict + reload hint).
 */
export async function commitArtifact(
  artifactId: string,
  payload: { expected_version: number; body: string },
): Promise<CommitArtifactResponse> {
  return request<CommitArtifactResponse>(
    `/api/v1/artifacts/${encodeURIComponent(artifactId)}/commits`,
    {
      method: 'POST',
      body: JSON.stringify(payload),
    },
  );
}

/**
 * Rollback to a prior version (设计 ⑦ owner-only). 服务器侧已闸 admin →
 * 401 / 非 owner → 403 / 锁持有=别人 → 409.
 */
export async function rollbackArtifact(
  artifactId: string,
  toVersion: number,
): Promise<RollbackArtifactResponse> {
  return request<RollbackArtifactResponse>(
    `/api/v1/artifacts/${encodeURIComponent(artifactId)}/rollback`,
    {
      method: 'POST',
      body: JSON.stringify({ to_version: toVersion }),
    },
  );
}

// ─── Iterations (CV-4.2 server #409 / CV-4.3 client) ─────────
//
// Spec: docs/implementation/modules/cv-4-spec.md §0 设计 ②
// (owner triggers iterate, agent commit goes through CV-1's existing
// /commits endpoint with ?iteration_id query). 文案锁:
// docs/qa/cv-4-content-lock.md §1 ③ (state 4 态 byte-identical).
// Stance: docs/qa/cv-4-stance-checklist.md §1 ①-⑦.
//
// Push frame: IterationStateChangedFrame 9 字段 byte-identical 跟
// server-go iteration_state_frame.go 同源 (BPP-1 #304 envelope CI lint).

export type IterationState = 'pending' | 'running' | 'completed' | 'failed';

/**
 * artifact_iterations row (server CV-4.1 schema #405 + CV-4.2 #409).
 *
 * 字段顺序锁: id / artifact_id / requested_by / intent_text /
 * target_agent_id / state / created_artifact_version_id NULL /
 * error_reason NULL / created_at / completed_at NULL.
 *
 * 设计 ⑦ admin god-mode 字段白名单不含 intent_text — 但 owner 拉自己
 * 触发的 iteration 时返完整 row (intent_text 是 owner 自己输入).
 */
export interface ArtifactIteration {
  id: string;
  artifact_id: string;
  requested_by: string;
  intent_text: string;
  target_agent_id: string;
  state: IterationState;
  /** AL-1a 6 reason 之一; state='failed' 时非空, 走 REASON_LABELS 渲染. */
  error_reason?: AgentRuntimeReason | null;
  /** state='completed' 时非空; FK PK 非用户号 version. */
  created_artifact_version_id?: number | null;
  created_at: number;
  completed_at?: number | null;
}

export interface CreateIterationResponse {
  iteration: ArtifactIteration;
}

/**
 * Trigger iterate — owner-only (server enforces; client also gates DOM
 * via CV-1 #347 line 254 同模式 owner-only DOM omit defense-in-depth).
 *
 * 反约束: 不开 `/iterations/:id/commit` 旁路 endpoint — agent commit
 * 走 CV-1 既有 `POST /artifacts/:id/commits` 加 query `?iteration_id=`
 * (cv-4-stance §1 ② CV-1 commit 单源).
 */
export async function createIteration(
  artifactId: string,
  payload: { intent_text: string; target_agent_id: string },
): Promise<CreateIterationResponse> {
  return request<CreateIterationResponse>(
    `/api/v1/artifacts/${encodeURIComponent(artifactId)}/iterate`,
    {
      method: 'POST',
      body: JSON.stringify(payload),
    },
  );
}

/**
 * GET single iteration body — 设计 ⑤ envelope-signal-only 后 pull 路径.
 * Push frame 仅信号, intent_text 不在 frame (admin 字段白名单反断同源).
 */
export async function getIteration(
  artifactId: string,
  iterationId: string,
): Promise<ArtifactIteration> {
  return request<ArtifactIteration>(
    `/api/v1/artifacts/${encodeURIComponent(artifactId)}/iterations/${encodeURIComponent(iterationId)}`,
  );
}

/**
 * GET iteration list — artifact panel "迭代历史" 折叠区
 * (cv-4-content-lock §1 ⑥ 头 5 条 + intent_text 头 40 字截断).
 */
export async function listIterations(
  artifactId: string,
  opts?: { limit?: number },
): Promise<{ iterations: ArtifactIteration[] }> {
  const qs = opts?.limit && opts.limit > 0 ? `?limit=${encodeURIComponent(opts.limit)}` : '';
  return request<{ iterations: ArtifactIteration[] }>(
    `/api/v1/artifacts/${encodeURIComponent(artifactId)}/iterations${qs}`,
  );
}

// ─── Anchors (CV-2.2 server / CV-2.3 client) ───────────────
//
// Spec: docs/implementation/modules/cv-2-spec.md §0 (3 条原则) + §1
// (CV-2.3 段). Server: packages/server-go/internal/api/anchors.go (#360).
//
// Pull-side契约: server's PushAnchorCommentAdded frame is signal-only
// (10 字段 envelope, no body). 当 anchor_comment_added frame 到达
// client, 必须再走 GET /api/v1/artifacts/:id/anchors +
// GET /api/v1/anchors/:id/comments 拿评论列表 (设计 ③ envelope 仅信号).

export interface AnchorThread {
  id: string;
  artifact_id: string;
  artifact_version_id: number;
  start_offset: number;
  end_offset: number;
  created_by: string;
  created_at: number;
  resolved_at: number | null;
}

export interface AnchorComment {
  id: number;
  anchor_id: string;
  body: string;
  /** 'human' | 'agent' — naming aligned with anchor_comments.author_kind. */
  author_kind: 'human' | 'agent';
  author_id: string;
  created_at: number;
}

/** POST /artifacts/:id/anchors — owner-only on server (kind='human' check); agent → 403 anchor.create_owner_only. */
export async function createAnchor(
  artifactId: string,
  payload: { version?: number; start_offset: number; end_offset: number },
): Promise<AnchorThread> {
  return request<AnchorThread>(`/api/v1/artifacts/${encodeURIComponent(artifactId)}/anchors`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

/** GET /artifacts/:id/anchors — list active + resolved anchors (channel members). */
export async function listAnchors(artifactId: string): Promise<{ anchors: AnchorThread[] }> {
  return request<{ anchors: AnchorThread[] }>(
    `/api/v1/artifacts/${encodeURIComponent(artifactId)}/anchors`,
  );
}

/** POST /anchors/:id/comments — channel members may reply; agent only on threads with a human author (server-enforced). */
export async function addAnchorComment(anchorId: string, body: string): Promise<AnchorComment> {
  return request<AnchorComment>(`/api/v1/anchors/${encodeURIComponent(anchorId)}/comments`, {
    method: 'POST',
    body: JSON.stringify({ body }),
  });
}

/** GET /anchors/:id/comments — pull comment list after WS signal. */
export async function listAnchorComments(anchorId: string): Promise<{ comments: AnchorComment[] }> {
  return request<{ comments: AnchorComment[] }>(
    `/api/v1/anchors/${encodeURIComponent(anchorId)}/comments`,
  );
}

/** POST /anchors/:id/resolve — owner / creator only (server-enforced). */
export async function resolveAnchor(
  anchorId: string,
): Promise<{ id: string; resolved_at: number }> {
  return request<{ id: string; resolved_at: number }>(
    `/api/v1/anchors/${encodeURIComponent(anchorId)}/resolve`,
    { method: 'POST' },
  );
}

// ─── CHN-3.2 user_channel_layout (CHN-3.3 client) ──────────
//
// Spec: docs/implementation/modules/chn-3-spec.md §1 CHN-3.2 段 + §0
// 设计 ② 个人偏好两维 collapsed + position. Server: api/layout.go
// (#412, stacked off CHN-3.1 schema #410 v=19).
//
// 设计 ④ 反约束 错码 byte-identical: server PUT 对 DM channel_id 返
// 400 with code `layout.dm_not_grouped` (5 源 #357/#353/#366/#402/
// #412); client 走 GET pull, PUT batch upsert, 不挂 push frame
// (设计 ⑥ ordering client 端事).

export interface LayoutRow {
  channel_id: string;
  collapsed: number; // 0 | 1 (BOOL); position is REAL.
  position: number;
  created_at?: number;
  updated_at?: number;
}

/** GET /me/layout — 本人 layout list (position ASC ordering). */
export async function getMyLayout(): Promise<{ layout: LayoutRow[] }> {
  return request<{ layout: LayoutRow[] }>(`/api/v1/me/layout`);
}

/**
 * PUT /me/layout — batch upsert (collapsed + position 两维, server 跑
 * ON CONFLICT (user_id, channel_id) DO UPDATE atomic). 反约束: DM
 * channel_id → 400 `layout.dm_not_grouped` (server 兜底).
 */
export async function putMyLayout(layout: LayoutRow[]): Promise<{ ok: boolean }> {
  return request<{ ok: boolean }>(`/api/v1/me/layout`, {
    method: 'PUT',
    body: JSON.stringify({ layout }),
  });
}

/**
 * BPP-3.2.2 — POST /api/v1/me/grants
 *
 * Owner one-click capability grant (or reject/snooze) from owner DM
 * SystemMessageBubble three buttons. Body byte-identical 跟
 * docs/qa/bpp-3.2-content-lock.md §2 + server side me_grants.go
 * meGrantsRequest.
 *
 * Returns server response body. action='grant' → user_permissions row
 * landed; action='reject'/'snooze' → audit-only (v1 不持久化 deny list).
 */
export interface MeGrantRequest {
  agent_id: string;
  capability: string;
  scope: string;
  request_id: string;
  action: 'grant' | 'reject' | 'snooze';
}
export interface MeGrantResponse {
  granted: boolean;
  action: 'grant' | 'reject' | 'snooze';
  agent_id?: string;
  capability?: string;
  scope?: string;
}
export async function postMeGrant(req: MeGrantRequest): Promise<MeGrantResponse> {
  const resp = await fetch(`${BASE}/api/v1/me/grants`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });
  if (!resp.ok) {
    let detail = `HTTP ${resp.status}`;
    try {
      const body = await resp.json();
      if (body?.error_code) detail = body.error_code;
    } catch {
      /* ignore */
    }
    throw new Error(`me/grants ${detail}`);
  }
  return (await resp.json()) as MeGrantResponse;
}

// AL-5.2 — owner agent error recovery (POST /api/v1/agents/:id/recover).
export interface AL5RecoverPayload {
  action: 'recover';
  agent_id: string;
  reason: string;
  request_id: string;
}
export interface AL5RecoverResponse {
  state: string;
  reason: string;
}
export async function postAgentRecover(req: AL5RecoverPayload): Promise<AL5RecoverResponse> {
  const resp = await fetch(`${BASE}/api/v1/agents/${encodeURIComponent(req.agent_id)}/recover`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ request_id: req.request_id }),
  });
  if (!resp.ok) {
    let detail = `HTTP ${resp.status}`;
    try {
      const body = await resp.json();
      if (body?.error) detail = body.error;
    } catch {
      /* ignore */
    }
    throw new Error(`agents/recover ${detail}`);
  }
  return (await resp.json()) as AL5RecoverResponse;
}

// DM-4.1 — agent message edit 多端同步.
// PATCH /api/v1/channels/{channelId}/messages/{messageId}
//
// 设计 (跟 dm-4-spec.md §0):
//   ① 复用 RT-3 既有 fan-out (events INSERT op="edit" + Hub broadcast)
//   ② edit 是 cursor 子集 (cursor 进展归 useDMSync DM-3 #508)
//   ③ thinking 5-pattern 反约束延伸第 3 处 (机械修订, 不暴露 reasoning)
export interface DM4EditResponse {
  message: {
    id: string;
    channel_id: string;
    sender_id: string;
    content: string;
    edited_at?: number | null;
    [key: string]: unknown;
  };
}

export async function patchDMMessage(
  channelID: string,
  messageID: string,
  content: string,
): Promise<DM4EditResponse> {
  const resp = await fetch(
    `${BASE}/api/v1/channels/${encodeURIComponent(channelID)}/messages/${encodeURIComponent(messageID)}`,
    {
      method: 'PATCH',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content }),
    },
  );
  if (!resp.ok) {
    let detail = `HTTP ${resp.status}`;
    try {
      const body = await resp.json();
      if (body?.error) detail = body.error;
    } catch {
      /* ignore */
    }
    throw new Error(`dm/edit ${detail}`);
  }
  return (await resp.json()) as DM4EditResponse;
}

// ─── CV-5 Artifact comments ─────────────────────────────────
//
// Spec: docs/implementation/modules/cv-5-spec.md §0 设计 ① — comment row
// lives in messages 表 with virtual `artifact:<id>` namespace channel
// (跟 DM-2 dm: 同模式). Server-side path POST/GET
// /api/v1/artifacts/{artifactId}/comments. body_preview 80 rune cap on
// push frame; full body returned by these REST endpoints (channel-member ACL).

export interface ArtifactComment {
  id: string;
  artifact_id: string;
  channel_id: string;
  sender_id: string;
  /** 'human' | 'agent' — server-stamped from users.role. */
  sender_role: 'human' | 'agent';
  body: string;
  created_at: number;
  cursor?: number;
}

/** POST /api/v1/artifacts/:id/comments — channel members may comment. */
export async function postArtifactComment(
  artifactId: string,
  body: string,
  agentId?: string,
): Promise<ArtifactComment> {
  return request<ArtifactComment>(`/api/v1/artifacts/${encodeURIComponent(artifactId)}/comments`, {
    method: 'POST',
    body: JSON.stringify(agentId ? { body, agent_id: agentId } : { body }),
  });
}

/** GET /api/v1/artifacts/:id/comments — pull comment list after WS signal. */
export async function listArtifactComments(
  artifactId: string,
): Promise<{ comments: ArtifactComment[] }> {
  return request<{ comments: ArtifactComment[] }>(
    `/api/v1/artifacts/${encodeURIComponent(artifactId)}/comments`,
  );
}

// ─── CV-8 Artifact comment thread reply ─────────────────────
//
// Spec: docs/implementation/modules/cv-8-spec.md §0 设计 ① — 走 messages
// 表既有 endpoint POST /api/v1/channels/{channelId}/messages with
// content_type='artifact_comment' + reply_to_id (既有 schema 字段).
// CV-8 不开新 client api;  此函数仅是 sendMessage 的 thin wrapper,
// 承担 errcode byte-identical surface (CV-5/CV-7 同字符).

/** POST /api/v1/channels/:channelId/messages — artifact_comment + reply_to_id thread reply (CV-8). */
export async function postArtifactCommentReply(
  channelId: string,
  parentCommentId: string,
  body: string,
): Promise<{ id: string }> {
  const data = await request<{ message: { id: string } }>(
    `/api/v1/channels/${channelId}/messages`,
    {
      method: 'POST',
      body: JSON.stringify({
        content: body,
        content_type: 'artifact_comment',
        reply_to_id: parentCommentId,
      }),
    },
  );
  return { id: data.message.id };
}

// ─── CV-12 Artifact comment search ──────────────────────────
//
// Spec: docs/implementation/modules/cv-12-spec.md §0 设计 ① — 走既有
// GET /api/v1/channels/{channelId}/messages/search?q= endpoint 单源.
// channelId 是 artifact: namespace channel UUID (CV-5 #530 设计 ①).
// 0 server code; CV-12 仅这 1 函数 thin wrapper.

export interface ArtifactCommentSearchHit {
  id: string;
  content: string;
  sender_id: string;
  created_at: number;
}

/** GET /api/v1/channels/:channelId/messages/search?q= — search messages
 *  in a virtual artifact: namespace channel. Returns matching comments. */
export async function searchArtifactComments(
  channelId: string,
  query: string,
): Promise<{ messages: ArtifactCommentSearchHit[] }> {
  return request<{ messages: ArtifactCommentSearchHit[] }>(
    `/api/v1/channels/${encodeURIComponent(channelId)}/messages/search?q=${encodeURIComponent(query)}`,
  );
}

// DM-7.3 — edit history GET (sender-only user-rail).
// Server: GET /api/v1/channels/{channelId}/messages/{messageId}/edit-history
export interface DM7EditHistoryEntry {
  old_content: string;
  ts: number;
  reason: string;
}

export interface DM7EditHistoryResponse {
  history: DM7EditHistoryEntry[];
}

export async function getEditHistory(
  channelID: string,
  messageID: string,
): Promise<DM7EditHistoryResponse> {
  const resp = await fetch(
    `${BASE}/api/v1/channels/${encodeURIComponent(channelID)}/messages/${encodeURIComponent(messageID)}/edit-history`,
    { method: 'GET', credentials: 'include' },
  );
  if (!resp.ok) {
    let detail = `HTTP ${resp.status}`;
    try {
      const body = await resp.json();
      if (body?.error) detail = body.error;
    } catch {
      /* ignore */
    }
    throw new Error(`dm7/edit-history ${detail}`);
  }
  return (await resp.json()) as DM7EditHistoryResponse;
}

// CHN-14.3 — channel description edit history GET (owner-only user-rail).
// Server: GET /api/v1/channels/{channelId}/description/history
// (跟 DM-7 #558 EditHistoryEntry 同模式 byte-identical: old_content/ts/reason).
export interface CHN14DescriptionHistoryEntry {
  old_content: string;
  ts: number;
  reason: string;
}

export interface CHN14DescriptionHistoryResponse {
  history: CHN14DescriptionHistoryEntry[];
}

export async function getChannelDescriptionHistory(
  channelID: string,
): Promise<CHN14DescriptionHistoryResponse> {
  const resp = await fetch(
    `${BASE}/api/v1/channels/${encodeURIComponent(channelID)}/description/history`,
    { method: 'GET', credentials: 'include' },
  );
  if (!resp.ok) {
    let detail = `HTTP ${resp.status}`;
    try {
      const body = await resp.json();
      if (body?.error) detail = body.error;
    } catch {
      /* ignore */
    }
    throw new Error(`chn14/description-history ${detail}`);
  }
  return (await resp.json()) as CHN14DescriptionHistoryResponse;
}

// CHN-10: set channel description (owner-only). Server: PUT
// /api/v1/channels/{channelId}/description body {description}; 500 char
// upper bound (DESCRIPTION_MAX_LENGTH byte-identical 跟 server const +
// channels.topic GORM size:500 同源 — 改一处 = 改三处反向锁守门).
export const DESCRIPTION_MAX_LENGTH = 500;

export async function setChannelDescription(
  channelId: string,
  description: string,
): Promise<Channel> {
  const data = await request<{ channel: Channel }>(
    `/api/v1/channels/${encodeURIComponent(channelId)}/description`,
    {
      method: 'PUT',
      body: JSON.stringify({ description }),
    },
  );
  return data.channel;
}

// RT-4: get channel presence (member-only — server filters channel.members
// ∩ IsOnline). Returns array of online user_id; 0 client-side caching v0.
export interface ChannelPresenceResponse {
  online_user_ids: string[];
  counted_at: number;
}

export async function getChannelPresence(channelId: string): Promise<ChannelPresenceResponse> {
  return request<ChannelPresenceResponse>(
    `/api/v1/channels/${encodeURIComponent(channelId)}/presence`,
  );
}

// ──────────────────────────────────────────────────────────────────────
// CV-6 (#cv-6): artifact full-text search via SQLite FTS5.
// ──────────────────────────────────────────────────────────────────────

/**
 * SEARCH_ERR_TOAST — byte-identical 跟 server `internal/api/search.go::
 * SearchErrCode*` const + `docs/qa/cv-6-content-lock.md` §4 同源.
 *
 * 改 = 改三处: server const + 此 map + content-lock §4.
 */
export const SEARCH_ERR_TOAST: Record<string, string> = {
  'search.query_empty': '请输入搜索词',
  'search.query_too_long': '搜索词太长 (最长 256 字符)',
  'search.channel_not_member': '无权访问此频道',
  'search.cross_org_denied': '跨组织搜索被禁',
  'search.not_owner': '需要频道所有者权限',
};

/** SearchResult — server response row shape. */
export interface SearchResult {
  artifact_id: string;
  title: string;
  snippet: string; // server-side `<mark>...</mark>` 字面 byte-identical
  kind: string; // 5 enum: markdown / code / image_link / video_link / pdf_link
  channel_id: string;
  current_version: number;
}

/** SearchArtifactsResponse — wire shape from GET /api/v1/artifacts/search. */
export interface SearchArtifactsResponse {
  query: string;
  results: SearchResult[];
  total: number;
}

/**
 * searchArtifacts — calls GET /api/v1/artifacts/search?q=&channel_id=&limit=.
 * Returns parsed response on 2xx; throws Error with `error_code` substring
 * for 4xx/5xx so caller can lookup SEARCH_ERR_TOAST.
 */
export async function searchArtifacts(
  query: string,
  channelID: string,
  limit?: number,
): Promise<SearchArtifactsResponse> {
  const params = new URLSearchParams({ q: query, channel_id: channelID });
  if (limit !== undefined) params.set('limit', String(limit));
  const resp = await fetch(`${BASE}/api/v1/artifacts/search?${params.toString()}`, {
    credentials: 'include',
  });
  if (!resp.ok) {
    let errCode = 'unknown';
    try {
      const body = await resp.json();
      errCode = (body?.error as string) || errCode;
    } catch {
      /* ignore */
    }
    throw new Error(errCode);
  }
  return (await resp.json()) as SearchArtifactsResponse;
}

// ──────────────────────────────────────────────────────────────────────
// CHN-15 (#chn-15): channel readonly toggle — owner-only.
// ──────────────────────────────────────────────────────────────────────

/**
 * CHANNEL_READONLY_TOAST — byte-identical 跟 server
 * `internal/api/chn_15_readonly.go::ChannelErrCodeReadonlyNoSend` const +
 * `docs/qa/chn-15-content-lock.md` §3 同源.
 *
 * 改 = 改三处: server const + 此 map + content-lock §3.
 */
export const CHANNEL_READONLY_TOAST: Record<string, string> = {
  'channel.readonly_no_send': '只读频道, 仅创建者可发言',
};

export interface ChannelReadonlyResponse {
  channel_id: string;
  collapsed: number;
  readonly: boolean;
}

/** PUT /api/v1/channels/{id}/readonly — set channel readonly (owner-only). */
export async function setChannelReadonly(channelID: string): Promise<ChannelReadonlyResponse> {
  const resp = await fetch(`${BASE}/api/v1/channels/${encodeURIComponent(channelID)}/readonly`, {
    method: 'PUT',
    credentials: 'include',
  });
  if (!resp.ok) {
    let detail = `HTTP ${resp.status}`;
    try {
      const body = await resp.json();
      if (body?.error) detail = body.error;
    } catch {
      /* ignore */
    }
    throw new Error(`channel/readonly ${detail}`);
  }
  return (await resp.json()) as ChannelReadonlyResponse;
}

/** DELETE /api/v1/channels/{id}/readonly — unset channel readonly (idempotent). */
export async function unsetChannelReadonly(channelID: string): Promise<ChannelReadonlyResponse> {
  const resp = await fetch(`${BASE}/api/v1/channels/${encodeURIComponent(channelID)}/readonly`, {
    method: 'DELETE',
    credentials: 'include',
  });
  if (!resp.ok) {
    let detail = `HTTP ${resp.status}`;
    try {
      const body = await resp.json();
      if (body?.error) detail = body.error;
    } catch {
      /* ignore */
    }
    throw new Error(`channel/readonly ${detail}`);
  }
  return (await resp.json()) as ChannelReadonlyResponse;
}

// ──────────────────────────────────────────────────────────────────────
// CV-15 (#cv-15): artifact comment edit history audit — sender-only GET.
// ──────────────────────────────────────────────────────────────────────

/**
 * COMMENT_EDIT_HISTORY_ERR_TOAST — byte-identical 跟 server
 * `internal/api/cv_15_comment_edit_history.go::CommentEditHistoryErrCode*`
 * + `docs/qa/cv-15-content-lock.md` §3 同源.
 *
 * 改 = 改三处: server const + 此 map + content-lock §3.
 */
export const COMMENT_EDIT_HISTORY_ERR_TOAST: Record<string, string> = {
  'comment.not_artifact_comment': '该消息不是 artifact 评论',
  'comment.not_owner': '仅评论作者可查看历史',
  'comment.message_not_found': '消息不存在',
};

export interface ArtifactCommentEditHistoryEntry {
  old_content: string;
  ts: number;
  reason?: string;
}

export interface ArtifactCommentEditHistoryResponse {
  history: ArtifactCommentEditHistoryEntry[];
}

/**
 * GET /api/v1/channels/{channelId}/messages/{messageId}/comment-edit-history.
 * Sender-only — only the comment author can view history. Returns
 * `{history: []}` when no edits.
 */
export async function getArtifactCommentEditHistory(
  channelID: string,
  messageID: string,
): Promise<ArtifactCommentEditHistoryResponse> {
  const resp = await fetch(
    `${BASE}/api/v1/channels/${encodeURIComponent(channelID)}/messages/${encodeURIComponent(messageID)}/comment-edit-history`,
    { credentials: 'include' },
  );
  if (!resp.ok) {
    let detail = `HTTP ${resp.status}`;
    try {
      const body = await resp.json();
      if (body?.error) detail = body.error;
    } catch {
      /* ignore */
    }
    throw new Error(`cv15/comment-edit-history ${detail}`);
  }
  return (await resp.json()) as ArtifactCommentEditHistoryResponse;
}

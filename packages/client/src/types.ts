// ─── Shared types (mirrors server types) ────────────────

export interface Channel {
  id: string;
  name: string;
  topic: string;
  type?: 'channel' | 'dm' | 'system';
  visibility?: 'public' | 'private';
  created_at: number;
  created_by: string;
  member_count?: number;
  is_member?: boolean;
  last_message_at?: number | null;
  unread_count?: number;
  position?: string;
  group_id?: string | null;
  // CHN-1.2 立场 ⑤: server-stamped archive timestamp. nil = active;
  // non-nil = archived (channel is read-only, hidden from default lists).
  // Distinct from a deleted channel — archive preserves history.
  archived_at?: number | null;
}

export interface User {
  id: string;
  display_name: string;
  role: 'member' | 'agent';
  avatar_url: string | null;
  owner_id?: string | null;
  created_at: number;
}

export interface DmPeer {
  id: string;
  display_name: string;
  avatar_url: string | null;
  role: string;
  state?: 'online' | 'offline' | 'error' | 'busy' | 'idle';
  reason?:
    | 'api_key_invalid'
    | 'quota_exceeded'
    | 'network_unreachable'
    | 'runtime_crashed'
    | 'runtime_timeout'
    | 'unknown';
  state_updated_at?: number;
}

export interface DmChannel {
  id: string;
  name: string;
  type: 'dm';
  created_at: number;
  peer: DmPeer;
  unread_count: number;
  last_message: { content: string; created_at: number } | null;
}

export interface Message {
  id: string;
  channel_id: string;
  sender_id: string;
  sender_name?: string;
  content: string;
  content_type: 'text' | 'image' | 'command';
  reply_to_id: string | null;
  created_at: number;
  edited_at: number | null;
  deleted_at?: number | null;
  mentions?: string[];
  reactions?: { emoji: string; count: number; user_ids: string[] }[];
  // CM-onboarding: server attaches a JSON-encoded {kind,label,action} payload
  // to the welcome system message. Renderer parses lazily.
  quick_action?: string | null;
  _pending?: boolean;
  _failed?: boolean;
  _clientMessageId?: string;
}

export type SendStatus = 'idle' | 'sending' | 'sent' | 'error';

export interface WsMessage {
  type: string;
  [key: string]: unknown;
}

export type ConnectionState = 'connecting' | 'connected' | 'disconnected' | 'reconnecting';

export interface PendingMessage {
  clientMessageId: string;
  channelId: string;
  content: string;
  contentType: 'text' | 'image' | 'command';
  status: 'pending' | 'failed';
  createdAt: number;
  senderName: string;
  senderId: string;
  mentions?: string[];
}

export interface ChannelGroup {
  id: string;
  name: string;
  position: string;
  created_by: string;
  created_at: number;
}

export interface WorkspaceFile {
  id: string;
  user_id: string;
  channel_id: string;
  parent_id: string | null;
  name: string;
  is_directory: number;
  mime_type: string | null;
  size_bytes: number;
  source: 'upload' | 'message_attachment';
  source_message_id: string | null;
  created_at: string;
  updated_at: string;
  channel_name?: string;
}

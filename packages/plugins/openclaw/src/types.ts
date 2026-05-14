// ─── Plugin Config Types ─────────────────────────────────

export type BorgeeTransport = "auto" | "sse" | "poll" | "ws";

export type BorgeeAccountConfig = {
  name?: string;
  enabled?: boolean;
  baseUrl?: string;
  apiKey?: string;
  botUserId?: string;
  botDisplayName?: string;
  pollTimeoutMs?: number;
  transport?: BorgeeTransport;
  allowFrom?: Array<string | number>;
  defaultTo?: string;
};

export type BorgeeChannelConfig = BorgeeAccountConfig & {
  accounts?: Record<string, Partial<BorgeeAccountConfig>>;
  defaultAccount?: string;
};

export type CoreConfig = {
  channels?: {
    borgee?: BorgeeChannelConfig;
  };
  session?: {
    store?: string;
  };
};

export type ResolvedBorgeeAccount = {
  accountId: string;
  enabled: boolean;
  configured: boolean;
  name?: string;
  baseUrl: string;
  apiKey: string;
  botUserId: string;
  botDisplayName: string;
  requireMention: boolean;
  pollTimeoutMs: number;
  transport: BorgeeTransport;
  config: BorgeeAccountConfig;
};

// ─── Borgee Server API Types ─────────────────────────────

export type BorgeeChannel = {
  id: string;
  name: string;
  topic: string;
  type?: 'channel' | 'dm';
  created_at: number;
  member_count?: number;
};

export type BorgeeUser = {
  id: string;
  display_name: string;
  role: 'admin' | 'member' | 'agent';
  avatar_url: string | null;
  require_mention?: boolean;
};

export type BorgeeMessage = {
  id: string;
  channel_id: string;
  channel_name?: string;
  sender_id: string;
  sender_name?: string;
  content: string;
  content_type: 'text' | 'image';
  reply_to_id: string | null;
  created_at: number;
  edited_at: number | null;
  mentions?: string[];
};

export type BorgeeEventKind =
  | 'message'
  | 'message_edited'
  | 'message_deleted'
  | 'mention'
  | 'channel_created'
  | 'member_joined'
  | 'member_left'
  | 'reaction_update';

export type BorgeeEvent = {
  cursor: number;
  kind: BorgeeEventKind;
  channel_id: string;
  payload: string;
  created_at: number;
};

export type BorgeePollResult = {
  cursor: number;
  events: BorgeeEvent[];
};

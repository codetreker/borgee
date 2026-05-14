import type { OpenClawConfig } from "openclaw/plugin-sdk/config-runtime";
import { dispatchInboundReplyWithBase } from "openclaw/plugin-sdk/inbound-reply-dispatch";
import { sendBorgeeMessage } from "./api-client.js";
import { buildBorgeeConversationMeta } from "./conversation-meta.js";
import { getBorgeeRuntime } from "./runtime.js";
import type { BorgeeEvent, CoreConfig, ResolvedBorgeeAccount } from "./types.js";

export async function handleBorgeeReactionInbound(params: {
  channelId: string;
  channelLabel: string;
  account: ResolvedBorgeeAccount;
  config: CoreConfig;
  event: BorgeeEvent;
  payload: {
    message_id: string;
    emoji: string;
    user_id: string;
    action: string;
  };
}): Promise<void> {
  const runtime = getBorgeeRuntime();
  const p = params.payload;
  const rawChannelId = stripChannelPrefix(params.event.channel_id);
  const target = buildBorgeeTarget(rawChannelId);
  const conversationMeta = buildBorgeeConversationMeta({
    channelLabel: params.channelLabel,
    channelType: "channel",
    message: {
      channel_id: params.event.channel_id,
      sender_id: p.user_id,
    },
  });

  const actionLabel = p.action === "added" ? "added" : "removed";
  const body = `[reaction_update] ${p.user_id} ${actionLabel} ${p.emoji} on message ${p.message_id}`;

  const route = runtime.channel.routing.resolveAgentRoute({
    cfg: params.config as OpenClawConfig,
    channel: params.channelId,
    accountId: params.account.accountId,
    peer: {
      kind: "channel",
      id: rawChannelId,
    },
  });

  const storePath = runtime.channel.session.resolveStorePath(params.config.session?.store, {
    agentId: route.agentId,
  });

  const ctxPayload = runtime.channel.reply.finalizeInboundContext({
    Body: body,
    BodyForAgent: body,
    RawBody: JSON.stringify(p),
    CommandBody: body,
    From: buildBorgeeTarget(stripChannelPrefix(p.user_id)),
    To: target,
    SessionKey: route.sessionKey,
    AccountId: route.accountId ?? params.account.accountId,
    ChatType: conversationMeta.chatType,
    ConversationLabel: conversationMeta.conversationLabel,
    GroupSubject: conversationMeta.groupSubject,
    GroupChannel: conversationMeta.groupChannel,
    NativeChannelId: conversationMeta.nativeChannelId,
    SenderId: p.user_id,
    Provider: params.channelId,
    Surface: params.channelId,
    MessageSid: `reaction:${p.message_id}:${p.emoji}`,
    MessageSidFull: `reaction:${p.message_id}:${p.emoji}`,
    Timestamp: params.event.created_at,
    OriginatingChannel: params.channelId,
    OriginatingTo: target,
    CommandAuthorized: true,
  });

  await dispatchInboundReplyWithBase({
    cfg: params.config as OpenClawConfig,
    channel: params.channelId,
    accountId: params.account.accountId,
    route,
    storePath,
    ctxPayload,
    core: runtime,
    deliver: async (payload) => {
      const text =
        payload && typeof payload === "object" && "text" in payload
          ? ((payload as { text?: string }).text ?? "")
          : "";
      if (!text.trim()) return;

      await sendBorgeeMessage({
        baseUrl: params.account.baseUrl,
        apiKey: params.account.apiKey,
        channelId: params.event.channel_id,
        content: text,
      });
    },
    onRecordError: (error) => {
      throw error instanceof Error
        ? error
        : new Error(`borgee session record failed: ${String(error)}`);
    },
    onDispatchError: (error) => {
      throw error instanceof Error
        ? error
        : new Error(`borgee dispatch failed: ${String(error)}`);
    },
  });
}

/**
 * Target format for Borgee:
 *   channel:<channel_id>
 *
 * Note: the SDK's routing layer prepends `channel:` from peer.kind when
 * building session keys.  We must pass the *raw* UUID as `peer.id` so the
 * key becomes `agent:<id>:borgee:channel:<uuid>` — not the double-prefixed
 * `agent:<id>:borgee:channel:channel:<uuid>`.
 */
function buildBorgeeTarget(channelId: string): string {
  return `channel:${stripChannelPrefix(channelId)}`;
}

/**
 * Strip leading "channel:" prefix if present so we never double-prefix.
 */
function stripChannelPrefix(id: string): string {
  const trimmed = id.trim();
  return trimmed.startsWith("channel:") ? trimmed.slice("channel:".length) : trimmed;
}

export function parseBorgeeTarget(raw: string): {
  chatType: "channel" | "dm";
  channelId: string;
  userId?: string;
} {
  const trimmed = raw.trim();
  if (trimmed.startsWith("dm:")) {
    const value = trimmed.slice("dm:".length);
    return { chatType: "dm", channelId: value, userId: value };
  }
  if (trimmed.startsWith("channel:")) {
    return { chatType: "channel", channelId: trimmed.slice("channel:".length) };
  }
  return { chatType: "channel", channelId: trimmed };
}

export async function handleBorgeeInbound(params: {
  channelId: string;
  channelLabel: string;
  account: ResolvedBorgeeAccount;
  config: CoreConfig;
  event: BorgeeEvent;
  channelType?: 'channel' | 'dm';
  message: {
    id: string;
    channel_id: string;
    sender_id: string;
    sender_name?: string;
    channel_name?: string;
    content: string;
    content_type: string;
    created_at: number;
    mentions?: string[];
    reply_to_id?: string | null;
  };
}): Promise<void> {
  const runtime = getBorgeeRuntime();
  const msg = params.message;
  const rawChannelId = stripChannelPrefix(msg.channel_id);
  const isDm = params.channelType === 'dm';
  const target = isDm ? `dm:${rawChannelId}` : buildBorgeeTarget(rawChannelId);
  const conversationMeta = buildBorgeeConversationMeta({
    channelLabel: params.channelLabel,
    channelType: isDm ? "dm" : "channel",
    message: msg,
  });

  const route = runtime.channel.routing.resolveAgentRoute({
    cfg: params.config as OpenClawConfig,
    channel: params.channelId,
    accountId: params.account.accountId,
    peer: {
      kind: isDm ? "direct" : "channel",
      id: rawChannelId,
    },
  });

  const storePath = runtime.channel.session.resolveStorePath(params.config.session?.store, {
    agentId: route.agentId,
  });

  const previousTimestamp = runtime.channel.session.readSessionUpdatedAt({
    storePath,
    sessionKey: route.sessionKey,
  });

  // Build the text envelope delivered to the agent.
  const body = runtime.channel.reply.formatAgentEnvelope({
    channel: params.channelLabel,
    from: msg.sender_name || msg.sender_id,
    timestamp: msg.created_at,
    previousTimestamp,
    envelope: runtime.channel.reply.resolveEnvelopeFormatOptions(params.config as OpenClawConfig),
    body: msg.content,
  });

  const ctxPayload = runtime.channel.reply.finalizeInboundContext({
    Body: body,
    BodyForAgent: msg.content,
    RawBody: msg.content,
    CommandBody: msg.content,
    From: buildBorgeeTarget(stripChannelPrefix(msg.sender_id)),
    To: target,
    SessionKey: route.sessionKey,
    AccountId: route.accountId ?? params.account.accountId,
    ChatType: conversationMeta.chatType,
    ConversationLabel: conversationMeta.conversationLabel,
    GroupSubject: conversationMeta.groupSubject,
    GroupChannel: conversationMeta.groupChannel,
    NativeChannelId: conversationMeta.nativeChannelId,
    NativeDirectUserId: conversationMeta.nativeDirectUserId,
    SenderName: msg.sender_name,
    SenderId: msg.sender_id,
    Provider: params.channelId,
    Surface: params.channelId,
    MessageSid: msg.id,
    MessageSidFull: msg.id,
    ReplyToId: msg.reply_to_id ?? undefined,
    Timestamp: msg.created_at,
    OriginatingChannel: params.channelId,
    OriginatingTo: target,
    CommandAuthorized: true,
  });

  await dispatchInboundReplyWithBase({
    cfg: params.config as OpenClawConfig,
    channel: params.channelId,
    accountId: params.account.accountId,
    route,
    storePath,
    ctxPayload,
    core: runtime,
    deliver: async (payload) => {
      const text =
        payload && typeof payload === "object" && "text" in payload
          ? ((payload as { text?: string }).text ?? "")
          : "";
      if (!text.trim()) return;

      await sendBorgeeMessage({
        baseUrl: params.account.baseUrl,
        apiKey: params.account.apiKey,
        channelId: msg.channel_id,
        content: text,
        replyToId: msg.id,
      });
    },
    onRecordError: (error) => {
      throw error instanceof Error
        ? error
        : new Error(`borgee session record failed: ${String(error)}`);
    },
    onDispatchError: (error) => {
      throw error instanceof Error
        ? error
        : new Error(`borgee dispatch failed: ${String(error)}`);
    },
  });
}

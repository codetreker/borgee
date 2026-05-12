import { resolveBorgeeAccount } from "./accounts.js";
import { sendBorgeeMessage, createOrGetBorgeeDm, addBorgeeReaction, removeBorgeeReaction, editBorgeeMessage, deleteBorgeeMessage } from "./api-client.js";
import { parseBorgeeTarget } from "./inbound.js";
import { getWsClient } from "./ws-util.js";
import type { CoreConfig, BorgeeMessage } from "./types.js";

export async function sendBorgeeText(params: {
  cfg: CoreConfig;
  accountId?: string | null;
  to: string;
  text: string;
  replyToId?: string | number | null;
}): Promise<{ to: string; messageId: string }> {
  const account = resolveBorgeeAccount({ cfg: params.cfg, accountId: params.accountId });
  const parsed = parseBorgeeTarget(params.to);

  let channelId = parsed.channelId;

  if (parsed.chatType === 'dm' && parsed.userId) {
    const wsClient = getWsClient(account);
    if (wsClient) {
      try {
        const res = await wsClient.apiCall('POST', `/api/v1/dm/${encodeURIComponent(parsed.userId)}`) as { status: number; body: { channel: { id: string } } };
        channelId = res.body.channel.id;
      } catch {
        const { channel } = await createOrGetBorgeeDm({
          baseUrl: account.baseUrl,
          apiKey: account.apiKey,
          userId: parsed.userId,
        });
        channelId = channel.id;
      }
    } else {
      const { channel } = await createOrGetBorgeeDm({
        baseUrl: account.baseUrl,
        apiKey: account.apiKey,
        userId: parsed.userId,
      });
      channelId = channel.id;
    }
  }

  const wsClient = getWsClient(account);
  if (wsClient) {
    try {
      const res = await wsClient.apiCall('POST', `/api/v1/channels/${encodeURIComponent(channelId)}/messages`, {
        content: params.text,
        content_type: 'text',
        reply_to_id: params.replyToId == null ? undefined : String(params.replyToId),
      }) as { status: number; body: { message: BorgeeMessage } };
      return { to: params.to, messageId: res.body.message.id };
    } catch {
      // Use the HTTP path if the WebSocket API call fails.
    }
  }

  const { message } = await sendBorgeeMessage({
    baseUrl: account.baseUrl,
    apiKey: account.apiKey,
    channelId,
    content: params.text,
    replyToId: params.replyToId == null ? undefined : String(params.replyToId),
  });

  return {
    to: params.to,
    messageId: message.id,
  };
}

export async function handleBorgeeReaction(params: {
  cfg: CoreConfig;
  accountId?: string | null;
  type: "add_reaction" | "remove_reaction";
  messageId: string;
  emoji: string;
}): Promise<void> {
  const account = resolveBorgeeAccount({ cfg: params.cfg, accountId: params.accountId });

  const wsClient = getWsClient(account);
  if (wsClient) {
    const method = params.type === "add_reaction" ? "PUT" : "DELETE";
    try {
      await wsClient.apiCall(method, `/api/v1/messages/${encodeURIComponent(params.messageId)}/reactions`, { emoji: params.emoji });
      return;
    } catch {
      // Use the HTTP path if the WebSocket API call fails.
    }
  }

  const fn = params.type === "add_reaction" ? addBorgeeReaction : removeBorgeeReaction;
  await fn({
    baseUrl: account.baseUrl,
    apiKey: account.apiKey,
    messageId: params.messageId,
    emoji: params.emoji,
  });
}

export async function handleBorgeeMessageEdit(params: {
  cfg: CoreConfig;
  accountId?: string | null;
  messageId: string;
  content: string;
}): Promise<{ messageId: string }> {
  const account = resolveBorgeeAccount({ cfg: params.cfg, accountId: params.accountId });

  const wsClient = getWsClient(account);
  if (wsClient) {
    try {
      const res = await wsClient.apiCall('PUT', `/api/v1/messages/${encodeURIComponent(params.messageId)}`, { content: params.content }) as { status: number; body: { message: BorgeeMessage } };
      return { messageId: res.body.message.id };
    } catch {
      // Use the HTTP path if the WebSocket API call fails.
    }
  }

  const { message } = await editBorgeeMessage({
    baseUrl: account.baseUrl,
    apiKey: account.apiKey,
    messageId: params.messageId,
    content: params.content,
  });
  return { messageId: message.id };
}

export async function handleBorgeeMessageDelete(params: {
  cfg: CoreConfig;
  accountId?: string | null;
  messageId: string;
}): Promise<void> {
  const account = resolveBorgeeAccount({ cfg: params.cfg, accountId: params.accountId });

  const wsClient = getWsClient(account);
  if (wsClient) {
    try {
      await wsClient.apiCall('DELETE', `/api/v1/messages/${encodeURIComponent(params.messageId)}`);
      return;
    } catch {
      // Use the HTTP path if the WebSocket API call fails.
    }
  }

  await deleteBorgeeMessage({
    baseUrl: account.baseUrl,
    apiKey: account.apiKey,
    messageId: params.messageId,
  });
}

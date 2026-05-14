export type BorgeeConversationChatType = "group" | "direct";

export type BorgeeConversationMeta = {
  chatType: BorgeeConversationChatType;
  conversationLabel: string;
  groupSubject?: string;
  groupChannel?: string;
  nativeChannelId: string;
  nativeDirectUserId?: string;
};

export type BorgeeConversationMessage = {
  channel_id: string;
  channel_name?: string;
  sender_id: string;
  sender_name?: string;
};

export function buildBorgeeConversationMeta(params: {
  channelLabel: string;
  channelType?: "channel" | "dm";
  message: BorgeeConversationMessage;
}): BorgeeConversationMeta {
  const providerLabel = normalizeLabel(params.channelLabel) ?? "Borgee";
  const nativeChannelId = params.message.channel_id;

  if (params.channelType === "dm") {
    const senderLabel =
      normalizeLabel(params.message.sender_name) ??
      normalizeLabel(params.message.sender_id) ??
      "unknown";
    return {
      chatType: "direct",
      conversationLabel: `${providerLabel} DM: ${senderLabel}`,
      nativeChannelId,
      nativeDirectUserId: normalizeLabel(params.message.sender_id),
    };
  }

  const channelName = normalizeLabel(params.message.channel_name);
  const groupChannel = channelName ? formatGroupChannel(channelName) : undefined;
  const fallbackSubject = `${providerLabel} group chat`;

  return {
    chatType: "group",
    conversationLabel: groupChannel ? `${providerLabel}/${groupChannel}` : fallbackSubject,
    groupSubject: channelName ?? fallbackSubject,
    groupChannel,
    nativeChannelId,
  };
}

function normalizeLabel(value: string | undefined): string | undefined {
  const trimmed = value?.trim();
  return trimmed ? trimmed : undefined;
}

function formatGroupChannel(channelName: string): string {
  return channelName.startsWith("#") ? channelName : `#${channelName}`;
}

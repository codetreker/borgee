import { describe, expect, it } from "vitest";
import { buildBorgeeConversationMeta } from "./conversation-meta.js";

describe("buildBorgeeConversationMeta", () => {
  it("uses a channel name instead of the raw channel id for group sessions", () => {
    const meta = buildBorgeeConversationMeta({
      channelLabel: "Borgee",
      channelType: "channel",
      message: {
        channel_id: "01KR0DKAPXR8DT7TJ82M1X6G88",
        channel_name: "product-room",
        sender_id: "user-1",
        sender_name: "Alice",
      },
    });

    expect(meta.chatType).toBe("group");
    expect(meta.conversationLabel).toBe("Borgee/#product-room");
    expect(meta.groupChannel).toBe("#product-room");
    expect(meta.groupSubject).toBe("product-room");
    expect(meta.nativeChannelId).toBe("01KR0DKAPXR8DT7TJ82M1X6G88");
    expect(meta.conversationLabel).not.toBe("01KR0DKAPXR8DT7TJ82M1X6G88");
  });

  it("uses the sender display name instead of the raw DM channel id for direct sessions", () => {
    const meta = buildBorgeeConversationMeta({
      channelLabel: "Borgee",
      channelType: "dm",
      message: {
        channel_id: "01KR0DIRECTCHANNEL0000000000",
        sender_id: "user-2",
        sender_name: "Bob",
      },
    });

    expect(meta.chatType).toBe("direct");
    expect(meta.conversationLabel).toBe("Borgee DM: Bob");
    expect(meta.groupChannel).toBeUndefined();
    expect(meta.groupSubject).toBeUndefined();
    expect(meta.nativeChannelId).toBe("01KR0DIRECTCHANNEL0000000000");
    expect(meta.nativeDirectUserId).toBe("user-2");
    expect(meta.conversationLabel).not.toBe("01KR0DIRECTCHANNEL0000000000");
  });
});

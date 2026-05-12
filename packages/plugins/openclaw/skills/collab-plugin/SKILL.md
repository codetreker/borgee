---
name: collab-plugin
description: Use the Borgee OpenClaw channel plugin to communicate on a self-hosted Borgee team chat instance. Covers message sending, reactions, editing, direct messages, channel management, file access, and workspace operations. Use when an OpenClaw agent needs to interact with Borgee or when debugging Borgee plugin connectivity, authentication, or message delivery issues.
---

# Borgee OpenClaw Plugin

Borgee is a self-hosted real-time team chat platform. The OpenClaw channel plugin connects agents to a Borgee server.

## Quick Config

```yaml
channels:
  collab:
    accounts:
      main:
        baseUrl: "https://collab.example.com"
        apiKey: "your-agent-api-key"
        defaultTo: "channel:general"
```

## Message Targets

- `channel:<id>` → group channel
- `dm:<userId>` → direct message

## Core Operations

Send: `message(action=send, channel=collab, target="channel:<id>", message="text")`
Reply: add `replyTo: "<messageId>"`
React: `message(action=react, channel=collab, emoji="👍", messageId="<id>")`
Edit: `message(action=edit, channel=collab, messageId="<id>", message="new text")`
Delete: `message(action=delete, channel=collab, messageId="<id>")`
Read: `message(action=read, channel=collab, target="channel:<id>", limit=50)`

## Auth

All API calls use `Authorization: Bearer <apiKey>` header. Never pass keys via query string.

## Real-time Transport

Plugin auto-selects: WebSocket (`/ws`) → SSE (`/api/v1/stream`) → Long-poll (`/api/v1/poll`).

## References (load as needed)

- **Full API reference**: See [references/api.md](references/api.md) for all 50+ endpoints
- **Debugging guide**: See [references/debugging.md](references/debugging.md) for connectivity, auth, and message delivery troubleshooting

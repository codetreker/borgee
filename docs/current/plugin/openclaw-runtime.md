# OpenClaw Runtime

This page documents the OpenClaw package/runtime side. It does not describe server BPP dispatcher wiring; see `../server/bpp-internals.md` for that.

## Package Shape

Responsible for: package identity, OpenClaw extension entry, and channel registration metadata. Not responsible for: server process startup or route registration.

The package is `@codetreker/borgee-openclaw-plugin`, version `0.1.1`, with `openclaw >= 2026.4.15` as peer dependency. It publishes `dist`, `openclaw.plugin.json`, and `skills`. The OpenClaw package metadata points extensions at `./dist/index.js`, and `src/index.ts` exports a bundled channel entry with id `borgee`, plugin specifier `./channel.js#borgeePlugin`, and runtime setter `./runtime.js#setBorgeeRuntime`. Evidence: `packages/plugins/openclaw/package.json`, `packages/plugins/openclaw/openclaw.plugin.json`, `packages/plugins/openclaw/src/index.ts`, `packages/plugins/openclaw/src/runtime.ts`.

`openclaw.plugin.json` is minimal: id `borgee`, channels `["borgee"]`, skills `./skills`, and an empty strict `configSchema` object at that manifest layer. The richer channel config schema is exported by the channel plugin code. Evidence: `packages/plugins/openclaw/openclaw.plugin.json`, `packages/plugins/openclaw/src/config-schema.ts`.

## Channel Plugin

Responsible for: OpenClaw channel contract implementation. Not responsible for: Borgee server authorization or event persistence.

`channel.ts` creates a chat channel plugin for id `borgee`, declares group and direct chat support, normalizes targets, parses explicit `channel:<id>` and `dm:<user_id>` destinations, builds outbound session routes, resolves account configuration, starts one gateway per account, and wires outbound text sending. Evidence: `packages/plugins/openclaw/src/channel.ts`, `packages/plugins/openclaw/src/inbound.ts`, `packages/plugins/openclaw/src/outbound.ts`.

The runtime store is created through OpenClaw's `createPluginRuntimeStore`; `runtime-api.ts` re-exports the OpenClaw channel/runtime APIs used by the plugin. Evidence: `packages/plugins/openclaw/src/runtime.ts`, `packages/plugins/openclaw/src/runtime-api.ts`.

## Accounts And Config

Responsible for: resolving OpenClaw config into an enabled/configured Borgee account. Not responsible for: server-side agent config blobs under `/api/v1/agents/{id}/config`.

Account resolution uses OpenClaw account helpers and `resolveMergedAccountConfig`. A resolved account requires `baseUrl` and `apiKey`, defaults `botUserId` to `openclaw-agent`, defaults `botDisplayName` to `OpenClaw`, defaults `pollTimeoutMs` to 30 seconds, defaults transport to `auto`, and sets `allowFrom` to `['*']` when not provided. Evidence: `packages/plugins/openclaw/src/accounts.ts`, `packages/plugins/openclaw/src/types.ts`.

The channel config schema exposes `name`, `enabled`, `baseUrl`, `apiKey`, bot identity fields, `pollTimeoutMs`, `transport`, `allowFrom`, `defaultTo`, per-account overrides, and `defaultAccount`. Current schema accepts only `auto`, `sse`, and `poll`; `types.ts` and gateway code also include `ws`, which is a documented current mismatch. Evidence: `packages/plugins/openclaw/src/config-schema.ts`, `packages/plugins/openclaw/src/types.ts`, `packages/plugins/openclaw/src/gateway.ts`.

Runtime status is computed from account snapshots and redacts configured API keys as `***`. Evidence: `packages/plugins/openclaw/src/status.ts`.

## Inbound Event Conversion

Responsible for: turning Borgee events into OpenClaw inbound contexts and delivering generated replies. Not responsible for: server event cursor generation.

The gateway filters inbound Borgee events to message/edit/delete/reaction kinds. It skips self messages by bot user id, and for non-DM message events enforces `requireMention` when bot identity requires it. Message events are wrapped with channel/from/timestamp context and dispatched through `dispatchInboundReplyWithBase`; generated text is sent back as a Borgee message, replying to the source message id when available. Evidence: `packages/plugins/openclaw/src/gateway.ts`, `packages/plugins/openclaw/src/sse-client.ts`, `packages/plugins/openclaw/src/inbound.ts`.

Reaction events are converted into text envelopes such as a reaction add/remove notice, routed through the same OpenClaw inbound reply path, and any generated text is sent back to the event channel. Evidence: `packages/plugins/openclaw/src/inbound.ts`, `packages/plugins/openclaw/src/api-client.ts`.

## Outbound Actions

Responsible for: sending OpenClaw action results back to Borgee. Not responsible for: Borgee message storage or channel ACL enforcement.

Outbound text resolves `channel:` or `dm:` targets. DM targets first create or get a Borgee DM channel. When a connected plugin WS client exists, outbound text/reaction/edit/delete tries `/ws/plugin` `api_request` first; on failure or no WS client, it falls back to REST helpers. Evidence: `packages/plugins/openclaw/src/outbound.ts`, `packages/plugins/openclaw/src/api-client.ts`, `packages/plugins/openclaw/src/ws-util.ts`.

## Local Process State

Responsible for: plugin-local resumability and optional file read service. Not responsible for: server SQLite or borgee-helper host grants.

Cursor persistence is a local JSON file at `${OPENCLAW_DATA_DIR || HOME || .}/data/collab-cursor-<accountId>.json`; corrupt/unreadable files are ignored and writes are best effort. Evidence: `packages/plugins/openclaw/src/cursor-store.ts`.

The plugin WS request path can read local files through `file-access.ts`. It reads `~/.config/collab/file-access.json`, requires the requested path to be under an allowed path, enforces a default 1 MiB max file size, and returns text plus MIME metadata or an error string. This is separate from `borgee-helper`. Evidence: `packages/plugins/openclaw/src/file-access.ts`, `packages/plugins/openclaw/src/gateway.ts`, `packages/plugins/openclaw/src/ws-client.ts`.

# Server Contracts

This page lists the server-facing contracts consumed by the OpenClaw plugin. It describes the plugin side of those contracts; server internals are in `../server/`.

## Contract Map

| Contract | Plugin Responsibility | Server Responsibility | Evidence |
| --- | --- | --- | --- |
| Identity | Call `GET /api/v1/users/me` with API key and use returned bot user fields | Authenticate API key and return current user | `packages/plugins/openclaw/src/api-client.ts`, `packages/server-go/internal/server/server.go` |
| Event stream | Consume `/api/v1/stream`, parse SSE frames, send `Last-Event-ID` | Emit `event`, `id`, `data`, heartbeat, and backfill from `Last-Event-ID` | `packages/plugins/openclaw/src/sse-client.ts`, `packages/server-go/internal/api/poll.go` |
| Poll fallback | POST cursor/timeout to `/api/v1/poll`, dispatch returned events | Filter by channel ACL, wait on Hub signal, return cursor/events | `packages/plugins/openclaw/src/api-client.ts`, `packages/server-go/internal/api/poll.go` |
| Outbound REST | Send messages, reactions, edits, deletes, DM creation | Persist messages and enforce server ACL/auth | `packages/plugins/openclaw/src/api-client.ts`, `packages/plugins/openclaw/src/outbound.ts` |
| Plugin WS RPC | Optionally send `/ws/plugin` `api_request` and answer `request` frames | Authenticate plugin socket and replay `api_request` into server HTTP handler | `packages/plugins/openclaw/src/ws-client.ts`, `packages/server-go/internal/ws/plugin.go` |
| BPP frames | OpenClaw package currently consumes event transports; server BPP SDK/package contract is separate | Define envelopes, route upstream frames, push config/permission frames | `packages/server-go/internal/bpp/envelope.go`, `packages/server-go/internal/bpp/plugin_frame_dispatcher.go`, `packages/server-go/sdk/bpp/*` |

## REST Contracts

Responsible for: OpenClaw HTTP calls to Borgee server. Not responsible for: server handler implementation details.

`BorgeeApiClient` and standalone helpers use bearer auth and JSON request/response bodies. They call `GET /api/v1/users/me`, `POST /api/v1/poll`, `GET /api/v1/channels`, `POST /api/v1/channels/{id}/messages`, `GET /api/v1/channels/{id}/messages`, reaction add/remove, message edit/delete, and `POST /api/v1/dm/{userId}`. Evidence: `packages/plugins/openclaw/src/api-client.ts`.

Outbound helpers prefer WS RPC when `getWsClient(account)` returns a connected client, then fall back to the REST helpers above. This fallback keeps the plugin usable even when the WS transport path is not active. Evidence: `packages/plugins/openclaw/src/outbound.ts`, `packages/plugins/openclaw/src/ws-util.ts`.

## Event Shape Contract

Responsible for: plugin-side event parsing and filtering. Not responsible for: server event insertion call sites.

The plugin event type includes `cursor`, `kind`, `channel_id`, `payload`, and `created_at`. Supported kinds are message, message edit, message delete, mention, channel membership changes, channel creation, and reaction update at the type level; current dispatch paths process message/edit/delete/reaction update. Payload is parsed from JSON string before inbound conversion. Evidence: `packages/plugins/openclaw/src/types.ts`, `packages/plugins/openclaw/src/gateway.ts`, `packages/plugins/openclaw/src/sse-client.ts`.

On the server, poll/SSE/backfill response rows use `cursor`, `kind`, `channel_id`, `payload`, and `created_at`, and are filtered to the authenticated user's channel membership. Evidence: `packages/server-go/internal/api/poll.go`.

## `/ws/plugin` Contract

Responsible for: plugin-side optional websocket RPC and local request handling. Not responsible for: browser `/ws` behavior.

The plugin WS client connects to `${baseUrl}/ws/plugin` with bearer auth. It sends `api_request` frames shaped as `{type:"api_request", id, data:{method,path,body}}` and expects `api_response` with the same id. It also handles server `request` frames and currently supports `read_file` by calling `file-access.ts`. Evidence: `packages/plugins/openclaw/src/ws-client.ts`, `packages/plugins/openclaw/src/gateway.ts`, `packages/plugins/openclaw/src/file-access.ts`.

server-go's `/ws/plugin` authenticates by bearer or `?apiKey=`, registers the plugin connection under the authenticated user id, handles `api_request` by creating an in-process HTTP request with the plugin API key, handles `api_response`/`response`, and routes all other frame types to BPP dispatch. Evidence: `packages/server-go/internal/ws/plugin.go`, `packages/server-go/internal/server/server.go`, `packages/server-go/internal/bpp/plugin_frame_dispatcher.go`.

## BPP Contract Touchpoints

Responsible for: identifying where plugin-facing BPP exists today. Not responsible for: claiming the OpenClaw package implements all BPP envelopes.

Server BPP envelopes include server-to-plugin config/permission/control frames and plugin-to-server heartbeat/semantic/error/config-ack/task/reconnect/cold-start frames. server-go currently registers upstream handlers for config ack, reconnect, cold start, and task lifecycle. The in-tree Go SDK mirrors BPP frames by importing `internal/bpp`, but OpenClaw's TypeScript package uses its own SSE/poll/WS RPC code paths and does not import that Go SDK. Evidence: `packages/server-go/internal/bpp/envelope.go`, `packages/server-go/internal/server/server.go`, `packages/server-go/sdk/bpp/client.go`, `packages/server-go/sdk/bpp/reconnect.go`, `packages/plugins/openclaw/src/*`.

## Local File Request Contract

Responsible for: OpenClaw plugin-local file reads over plugin WS request frames. Not responsible for: `remote-agent` or `borgee-helper` filesystem access.

When the plugin WS request handler receives `{action:"read_file", path:"..."}`, it checks `~/.config/collab/file-access.json`, verifies the path is inside an allowed path, enforces max file size, and returns content, size, and MIME type or an error code. This is plugin-local and separate from server remote-node or helper-host-grant paths. Evidence: `packages/plugins/openclaw/src/gateway.ts`, `packages/plugins/openclaw/src/file-access.ts`.

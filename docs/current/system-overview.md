# System Overview

Borgee is a browser-first collaboration system with a Go server at the center. The server owns identity, authorization, durable collaboration state, static assets, realtime fanout, plugin protocol routing, and remote-node coordination. Browser apps, OpenClaw plugins, remote agents, and host-bridge helpers connect to that core through narrow process boundaries.

## System Shape

```mermaid
flowchart TB
  humans[Humans and admins]
  browser[Browser apps]
  core[server-go core]
  durable[Durable state]
  agents[External runtimes]
  host[Host bridge]

  humans --> browser
  browser --> core
  core --> durable
  agents --> core
  host --> durable
```

The important design choice is centralization: server-go is the only component that owns collaboration state and policy. External runtimes can observe, send, or proxy through server contracts, but they do not become peer sources of truth.

## Boundaries

| Boundary | What Crosses It | Why It Exists |
| --- | --- | --- |
| Browser to server | User API calls, realtime frames, cursor backfill | Keeps UI optimistic but server-authoritative |
| Plugin to server | Event consumption, outbound actions, BPP/RPC frames | Lets OpenClaw act as a runtime without owning Borgee state |
| Remote-agent to server | File listing/read requests and responses | Keeps machine-local IO outside server-go |
| Helper to grants DB | Read-only grant checks | Keeps host bridge policy grounded in server-owned data |
| Installer to server/helper | Signed manifest and service deployment | Separates distribution from runtime behavior |

## Design Principles

- The server is the source of truth for users, channels, messages, permissions, agent config, event cursors, and plugin liveness interpretation.
- Realtime is best-effort delivery plus cursor recovery; clients and plugins use backfill or pull paths to converge.
- BPP is a server/plugin control plane, not a replacement for all browser realtime.
- Host bridge and remote-agent file access are separate paths; they should not be conflated with OpenClaw plugin-local file reads.

## Out Of Scope

This page does not describe UI component state, admin screen layout, or individual database migrations.

## Implementation Anchors

- Server composition: `packages/server-go/internal/server/server.go`, `Server`
- Hub and realtime model: `packages/server-go/internal/ws`, `Hub`, `Client`, `PluginConn`, `RemoteConn`
- Plugin package: `packages/plugins/openclaw/src`, `BorgeeEvent`, `ResolvedBorgeeAccount`
- Host bridge and installer: `packages/borgee-helper`, `packages/borgee-installer`

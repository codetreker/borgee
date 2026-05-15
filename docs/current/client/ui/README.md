# Client UI Architecture Sketches

These files are Interaction And Layout Reference sketches for the user SPA. They help maintainers understand surface placement, navigation shape, and interaction flow after reading [../ui-map.md](../ui-map.md) and [../feature-surfaces.md](../feature-surfaces.md).

They do not define product behavior, implementation contracts, verification status, or design-system rules. Current architecture, state ownership, and data authority remain in the parent client documents.

## Catalog

| Architecture surface | Sketches | Use them for |
| --- | --- | --- |
| Auth gate | [login.md](login.md) | Login/register placement and basic form shape. |
| Shell and channel host | [main-desktop.md](main-desktop.md), [main-mobile.md](main-mobile.md) | Desktop/mobile shell layout and channel tab placement. |
| Channel rail | [channel-sort-groups.md](channel-sort-groups.md) | Channel grouping, owner controls, and read-only member views. |
| Chat and DM | [message.md](message.md), [dm.md](dm.md), [slash-commands.md](slash-commands.md), [preview.md](preview.md) | Message layout, direct messages, command panel shape, and public preview flow. |
| Canvas and workspace | [canvas-modal.md](canvas-modal.md), [workspace.md](workspace.md) | Artifact decision flows, file tree layout, and file viewer shape. |
| Agents | [agent-manager.md](agent-manager.md), [agent-config.md](agent-config.md), [agent-collab.md](agent-collab.md) | Owner-side agent management, config editing, and collaboration visibility. |
| Sidepanes and settings | [sidepane.md](sidepane.md), [settings.md](settings.md) | Global sidepane navigation, channel-management overview, and user admin-awareness layout. |

When following admin or remote-agent reader paths, use their module entries first: [../../admin/README.md](../../admin/README.md) for admin SPA context and [../../remote-agent/README.md](../../remote-agent/README.md) for Remote Agent protocol and boundary context. Their UI sketches sit under those modules as supporting interaction references.

## Notation

| Symbol | Meaning |
| --- | --- |
| Green/yellow/dark status dots | Online, away, and offline presence indicators. |
| Robot marker | Agent identity in a user-facing surface. |
| `AV` | Avatar placeholder. |
| Eye icon | Show/hide affordance for masked values. |
| Copy icon or `Copy` | Copy-to-clipboard affordance. |
| `X` icon | Close affordance for modal or panel sketches. |
| `░░` | Dimmed background or modal overlay. |

## Reading Order

1. Start with [../README.md](../README.md) for the user SPA boundary.
2. Use [../ui-map.md](../ui-map.md) to place the surface in the shell hierarchy.
3. Use [../feature-surfaces.md](../feature-surfaces.md) to identify state ownership and data authority.
4. Open the sketch only after those boundaries are clear.

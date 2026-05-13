# Settings And Admin-Awareness Sketch

## Purpose

This sketch is an Interaction And Layout Reference for the user settings sidepane and admin-awareness content. It does not define product behavior, implementation contracts, privacy policy, copy authority, or verification status.

## Surface

Settings is a global sidepane in the user SPA. It can show user-owned privacy/admin-impact metadata and impersonation grant state without creating an admin session or mounting the admin SPA.

## Interaction Model

- The user opens settings from the shell navigation rail.
- Settings uses the same sidepane navigation model as agents, invitations, workspaces, and remote nodes.
- Admin-awareness content is scoped to the signed-in user.
- Grant state can affect a shell-level banner, but the settings form state remains local to the surface.

## Layout Sketch

```
+──────────────────────────────────────────────+
│  Settings                              [Back] │
├──────────────────────────────────────────────┤
│  Privacy                                      │
│                                              │
│  Admin visibility                            │
│  - Account and channel metadata              │
│  - No message, file, or artifact body view   │
│    unless a user-controlled grant is active  │
│                                              │
│  Temporary support grant                     │
│  Current status: not granted                 │
│  [Grant 24h]                                 │
│                                              │
│  Admin impact history                        │
│  No recent admin impact records              │
+──────────────────────────────────────────────+
```

## Architecture Notes

- This is a user rail surface backed by user endpoints, not an admin SPA page.
- The admin privacy/audit module owns the durable audit projection and current limitations.
- The shell may show a global banner when user-owned grant state is active.
- Settings should not become a viewer for admin-wide audit data.

## Related Docs

- `../feature-surfaces.md`
- `../ui-map.md`
- `../../admin/privacy-audit.md`
- `../../admin/spa.md`

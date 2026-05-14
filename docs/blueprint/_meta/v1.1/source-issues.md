# Source Issues For Blueprint v1.1 Candidate

Picked backlog inputs grouped by next-blueprint anchor. This file is traceability only; selected issues are not current behavior and do not replace the status ledger in `docs/blueprint/next/README.md`.

## `HB-RA-1` Helper Bounded Remote Actuator

- gh#681 — Expand Helper host-management onboarding so Web-side Configure OpenClaw can install the plugin, create or update agent config, and bind channels through a bounded Helper remote-actuator path, without reusing Remote Agent file-proxy credentials or authority.
- gh#659 — Ensure the long-lived helper / agent service path needed for Configure OpenClaw survives OS reboot and crash restart without turning the installer into a persistent privileged daemon.

## `MR-1` Mention Routing

- gh#674 — Add per-channel `requireMention` control without letting channel owners broaden external-agent attention or capability beyond owner authorization.
- gh#693 — Add `@Everyone` broadcast semantics with server-authoritative fanout, ACL filtering, rate limits, and loop prevention.

## `CH-1` Channel Authority

- gh#685 — Add a user-side Channel management surface for ownership, membership, and allowed channel actions.
- gh#688 — Clarify that owners do not leave self-owned channels; management or deletion is the appropriate path.
- gh#690 — Reduce private-channel lock visual weight and prevent conflicts with unread, fault, or presence indicators.

## `CT-1` Client Truthfulness

- gh#724 — Make already-built client surfaces actually reachable in production, especially ArtifactComments mount, ACL forbidden UX, and security/permission bundle UI.

## `PS-1` Privacy Scope Guard

- gh#654 — Avoid expanding user-facing privacy/compliance product scope while preserving backend security, admin, capability, and privacy boundaries.

## `IA-1` Sidebar And Account IA

- gh#669 — Reduce sidebar footer clutter to a small set of primary entries.
- gh#670 — Make the lower-left avatar the account entry and move logout into the account panel.

## Conditional Inputs Not Picked

- gh#702 — Bring in only if agent config / onboarding copy is reopened.
- gh#707 and gh#697 — Keep quality gate / a11y follow-up in backlog unless explicitly pulled into `CT-1`.
- gh#607 — Keep file naming maintenance in backlog.
- gh#675 — Keep visual redesign out unless the user opens a separate visual redesign discussion.

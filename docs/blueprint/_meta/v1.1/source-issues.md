# Source Issues — Blueprint v1.1

Picked backlog inputs grouped by next-blueprint anchor, with the GitHub issue/PR evidence that delivered each anchor's accepted scope. This file is traceability only; selected issues are not current behavior and do not replace the status ledger in `docs/blueprint/next/README.md`.

## Per-anchor traceability

| Anchor | Source issue(s) | Delivered by PR(s) | Promoted into |
|---|---|---|---|
| `HB-RA-1A` | gh#681, gh#659 | #934, #936, #937, #938, #939, #942, #943 (M1 t1-t7) | `current/host-bridge.md` §1.2 / §2 / §3 |
| `HB-RA-1B` | gh#681, gh#659 | #954, #956, #958, #963, #964 (M1 t8-t12) | `current/host-bridge.md` §1.6 / §3.1-§3.3 |
| `MR-1` | gh#674, gh#693 | #949 (M2 t1), #951 (M2 t2), #955 (M2 t3) | `current/channel-model.md` §5 |
| `CH-1` | gh#685, gh#688, gh#690 | #948 (M2 t4), #953 (M2 t5), #959 (M2 t6), #945 (M2 t7), #952 (M2 t8), #961 (M2 t9), #986 (M2 t10) | `current/channel-model.md` §6 |
| `CT-1` | gh#724 | #946 (M3 t1), #957 (M3 t2), #944 (M3 t3), #960 (M3 t4) | `current/client-shape.md` §5 + `canvas-vision.md` §6 |
| `PS-1` | gh#654 | preserved across all v1.1 PRs (no new user-facing privacy/compliance surface) | `current/host-bridge.md` §1.2/§2 (rail separation), `client-shape.md` §5.3, `admin-model.md` (no expansion) |
| `IA-1` | gh#669, gh#670 | #947 (M3 t5), #950 (M3 t6), #962 (M3 t7) | `current/client-shape.md` §6 |

## Conditional inputs not picked

| Issue | Reason held in backlog |
|---|---|
| gh#702 | Bring in only if agent config / onboarding copy is reopened. |
| gh#707 | Quality gate / a11y follow-up; backlog unless explicitly pulled into `CT-1`. |
| gh#697 | Same as gh#707. |
| gh#607 | File naming maintenance; backlog. |
| gh#675 | Visual redesign; backlog unless separately opened. |

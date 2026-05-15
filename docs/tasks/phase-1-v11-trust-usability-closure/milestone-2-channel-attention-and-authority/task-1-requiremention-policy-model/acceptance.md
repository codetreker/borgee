# Acceptance: requireMention Policy Model

## Source Alignment

- Task: `task-1-requiremention-policy-model`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority`
- Blueprint anchors: `migration-analysis.md` section 3.3 and section 6.1
- Dependency: explicit parallel Milestone 2 task start from the user; independent from Helper/OpenClaw task 6.

## Segment A: Policy Storage

Acceptance checks:

- Channel memberships store a per-channel attention policy with only `inherit`, `on`, or `off` values.
- Existing rows default to `inherit` and continue to resolve through the agent's global setting.

Negative checks:

- Invalid policy literals cannot persist.
- No history backfill or mention row mutation occurs.

## Segment B: Policy Resolution

Acceptance checks:

- `inherit` follows the agent owner's global `require_mention` value.
- `on` requires mention even when the owner has globally allowed broader delivery.
- `off` disables mention-required behavior only when the agent owner has globally opted into broader delivery.

Negative checks:

- `off` cannot broaden an agent whose owner global policy still requires mention.

## Segment C: API Authority

Acceptance checks:

- A channel manager can update an agent member's policy.
- Non-managers are rejected.
- Human members and non-members are rejected for this policy.

Negative checks:

- Cross-channel or cross-org callers cannot update policy through membership alone.
- No admin route is added.

## Segment D: Mention/Message Integration

Acceptance checks:

- Unmentioned agent recipients are selected only when effective policy allows non-mention delivery.
- Explicit `@agent` mention validation and dispatch remains unchanged.

Negative checks:

- No `@Everyone` behavior, client-supplied recipient IDs, or offline owner-body forwarding enters this task.

## Segment E: Current Docs And Verification

Acceptance checks:

- Current docs describe the implemented server/data/security boundary and the remaining client-control gap.
- Focused API/store/migration tests and relevant broader server tests pass.

## Acceptance Evidence

| Check | Evidence | Result |
|---|---|---|
| Segment A: policy storage | Migration v52 and store baseline add `channel_members.require_mention_policy` with `inherit` / `on` / `off`; migration and store tests verify default `inherit` and invalid literal rejection. | PASS |
| Segment B: policy resolution | Store tests verify inherit follows global `require_mention`, `on` forces mention-required, `off` works only after global opt-out, and rejected `off` does not mutate stored policy. | PASS |
| Segment C: API authority | API tests verify manager success, invalid policy rejection, non-manager 403, human target 400, and owner-ceiling 400 before global opt-out. | PASS |
| Segment D: mention/message integration | API message integration test verifies inherit/require-mention does not implicitly dispatch, `off` dispatches without writing `message_mentions`, and explicit mention still writes `message_mentions`. | PASS |
| Segment E: current docs and verification | Current docs updated under `docs/current/server`, `docs/current/security`, and `docs/current/known-gaps`; focused package and full server-go test suites pass. | PASS |

Verifier: owner worker
Date: 2026-05-15
Scope: API/data/security/current-doc
Fixtures: server-go test fixtures; secrets redacted
Out-of-scope findings: N/A
Decision: LGTM for PR review

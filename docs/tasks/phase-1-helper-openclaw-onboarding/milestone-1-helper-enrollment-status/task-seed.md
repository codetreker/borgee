# Task Seed: task-1-helper-enrollment-model-and-status

## Purpose

Create the first implementation task for Helper enrollment/status foundation so later typed jobs have a trustworthy authority and visibility base.

## Source Anchors

- `HB-RA-1A`: bounded Helper actuator guardrails.
- `HB-RA-1B`: helper credential, stale-device, revoke/uninstall, and status/log execution-contract requirements.
- `PS-1`: preserve existing admin/privacy/security controls without new compliance-product scope.

## Expected PR Atom

- Add the Helper enrollment model and owner/org/host binding needed for visibility and later authorization.
- Add connected/offline/last-seen/revoked/uninstalled status shape.
- Add server-side tests for owner/org isolation and Remote Agent credential non-reuse.
- Update `docs/current` for accepted enrollment/status contracts.

## Non-Goals

- No job queue, lease, result, local policy engine, service lifecycle, or Configure OpenClaw action execution.
- No Remote Agent credential reuse or rail merge.
- No user-facing privacy/compliance product surface.

## First Acceptance Check

A reviewer can see a Helper enrollment as a distinct host-management authority with truthful status and revoke/uninstall state, while Remote Agent file-proxy authority remains separate.

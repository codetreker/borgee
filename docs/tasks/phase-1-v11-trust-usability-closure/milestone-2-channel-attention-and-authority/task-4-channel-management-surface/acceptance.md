# Acceptance: Channel Management Surface

## Source Alignment

- Task: `task-4-channel-management-surface`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority`
- Blueprint anchors: `migration-analysis.md` section 4.3; `docs/blueprint/next/README.md` section 2.4; `migration-analysis.md` section 6.1

## Segment A: Settings Entry

Acceptance checks:

- Settings exposes a reachable channel-management entry without adding a new sidebar/footer primary entry.
- The entry keeps existing Settings privacy behavior reachable.

Negative checks:

- No Milestone 3 sidebar/footer production entry is added.
- No private-channel sidebar indicator treatment is changed.

## Segment B: Created And Joined Listing

Acceptance checks:

- The channel management surface renders channels created by the current user in a distinct created section.
- It renders channels joined by the current user but created by someone else in a distinct joined section.
- Created channels are not duplicated in the joined-only section.

Negative checks:

- The surface does not render leave/delete/archive/owner-transfer controls.
- The surface does not rewrite notification, collapse, sort, pin, or group behavior.

## Segment C: API/Client Authority

Acceptance checks:

- Client grouping uses channel `created_by`, `is_member`, and current user id from existing app state or existing channel API response.
- API/client tests prove the existing channel list payload carries the metadata needed by the surface without adding a mutation endpoint.

Negative checks:

- No client-only ownership or membership guess is used when the API field is absent.
- No new server action endpoint is added in this task.

## Segment D: Empty And Privacy-Safe States

Acceptance checks:

- Empty created/joined states render clear copy when each section has no rows.
- Rows show only channel metadata already present in the authorized channel list.

Negative checks:

- No private channel bodies, message content, file content, or hidden channel names are exposed.

## Verification Evidence

Record fresh command evidence in `progress.md` before PR open:

- Focused red/green client tests for channel management classification and Settings entry.
- API/client test for channel list metadata used by management classification.
- Client typecheck.
- Client test suite or focused suite plus documented reason for any narrower run.
- Current-doc sync review for changed Settings/channel-management behavior.

## Recorded Evidence

| Check | Evidence | Result |
|---|---|---|
| RED tests | Missing `channelManagement` helper and `ChannelManagementSurface` component caused the new tests to fail before production implementation | PASS |
| Client behavior | Full Vitest suite reported `131` files and `829` tests passed with `1` skipped after implementation | PASS |
| Typecheck | `timeout 600s pnpm --filter @borgee/client typecheck` exited 0 after implementation | PASS |
| Build | `timeout 600s pnpm --filter @borgee/client build` exited 0 after implementation | PASS |
| Docs/current | Current docs updated for Settings channel-management tab and display-only action gap | PASS |

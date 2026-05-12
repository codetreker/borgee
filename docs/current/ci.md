# CI Lint — Current 同步

> Introduced in Phase 0 / Task 28. Blueprint: README rule 4 (required PR description) + 规则 6 (required Current 同步 check).

## 1. Active Lint Job

`.github/workflows/lint.yml` runs one job on every PR to enforce the Current 同步 part of 规则 6:

### `pr-lint`

Check name: `PR lint (current 同步)`.

Step name: `current sync (规则 6)`.

The step reads the PR body and diff. Module mappings live in `.github/lint-current-sync.yml`:

```
packages/server-go/internal/  → docs/current/server/
packages/server-go/cmd/       → docs/current/server/
packages/client/src/          → docs/current/client/
packages/plugins/             → docs/current/plugin/
packages/helper/              → docs/current/helper/
packages/remote-agent/        → docs/current/remote-agent/
```

If a PR changes a `code_prefix` but not the matching `docs_prefix`, the job fails.

**`exclude_globs`**: Changes limited to `_test.go`, `*.test.ts(x)`, `*.spec.ts(x)`, `__snapshots__/`, or `testdata/` are not treated as behavior changes and do not trigger current-sync. This came from review feedback on PR #170. Keep the `exclude_globs:` list in `.github/lint-current-sync.yml` synchronized with the workflow filter.

**Opt-out**: If the PR body writes `- N/A — <reason>` under `## Current 同步`, the job emits a warning instead of failing. Reviewers still decide whether the reason is valid.

The workflow does not currently enforce the Blueprint, Touches, Acceptance, or Stage fields from the PR template. Those fields remain reviewer-checked process requirements unless a future workflow adds explicit validation.

## 2. PR Template

`.github/pull_request_template.md` provides the placeholder structure:

- `## What` — 1-3 sentences explaining why the change is needed
- `## Blueprint: <module> §X.Y` — gate-2 anchor
- `## Touches` — subsystem list; 2 or more subsystems require split PRs (interface-contract PR ≤300 lines, then implementation PRs)
- `## Current 同步` — `docs/current/...` list or `N/A — reason`
- `## Acceptance` — choose one acceptance form; standout milestones require both 4.1 and 4.2
- `Stage: v0` line

## 3. Phase 0 Gate Mapping

- G0.5 Current 同步 CI lint active → `PR lint (current 同步)` fails on a PR that changes a mapped code path without the matching docs/current path, then passes when the docs/current path is included.
- A documented `N/A` under `## Current 同步` can downgrade the missing-docs case to a warning. Reviewers still decide whether the reason is valid.
- G0.3 PR-template validation is not currently automated in `.github/workflows/lint.yml`.

## 4. Out Of Scope

- Auto-generating PR bodies. Rule 4 requires authors to write the body; lint should not replace review judgment.
- Internationalization / lint-staged. These can be added during later Phase 0 cleanup.

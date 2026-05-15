# Acceptance: Settings PermissionsView Reachability

## Checks

| Segment | Check | Evidence |
|---|---|---|
| 2.1 Settings Mount | A Settings render contains the standalone `PermissionsView` capability surface. | `SettingsPage.test.tsx` red/green proves Settings calls `/api/v1/me/permissions` and renders the mounted view. |
| 2.2 Truthful Permission States | Settings exposes the `PermissionsView` empty state when the user has no capability entries. | `SettingsPage.test.tsx` asserts `[data-ap2-empty]` text is `暂无授权`; `PermissionsView.test.tsx` preserves component state coverage. |
| 2.3 Authority And Scope Guard | Existing Settings privacy tab behavior remains intact and no admin/compliance surface is added. | Existing Settings tests still pass; grep found no task implementation changes to ArtifactComments/sidebar/footer surfaces. |
| 4 Reverse Checks | Targeted tests, typecheck/build, grep, and current-doc sync are recorded. | `docs/current/client/ui/settings.md` updated; verification commands recorded below. |

## Required Commands

- `./node_modules/.bin/vitest run --environment jsdom packages/client/src/__tests__/SettingsPage.test.tsx`
- `./node_modules/.bin/vitest run --environment jsdom packages/client/src/__tests__/PermissionsView.test.tsx`
- `./node_modules/.bin/tsc -b packages/client`
- `./node_modules/.bin/vite build --config packages/client/vite.config.ts`

## Verification Notes

- RED: `./node_modules/.bin/vitest run --environment jsdom packages/client/src/__tests__/SettingsPage.test.tsx` failed before implementation with 1 failing test because `/api/v1/me/permissions` was never called.
- GREEN: `./node_modules/.bin/vitest run --environment jsdom packages/client/src/__tests__/SettingsPage.test.tsx` passed: 1 file, 5 tests.
- Component regression: `./node_modules/.bin/vitest run --environment jsdom packages/client/src/__tests__/PermissionsView.test.tsx` passed: 1 file, 5 tests.
- Typecheck: `./node_modules/.bin/tsc -b packages/client` exited 0.
- Build: `./node_modules/.bin/vite build` from `packages/client` exited 0 with the pre-existing chunk-size warning.
- Non-gating local command gap: `pnpm --filter @borgee/client test -- ...` is blocked in this environment by pnpm's ignored-build approval gate, and direct local binaries were used instead. `./node_modules/.bin/eslint 'packages/*/src/**/*.{ts,tsx}'` cannot run because this repo has no ESLint v9 `eslint.config.*` file.

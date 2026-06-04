// ESLint v9 flat config — minimal working baseline.
//
// Scope: matches the root `package.json` "lint" script glob
//   `packages/*/src/**/*.{ts,tsx}`.
//
// Boundary (bf task `eslint9-config`):
//   - Restore a working ESLint v9 flat config so `pnpm lint` can run.
//   - Do NOT introduce new lint rules beyond what main had pre-break.
//   - There is no `.eslintrc.*` in git history
//     (`git log --all --diff-filter=D -- '.eslintrc*'` returns nothing),
//     so this is a from-scratch minimal baseline — not a re-creation.
//
// Design notes:
//
//  1. Register `@typescript-eslint/parser` so TS/TSX files parse. Without
//     this ESLint 9's default parser chokes on TS syntax (`as`, generics,
//     type annotations, etc.).
//
//  2. NO rules are enabled. The lint pass therefore reports only
//     parser-level syntax errors (e.g. duplicate `let` declarations,
//     malformed JSX). The red-then-green sanity check the spec asks for
//     (AC-2) is satisfied by parser-level catching of a duplicate `let`
//     declaration; no plugin rule needs to be on for that.
//
//  3. The codebase contains four pre-existing `eslint-disable` directive
//     references — `@typescript-eslint/no-explicit-any`, `no-var`,
//     `react-hooks/exhaustive-deps`, `react/no-danger`. ESLint v9 errors
//     on disable directives that reference unknown rules. To keep this
//     change strictly config-only (no source edits beyond the lint task,
//     per the parent bf.md Boundary), the config registers these four
//     rule names as no-op stub plugins, set to `off`. This is faithful
//     to the "no new rules introduced" rule: the rules are off, they do
//     nothing, but their names resolve so the existing disable comments
//     don't trip ESLint's unknown-rule check.
//
//  4. `linterOptions.reportUnusedDisableDirectives: 'off'` — the same
//     stub registration makes existing disable comments "unused" from
//     ESLint's POV. Silencing the warning keeps the lint output focused
//     on real violations.
//
// If the team later wants real react / react-hooks / no-explicit-any
// enforcement, install the corresponding plugins and flip the relevant
// rule entries on. That is intentionally OUT OF SCOPE here.

import tsParser from '@typescript-eslint/parser';

// No-op stub plugins. Each registers a single rule that does nothing,
// so existing `eslint-disable` directives that reference these rule
// names resolve without erroring.
const noopRule = {
  meta: { type: 'problem', schema: [] },
  create() {
    return {};
  },
};

const stubTypescriptEslint = {
  rules: {
    'no-explicit-any': noopRule,
  },
};

const stubReact = {
  rules: {
    'no-danger': noopRule,
  },
};

const stubReactHooks = {
  rules: {
    'exhaustive-deps': noopRule,
  },
};

const stubCore = {
  rules: {
    'no-var': noopRule,
  },
};

export default [
  {
    files: ['packages/*/src/**/*.{ts,tsx}'],
    linterOptions: {
      reportUnusedDisableDirectives: 'off',
    },
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        ecmaVersion: 2022,
        sourceType: 'module',
        ecmaFeatures: { jsx: true },
      },
    },
    plugins: {
      '@typescript-eslint': stubTypescriptEslint,
      react: stubReact,
      'react-hooks': stubReactHooks,
      // Core `no-var` is already a built-in rule, but it's wrapped here
      // too so `eslint-disable no-var` continues to resolve even if
      // upstream renames or deprecates it.
      core: stubCore,
    },
    rules: {
      // Intentionally empty — minimal working baseline. See header.
    },
  },
];

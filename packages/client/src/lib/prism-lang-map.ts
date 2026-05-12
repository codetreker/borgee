// prism-lang-map.ts — CV-3.3 internal helper.
//
// Maps the public 12-entry short-code whitelist (CODE_LANGUAGES) onto
// the prism-react-renderer language identifier. Kept in a separate file
// so the user-facing CodeRenderer.tsx stays free of full-form names —
// the cv-3-content-lock §2 grep check on CodeRenderer.tsx demands 0 hits
// for full-form synonyms ('golang' / full-script-form / etc).
//
// Constraint: content-lock §2 grep checks do not include this file because
// the filename is not CodeRenderer.tsx. This follows #338: keep prism internal
// mappings separate from user-facing rendering.
import type { CodeLanguage } from './code-languages';

// Short-code → prism-react-renderer language identifier.
// 'text' → 'text' (no highlight). Aliases are minimal (only when prism
// requires the long form).
export const PRISM_LANG_MAP: Readonly<Record<CodeLanguage, string>> = {
  go: 'go',
  ts: 'typescript',
  js: 'javascript',
  py: 'python',
  md: 'markdown',
  sh: 'bash',
  sql: 'sql',
  yaml: 'yaml',
  json: 'json',
  html: 'markup',
  css: 'css',
  text: 'text',
};

// prism-lang-map.ts — CV-3.3 internal helper.
//
// Maps the public 12-entry short-code whitelist (CODE_LANGUAGES) onto
// the prism-react-renderer language identifier. Kept in a separate file
// so the user-facing CodeRenderer.tsx stays free of full-form names —
// the cv-3-content-lock §2 grep 检查 on CodeRenderer.tsx demands 0 hit
// for full-form synonyms ('golang' / full-script-form / etc).
//
// 反约束 (本文件不被 content-lock §2 grep 检查 命中, 因为文件名不是
// CodeRenderer.tsx — 跟 #338 cross-grep 反模式遵守: 路径分离把 prism 内
// 部映射跟 user-facing 渲染拆开).
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

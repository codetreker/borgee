// code-languages.ts — CV-3.3 11+1 项语言短码白名单 (server 同源).
//
// byte-identical 跟 cv_3_2_artifact_validation.go::ValidCodeLanguages
// + cv-3-content-lock.md §1 ② 同源. 短码唯一; 全名映射在
// prism-lang-map.ts 隔离, 让 CodeRenderer.tsx grep 检查 干净.
export const CODE_LANGUAGES = [
  'go', 'ts', 'js', 'py', 'md', 'sh',
  'sql', 'yaml', 'json', 'html', 'css',
  'text',
] as const;

export type CodeLanguage = typeof CODE_LANGUAGES[number];

// LANG_LABEL: lang 大写 byte-identical (content-lock §1 ② 字面).
export const LANG_LABEL: Readonly<Record<CodeLanguage, string>> = {
  go: 'GO',
  ts: 'TS',
  js: 'JS',
  py: 'PY',
  md: 'MD',
  sh: 'SH',
  sql: 'SQL',
  yaml: 'YAML',
  json: 'JSON',
  html: 'HTML',
  css: 'CSS',
  text: 'TEXT',
};

export function normalizeLang(raw: string | undefined | null): CodeLanguage {
  if (!raw) return 'text';
  const lower = raw.toLowerCase();
  return (CODE_LANGUAGES as readonly string[]).includes(lower)
    ? (lower as CodeLanguage)
    : 'text';
}

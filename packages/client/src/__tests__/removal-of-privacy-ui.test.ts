// removal-of-privacy-ui.test.ts — #975 reverse-grep guard (RM-1).
//
// Pins the cleanup committed by #975 against silent re-introduction. Per
// skeptic-owner contract C1:
//
//   1. Match RAW BYTES of every file — DO NOT strip `//` comments.
//      Comment-strip lets a future PR silence the guard by writing
//      `import X from './p'; // PrivacyPromise legacy shim`.
//   2. Whitelist is an ENUMERATED `path:line` list — NOT a blanket
//      `grep -v /admin/`. The admin/ directory is a code-organization line,
//      not a runtime-isolation line; the admin SPA ships in the same client
//      bundle, so a re-introduced `PrivacyPromise` under `admin/components/`
//      still re-introduces it into the user-product tree.
//   3. Meta-assertion `WHITELIST.length <= 5` so any growth surfaces in a
//      code review.
//
// Scope: `.ts` / `.tsx` / `.css` under `packages/client/src/` (test files
// themselves get filtered out by the substring "__tests__/" path check —
// the test contents would otherwise self-trigger).
import { describe, expect, it } from 'vitest';
// @ts-expect-error — node:module not in @types/node, vitest node context can reach it.
import { createRequire } from 'module';

const nodeRequire = createRequire(import.meta.url);
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const fs: any = nodeRequire('fs');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodePath: any = nodeRequire('path');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const url: any = nodeRequire('url');

const HERE: string = nodePath.dirname(url.fileURLToPath(import.meta.url));
// repo root = packages/client/src/__tests__ → ../../../..
const REPO_ROOT: string = nodePath.resolve(HERE, '..', '..', '..', '..');
const SRC_ROOT: string = nodePath.join(REPO_ROOT, 'packages', 'client', 'src');

// WHITELIST — exact "<repo-relative-path>:<line>" entries. Only intentional
// tombstone-comment anchors allowed. Anything else is a violation.
//
// Per skeptic-owner C1.3 meta-assertion: size must stay <= 5. Growth forces
// a review conversation about why a fresh tombstone is legitimate.
const WHITELIST: string[] = [
  // Admin SPA explanatory comment — points at the deleted user-side surface
  // to document why the cross-end literal lock collapsed in #975.
  'packages/client/src/admin/pages/AdminAuditLogPage.tsx:19',
  'packages/client/src/admin/pages/AdminAuditLogPage.tsx:28',
];

// Tokens — substring match on raw bytes. Component/export names use bare
// substrings (catches `PrivacyPromiseV2` rename attempts via humanish review
// of the violation report). CSS class names match the dot-prefixed selector.
const BANNED_TOKENS: string[] = [
  // 4 deleted user-facing components.
  'PrivacyPromise',
  'AdminActionsList',
  'ImpersonateGrantSection',
  'BannerImpersonate',
  // 4 deleted user-side API helpers.
  'getMyImpersonateGrant',
  'createMyImpersonateGrant',
  'revokeMyImpersonateGrant',
  'getMyAdminActions',
  // Deleted CSS class anchors.
  '.privacy-promise',
  '.privacy-row-',
  // Deleted stance-comment refs (privacy §13 / privacy constraint ADM-0 §1.3
  // were rewritten to "server data-trim" by #975 R2).
  'privacy §13',
  'privacy constraint ADM-0 §1.3',
];

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of fs.readdirSync(dir)) {
    const p = nodePath.join(dir, entry);
    let st;
    try {
      st = fs.statSync(p);
    } catch {
      continue;
    }
    if (st.isDirectory()) {
      walk(p, out);
    } else if (
      entry.endsWith('.ts') ||
      entry.endsWith('.tsx') ||
      entry.endsWith('.css')
    ) {
      out.push(p);
    }
  }
  return out;
}

function relFromRepo(absPath: string): string {
  return nodePath.relative(REPO_ROOT, absPath).replace(/\\/g, '/');
}

describe('#975 reverse-grep guard (RM-1)', () => {
  it('whitelist stays small (meta-assertion C1.3)', () => {
    // Skeptic-owner contract: a growing whitelist becomes the new attack
    // surface. Cap forces a review conversation.
    expect(WHITELIST.length).toBeLessThanOrEqual(5);
  });

  it('source tree contains no re-introduced privacy/compliance UI tokens', () => {
    const allFiles = walk(SRC_ROOT);
    // Exclude the test file directory from the scan; the banned tokens
    // appear literally inside this very file as the BANNED_TOKENS list.
    const files = allFiles.filter((f) => !f.includes(`${nodePath.sep}__tests__${nodePath.sep}`));

    const violations: string[] = [];
    for (const f of files) {
      const raw: string = fs.readFileSync(f, 'utf8');
      const lines: string[] = raw.split('\n');
      lines.forEach((line: string, idx: number) => {
        for (const token of BANNED_TOKENS) {
          if (line.includes(token)) {
            const anchor = `${relFromRepo(f)}:${idx + 1}`;
            if (!WHITELIST.includes(anchor)) {
              violations.push(`${anchor}\t[token=${token}]\t${line.trim()}`);
            }
          }
        }
      });
    }

    if (violations.length > 0) {
      throw new Error(
        `#975 reverse-grep guard fired. Re-introduced privacy/compliance UI tokens:\n` +
          violations.join('\n') +
          `\n\nIf a hit is intentional (genuine tombstone comment), add its exact\n` +
          `path:line to WHITELIST in this test file (max ${5} entries).`,
      );
    }
    expect(violations).toEqual([]);
  });
});

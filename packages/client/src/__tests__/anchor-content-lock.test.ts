// anchor-content-lock.test.ts — CV-2.3 content-lock literal guard.
//
// Pins the 4 exact literals from docs/qa/cv-2-content-lock.md so
// drift in AnchorThreadPanel.tsx / ArtifactPanel.tsx 文案 is caught
// pre-merge instead of post-merge by the reverse grep step.
//
// 锁来源: docs/qa/cv-2-content-lock.md §1 字面表 ① / ② / ③ / ⑥ + ⑦.
// Test reads the source files directly and asserts the expected literal
// substrings are present (the reverse grep there asserts forbidden
// synonyms are absent — paired guard).

import { describe, it, expect } from 'vitest';
// @ts-expect-error — node:module 没 @types/node, vitest node 上下文可达.
import { createRequire } from 'module';

const nodeRequire = createRequire(import.meta.url);
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const fs: any = nodeRequire('fs');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodePath: any = nodeRequire('path');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const url: any = nodeRequire('url');

const HERE = nodePath.dirname(url.fileURLToPath(import.meta.url));
const SRC_ROOT = nodePath.resolve(HERE, '..');

function read(rel: string): string {
  return fs.readFileSync(nodePath.join(SRC_ROOT, rel), 'utf8');
}

describe('CV-2 content-lock literals', () => {
  const threadPanel = read('components/AnchorThreadPanel.tsx');
  const artifactPanel = read('components/ArtifactPanel.tsx');

  it('① anchor entry tooltip = "评论此段" exact literal', () => {
    expect(artifactPanel).toContain("'评论此段'");
  });

  it('② thread header = "段落讨论" exact literal', () => {
    expect(threadPanel).toContain("'段落讨论'");
  });

  it('③ textarea placeholder = "针对此段写下你的 review…" exact literal', () => {
    expect(threadPanel).toContain('针对此段写下你的 review…');
  });

  it('⑥ resolve button = "标为已解决" exact literal', () => {
    expect(threadPanel).toContain("'标为已解决'");
  });

  it('⑥ reopen button = "重新打开" exact literal', () => {
    expect(threadPanel).toContain("'重新打开'");
  });

  it('⑦ stale label includes "锚点指向 v" + "文档已更新到 v" exact literals', () => {
    // staleLabel template lives in AnchorThreadPanel; we also render it
    // inline in ArtifactPanel anchor row — both sites must carry the
    // literal so DOM grep matches in either entry path.
    expect(threadPanel).toContain('锚点指向 v');
    expect(threadPanel).toContain('文档已更新到 v');
    expect(artifactPanel).toContain('锚点指向 v');
    expect(artifactPanel).toContain('文档已更新到 v');
  });

  it('reverse assertion: synonyms NOT present (cv-2-content-lock.md §2 reverse grep)', () => {
    for (const forbidden of ['"Resolve"', '"Close"', "'Resolve'", "'Close'"]) {
      expect(threadPanel).not.toContain(forbidden);
    }
    // ④ agent badge naming lock — must NOT use Bot / AI / Assistant.
    for (const forbidden of ['"Bot"', '"AI"', '"Assistant"']) {
      expect(threadPanel).not.toContain(forbidden);
    }
    // ⑦ stale 同义词漂移 lock.
    for (const forbidden of ['"stale"', '"outdated"', '"过期"', '"已失效"']) {
      expect(threadPanel).not.toContain(forbidden);
      expect(artifactPanel).not.toContain(forbidden);
    }
  });
});

// RT-3 ⭐ presence — content-lock §3+§4 grep 检查 tests.
//
// 设计沿用 (rt-3-spec.md §0 + content-lock):
//   - §3 typing 类同义词 0 hit in RT-3 client paths (反 type-T-indicator 漂)
//   - §4 thought-process 5-pattern 在 RT3PresenceDot.tsx + useRT3Presence.ts
//     0 hit (跟 BPP-3 + CV-* + DM-* 既有锁链承袭)
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

const RT3_FILES = [
  nodePath.join(SRC_ROOT, 'components', 'RT3PresenceDot.tsx'),
  nodePath.join(SRC_ROOT, 'hooks', 'useRT3Presence.ts'),
];

function read(p: string): string {
  return fs.readFileSync(p, 'utf-8');
}

describe('RT-3 ⭐ content-lock §3+§4 grep 检查', () => {
  it('§3 typing 类同义词 0 hit in RT-3 paths (英 5 + 中 4)', () => {
    const patterns = [
      /\b(typing|composing|isTyping|userTyping|composingIndicator)\b/,
      /正在输入|正在打字|输入中|打字中/,
    ];
    const hits: string[] = [];
    for (const f of RT3_FILES) {
      const body = read(f);
      for (const re of patterns) {
        const m = body.match(re);
        if (m) hits.push(`${f}: ${m[0]}`);
      }
    }
    expect(hits).toEqual([]);
  });

  it('§4 thought-process 5-pattern 锁链 RT-3 = 第 N+1 处 — 反向断言 0 hit', () => {
    const patterns = [
      /\bprocessing\b/,
      /\bresponding\b/,
      /\banalyzing\b/,
      /\bplanning\b/,
      /"AI is thinking"/,
    ];
    const hits: string[] = [];
    for (const f of RT3_FILES) {
      const body = read(f);
      for (const re of patterns) {
        const m = body.match(re);
        if (m) hits.push(`${f}: ${m[0]}`);
      }
    }
    expect(hits).toEqual([]);
  });

  it('§5 RT-3 4 态 enum 字面 byte-identical (online/away/offline/thinking)', () => {
    const body = read(nodePath.join(SRC_ROOT, 'hooks', 'useRT3Presence.ts'));
    expect(body).toContain(`'online'`);
    expect(body).toContain(`'away'`);
    expect(body).toContain(`'offline'`);
    expect(body).toContain(`'thinking'`);
  });

  it('§6 DOM data-attr SSOT (data-rt3-presence-dot/last-seen/cursor-user) 真挂', () => {
    const body = read(nodePath.join(SRC_ROOT, 'components', 'RT3PresenceDot.tsx'));
    expect(body).toContain('data-rt3-presence-dot');
    expect(body).toContain('data-rt3-last-seen');
    expect(body).toContain('data-rt3-cursor-user');
  });
});

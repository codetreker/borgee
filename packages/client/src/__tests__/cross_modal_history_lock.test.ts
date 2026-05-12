// cross_modal_history_lock.test.ts — feima review #2 锚: 跨 DM-7
// EditHistoryModal + CHN-14 DescriptionHistoryModal 字面 3 处 exact-match
// grep 守门. 文案 content-lock source-of-truth 模式 — 不是 component-reuse
// reuse (字段 key 别 + 空态设计分歧, see chn-14-content-lock.md §1.1).
//
// 反向断: 任一字面在任一 modal 漂走 → fail (cross-modal drift 守门).

import { describe, it, expect } from 'vitest';
// @ts-expect-error — node:module no @types/node
import { createRequire } from 'module';

const nodeRequire = createRequire(import.meta.url);
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const fs: any = nodeRequire('fs');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodePath: any = nodeRequire('path');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodeUrl: any = nodeRequire('url');
const HERE = nodePath.dirname(nodeUrl.fileURLToPath(import.meta.url));

function readModal(name: string): string {
  const p = nodePath.resolve(HERE, '..', 'components', name);
  return fs.readFileSync(p, 'utf8') as string;
}

describe('CHN-14 cross-modal exact-match 锚 (feima review #2 + chn-14-content-lock.md §1.1)', () => {
  const dm7 = readModal('EditHistoryModal.tsx');
  const chn14 = readModal('DescriptionHistoryModal.tsx');

  it('① modal title `编辑历史` exact-match 跨 DM-7 + CHN-14', () => {
    // DM-7 EditHistoryModal title 必含 `<h3>编辑历史</h3>`.
    expect(dm7.includes('<h3>编辑历史</h3>')).toBe(true);
    // CHN-14 DescriptionHistoryModal title 必含 `<h3>编辑历史</h3>`.
    expect(chn14.includes('<h3>编辑历史</h3>')).toBe(true);
  });

  it('② close aria-label `关闭` exact-match 跨 DM-7 + CHN-14', () => {
    expect(dm7.includes('aria-label="关闭"')).toBe(true);
    expect(chn14.includes('aria-label="关闭"')).toBe(true);
  });

  it('③ RFC3339 ts 表达 `new Date(entry.ts).toISOString()` exact-match 跨两 modal', () => {
    expect(dm7.includes('new Date(entry.ts).toISOString()')).toBe(true);
    expect(chn14.includes('new Date(entry.ts).toISOString()')).toBe(true);
  });
});

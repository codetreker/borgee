// chn-3-content-lock.test.ts — CHN-3.3 client SPA 文案 + DOM attr lock.
//
// Pins the 6 byte-identical literals + DOM attrs from
// docs/qa/chn-3-content-lock.md so drift in SortableChannelItem.tsx /
// GroupHeader.tsx / ChannelContextMenu.tsx / useUserLayout.ts is caught
// pre-merge instead of post-merge by reverse grep.
//
// Sources cross-referenced (5 字面 byte-identical 多源 同根, 改一处必
// 改全部 — 这就是 content-lock test 的存在理由):
//   - 拖拽 handle DOM ⋮⋮ + aria-label "拖拽调整顺序" (#371 spec §1
//     CHN-3.3, byte-identical 锁)
//   - 失败 toast "侧栏顺序保存失败, 请重试" (5 源: #371 spec / #376
//     acceptance §3.5 / #402 文案锁 ④ / #412 server const layoutSaveErrorMsg
//     / 本 client useUserLayout LAYOUT_SAVE_TOAST)
//   - 右键菜单 "置顶" / "取消置顶" (双菜单项 字面锁)
//   - data-collapsed 二态锁 (group header)
//   - DM 行反约束 (5 源 byte-identical, ChannelList 不为 DM 渲染 —
//     DM 走 Sidebar.MergedDmList 完全独立路径)

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

describe('CHN-3 content-lock literals + DOM attrs', () => {
  const sortableItem = read('components/SortableChannelItem.tsx');
  const groupHeader = read('components/GroupHeader.tsx');
  const contextMenu = read('components/ChannelContextMenu.tsx');
  const useLayout = read('hooks/useUserLayout.ts');

  it('① drag handle DOM byte-identical: data-sortable-handle + aria-label "拖拽调整顺序" + ⋮⋮', () => {
    expect(sortableItem).toContain('data-sortable-handle=""');
    expect(sortableItem).toContain('aria-label="拖拽调整顺序"');
    expect(sortableItem).toContain('⋮⋮');
  });

  it('② group folding DOM byte-identical: data-collapsed 二态 + aria-label "折叠分组"', () => {
    expect(groupHeader).toMatch(/data-collapsed=\{collapsed \?/);
    expect(groupHeader).toContain('aria-label="折叠分组"');
    // ▶ 折叠 / ▼ 展开 二 icon byte-identical 跟 #371 + 文案锁 ② 同源.
    expect(groupHeader).toContain('▶');
    expect(groupHeader).toContain('▼');
  });

  it('③ pin menu literals byte-identical: "置顶" / "取消置顶"', () => {
    expect(contextMenu).toContain("'置顶'");
    expect(contextMenu).toContain("'取消置顶'");
    // grep 检查 ≥2 — 双菜单项各 1 hit.
    const matches = contextMenu.match(/'置顶'|'取消置顶'/g);
    expect(matches).not.toBeNull();
    expect((matches ?? []).length).toBeGreaterThanOrEqual(2);
  });

  it('③ pin menu DOM byte-identical: data-context="channel-pin" + role="menu"', () => {
    expect(contextMenu).toContain('data-context="channel-pin"');
    expect(contextMenu).toContain('role="menu"');
  });

  it('④ failure toast 字面 byte-identical 5 源: "侧栏顺序保存失败, 请重试"', () => {
    expect(useLayout).toContain("'侧栏顺序保存失败, 请重试'");
  });

  it('反约束: drag handle 同义词漂移 0 hit (sortableItem)', () => {
    for (const forbidden of ['"Drag"', '"拖动"', '"排序"', '"移动"', '"Move"']) {
      expect(sortableItem).not.toContain(forbidden);
    }
  });

  it('反约束: group folding 同义词漂移 0 hit (groupHeader)', () => {
    for (const forbidden of ['"Collapse"', '"Expand"', '"收起"', '"展开"']) {
      expect(groupHeader).not.toContain(forbidden);
    }
  });

  it('反约束: pin 同义词漂移 0 hit (contextMenu)', () => {
    for (const forbidden of ['"Pin"', '"Unpin"', '"固定"', '"取消固定"', '"Stick"']) {
      expect(contextMenu).not.toContain(forbidden);
    }
  });

  it('反约束: failure toast 同义词漂移 0 hit (useLayout)', () => {
    for (const forbidden of ['"保存失败"', '"Save failed"', '"请稍后重试"', '"网络错误"']) {
      expect(useLayout).not.toContain(forbidden);
    }
  });

  it('反约束: pinned BOOL 列名 0 hit — pin 走 position 单调小数 (#366 设计 ③)', () => {
    // 这道反约束守 schema 反向 — server v=19 不应有 pinned 列, 但 client
    // 也不能引入 pinned 字段. useUserLayout / api 层无 'pinned' 字面.
    expect(useLayout).not.toMatch(/pinned\s*[:=]\s*(true|false)/);
  });

  it('反约束: LayoutChangedFrame push frame 不存在 (设计 ⑥ + 文案锁 ⑥)', () => {
    expect(useLayout).not.toContain('LayoutChangedFrame');
    expect(useLayout).not.toContain('UserChannelLayoutChanged');
    // useUserLayout 不订阅 ws frame — 仅 GET /me/layout once + PUT debounce.
    expect(useLayout).not.toContain('useWsHubFrames');
  });
});

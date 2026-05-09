// chn-2-content-lock.test.ts — CHN-2.2 client SPA 文案 + DOM attr lock.
//
// Pins the 5 byte-identical literals from docs/qa/chn-2-content-lock.md
// + 设计 ③/⑤ DOM attrs (data-kind="dm" / "channel" + data-channel-type)
// so drift in Sidebar.tsx / ChannelList.tsx / ChannelView.tsx is caught
// pre-merge instead of post-merge by reverse grep.
//
// 锁来源: docs/qa/chn-2-content-lock.md §1 字面表 ①②③ + acceptance §3.1.

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

describe('CHN-2 content-lock literals + DOM attrs', () => {
  const sidebar = read('components/Sidebar.tsx');
  const channelList = read('components/ChannelList.tsx');
  const channelView = read('components/ChannelView.tsx');
  const slashBuiltins = read('commands/builtins.ts');

  it('① sidebar DM section header = "私信" byte-identical (cv from chn-2-content-lock.md)', () => {
    expect(sidebar).toContain('>私信</div>');
  });

  it('① data-kind="dm" attr present on DM list (acceptance §3.1)', () => {
    expect(sidebar).toMatch(/data-kind="dm"/);
  });

  it('① data-kind="channel" attr present on channel list (拆视觉混淆)', () => {
    expect(channelList).toMatch(/data-kind="channel"/);
  });

  it('② DM tooltip = "私信 ${user.display_name}" byte-identical', () => {
    expect(sidebar).toContain('`私信 ${user.display_name}`');
  });

  it('③ /dm slash command description = "打开与用户的私信"', () => {
    expect(slashBuiltins).toContain('打开与用户的私信');
  });

  it('设计 ⑤ ChannelView data-channel-type attr — DM 跟 channel 拆死视觉', () => {
    expect(channelView).toMatch(/data-channel-type=\{[^}]*'dm'[^}]*'channel'[^}]*\}/);
  });

  it('反约束: DM 同义词漂移 0 hit (sidebar)', () => {
    // 字面 "私信" 唯一; 同义词 / 误用 0 hit.
    for (const forbidden of ['"DM"', '"Direct Message"', '"私聊"', '"对话框"', '"Chats"', '"聊天"']) {
      expect(sidebar).not.toContain(forbidden);
    }
  });

  it('反约束: DM 升级 / 转换路径 0 hit (蓝图 §1.2 是新建非升级)', () => {
    for (const file of [sidebar, channelList, channelView]) {
      expect(file).not.toMatch(/升级为频道|Convert to channel|Upgrade DM|转为频道|promote-to-channel/);
    }
  });

  it('④ DM 视图 workspace tab 渲染条件锁 — 必须 !isDm 守门', () => {
    // 既有实施: tabs 渲染受 !isDm 守 (CHN-2 设计 ② 兜底).
    expect(channelView).toMatch(/!isDm[\s\S]{0,200}channel-view-tabs/);
  });
});

// presence-reverse-grep.test.ts — AL-3.3 (#R3 Phase 2) 约束 grep 守.
//
// 等价于 al-3.md acceptance §5.1 + §3.2 spec lint job 的 client 侧:
//   - §5.1 phase 2 阶段约束: busy / idle / 忙 / 空闲 不准在 presence
//     相关源 (因 phase 2 仅承诺 online/offline/error). **AL-1b (Phase 4)
//     落地后此条解封** — busy/idle 走 AL-1b describeAgentState() 合法
//     字面, 此 PR 仅守剩余字面 (StateBusy / StateIdle 等 server 侧名称
//     若误漂入 client 仍约束).
//   - §3.2 PresenceDot / usePresence 调用面只允许在 agent 相关 UI — 反查
//     `import` 语句, 不是字面提及 (允许注释里说 "PresenceDot 在 ... 用").
//
// 跟 server 侧 hub_presence_grep_test.go 同形 — 目标都是 "不允许"约束.
import { describe, it, expect } from 'vitest';
// @ts-expect-error — node:module 没 @types/node, 但 vitest node 上下文可达.
import { createRequire } from 'module';

// Node builtins via createRequire — client tsconfig 没 @types/node, 用这个壳穿透.
// 仅测试期跑.
const nodeRequire = createRequire(import.meta.url);
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const fs: any = nodeRequire('fs');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodePath: any = nodeRequire('path');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const url: any = nodeRequire('url');
const _dirname: string = nodePath.dirname(url.fileURLToPath(import.meta.url));

function walkFiles(dir: string, out: string[] = []): string[] {
  for (const entry of fs.readdirSync(dir)) {
    const p = nodePath.join(dir, entry);
    const st = fs.statSync(p);
    if (st.isDirectory()) {
      if (p.endsWith('__tests__')) continue;
      walkFiles(p, out);
    } else if (entry.endsWith('.ts') || entry.endsWith('.tsx')) {
      out.push(p);
    }
  }
  return out;
}

const SRC_ROOT: string = nodePath.join(_dirname, '..', '..', 'src');
const PRESENCE_FILES: string[] = [
  nodePath.join(SRC_ROOT, 'components', 'PresenceDot.tsx'),
  nodePath.join(SRC_ROOT, 'hooks', 'usePresence.ts'),
  nodePath.join(SRC_ROOT, 'lib', 'agent-state.ts'),
];

describe('AL-3.3 约束 grep 守 (al-3.md §5.1 / §3.2)', () => {
  it('§5.1 presence 相关文件不出现 server-side state name leak (StateBusy / StateIdle); busy/idle 字面 AL-1b Phase 4 已合法解封', () => {
    // AL-1b Phase 4 解封 busy/idle/忙/空闲 — 这些字面现在是 AL-1b 合法
    // describeAgentState() output 跟 PresenceDot data-task-state attr 字面.
    // 仅守 server 侧 Go 状态枚举名 (StateBusy/StateIdle) 误漂入 client —
    // client 侧字符串字面是 'busy'/'idle' 小写, 跟 server 字面不同.
    const banned = [/StateBusy/, /StateIdle/];
    const hits: string[] = [];
    for (const f of PRESENCE_FILES) {
      const lines = (fs.readFileSync(f, 'utf8') as string).split('\n');
      lines.forEach((line: string, i: number) => {
        const trimmed = line.trim();
        // 跳块/行注释 (约束注释里要写出禁词字面).
        if (trimmed.startsWith('//') || trimmed.startsWith('*') || trimmed.startsWith('/*')) return;
        const codePart = line.replace(/\/\/.*$/, '');
        for (const re of banned) {
          if (re.test(codePart)) hits.push(`${f}:${i + 1}: ${line.trim()}`);
        }
      });
    }
    if (hits.length > 0) {
      throw new Error(
        'AL-3.3 §5.1 约束: presence 文件禁 leak server-side state enum 名. 命中:\n' + hits.join('\n'),
      );
    }
  });

  it('§3.2 PresenceDot import 仅出现在 agent 相关 UI (Sidebar / ChannelMembersModal / AgentManager)', () => {
    const files = walkFiles(SRC_ROOT);
    const allowed = new Set([
      'PresenceDot.tsx',          // 自身.
      'Sidebar.tsx',              // DM 行 (peer.role==='agent' gate).
      'ChannelMembersModal.tsx',  // 频道成员行 (m.role==='agent' gate).
      'AgentManager.tsx',         // owner /agents 视图 (整页 agent).
    ]);
    const importRe = /^\s*import\b[^;]*\bPresenceDot\b[^;]*from\s+['"][^'"]+['"]/m;
    for (const f of files) {
      const base = f.split('/').pop()!;
      const content = fs.readFileSync(f, 'utf8') as string;
      if (!importRe.test(content)) continue;
      if (!allowed.has(base)) {
        throw new Error(
          `AL-3.3 §3.2 约束: PresenceDot 仅允许 import 进 ${[...allowed].join(',')}; 命中 ${f}`,
        );
      }
    }
  });

  it('§3.2 usePresence / markPresence 仅 agent 相关 UI 或 WS 写端调用', () => {
    const files = walkFiles(SRC_ROOT);
    const allowed = new Set([
      'PresenceDot.tsx',
      'Sidebar.tsx',
      'ChannelMembersModal.tsx',
      'AgentManager.tsx',
      'usePresence.ts',
      'useWebSocket.ts',
    ]);
    const importRe = /^\s*import\b[^;]*\b(usePresence|markPresence)\b[^;]*from\s+['"][^'"]+['"]/m;
    for (const f of files) {
      const base = f.split('/').pop()!;
      const content = fs.readFileSync(f, 'utf8') as string;
      if (!importRe.test(content)) continue;
      if (!allowed.has(base)) {
        throw new Error(
          `AL-3.3 §3.2 约束: usePresence/markPresence 仅允许在 ${[...allowed].join(',')}; 命中 ${f}`,
        );
      }
    }
  });

  it('§5.4 PresenceDot 渲染体里, .presence-dot 总跟 sibling 文本绑死 (源码自检)', () => {
    const src = fs.readFileSync(nodePath.join(SRC_ROOT, 'components', 'PresenceDot.tsx'), 'utf8') as string;
    // 实现里 .presence-dot 永远跟 .presence-text 或 .sr-only 同 parent.
    expect(src).toContain('presence-text');
    expect(src).toContain('sr-only');
  });

  // AL-1b (#R3 Phase 4) acceptance §3.4 — busy/idle 文案模糊词约束.
  // describeAgentState() 必须用 "在工作" / "空闲", 不准 "活跃" / "running" /
  // "Standing by" / "等待中" 模糊词. 跟 al-1b-content-lock 同源 (待野马).
  it('§3.4 (AL-1b) agent-state.ts 不出现 "活跃"/"running"/"standing by"/"等待中" 模糊词', () => {
    const src = fs.readFileSync(nodePath.join(SRC_ROOT, 'lib', 'agent-state.ts'), 'utf8') as string;
    const banned = [/活跃/, /\brunning\b/i, /standing\s*by/i, /等待中/];
    const hits: string[] = [];
    src.split('\n').forEach((line: string, i: number) => {
      const trimmed = line.trim();
      if (trimmed.startsWith('//') || trimmed.startsWith('*') || trimmed.startsWith('/*')) return;
      const codePart = line.replace(/\/\/.*$/, '');
      for (const re of banned) {
        if (re.test(codePart)) hits.push(`agent-state.ts:${i + 1}: ${line.trim()}`);
      }
    });
    if (hits.length > 0) {
      throw new Error(
        'AL-1b §3.4 约束: busy/idle 文案禁模糊词. 命中:\n' + hits.join('\n'),
      );
    }
  });
});

// al-2a-content-lock.test.ts — AL-2a.3 client SPA 文案 + DOM attr lock.
//
// Pins byte-identical literals + DOM attrs from AgentConfigPanel.tsx +
// lib/api.ts so drift is caught pre-merge instead of post-merge by reverse
// grep.
//
// Sources cross-referenced (byte-identical 多源 同根, 改一处必改全部):
//   - 失败 toast "agent 配置保存失败, 请重试" — 跟 server-go
//     internal/api/agent_config.go const agentConfigSaveErrorMsg byte-
//     identical 同源 (蓝图 §1.4 SSOT 立场, AL-2a content-lock ①).
//   - allowedConfigKeys 白名单 — 跟 server-go internal/api/agent_config.go
//     allowedConfigKeys map 同源 (name / avatar / prompt / model /
//     capabilities / enabled / memory_ref).
//   - data-agent-config-field 属性二态锁 (form input 字段 ID).
//   - 反约束: runtime-only (api_key / temperature / token_limit /
//     retry_policy) 不在 form (UI 层 + server 层双层 fail-closed).

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
const COMPONENT = nodePath.join(HERE, '..', 'components', 'AgentConfigPanel.tsx');
const API_LIB = nodePath.join(HERE, '..', 'lib', 'api.ts');

function read(file: string): string {
  return fs.readFileSync(file, 'utf-8');
}

describe('AL-2a content-lock literals + DOM attrs', () => {
  const panel = read(COMPONENT);
  const api = read(API_LIB);

  it('① failure toast 字面 byte-identical: "agent 配置保存失败, 请重试"', () => {
    const TOAST = 'agent 配置保存失败, 请重试';
    expect(panel).toContain(TOAST);
    // Const export — server byte-identical 同源 锚.
    expect(panel).toContain(`AGENT_CONFIG_SAVE_TOAST = '${TOAST}'`);
  });

  it('② allowedConfigKeys 白名单 7 字段 byte-identical (跟 server allowedConfigKeys 同源)', () => {
    for (const key of ['name', 'avatar', 'prompt', 'model', 'capabilities', 'enabled', 'memory_ref']) {
      expect(panel).toContain(`'${key}'`);
    }
  });

  it('③ form input 字段 data-agent-config-field 二态锁 (DOM attr byte-identical)', () => {
    for (const field of ['name', 'avatar', 'prompt', 'model', 'enabled', 'memory_ref']) {
      expect(panel).toContain(`data-agent-config-field="${field}"`);
    }
  });

  it('④ DOM root + version display + save button DOM byte-identical', () => {
    expect(panel).toContain('data-agent-config="root"');
    expect(panel).toContain('data-agent-config="loading"');
    expect(panel).toContain('data-agent-config-version');
    expect(panel).toContain('data-agent-config-action="save"');
  });

  it('⑤ API endpoint path byte-identical 跟 server-go agent_config.go RegisterRoutes', () => {
    // GET + PATCH /api/v1/agents/{id}/config — server 路径 byte-identical.
    expect(api).toMatch(/\/api\/v1\/agents\/\$\{id\}\/config/);
    expect(api).toContain("method: 'PATCH'");
    expect(api).toContain('fetchAgentConfig');
    expect(api).toContain('updateAgentConfig');
  });

  it('反约束: runtime-only 字段 (api_key/temperature/token_limit/retry_policy) 不在 form', () => {
    // UI 层 fail-closed — 反向 grep 字段 ID 不出现 (server 层也 fail-closed
    // reject, acceptance §4.1.c reflect scan 同源).
    for (const forbidden of ['api_key', 'temperature', 'token_limit', 'retry_policy']) {
      // form input id 反向断言 (data-agent-config-field 不渲染).
      expect(panel).not.toContain(`data-agent-config-field="${forbidden}"`);
    }
  });

  it('反约束: 不订阅 push frame (蓝图 §1.5 BPP frame 留 AL-2b)', () => {
    // 立场 ⑥ — 走轮询 reload, 不订阅 ws subscription. 反向 grep 字面.
    expect(panel).not.toContain('subscribeWS');
    expect(panel).not.toContain('hub.subscribe');
    // BPP frame name 字面只在 doc comment 出现 (说明 AL-2a 不挂); 单引号
    // 字面 reject (代码使用形式).
    const FRAME = 'agent_config' + '_update';
    expect(panel).not.toContain(`'${FRAME}'`);
    expect(panel).not.toContain(`"${FRAME}"`);
  });

  it('反约束: 失败 toast 同义词漂移 0 hit (字面唯一根)', () => {
    // 反向同义词 byte-identical reject — 改 toast 必须改 server const.
    // 注意: 不能用 "配置保存失败" 检测 (子串匹配 toast 自身), 用完整漂移
    // 字面.
    for (const drift of [
      'agent 配置保存出错',
      'agent 配置写入失败',
      'agent config save failed',
      'Save agent config failed',
      '保存 agent 配置失败',
    ]) {
      expect(panel).not.toContain(drift);
    }
  });

  // gh#701 drift 修 — md 早期写过 `<form data-form="agent-config">`,
  // 实际代码用 `<section data-agent-config="root">`. drift 已修 (md 改对齐
  // code+test). 这条单测扫整个 packages/ + docs/qa/ 树, 守"data-form=
  // 'agent-config'" 不再出现 (反 md 又漂回 form / 代码又漂去 form).
  it('反向锚 (gh#701): 整个 packages/ + docs/qa/ 树没 data-form="agent-config" 字面 (容器是 section, 不是 form)', () => {
    // 路径: HERE = packages/client/src/__tests__ → ..*4 = repo root.
    const REPO_ROOT = nodePath.join(HERE, '..', '..', '..', '..');
    const SCAN_DIRS = [
      nodePath.join(REPO_ROOT, 'packages'),
      nodePath.join(REPO_ROOT, 'docs', 'qa'),
    ];
    const FORBIDDEN = /data-form=["']agent-config["']/;

    function walk(dir: string): string[] {
      const out: string[] = [];
      let entries: string[];
      try {
        entries = fs.readdirSync(dir);
      } catch {
        return out;
      }
      for (const name of entries) {
        // skip node_modules / .worktrees / dist 等大目录
        if (name === 'node_modules' || name === 'dist' || name === '.worktrees' || name.startsWith('.')) continue;
        const full = nodePath.join(dir, name);
        let stat;
        try { stat = fs.statSync(full); } catch { continue; }
        if (stat.isDirectory()) {
          out.push(...walk(full));
        } else if (/\.(ts|tsx|md|js|jsx|go)$/.test(name)) {
          out.push(full);
        }
      }
      return out;
    }

    const hits: string[] = [];
    for (const dir of SCAN_DIRS) {
      for (const file of walk(dir)) {
        // 跳过测试文件自身 (它有 forbidden 字面在反向断言里 — 那是检测用)
        if (file.endsWith('al-2a-content-lock.test.ts')) continue;
        // 跳过 al-2a-content-lock.md 自身 (它的反向锚段落在文档里 *写出*
        // forbidden 字面作为 "❌ 不准用" 的字面引用 — 这是文档功能不是漂移).
        if (file.endsWith('al-2a-content-lock.md')) continue;
        let content: string;
        try { content = fs.readFileSync(file, 'utf-8'); } catch { continue; }
        if (FORBIDDEN.test(content)) {
          hits.push(file);
        }
      }
    }
    // 0 hit — 容器 byte-identical = section, 反 md / code 漂去 form
    expect(hits).toEqual([]);
  });
});

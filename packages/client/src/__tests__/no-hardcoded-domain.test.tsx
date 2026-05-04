// no-hardcoded-domain.test.tsx — REG-NHD-001..006 reverse-grep + env injection check
//
// 立场: 0 hardcoded codetrek.cn 字面 in client production source / 0 hardcoded
// CORS_ORIGIN default in server production source. fork / staging / on-prem
// deploy 真生效, 反 silent prod default 烧.

import { describe, expect, test } from 'vitest';
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
const REPO_ROOT = nodePath.resolve(HERE, '../../../..');

function read(p: string): string {
  return fs.readFileSync(nodePath.join(REPO_ROOT, p), 'utf-8');
}

function stripComments(src: string): string {
  // Strip // line comments and /* … */ block comments. Approximate (good
  // enough for production source — code uses canonical ts/go style; tests
  // only check authoritative string literals, not comment-text).
  return src
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/^[ \t]*\/\/.*$/gm, '');
}

describe('NO-HARDCODED-DOMAIN — REG-NHD reverse-grep + env injection', () => {
  test('REG-NHD-001 — NodeManager.tsx 0 hardcoded codetrek.cn (uses VITE_AGENT_WS_SERVER)', () => {
    const src = read('packages/client/src/components/NodeManager.tsx');
    const code = stripComments(src);
    // 反向锁: production code (excl. comments) 0 hit `codetrek.cn` 字面.
    expect(code).not.toMatch(/codetrek\.cn/);
    // 正向: import.meta.env.VITE_AGENT_WS_SERVER 真用 (build-time inject).
    expect(code).toMatch(/import\.meta\.env\.VITE_AGENT_WS_SERVER/);
    // 正向: 默认 fallback `wss://localhost:4900` 真挂 (本地 sandbox).
    expect(code).toMatch(/wss:\/\/localhost:4900/);
  });

  test('REG-NHD-002 — config.go 0 hardcoded CORS default (CORS_ORIGIN required env)', () => {
    const src = read('packages/server-go/internal/config/config.go');
    const code = stripComments(src);
    // 反向锁: production code (excl. comments) 0 hit `borgee.codetrek.cn` 字面.
    expect(code).not.toMatch(/borgee\.codetrek\.cn/);
    // 正向: CORS_ORIGIN env 默认空 (envStr 第二参数 ""), 反 silent prod default.
    expect(code).toMatch(/envStr\("CORS_ORIGIN", ""\)/);
    // 正向: production 路径 fail-loud (Validate 真返 err).
    expect(code).toMatch(/CORS_ORIGIN env required/);
  });

  test('REG-NHD-003 — .env.example 列 VITE_AGENT_WS_SERVER + 4 env 注释', () => {
    const src = read('packages/client/.env.example');
    expect(src).toMatch(/VITE_AGENT_WS_SERVER=/);
    // 4 env 真注 (prod / staging / testing / dev)
    expect(src).toMatch(/prod:.*borgee\.codetrek\.cn/);
    expect(src).toMatch(/staging:.*staging-borgee\.codetrek\.cn/);
    expect(src).toMatch(/testing:.*testing-borgee\.codetrek\.cn/);
    expect(src).toMatch(/dev:.*localhost:4900/);
  });

  test('REG-NHD-004 — Dockerfile 接 ARG VITE_AGENT_WS_SERVER (build-time inject)', () => {
    const src = read('packages/server-go/Dockerfile');
    expect(src).toMatch(/ARG VITE_AGENT_WS_SERVER/);
    expect(src).toMatch(/ENV VITE_AGENT_WS_SERVER=\$\{VITE_AGENT_WS_SERVER\}/);
  });

  test('REG-NHD-005 — deploy workflows pass --build-arg per env + CORS_ORIGIN inline override', () => {
    // testing → testing-borgee.codetrek.cn
    const test = read('.github/workflows/deploy-test.yml');
    expect(test).toMatch(/--build-arg VITE_AGENT_WS_SERVER=wss:\/\/testing-borgee\.codetrek\.cn/);
    // testing CORS_ORIGIN env 真挂 inline compose
    expect(test).toMatch(/CORS_ORIGIN=https:\/\/testing-borgee\.codetrek\.cn/);
    // staging+prod → borgee.codetrek.cn (staging 是 smoke-test prod artifact, 共用 build)
    const prod = read('.github/workflows/deploy.yml');
    expect(prod).toMatch(/--build-arg VITE_AGENT_WS_SERVER=wss:\/\/borgee\.codetrek\.cn/);
    // staging+prod CORS_ORIGIN inline override 真挂 (反靠 host compose 不可控)
    expect(prod).toMatch(/CORS_ORIGIN=https:\/\/staging-borgee\.codetrek\.cn/);
    expect(prod).toMatch(/CORS_ORIGIN=https:\/\/borgee\.codetrek\.cn/);
    // 反向锁: 2 inline override 真生效 (反 silent host compose dep)
    const overrideHits = (prod.match(/docker-compose\.override\.yml/g) || []).length;
    expect(overrideHits).toBeGreaterThanOrEqual(2);
  });

  test('REG-NHD-006 — production source 0 hit codetrek.cn (cross-package reverse-grep, code only)', () => {
    // 反向锁: 2 production code file (excl. comments). 真 production code 0 hit.
    const nodeManager = stripComments(read('packages/client/src/components/NodeManager.tsx'));
    const config = stripComments(read('packages/server-go/internal/config/config.go'));
    expect(nodeManager).not.toMatch(/codetrek\.cn/);
    expect(config).not.toMatch(/codetrek\.cn/);
  });
});

// admin-spa-ui-coverage-wave2.test.tsx — REG-ASUC2-001..007 + 4 page UI render
//
// 设计: ADM-0 §1.3 admin god-mode 路径独立, shape SSOT byte-identical 跟
// server (LagSnapshot 9 字段 / runtimeRow 7 字段 / archived 子集 / history
// entry 3 字段). 0 server / 0 endpoint 改 (server 已挂 endpoint, 仅 client 接 UI).

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
const ADMIN_DIR = nodePath.resolve(HERE, '../admin');

function read(p: string): string {
  return fs.readFileSync(nodePath.join(ADMIN_DIR, p), 'utf-8');
}

const PAGES = [
  'pages/RuntimesPage.tsx',
  'pages/HeartbeatLagPage.tsx',
  'pages/ArchivedChannelsPage.tsx',
  'pages/ChannelDescriptionHistoryPage.tsx',
];

describe('ADMIN-SPA-UI-COVERAGE-WAVE2 — REG-ASUC2 content-lock + DOM 锚 + shape SSOT', () => {
  test('REG-ASUC2-001 — api.ts exports 4 helper + endpoint path byte-identical', () => {
    const src = read('api.ts');
    expect(src).toMatch(/export async function fetchAdminRuntimes/);
    expect(src).toMatch(/export async function fetchAdminHeartbeatLag/);
    expect(src).toMatch(/export async function fetchAdminArchivedChannels/);
    expect(src).toMatch(/export async function fetchAdminChannelDescriptionHistory/);
    // server endpoint path byte-identical (runtimes.go:538 / host_lag.go:52 /
    // channel_archived.go:44 / channel_history.go:48).
    expect(src).toMatch(/['"`]\/runtimes['"`]/);
    expect(src).toMatch(/['"`]\/heartbeat-lag['"`]/);
    expect(src).toMatch(/['"`]\/channels\/archived['"`]/);
    expect(src).toMatch(/\/channels\/\$\{encodeURIComponent\(channelId\)\}\/description\/history/);
  });

  test('REG-ASUC2-002 — 4 interface 字段 byte-identical 跟 server SSOT', () => {
    const src = read('api.ts');
    // LagSnapshot 9 字段 byte-identical 跟 host_lag.go::LagSnapshot.
    expect(src).toMatch(/interface LagSnapshot/);
    expect(src).toMatch(/count: number;/);
    expect(src).toMatch(/p50_ms: number;/);
    expect(src).toMatch(/p95_ms: number;/);
    expect(src).toMatch(/p99_ms: number;/);
    expect(src).toMatch(/threshold_ms: number;/);
    expect(src).toMatch(/at_risk: boolean;/);
    expect(src).toMatch(/sampled_at: number;/);
    expect(src).toMatch(/window_seconds: number;/);
    expect(src).toMatch(/reason_if_at_risk\?: string;/);
    // AdminRuntime 7+1 字段 (last_error_reason OMITTED 隐私 ADM-0 §1.3).
    expect(src).toMatch(/interface AdminRuntime/);
    expect(src).toMatch(/agent_id: string;/);
    expect(src).toMatch(/endpoint_url: string;/);
    expect(src).toMatch(/process_kind: string;/);
    // last_error_reason 反向锁: client interface AdminRuntime 不声明此字段 (server-side OMITTED).
    const adminRuntimeBlock = src.match(/interface AdminRuntime\s*\{[^}]+\}/);
    expect(adminRuntimeBlock).not.toBeNull();
    expect(adminRuntimeBlock![0]).not.toMatch(/last_error_reason\s*[?:]/);
    // ChannelDescriptionHistoryEntry 3 字段
    expect(src).toMatch(/interface ChannelDescriptionHistoryEntry/);
    expect(src).toMatch(/old_content: string;/);
    expect(src).toMatch(/ts: number;/);
    expect(src).toMatch(/reason: string;/);
  });

  test('REG-ASUC2-003 — 4 page DOM 锚 byte-identical (data-asuc2-* SSOT)', () => {
    const anchors: Record<string, string[]> = {
      'pages/RuntimesPage.tsx': [
        'data-page="admin-runtimes"',
        'data-asuc2-runtimes-list',
        'data-asuc2-runtime-row',
        'data-asuc2-runtimes-refresh',
      ],
      'pages/HeartbeatLagPage.tsx': [
        'data-page="admin-heartbeat-lag"',
        'data-asuc2-lag-card',
        'data-asuc2-lag-refresh',
      ],
      'pages/ArchivedChannelsPage.tsx': [
        'data-page="admin-archived-channels"',
        'data-asuc2-archived-list',
        'data-asuc2-archived-row',
        'data-asuc2-history-link',
      ],
      'pages/ChannelDescriptionHistoryPage.tsx': [
        'data-page="admin-channel-description-history"',
        'data-asuc2-history-list',
        'data-asuc2-history-row',
      ],
    };
    let total = 0;
    for (const [page, list] of Object.entries(anchors)) {
      const src = read(page);
      for (const a of list) {
        expect(src).toContain(a);
        total++;
      }
    }
    expect(total).toBeGreaterThanOrEqual(12);
  });

  test('REG-ASUC2-004 — 中文 UI 文案 byte-identical (content-lock §1.4)', () => {
    expect(read('pages/RuntimesPage.tsx')).toContain('运行时');
    expect(read('pages/RuntimesPage.tsx')).toContain('暂无运行时');
    expect(read('pages/HeartbeatLagPage.tsx')).toContain('心跳滞后');
    expect(read('pages/HeartbeatLagPage.tsx')).toContain('样本数');
    expect(read('pages/HeartbeatLagPage.tsx')).toContain('阈值');
    expect(read('pages/ArchivedChannelsPage.tsx')).toContain('已归档频道');
    expect(read('pages/ArchivedChannelsPage.tsx')).toContain('暂无归档频道');
    expect(read('pages/ChannelDescriptionHistoryPage.tsx')).toContain('描述变更历史');
    expect(read('pages/ChannelDescriptionHistoryPage.tsx')).toContain('暂无变更历史');
    expect(read('pages/ArchivedChannelsPage.tsx')).toContain('归档时间');
  });

  test('REG-ASUC2-005 — admin god-mode 路径独立 (ADM-0 §1.3 红线) — 4 page 仅 /admin-api/*', () => {
    for (const page of PAGES) {
      const src = read(page);
      // 不直 fetch '/api/v1/' (user-rail), 走 admin api 模块.
      expect(src).not.toMatch(/fetch\(['"`]\/api\/v1/);
      // 也不 import user-rail api (反 cross-rail leak).
      expect(src).not.toMatch(/from ['"]\.\.\/\.\.\/lib\/api['"]/);
    }
  });

  test('REG-ASUC2-006 — 4 Route 挂 AdminApp.tsx + 3+ nav 入口', () => {
    const src = read('AdminApp.tsx');
    expect(src).toMatch(/path="runtimes"/);
    expect(src).toMatch(/path="heartbeat-lag"/);
    expect(src).toMatch(/path="channels-archived"/);
    expect(src).toMatch(/path="channels\/:id\/description-history"/);
    // nav 入口 (Runtimes / Heartbeat Lag / Archived Channels)
    expect(src).toContain("'/admin/runtimes'");
    expect(src).toContain("'/admin/heartbeat-lag'");
    expect(src).toContain("'/admin/channels-archived'");
  });

  test('REG-ASUC2-007 — 设计沿用锁链 (4 page import api 单源)', () => {
    // 4 page 全 import 自 '../api' (admin api 单源, 不串 user-rail lib/api).
    for (const page of PAGES) {
      const src = read(page);
      expect(src).toMatch(/from ['"]\.\.\/api['"]/);
    }
  });
});

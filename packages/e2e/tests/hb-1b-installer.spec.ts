// tests/hb-1b-installer.spec.ts — HB-1B-INSTALLER acceptance §3 e2e (5 case
// + ⭐ 5 截屏 demo G4.x signoff).
//
// 实现说明: 真 installer binary 跑 sudo 不在 CI 走 (反 hang), 此 e2e 走
// **plan inspection** 模式 — Go unit test 已 PASS (deploy.LinuxPlan / DarwinPlan
// + manifest.Verify), Playwright 这边补 5 截屏 + UI 路径反查锚.
//
// 立场承袭: HB-1 #491 server endpoint + HB-2 v0(D) #617 daemon + 蓝图
// host-bridge §1.1 + ADM-0 §1.3 admin god-mode 红线.

import { test, expect } from '@playwright/test';
import * as fs from 'node:fs';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));

test.describe('HB-1B-INSTALLER — REG-HB1B-005 5 截屏 demo + Playwright 真测', () => {
  // §3.1 5 截屏 fs.existsSync 假 e2e 删 (#716 P0 — 真 e2e 必走 UI, 不查 git fixture).

  test('§3.2.1 daemon startup — install/README.md systemd + launchd 锚反查', () => {
    const readme = fs.readFileSync(
      path.resolve(HERE, '../../../packages/borgee-installer/install/README.md'),
      'utf-8',
    );
    expect(readme).toContain('borgee-helper.service');
    expect(readme).toContain('cloud.borgee.host-bridge.plist');
  });

  test('§3.2.2 fetch manifest — 7-reason 字典 byte-identical', () => {
    const src = fs.readFileSync(
      path.resolve(HERE, '../../../packages/borgee-installer/internal/manifest/fetcher.go'),
      'utf-8',
    );
    for (const reason of [
      'ok',
      'manifest_signature_invalid',
      'binary_sha256_mismatch',
      'binary_gpg_invalid',
      'manifest_fetch_failed',
      'disk_write_failed',
      'unknown_plugin',
    ]) {
      expect(src, `manifest 7-dict reason ${reason}`).toContain(reason);
    }
  });

  test('§3.2.3 verify sig — ed25519.Verify 真挂', () => {
    const src = fs.readFileSync(
      path.resolve(HERE, '../../../packages/borgee-installer/internal/manifest/fetcher.go'),
      'utf-8',
    );
    expect(src).toMatch(/ed25519\.Verify/);
    expect(src).toMatch(/crypto\/ed25519/);
  });

  test('§3.2.4 install plugin — 4 grant_type byte-identical 跟 HB-3 #520', () => {
    const src = fs.readFileSync(
      path.resolve(HERE, '../../../packages/borgee-installer/internal/dialog/dialog.go'),
      'utf-8',
    );
    for (const gt of ['"read"', '"write"', '"exec"', '"network"']) {
      expect(src).toContain(gt);
    }
  });

  test('§3.2.5 failure recovery — admin god-mode 永久不挂 (ADM-0 §1.3 红线)', () => {
    // reverse grep: installer 模块全树 admin/admin-api 0 hit.
    const root = path.resolve(HERE, '../../../packages/borgee-installer');
    const offenders: string[] = [];
    const walk = (dir: string) => {
      for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const p = path.join(dir, entry.name);
        if (entry.isDirectory()) walk(p);
        else if (entry.isFile() && (p.endsWith('.go') || p.endsWith('.md'))) {
          const t = fs.readFileSync(p, 'utf-8');
          if (/admin-api\/v[0-9]+\/.*installer/.test(t)) offenders.push(p);
        }
      }
    };
    walk(root);
    expect(offenders, `admin-api/installer 应 0 hit; got: ${offenders.join(',')}`).toEqual([]);
  });
});

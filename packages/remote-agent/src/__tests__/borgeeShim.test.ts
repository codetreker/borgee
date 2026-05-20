// bin/borgee shim — unit tests for the platform binary resolver.
//
// chore/collapse-npm (2026-05-20): 4 platform binaries now live inside the
// SAME npm tarball at `bin/platforms/<plat>-<arch>/borgee`. This test locks
// the resolver's dispatch table: it must pick the right path per platform/arch,
// reject unsupported targets, surface a helpful error when the binary file is
// missing, and never bake Windows into the supported set.

import { describe, it } from 'node:test';
import { strict as assert } from 'node:assert';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { resolveBinary, SUPPORTED } from '../../bin/borgee.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
void __dirname; // reserved for future fixture paths

describe('borgee shim platform matrix', () => {
  it('TS-1 SupportedPlatform: resolves bin/platforms/<plat>-<arch>/borgee for each supported target', () => {
    const stubDir = '/tmp/stub-shim-dir';
    const supported = ['linux-x64', 'linux-arm64', 'darwin-x64', 'darwin-arm64'];
    for (const key of supported) {
      const [platform, arch] = key.split('-');
      const got = resolveBinary({
        platform,
        arch,
        dir: stubDir,
        exists: () => true,
      });
      const want = path.join(stubDir, 'platforms', key, 'borgee');
      assert.equal(got, want, `expected resolver to pick ${want} for ${key}`);
    }
  });

  it('TS-2 UnsupportedPlatform: win32 raises with structured message', () => {
    assert.throws(
      () =>
        resolveBinary({
          platform: 'win32',
          arch: 'x64',
          dir: '/tmp/unused',
          exists: () => true,
        }),
      (err: unknown) => {
        if (!(err instanceof Error)) return false;
        return /unsupported platform\/arch win32-x64/.test(err.message) &&
          /Windows is intentionally out of scope/.test(err.message);
      },
    );
  });

  it('TS-2b UnsupportedArch: linux-mips raises with structured message', () => {
    assert.throws(
      () =>
        resolveBinary({
          platform: 'linux',
          arch: 'mips',
          dir: '/tmp/unused',
          exists: () => true,
        }),
      (err: unknown) => err instanceof Error && /unsupported platform\/arch linux-mips/.test(err.message),
    );
  });

  it('TS-3 BinaryMissing: surfaces repair hint when the embedded binary is absent', () => {
    assert.throws(
      () =>
        resolveBinary({
          platform: 'linux',
          arch: 'x64',
          dir: '/tmp/stub-shim-dir',
          exists: () => false,
        }),
      (err: unknown) => {
        if (!(err instanceof Error)) return false;
        return /binary not found at/.test(err.message) &&
          /npm i -g @codetreker\/borgee-remote-agent/.test(err.message);
      },
    );
  });

  it('TS-4 SupportSet: does not advertise Windows as a target', () => {
    for (const key of SUPPORTED) {
      assert.ok(!key.startsWith('win'), `Windows must remain out-of-scope; got ${key}`);
    }
    assert.equal(SUPPORTED.size, 4, 'exactly 4 platform targets');
  });
});

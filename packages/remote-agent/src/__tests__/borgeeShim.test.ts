// Default package CLI — unit tests for the embedded platform binary resolver.
//
// chore/collapse-npm (2026-05-20): 4 platform binaries now live inside the
// SAME npm tarball at `bin/platforms/<plat>-<arch>/borgee`. The package should
// not expose a second public `borgee` bin; the default `borgee-remote-agent`
// bin owns dispatch and uses this resolver internally.

import { describe, it } from 'node:test';
import { strict as assert } from 'node:assert';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import fs from 'node:fs';
import { resolveBorgeeBinary, SUPPORTED } from '../platform-binary.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const packageRoot = path.resolve(__dirname, '..', '..');

describe('default package CLI shape', () => {
  it('TS-0 PackageBin: exposes only borgee-remote-agent as the public npm bin', () => {
    const manifest = JSON.parse(fs.readFileSync(path.join(packageRoot, 'package.json'), 'utf8')) as {
      bin?: Record<string, string>;
      files?: string[];
    };

    assert.deepEqual(manifest.bin, {
      'borgee-remote-agent': 'dist/index.js',
    });
    assert.deepEqual(manifest.files, ['dist', 'bin/platforms/**']);
    assert.equal(fs.existsSync(path.join(packageRoot, 'bin', 'borgee.js')), false);
  });
});

describe('embedded borgee platform matrix', () => {
  it('TS-1 SupportedPlatform: resolves bin/platforms/<plat>-<arch>/borgee for each supported target', () => {
    const stubRoot = '/tmp/stub-package-bin-dir';
    const supported = ['linux-x64', 'linux-arm64', 'darwin-x64', 'darwin-arm64'];
    for (const key of supported) {
      const [platform, arch] = key.split('-');
      const got = resolveBorgeeBinary({
        platform,
        arch,
        binRoot: stubRoot,
        exists: () => true,
      });
      const want = path.join(stubRoot, 'platforms', key, 'borgee');
      assert.equal(got, want, `expected resolver to pick ${want} for ${key}`);
    }
  });

  it('TS-2 UnsupportedPlatform: win32 raises with structured message', () => {
    assert.throws(
      () =>
        resolveBorgeeBinary({
          platform: 'win32',
          arch: 'x64',
          binRoot: '/tmp/unused',
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
        resolveBorgeeBinary({
          platform: 'linux',
          arch: 'mips',
          binRoot: '/tmp/unused',
          exists: () => true,
        }),
      (err: unknown) => err instanceof Error && /unsupported platform\/arch linux-mips/.test(err.message),
    );
  });

  it('TS-3 BinaryMissing: surfaces repair hint when the embedded binary is absent', () => {
    assert.throws(
      () =>
        resolveBorgeeBinary({
          platform: 'linux',
          arch: 'x64',
          binRoot: '/tmp/stub-package-bin-dir',
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

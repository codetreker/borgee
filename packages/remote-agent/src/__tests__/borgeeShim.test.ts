// bin/borgee shim — smoke tests. Verify the platform → subpackage mapping
// matrix matches the npm bundle ship plan (#993 #994 #995 rework). We don't
// spawn the real binary here — that is exercised by `borgee daemon --help`
// in CI's real install gate. This file only locks the dispatch table.
//
// node:test runs files under `src/__tests__/*.test.ts` per the package
// script. We require() the shim from the build root so the test exercises
// the published artifact, not a TS-transpiled copy.

import { describe, it } from 'node:test';
import { strict as assert } from 'node:assert';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const require = createRequire(import.meta.url);
const shim = require(path.resolve(__dirname, '../../bin/borgee.js')) as {
  resolveBinary: () => string;
  SUPPORTED: Record<string, string>;
};

describe('borgee shim platform matrix', () => {
  it('maps each supported platform/arch to a single subpackage', () => {
    const want = {
      'linux-x64': '@codetreker/borgee-remote-agent-linux-x64',
      'linux-arm64': '@codetreker/borgee-remote-agent-linux-arm64',
      'darwin-x64': '@codetreker/borgee-remote-agent-darwin-x64',
      'darwin-arm64': '@codetreker/borgee-remote-agent-darwin-arm64',
    };
    assert.deepStrictEqual(shim.SUPPORTED, want);
  });

  it('does not advertise Windows as a target', () => {
    const keys = Object.keys(shim.SUPPORTED);
    for (const k of keys) {
      assert.ok(!k.startsWith('win'), `Windows must remain out-of-scope; got ${k}`);
    }
  });

  it('throws a helpful error when the platform subpackage is missing', () => {
    // Without optionalDependencies actually installed in CI's dev install,
    // resolveBinary on linux-x64 will fail with a structured error. We assert
    // the operator-facing message contains the repair hint so it stays
    // grep-able from support tickets.
    assert.throws(
      () => shim.resolveBinary(),
      (err: unknown) => {
        if (!(err instanceof Error)) return false;
        return /unsupported platform\/arch|is not installed|is missing/.test(err.message);
      },
    );
  });
});

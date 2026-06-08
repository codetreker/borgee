#!/usr/bin/env node
'use strict';

// Thin dispatcher for @codetreker/borgee-remote-agent.
//
// The npm package ships 4 cross-compiled Go binaries under
// bin/platforms/<plat>-<arch>/borgee. This shim resolves the binary for the
// current platform, makes it executable (npm strips the exec bit on non-bin
// files at pack/install — 0755 -> 0644 — so spawn would EACCES without the
// re-chmod), and forwards ALL argv + the exit code to it. The Go binary owns
// install / daemon / uninstall / --version / --help and their validation.

const fs = require('node:fs');
const path = require('node:path');
const { spawn } = require('node:child_process');

const SUPPORTED = new Set([
  'linux-x64',
  'linux-arm64',
  'darwin-x64',
  'darwin-arm64',
]);

// resolveBorgeeBinary — port of remote-agent's platform-binary.ts. Returns the
// absolute path to the platform binary, or throws a structured error. binRoot
// defaults to <this .cjs dir>/bin so it resolves correctly from the installed
// package layout (the .cjs and bin/ are siblings — see `files`).
function resolveBorgeeBinary(options = {}) {
  const platform = options.platform ?? process.platform;
  const arch = options.arch ?? process.arch;
  const binRoot = options.binRoot ?? path.join(__dirname, 'bin');
  const exists = options.exists ?? fs.existsSync;

  const key = `${platform}-${arch}`;
  if (!SUPPORTED.has(key)) {
    const supported = Array.from(SUPPORTED).join(', ');
    throw new Error(
      `borgee-remote-agent: unsupported platform/arch ${key}. Supported: ${supported}. ` +
        `Windows is intentionally out of scope; track issue #659 for status.`,
    );
  }

  const binaryPath = path.join(binRoot, 'platforms', key, 'borgee');
  if (!exists(binaryPath)) {
    throw new Error(
      `borgee-remote-agent: embedded borgee binary not found at ${binaryPath}. ` +
        `This usually means the npm install was incomplete — ` +
        `try \`npm i -g @codetreker/borgee-remote-agent\` to repair.`,
    );
  }
  return binaryPath;
}

// run — the dispatch seam. Deps are injectable so the unit test can drive it
// without a real binary, real chmod, or real child process.
function run(argv, deps = {}) {
  const resolve = deps.resolveBorgeeBinary ?? resolveBorgeeBinary;
  const chmod = deps.chmod ?? fs.chmodSync;
  const spawnFn = deps.spawn ?? spawn;
  const stderr = deps.stderr ?? process.stderr;

  let binary;
  try {
    binary = resolve();
  } catch (err) {
    stderr.write(`${err.message}\n`);
    return Promise.resolve(2);
  }

  // npm strips the exec bit off bin/platforms/**; re-assert it before spawn.
  // Best-effort: a read-only mount (EPERM/EROFS) or an already-exec binary is
  // fine — if the bit is already set this is a no-op; if we cannot set it the
  // spawn below surfaces the real error.
  try {
    chmod(binary, 0o755);
  } catch (_err) {
    /* ignore — best-effort on read-only mounts */
  }

  return new Promise((resolveCode) => {
    const child = spawnFn(binary, argv, { stdio: 'inherit' });
    child.on('error', (err) => {
      stderr.write(`borgee-remote-agent: failed to spawn ${binary}: ${err.message}\n`);
      resolveCode(2);
    });
    child.on('exit', (code) => {
      resolveCode(code ?? 0);
    });
  });
}

module.exports = { run, resolveBorgeeBinary, SUPPORTED };

if (require.main === module) {
  run(process.argv.slice(2)).then((code) => {
    process.exitCode = code;
  });
}

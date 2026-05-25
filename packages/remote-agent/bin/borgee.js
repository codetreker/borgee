#!/usr/bin/env node
// bin/borgee.js — npm bundle shim for the `borgee` Go binary.
//
// The 4 platform binaries (linux-x64, linux-arm64, darwin-x64, darwin-arm64)
// ship inside this SAME npm tarball at `bin/platforms/<plat>-<arch>/borgee`.
// At runtime the shim picks the right one for the current host and spawns it,
// passing through every argv. Bundle size is ~15-20 MB gzipped tarball — same
// ballpark as `typescript`, well below the cost of per-platform subpackages.
//
// History: #993 #994 #995 split the binary into 4 platform subpackages routed
// via `optionalDependencies`. That layout was reverted in chore/collapse-npm
// (2026-05-20): single npm package, single publish workflow, lower complexity.
//
// Boundary: this shim must NOT do anything beyond resolve + spawn — any
// install-time logic (system user, systemd unit, state dirs) lives in
// `borgee setup`, which is invoked AFTER `npm i -g @codetreker/borgee-remote-agent`.

import { spawn } from 'node:child_process';
import path from 'node:path';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

export const SUPPORTED = new Set([
  'linux-x64',
  'linux-arm64',
  'darwin-x64',
  'darwin-arm64',
]);

export function resolveBinary(env) {
  env = env || {};
  const platform = env.platform || process.platform;
  const arch = env.arch || process.arch;
  const dir = env.dir || __dirname;
  const exists = env.exists || fs.existsSync;

  const key = `${platform}-${arch}`;
  if (!SUPPORTED.has(key)) {
    const supported = Array.from(SUPPORTED).join(', ');
    throw new Error(
      `borgee: unsupported platform/arch ${key}. Supported: ${supported}. ` +
        `Windows is intentionally out of scope; track issue #659 for status.`,
    );
  }
  const binaryName = platform === 'win32' ? 'borgee.exe' : 'borgee';
  const binaryPath = path.join(dir, 'platforms', key, binaryName);
  if (!exists(binaryPath)) {
    throw new Error(
      `borgee: binary not found at ${binaryPath}. ` +
        `This usually means the npm install was incomplete — ` +
        `try \`npm i -g @codetreker/borgee-remote-agent\` to repair.`,
    );
  }
  return binaryPath;
}

function main() {
  let binary;
  try {
    binary = resolveBinary();
  } catch (err) {
    console.error(err.message);
    process.exit(2);
    return;
  }
  const child = spawn(binary, process.argv.slice(2), { stdio: 'inherit' });
  child.on('exit', (code, signal) => {
    if (signal) {
      // Mirror the child's signal — `kill -9 borgee` should look like the
      // binary was killed, not the shim.
      process.kill(process.pid, signal);
      return;
    }
    process.exit(code ?? 0);
  });
  child.on('error', (err) => {
    console.error(`borgee: failed to spawn ${binary}: ${err.message}`);
    process.exit(2);
  });
}

// Run as CLI only when invoked directly (not when imported by tests).
//
// Node 20's `import.meta.url` follows symlinks while `process.argv[1]`
// does not, so `npm i -g`-installed shims (which sit behind a symlink
// at `/usr/bin/borgee`) never matched the simple string comparison and
// the binary spawn never happened — the shim exited silently. Canonicalize
// both sides via fs.realpathSync before comparing.
const isDirectInvocation = (() => {
  if (!process.argv[1]) return false;
  try {
    const realInvoked = fs.realpathSync(process.argv[1]);
    const realModule = fileURLToPath(import.meta.url);
    return realInvoked === realModule;
  } catch {
    return false;
  }
})();
if (isDirectInvocation) {
  main();
}

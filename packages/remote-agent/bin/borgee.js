#!/usr/bin/env node
// bin/borgee.js — npm bundle shim for the `borgee` Go binary (#993 #994 #995
// rework: distribution went through `@codetreker/borgee-remote-agent` npm
// package + 4 platform subpackages instead of nfpm .deb/.pkg).
//
// At install time, npm's optionalDependencies machinery picks exactly one of
// the four platform subpackages and skips the other three, so node_modules
// ends up with `@codetreker/borgee-remote-agent-<plat>/bin/borgee` for the
// current host. This shim resolves that subpackage and exec's the binary
// with all argv passed through.
//
// Why not exec the binary directly via npm's `bin` field on each subpackage?
// npm CLI registers binaries from every package that declares `bin`, so the
// host would end up with four `borgee` symlinks (one per subpackage) all
// pointing at different copies. Routing through a Node shim in the main
// package gives operators ONE PATH entry that always finds the right binary.
//
// Boundary: this shim must NOT do anything beyond resolve + spawn — any
// install-time logic (system user, systemd unit, state dirs) lives in
// `borgee setup`, which is invoked AFTER `npm i -g @codetreker/borgee-remote-agent`.

'use strict';

const { spawn } = require('node:child_process');
const path = require('node:path');
const fs = require('node:fs');

const SUPPORTED = {
  'linux-x64': '@codetreker/borgee-remote-agent-linux-x64',
  'linux-arm64': '@codetreker/borgee-remote-agent-linux-arm64',
  'darwin-x64': '@codetreker/borgee-remote-agent-darwin-x64',
  'darwin-arm64': '@codetreker/borgee-remote-agent-darwin-arm64',
};

function resolveBinary() {
  const key = `${process.platform}-${process.arch}`;
  const subpkg = SUPPORTED[key];
  if (!subpkg) {
    const supported = Object.keys(SUPPORTED).join(', ');
    throw new Error(
      `borgee: unsupported platform/arch ${key}. Supported: ${supported}. ` +
        `Windows is intentionally out of scope; track issue #659 for status.`,
    );
  }
  // require.resolve walks the node_modules tree starting from this shim, so
  // it finds the platform subpackage regardless of whether the main package
  // is installed globally or locally.
  let pkgJsonPath;
  try {
    pkgJsonPath = require.resolve(`${subpkg}/package.json`);
  } catch (err) {
    throw new Error(
      `borgee: platform subpackage ${subpkg} is not installed. ` +
        `This usually means optionalDependencies were skipped at install time. ` +
        `Try \`npm i -g --include=optional @codetreker/borgee-remote-agent\` to repair. ` +
        `Underlying error: ${err.message}`,
    );
  }
  const binaryPath = path.join(path.dirname(pkgJsonPath), 'bin', 'borgee');
  if (!fs.existsSync(binaryPath)) {
    throw new Error(
      `borgee: subpackage ${subpkg} is installed but bin/borgee is missing at ${binaryPath}. ` +
        `Try reinstalling the main package.`,
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
    process.exit(1);
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
    process.exit(1);
  });
}

// Allow unit tests to import resolveBinary without spawning the child.
if (require.main === module) {
  main();
} else {
  module.exports = { resolveBinary, SUPPORTED };
}

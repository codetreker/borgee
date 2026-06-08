'use strict';

// Unit tests for the borgee-remote-agent dispatcher (node:test, zero-dep).
// Ports remote-agent's borgeeShim.test.ts + cliDispatch.test.ts to the .cjs.
// Run bounded: `timeout 60 node --test --test-timeout=10000 borgee-remote-agent.test.cjs`.

const { test } = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const fs = require('node:fs');
const { EventEmitter } = require('node:events');
const { run, resolveBorgeeBinary, SUPPORTED } = require('./borgee-remote-agent.cjs');

// A fake spawned child: an EventEmitter that emits `exit` (or `error`) on the
// next microtask, mirroring child_process.spawn's async event delivery.
function fakeChild({ exitCode, error } = {}) {
  const child = new EventEmitter();
  queueMicrotask(() => {
    if (error) {
      child.emit('error', error);
    } else {
      child.emit('exit', exitCode, null);
    }
  });
  return child;
}

// --- resolver tests (port of borgeeShim.test.ts) ---

test('resolver_resolves_each_supported_target', () => {
  for (const key of ['linux-x64', 'linux-arm64', 'darwin-x64', 'darwin-arm64']) {
    const [platform, arch] = key.split('-');
    const resolved = resolveBorgeeBinary({
      platform,
      arch,
      binRoot: '/stub',
      exists: () => true,
    });
    assert.equal(resolved, path.join('/stub', 'platforms', key, 'borgee'));
  }
});

test('resolver_win32_structured_error', () => {
  assert.throws(
    () => resolveBorgeeBinary({ platform: 'win32', arch: 'x64', exists: () => true }),
    (err) => {
      assert.match(err.message, /unsupported platform\/arch win32-x64/);
      assert.match(err.message, /Windows is intentionally out of scope/);
      return true;
    },
  );
});

test('resolver_linux_mips_structured_error', () => {
  assert.throws(
    () => resolveBorgeeBinary({ platform: 'linux', arch: 'mips', exists: () => true }),
    /unsupported platform\/arch linux-mips/,
  );
});

test('resolver_missing_binary_repair_hint', () => {
  assert.throws(
    () => resolveBorgeeBinary({ platform: 'linux', arch: 'x64', binRoot: '/stub', exists: () => false }),
    (err) => {
      assert.match(err.message, /binary not found at/);
      assert.match(err.message, /npm i -g @codetreker\/borgee-remote-agent/);
      return true;
    },
  );
});

test('supported_set_no_windows_exactly_four', () => {
  assert.equal(SUPPORTED.size, 4);
  for (const key of SUPPORTED) {
    assert.ok(!key.startsWith('win'), `unexpected windows key ${key}`);
  }
});

// --- manifest shape (EV-1 as a test) ---

test('manifest_bin_and_files_whitelist', () => {
  const manifest = JSON.parse(
    fs.readFileSync(path.join(__dirname, 'package.json'), 'utf8'),
  );
  assert.deepEqual(manifest.bin, { 'borgee-remote-agent': 'borgee-remote-agent.cjs' });
  assert.deepEqual(manifest.files, ['borgee-remote-agent.cjs', 'bin/platforms/**']);
  assert.equal(manifest.bin['borgee.js'], undefined);
});

// --- dispatch tests (port of cliDispatch.test.ts) ---

test('run_forwards_install_argv_and_exit', async () => {
  const calls = [];
  const deps = {
    resolveBorgeeBinary: () => '/pkg/bin/platforms/linux-x64/borgee',
    chmod: () => {},
    spawn: (command, args, options) => {
      calls.push({ command, args, options });
      return fakeChild({ exitCode: 0 });
    },
  };
  const argv = ['install', '--server', 'wss://x', '--token', 'y', '--dirs', '/a,/b'];
  const code = await run(argv, deps);
  assert.equal(calls.length, 1);
  assert.deepEqual(calls[0], {
    command: '/pkg/bin/platforms/linux-x64/borgee',
    args: ['install', '--server', 'wss://x', '--token', 'y', '--dirs', '/a,/b'],
    options: { stdio: 'inherit' },
  });
  assert.equal(code, 0);
});

test('run_chmods_before_spawn', async () => {
  const order = [];
  const deps = {
    resolveBorgeeBinary: () => '/pkg/bin/platforms/linux-x64/borgee',
    chmod: () => order.push('chmod'),
    spawn: () => {
      order.push('spawn');
      return fakeChild({ exitCode: 0 });
    },
  };
  await run(['install'], deps);
  assert.deepEqual(order, ['chmod', 'spawn']);
});

test('run_forwards_nonzero_exit', async () => {
  const deps = {
    resolveBorgeeBinary: () => '/pkg/bin/platforms/linux-x64/borgee',
    chmod: () => {},
    spawn: () => fakeChild({ exitCode: 7 }),
  };
  const code = await run(['ls'], deps);
  assert.equal(code, 7);
});

test('run_spawn_error_returns_two', async () => {
  let captured = '';
  const deps = {
    resolveBorgeeBinary: () => '/pkg/bin/platforms/linux-x64/borgee',
    chmod: () => {},
    spawn: () => fakeChild({ error: new Error('ENOENT') }),
    stderr: { write: (s) => { captured += s; } },
  };
  const code = await run(['install'], deps);
  assert.equal(code, 2);
  assert.match(captured, /failed to spawn/);
});

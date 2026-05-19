// tokenStore unit tests (#1004) — Node built-in test runner via tsx.
//
// Run via: pnpm test (delegates to `tsx --test src/__tests__/*.test.ts`)
import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import { defaultTokenPath, readToken, writeToken } from '../tokenStore.js';

function mkTmpDir(prefix = 'borgee-remote-agent-tokenStore-'): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), prefix));
}

test('TS-1: writeToken creates file with mode 0600', () => {
  const dir = mkTmpDir();
  const p = path.join(dir, 'token');
  writeToken(p, 'abc123');
  const st = fs.statSync(p);
  // mask owner/group/other perm bits; we want exactly 0600
  assert.equal(st.mode & 0o777, 0o600, `expected mode 0600, got 0${(st.mode & 0o777).toString(8)}`);
  assert.equal(fs.readFileSync(p, 'utf8'), 'abc123');
  fs.rmSync(dir, { recursive: true, force: true });
});

test('TS-2: writeToken is atomic — no .tmp lingers on success; mid-write failure leaves no garbage', () => {
  const dir = mkTmpDir();
  const p = path.join(dir, 'token');

  // Happy path: after writeToken there must be no <p>.tmp left behind, only
  // the final file. This proves the rename completed (atomic boundary).
  writeToken(p, 'happy');
  assert.equal(fs.existsSync(p), true);
  assert.equal(fs.existsSync(`${p}.tmp`), false);

  // A second write also leaves no .tmp behind.
  writeToken(p, 'happy-2');
  assert.equal(fs.readFileSync(p, 'utf8'), 'happy-2');
  assert.equal(fs.existsSync(`${p}.tmp`), false);

  // Failure path: place a directory at <p>.tmp so the openSync call inside
  // writeToken fails before any partial file can exist. The cleanup
  // contract is "no half-written file lands at <p>"; we verify the original
  // (good) content is preserved and no stray file is created.
  //
  // (Per #1004 worker spec: full fsync-crash simulation isn't easy in Node
  // pure-userland; this proves the weaker but still load-bearing property
  // that an open-time failure does not corrupt the destination.)
  const dir2 = mkTmpDir();
  const p2 = path.join(dir2, 'token');
  writeToken(p2, 'pre-existing');
  fs.mkdirSync(`${p2}.tmp`);
  let threw = false;
  try {
    writeToken(p2, 'will-fail');
  } catch {
    threw = true;
  }
  assert.equal(threw, true, 'writeToken should propagate openSync failure');
  assert.equal(fs.readFileSync(p2, 'utf8'), 'pre-existing', 'destination must not be corrupted');

  fs.rmSync(dir, { recursive: true, force: true });
  fs.rmSync(dir2, { recursive: true, force: true });
});

test('TS-3: readToken returns null on ENOENT', () => {
  const dir = mkTmpDir();
  const p = path.join(dir, 'no-such-file');
  assert.equal(readToken(p), null);
  fs.rmSync(dir, { recursive: true, force: true });
});

test('TS-4: readToken returns null on empty file', () => {
  const dir = mkTmpDir();
  const p = path.join(dir, 'empty');
  fs.writeFileSync(p, '');
  assert.equal(readToken(p), null);
  // whitespace-only also counts as empty
  fs.writeFileSync(p, '   \n\t\n');
  assert.equal(readToken(p), null);
  fs.rmSync(dir, { recursive: true, force: true });
});

test('TS-5: readToken trims trailing whitespace/newline', () => {
  const dir = mkTmpDir();
  const p = path.join(dir, 'token');
  fs.writeFileSync(p, '  tok-123\n');
  assert.equal(readToken(p), 'tok-123');
  fs.writeFileSync(p, 'tok-456\r\n');
  assert.equal(readToken(p), 'tok-456');
  fs.rmSync(dir, { recursive: true, force: true });
});

test('TS-6: writeToken creates parent dir if missing with mode 0700', () => {
  const dir = mkTmpDir();
  const nested = path.join(dir, 'a', 'b', 'c');
  const p = path.join(nested, 'token');
  writeToken(p, 'xyz');
  assert.equal(fs.existsSync(p), true);
  const dirSt = fs.statSync(nested);
  // mkdir with mode 0o700 — actual perms may be affected by umask, so we
  // assert "no group/other access", not exact 0700.
  assert.equal((dirSt.mode & 0o077), 0, `parent dir leaks group/other bits: 0${(dirSt.mode & 0o777).toString(8)}`);
  fs.rmSync(dir, { recursive: true, force: true });
});

test('TS-7: defaultTokenPath returns expected path per platform', () => {
  const p = defaultTokenPath();
  const platform = os.platform();
  if (platform === 'darwin') {
    assert.equal(p, '/Library/Application Support/Borgee/RemoteAgent/token');
  } else {
    // Linux: either root system path or per-user XDG path; in either case it
    // must end with 'borgee-remote-agent/token' for consistency.
    assert.ok(p.endsWith(path.join('borgee-remote-agent', 'token')), `unexpected path: ${p}`);
    if (typeof process.getuid === 'function' && process.getuid() === 0) {
      assert.equal(p, '/var/lib/borgee-remote-agent/token');
    } else {
      // Non-root: must be under XDG_STATE_HOME or ~/.local/state
      const xdg = process.env.XDG_STATE_HOME?.trim();
      const expectedRoot = xdg && xdg.length > 0
        ? xdg
        : path.join(os.homedir(), '.local', 'state');
      assert.equal(p, path.join(expectedRoot, 'borgee-remote-agent', 'token'));
    }
  }
});

test('TS-bonus: writeToken preserves existing mode on overwrite', () => {
  // If an operator has manually chmod'd the file (e.g. to 0400 for an
  // even tighter posture), we don't want a re-write to clobber that.
  const dir = mkTmpDir();
  const p = path.join(dir, 'token');
  writeToken(p, 'first');
  fs.chmodSync(p, 0o400);
  writeToken(p, 'second');
  const st = fs.statSync(p);
  assert.equal(st.mode & 0o777, 0o400);
  assert.equal(fs.readFileSync(p, 'utf8'), 'second');
  fs.rmSync(dir, { recursive: true, force: true });
});

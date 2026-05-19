// tokenStore — file-backed persistence for the remote-agent enrollment token.
//
// 按 #1004: remote-agent's --token was a one-shot CLI flag; host reboot wiped
// the credential and broke the control rail. This module persists the token
// to a known state directory (0600 owner-only, atomic write) so the agent can
// restart without operator intervention.
//
// Layout mirrors borgee-helper-claim (packages/borgee-helper/cmd/borgee-helper-claim/main.go:
// writeFileAtomic + defaultLinuxStateDir/defaultMacStateDir idioms).
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';

const DEFAULT_LINUX_STATE_DIR_ROOT = '/var/lib/borgee-remote-agent';
const DEFAULT_MAC_STATE_DIR = '/Library/Application Support/Borgee/RemoteAgent';

/**
 * Returns the platform-aware default path for the persisted token file.
 *
 * Linux (root or system service): /var/lib/borgee-remote-agent/token
 * Linux (non-root user):          $XDG_STATE_HOME/borgee-remote-agent/token
 *                                 (or ~/.local/state/borgee-remote-agent/token)
 * macOS:                          /Library/Application Support/Borgee/RemoteAgent/token
 *
 * Windows is out of scope per the host-bridge blueprint.
 */
export function defaultTokenPath(): string {
  const platform = os.platform();
  if (platform === 'darwin') {
    return path.join(DEFAULT_MAC_STATE_DIR, 'token');
  }
  // Linux / other POSIX: prefer the system dir when running as root, else
  // fall back to XDG_STATE_HOME (~/.local/state) so a non-root operator can
  // still persist a token without sudo.
  if (typeof process.getuid === 'function' && process.getuid() === 0) {
    return path.join(DEFAULT_LINUX_STATE_DIR_ROOT, 'token');
  }
  const xdgStateHome = process.env.XDG_STATE_HOME?.trim();
  const stateRoot = xdgStateHome && xdgStateHome.length > 0
    ? xdgStateHome
    : path.join(os.homedir(), '.local', 'state');
  return path.join(stateRoot, 'borgee-remote-agent', 'token');
}

/**
 * Reads a token from disk. Returns null if the file is missing, empty (after
 * trim), or unreadable. Never throws — callers treat null as "no persisted
 * token, fall back to CLI".
 */
export function readToken(tokenPath: string): string | null {
  let raw: string;
  try {
    raw = fs.readFileSync(tokenPath, 'utf8');
  } catch {
    return null;
  }
  const trimmed = raw.trim();
  return trimmed.length === 0 ? null : trimmed;
}

/**
 * Writes a token atomically with file mode 0600 (owner-only).
 *
 * Sequence (matches borgee-helper-claim's writeFileAtomic):
 *   1. mkdir -p parent dir with mode 0700 if missing
 *   2. open <path>.tmp with O_WRONLY|O_CREAT|O_TRUNC and mode 0600
 *   3. write + fsync + close
 *   4. rename .tmp -> path (atomic on POSIX)
 *
 * If the destination file already exists, its mode is preserved (chmod after
 * rename to its prior mode, falling back to 0600 if stat fails).
 */
export function writeToken(tokenPath: string, token: string): void {
  if (typeof token !== 'string' || token.length === 0) {
    throw new Error('writeToken: token must be a non-empty string');
  }

  const parent = path.dirname(tokenPath);
  // mkdir -p 0700 — owner-only state dir. If the dir exists with a different
  // mode (e.g. /var/lib/... preset by an installer), leave it alone.
  try {
    fs.mkdirSync(parent, { recursive: true, mode: 0o700 });
  } catch (err) {
    const e = err as NodeJS.ErrnoException;
    if (e.code !== 'EEXIST') throw err;
  }

  // Preserve existing mode if the file already exists; otherwise 0600.
  let targetMode = 0o600;
  try {
    const prior = fs.statSync(tokenPath);
    targetMode = prior.mode & 0o777;
    if (targetMode === 0) targetMode = 0o600;
  } catch {
    // ENOENT — keep default 0600
  }

  const tmpPath = `${tokenPath}.tmp`;
  // Open the tmp file with explicit 0600 so even if the rename races, no
  // intermediate world-readable file ever exists.
  const fd = fs.openSync(tmpPath, fs.constants.O_WRONLY | fs.constants.O_CREAT | fs.constants.O_TRUNC, 0o600);
  try {
    fs.writeFileSync(fd, token);
    try {
      fs.fsyncSync(fd);
    } catch {
      // fsync can fail on some filesystems (e.g. tmpfs in CI containers); the
      // rename below is still atomic, so a missing fsync is non-fatal.
    }
  } catch (err) {
    try { fs.closeSync(fd); } catch { /* ignore */ }
    try { fs.unlinkSync(tmpPath); } catch { /* ignore */ }
    throw err;
  }
  fs.closeSync(fd);

  // Apply the target mode before renaming so the final inode lands with the
  // correct perms.
  try {
    fs.chmodSync(tmpPath, targetMode);
  } catch {
    // ignore — fallback is the 0600 from openSync
  }

  fs.renameSync(tmpPath, tokenPath);
}

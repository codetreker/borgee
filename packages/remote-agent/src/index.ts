#!/usr/bin/env node
import { Command } from 'commander';
import { RemoteAgent } from './agent.js';
import { defaultTokenPath, readToken, writeToken } from './tokenStore.js';

const program = new Command();

program
  .name('borgee-remote-agent')
  .description('Borgee Remote Agent — expose local directories to Borgee server')
  .requiredOption('--server <url>', 'Borgee server WebSocket URL (e.g. ws://localhost:4900)')
  .option('--token <token>', 'Connection token from Borgee UI (first run; persisted on success)')
  .option('--token-file <path>', 'Path to persisted token file (mode 0600, owner-only)', defaultTokenPath())
  .requiredOption('--dirs <dirs>', 'Comma-separated list of directories to expose')
  .parse(process.argv);

const opts = program.opts<{ server: string; token?: string; tokenFile: string; dirs: string }>();

const dirs = opts.dirs.split(',').map(d => d.trim()).filter(Boolean);
if (dirs.length === 0) {
  console.error('Error: at least one directory is required');
  process.exit(1);
}

// Token resolution order (#1004):
//   1. --token <token> on CLI → use it directly; persist to --token-file after
//      the first successful handshake (operator's first run).
//   2. --token absent → read persisted token from --token-file.
//   3. Neither → fail fast with an actionable message.
let token: string;
const cliToken = opts.token?.trim();
const persistFirstHandshake = Boolean(cliToken);

if (cliToken) {
  token = cliToken;
} else {
  const persisted = readToken(opts.tokenFile);
  if (!persisted) {
    console.error(
      `Error: no token provided via --token and no persisted token at ${opts.tokenFile}.\n` +
      `Run with --token <one-shot from Borgee UI> the first time; subsequent runs ` +
      `(including after reboot) will read the persisted file automatically.`,
    );
    process.exit(1);
  }
  token = persisted;
  console.log(`[remote-agent] Loaded persisted token from ${opts.tokenFile}`);
}

console.log(`[remote-agent] Allowed directories: ${dirs.join(', ')}`);

const agent = new RemoteAgent(opts.server, token, dirs, {
  onFirstHandshake: persistFirstHandshake
    ? (t) => {
        writeToken(opts.tokenFile, t);
        console.log(`[remote-agent] Persisted token to ${opts.tokenFile}`);
      }
    : undefined,
  onAuthRejected: () => {
    // Exit non-zero so systemd / pm2 / the operator notices instead of
    // silently retrying with a revoked token forever.
    process.exit(2);
  },
});
agent.connect();

process.on('SIGINT', () => {
  console.log('[remote-agent] Shutting down...');
  agent.close();
  process.exit(0);
});

process.on('SIGTERM', () => {
  agent.close();
  process.exit(0);
});

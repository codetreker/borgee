import { spawn as childProcessSpawn } from 'node:child_process';
import { Command } from 'commander';
import { RemoteAgent } from './agent.js';
import { defaultTokenPath, readToken, writeToken } from './tokenStore.js';
import { resolveBorgeeBinary } from './platform-binary.js';

type SpawnedProcess = {
  on(event: 'exit', listener: (code: number | null, signal: NodeJS.Signals | null) => void): SpawnedProcess;
  on(event: 'error', listener: (err: Error) => void): SpawnedProcess;
};

type SpawnFn = (
  command: string,
  args: string[],
  options: { stdio: 'inherit' },
) => SpawnedProcess;

interface RunDeps {
  spawn?: SpawnFn;
  resolveBorgeeBinary?: () => string;
  stderr?: Pick<NodeJS.WriteStream, 'write'>;
}

const BORGEE_SUBCOMMANDS = new Set([
  'install',
  'uninstall-host',
  'daemon',
  'rootd',
  'install-plugin',
  'version',
  '--version',
  '-v',
  'help',
  '--help',
  '-h',
]);

export async function run(argv: string[] = process.argv.slice(2), deps: RunDeps = {}): Promise<number> {
  if (argv[0] && BORGEE_SUBCOMMANDS.has(argv[0])) {
    return runBorgee(argv, deps);
  }
  startLegacyRemoteAgent(argv);
  return 0;
}

async function runBorgee(argv: string[], deps: RunDeps): Promise<number> {
  const stderr = deps.stderr ?? process.stderr;
  const spawn = deps.spawn ?? (childProcessSpawn as SpawnFn);
  const resolve = deps.resolveBorgeeBinary ?? resolveBorgeeBinary;

  let binary: string;
  try {
    binary = resolve();
  } catch (err) {
    stderr.write(`${(err as Error).message}\n`);
    return 2;
  }

  return new Promise((resolveCode) => {
    const child = spawn(binary, argv, { stdio: 'inherit' });
    child.on('exit', (code) => {
      resolveCode(code ?? 0);
    });
    child.on('error', (err) => {
      stderr.write(`borgee-remote-agent: failed to spawn ${binary}: ${err.message}\n`);
      resolveCode(2);
    });
  });
}

function startLegacyRemoteAgent(argv: string[]): void {
  const program = new Command();

  program
    .name('borgee-remote-agent')
    .description('Borgee Remote Agent — expose local directories to Borgee server')
    .requiredOption('--server <url>', 'Borgee server WebSocket URL (e.g. ws://localhost:4900)')
    .option('--token <token>', 'Connection token from Borgee UI (first run; persisted on success)')
    .option('--token-file <path>', 'Path to persisted token file (mode 0600, owner-only)', defaultTokenPath())
    .requiredOption('--dirs <dirs>', 'Comma-separated list of directories to expose')
    .parse(['node', 'borgee-remote-agent', ...argv]);

  const opts = program.opts<{ server: string; token?: string; tokenFile: string; dirs: string }>();

  const dirs = opts.dirs.split(',').map(d => d.trim()).filter(Boolean);
  if (dirs.length === 0) {
    console.error('Error: at least one directory is required');
    process.exit(1);
  }

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

  console.warn(
    '[remote-agent] Deprecated: direct remote filesystem bridge startup through ' +
    '`npx @codetreker/borgee-remote-agent --server ...` will be removed. ' +
    'Use `npx @codetreker/borgee-remote-agent install ...` for host setup.',
  );
  console.log(`[remote-agent] Allowed directories: ${dirs.join(', ')}`);

  const agent = new RemoteAgent(opts.server, token, dirs, {
    onFirstHandshake: persistFirstHandshake
      ? (t) => {
          writeToken(opts.tokenFile, t);
          console.log(`[remote-agent] Persisted token to ${opts.tokenFile}`);
        }
      : undefined,
    onAuthRejected: () => {
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
}

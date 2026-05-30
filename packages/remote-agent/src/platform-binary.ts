import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const SUPPORTED = new Set([
  'linux-x64',
  'linux-arm64',
  'darwin-x64',
  'darwin-arm64',
]);

interface ResolveBorgeeBinaryOptions {
  platform?: string;
  arch?: string;
  binRoot?: string;
  exists?: (path: string) => boolean;
}

function defaultBinRoot(): string {
  const moduleDir = path.dirname(fileURLToPath(import.meta.url));
  return path.resolve(moduleDir, '..', 'bin');
}

export function resolveBorgeeBinary(options: ResolveBorgeeBinaryOptions = {}): string {
  const platform = options.platform ?? process.platform;
  const arch = options.arch ?? process.arch;
  const binRoot = options.binRoot ?? defaultBinRoot();
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

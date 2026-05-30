import { EventEmitter } from 'node:events';
import { describe, it } from 'node:test';
import { strict as assert } from 'node:assert';
import { run } from '../cli.js';

describe('default CLI dispatch', () => {
  it('TS-1 InstallDispatch: forwards install to the embedded borgee binary', async () => {
    const calls: Array<{ command: string; args: string[]; options: unknown }> = [];
    const child = new EventEmitter();

    const code = await run(
      ['install', '--server', 'wss://borgee.example.com', '--token', 'enr.secret'],
      {
        resolveBorgeeBinary: () => '/pkg/bin/platforms/linux-x64/borgee',
        spawn: (command, args, options) => {
          calls.push({ command, args, options });
          queueMicrotask(() => child.emit('exit', 0, null));
          return child;
        },
      },
    );

    assert.equal(code, 0);
    assert.deepEqual(calls, [
      {
        command: '/pkg/bin/platforms/linux-x64/borgee',
        args: ['install', '--server', 'wss://borgee.example.com', '--token', 'enr.secret'],
        options: { stdio: 'inherit' },
      },
    ]);
  });
});

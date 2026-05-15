// vitest.config.ts — keep browser-dependent tests in jsdom while letting
// pure TypeScript/source-scan tests run in node. This preserves the RT-0
// CustomEvent coverage without paying jsdom startup cost for every file.
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

const browserTsTests = [
  'src/__tests__/al-2a-content-lock.test.ts',
  'src/__tests__/channel-groups-ui.test.ts',
  'src/__tests__/cs4-idb.test.ts',
  'src/__tests__/last-seen-cursor.test.ts',
  'src/__tests__/markdown-mention.test.ts',
  'src/__tests__/presence-reverse-grep.test.ts',
  'src/__tests__/pushSubscribe.test.ts',
  'src/__tests__/useDMSync.test.ts',
  'src/__tests__/ws-anchor-comment-added.test.ts',
  'src/__tests__/ws-artifact-comment-added.test.ts',
  'src/__tests__/ws-artifact-updated.test.ts',
  'src/__tests__/ws-envelope-flatten.test.ts',
  'src/__tests__/ws-invitation.test.ts',
  'src/__tests__/ws-mention-pushed.test.ts',
];

export default defineConfig({
  plugins: [react()],
  test: {
    globals: false,
    pool: 'threads',
    silent: 'passed-only',
    projects: [
      {
        extends: true,
        test: {
          name: 'node',
          environment: 'node',
          include: ['src/**/*.test.ts'],
          exclude: browserTsTests,
        },
      },
      {
        extends: true,
        test: {
          name: 'jsdom',
          environment: 'jsdom',
          setupFiles: ['./src/test/setup.ts'],
          include: ['src/**/*.test.tsx', ...browserTsTests],
        },
      },
    ],
  },
});

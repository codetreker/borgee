// fixtures/stopwatch.ts — INFRA-2 latency-measurement helper.
//
// Exists to support the G2.4 latency requirement (invitation sent → owner
// notification ≤ 3s, with the stopwatch measurement attached as acceptance
// evidence). RT-0 (#40) is the first real consumer.
//
// Why it lives here and not inline in each test: every Phase 2 latency
// gate (G2.1 invitation approval E2E, G2.2 offline fallback E2E, G2.4 team-awareness sign-off)
// will measure something against a deadline. Centralizing the contract
// (start / stop / annotate test info with measured latency for the
// HTML report) keeps the assertion shape identical across tests, which
// matters when reviewers read the report.
//
// Usage (preview, RT-0 will land the first real call):
//
//   const sw = stopwatch();
//   await page.click('[data-testid=invite-send]');
//   await otherPage.waitForSelector('[data-testid=invitation-toast]');
//   sw.stop();
//   await sw.attach(testInfo, 'invitation-to-notification latency');
//   expect(sw.ms).toBeLessThanOrEqual(3000);
//
import type { TestInfo } from '@playwright/test';

export interface Stopwatch {
  /** Stop the watch. Idempotent — second call is a no-op. */
  stop(): void;
  /** Elapsed milliseconds. Throws if read before stop(). */
  readonly ms: number;
  /**
   * Attach the measurement to the Playwright HTML report so reviewers can
   * read it without opening the trace viewer.
   */
  attach(testInfo: TestInfo, label: string): Promise<void>;
}

export function stopwatch(): Stopwatch {
  const start = performance.now();
  let end: number | undefined;

  return {
    stop() {
      if (end === undefined) end = performance.now();
    },
    get ms(): number {
      if (end === undefined) {
        throw new Error('stopwatch: read .ms before stop()');
      }
      return Math.round(end - start);
    },
    async attach(testInfo, label) {
      if (end === undefined) {
        throw new Error('stopwatch: attach() before stop()');
      }
      const ms = Math.round(end - start);
      await testInfo.attach(label, {
        body: `${ms} ms\n`,
        contentType: 'text/plain',
      });
    },
  };
}

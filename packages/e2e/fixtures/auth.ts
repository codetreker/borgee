// fixtures/auth.ts — INFRA-2 auth fixture skeleton.
//
// **Status: PLACEHOLDER.** This file documents the pattern for future PRs
// (CM-onboarding #42, RT-0 #40, CM-4 gate 4 demo) to add real auth fixtures
// without rediscovering the API surface. The smoke test in this PR does
// NOT use any of this — it only hits unauthenticated endpoints.
//
// Why placeholder and not a working fixture: the real fixture needs an
// invite-code seed path and a stable test-user lifecycle which depend on
// CM-onboarding (#42) auto-creating the welcome channel. Implementing it
// here would couple INFRA-2 to undecided onboarding flow.
//
// **Pattern for future PRs**:
//
//   1. seedUser(api, email, password): POST /api/v1/auth/register with an
//      invite code provisioned at server-go startup via env var
//      (BORGEE_E2E_INVITE_CODE — TBD in CM-onboarding).
//   2. login(page, email, password): fill #login-form and wait for the
//      cookie to be set (page.context().storageState()).
//   3. Reuse storageState across tests via test.use({ storageState: ... }).
//
// See https://playwright.dev/docs/auth#multiple-signed-in-roles for the
// final shape — multiple user roles (admin / member / agent owner) get
// their own storageState files in .playwright-data/auth/.
import type { APIRequestContext, Page } from '@playwright/test';

export interface SeedUserOptions {
  email: string;
  password: string;
  displayName: string;
  inviteCode: string;
}

/**
 * Stub. Returns 501 Not Implemented intentionally — fail loudly if a test
 * tries to use auth before CM-onboarding lands the invite-code seed.
 */
export async function seedUser(
  _api: APIRequestContext,
  _opts: SeedUserOptions,
): Promise<{ userId: string }> {
  throw new Error(
    'INFRA-2: auth fixture is a placeholder. Implement in CM-onboarding (#42) ' +
      'once invite-code seed env is wired.',
  );
}

/**
 * Stub. Pattern documented inline; implementation deferred.
 */
export async function login(
  _page: Page,
  _email: string,
  _password: string,
): Promise<void> {
  throw new Error(
    'INFRA-2: login fixture is a placeholder. Implement in CM-onboarding (#42).',
  );
}

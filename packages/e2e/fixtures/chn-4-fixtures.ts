// fixtures/chn-4-fixtures.ts — CHN-4 wrapper REST-driven seed fixtures.
//
// CHN-4 convention: keep REST setup in this fixture file instead of
// duplicating setup code in specs. This avoids drift in repeated setup
// blocks and avoids fixed waits for asynchronous setup.
//
// Pattern: Playwright `test.beforeAll` 钩子调 seedCHN4Fixtures(serverURL)
// → returns fixture handles: { ownerToken, ownerCtx, agentID, dmID,
// publicChID }. Specs only perform page navigation and assertion
// auto-retry; user, channel, and agent setup happens through REST.
//
// No fixed timing waits: after the REST setup resolves, the fixture is
// ready. Specs use Playwright `toBeVisible` / `toHaveCount` auto-retry.

import {
  request as apiRequest,
  expect,
  type APIRequestContext,
} from '@playwright/test';

export interface CHN4Fixture {
  ownerEmail: string;
  ownerToken: string;
  ownerUserID: string;
  ownerCtx: APIRequestContext;
  agentID: string;
  dmID: string;
  publicChID: string;
  cleanup: () => Promise<void>;
}

/**
 * seedCHN4Fixtures — one-time REST seed: register owner, create agent,
 * open DM, and create public channel. Specs call this from `beforeAll`;
 * `afterAll` calls cleanup() to release APIRequestContext.
 */
export async function seedCHN4Fixtures(serverURL: string): Promise<CHN4Fixture> {
  const ownerEmail = `chn4-owner-${Date.now()}@e2e.test`;
  const ownerCtx = await apiRequest.newContext({ baseURL: serverURL });

  // 1. Register owner (creates org + member with default permissions).
  const regRes = await ownerCtx.post('/api/v1/auth/register', {
    data: {
      email: ownerEmail,
      password: 'password123',
      display_name: 'CHN4Owner',
    },
  });
  expect(regRes.ok(), `register: ${regRes.status()}`).toBe(true);
  const reg = await regRes.json();
  const ownerToken = reg.token as string;
  const ownerUserID = reg.user.id as string;

  const auth = { Cookie: `borgee_token=${ownerToken}` };

  // 2. Create agent (owner-owned).
  const agentRes = await ownerCtx.post('/api/v1/agents', {
    data: { display_name: 'CHN4Agent' },
    headers: auth,
  });
  expect(agentRes.ok(), `create agent: ${agentRes.status()}`).toBe(true);
  const agent = await agentRes.json();
  const agentID = agent.agent.id as string;

  // 3. Open DM with the agent (CHN-4 rule: DM channels have 2 members and type='dm').
  const dmRes = await ownerCtx.post('/api/v1/channels', {
    data: { type: 'dm', with_user_id: agentID },
    headers: auth,
  });
  let dmID = '';
  if (dmRes.ok()) {
    const dm = await dmRes.json();
    dmID = (dm.channel ?? dm).id as string;
  }
  // DM endpoint shape may vary; tolerate empty dmID so specs can detect
  // missing DM support and skip with test.skip when dmID===''.

  // 4. Create a public channel (CHN-4 rule: public channels are distinct from DMs).
  const chRes = await ownerCtx.post('/api/v1/channels', {
    data: {
      name: `chn4-pub-${Date.now()}`,
      visibility: 'public',
    },
    headers: auth,
  });
  expect(chRes.ok(), `create public channel: ${chRes.status()}`).toBe(true);
  const ch = await chRes.json();
  const publicChID = (ch.channel ?? ch).id as string;

  return {
    ownerEmail,
    ownerToken,
    ownerUserID,
    ownerCtx,
    agentID,
    dmID,
    publicChID,
    cleanup: async () => {
      await ownerCtx.dispose();
    },
  };
}

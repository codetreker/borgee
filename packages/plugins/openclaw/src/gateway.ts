import { fetchBotIdentity, pollBorgeeEvents } from "./api-client.js";
import { handleBorgeeInbound, handleBorgeeReactionInbound } from "./inbound.js";
import { readPersistedCursor, persistCursor } from "./cursor-store.js";
import { probeSSE, runSSELoop } from "./sse-client.js";
import { PluginWsClient } from "./ws-client.js";
import { dispatchSSEEvent } from "./sse-client.js";
import { readFile } from "./file-access.js";
import type { ChannelGatewayContext } from "./runtime-api.js";
import type { BorgeeEvent, CoreConfig, ResolvedBorgeeAccount } from "./types.js";

const RETRY_BASE_MS = 1000;
const RETRY_MAX_MS = 30_000;
const SSE_RECOVERY_INTERVAL_MS = 5 * 60_000;

async function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(Object.assign(new Error("Aborted"), { name: "AbortError" }));
      return;
    }
    const onAbort = (): void => {
      clearTimeout(timer);
      reject(Object.assign(new Error("Aborted"), { name: "AbortError" }));
    };
    const timer = setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    signal?.addEventListener("abort", onAbort, { once: true });
  });
}

// ─── Poll transport loop ────────────────────────────────

async function runPollLoop(params: {
  channelId: string;
  channelLabel: string;
  account: ResolvedBorgeeAccount;
  config: CoreConfig;
  ctx: ChannelGatewayContext<ResolvedBorgeeAccount>;
  cursorRef: { value: number };
  /** Abort only this poll session so the gateway can retry SSE. */
  sessionSignal: AbortSignal;
}): Promise<void> {
  let consecutiveErrors = 0;
  const account = params.account;

  while (!params.ctx.abortSignal.aborted && !params.sessionSignal.aborted) {
    try {
      const result = await pollBorgeeEvents({
        baseUrl: account.baseUrl,
        apiKey: account.apiKey,
        cursor: params.cursorRef.value,
        timeoutMs: account.pollTimeoutMs,
        signal: params.sessionSignal,
      });

      if (result.events.length > 0) {
        params.cursorRef.value = result.cursor;
        persistCursor(account.accountId, result.cursor);
      }
      consecutiveErrors = 0;

      for (const event of result.events) {
        if (event.kind !== "message" && event.kind !== "message_edited" && event.kind !== "message_deleted" && event.kind !== "reaction_update") continue;
        let payload: Record<string, unknown>;
        try {
          payload = JSON.parse(event.payload);
        } catch {
          continue;
        }
        const senderId = payload.sender_id as string | undefined ?? payload.user_id as string | undefined;
        if (senderId && senderId === account.botUserId) continue;

        if (event.kind === "reaction_update") {
          await handleBorgeeReactionInbound({
            channelId: params.channelId,
            channelLabel: params.channelLabel,
            account,
            config: params.config,
            event,
            payload: {
              message_id: (payload.message_id as string) ?? "",
              emoji: (payload.emoji as string) ?? "",
              user_id: (payload.user_id as string) ?? "",
              action: (payload.action as string) ?? "",
            },
          });
          continue;
        }

        const isDmChannel = (payload.channel_type as string | undefined) === "dm";
        if (event.kind === "message") {
          if (!isDmChannel && account.requireMention) {
            const mentions = (payload.mentions as string[] | undefined) ?? [];
            if (!mentions.includes(account.botUserId)) continue;
          }
        }
        await handleBorgeeInbound({
          channelId: params.channelId,
          channelLabel: params.channelLabel,
          account,
          config: params.config,
          event,
          channelType: isDmChannel ? "dm" : "channel",
          message: payload as Parameters<typeof handleBorgeeInbound>[0]["message"],
        });
      }
    } catch (error) {
      if (error instanceof Error && error.name === "AbortError") return;
      consecutiveErrors++;
      const backoff = Math.min(
        RETRY_BASE_MS * Math.pow(2, consecutiveErrors - 1),
        RETRY_MAX_MS,
      );
      console.error(
        `[borgee-plugin] Poll error (retry #${consecutiveErrors} in ${backoff}ms):`,
        error instanceof Error ? error.message : error,
      );
      try {
        await sleep(backoff, params.sessionSignal);
      } catch {
        return;
      }
    }
  }
}

// ─── Initialize cursor from server ───────────────────────

async function bootstrapCursor(params: {
  account: ResolvedBorgeeAccount;
  ctx: ChannelGatewayContext<ResolvedBorgeeAccount>;
}): Promise<number> {
  try {
    const bootstrap = await pollBorgeeEvents({
      baseUrl: params.account.baseUrl,
      apiKey: params.account.apiKey,
      cursor: 0,
      timeoutMs: 1000,
      signal: params.ctx.abortSignal,
    });
    persistCursor(params.account.accountId, bootstrap.cursor);
    console.log(`[borgee-plugin] Bootstrapped cursor: ${bootstrap.cursor}`);
    return bootstrap.cursor;
  } catch (error) {
    if (error instanceof Error && error.name === "AbortError") throw error;
    console.warn(
      "[borgee-plugin] Bootstrap poll failed, starting from cursor 0:",
      error instanceof Error ? error.message : error,
    );
    return 0;
  }
}

// ─── Gateway transport orchestration ─────────────────────

export async function startBorgeeGateway(
  channelId: string,
  channelLabel: string,
  ctx: ChannelGatewayContext<ResolvedBorgeeAccount>,
): Promise<void> {
  const account = ctx.account;
  if (!account.configured) {
    throw new Error(`Borgee channel is not configured for account "${account.accountId}"`);
  }

  if (!account.config.botUserId || !account.config.botDisplayName) {
    try {
      const identity = await fetchBotIdentity({
        baseUrl: account.baseUrl,
        apiKey: account.apiKey,
      });
      if (!account.config.botUserId) {
        account.botUserId = identity.userId;
      }
      if (!account.config.botDisplayName) {
        account.botDisplayName = identity.displayName;
      }
      account.requireMention = identity.requireMention;
      console.log(
        `[borgee-plugin] Bot identity: ${account.botDisplayName} (${account.botUserId})`,
      );
    } catch (error) {
      throw new Error(
        `[borgee-plugin] Failed to fetch bot identity from ${account.baseUrl}: ${error instanceof Error ? error.message : String(error)}`,
      );
    }
  }

  ctx.setStatus({
    accountId: account.accountId,
    running: true,
    configured: true,
    enabled: account.enabled,
    baseUrl: account.baseUrl,
  });

  let cursor = readPersistedCursor(account.accountId);
  if (cursor < 0) {
    cursor = await bootstrapCursor({ account, ctx });
  }

  const transport = account.transport;
  const cfg = ctx.cfg as CoreConfig;

  try {
    if (transport === "ws") {
      console.log(`[borgee-plugin] transport=ws`);
      await runWsTransport({
        channelId,
        channelLabel,
        account,
        config: cfg,
        ctx,
      });
    } else if (transport === "poll") {
      console.log(`[borgee-plugin] transport=poll (forced)`);
      const cursorRef = { value: cursor };
      const sessionCtrl = new AbortController();
      ctx.abortSignal.addEventListener("abort", () => sessionCtrl.abort(), { once: true });
      await runPollLoop({
        channelId,
        channelLabel,
        account,
        config: cfg,
        ctx,
        cursorRef,
        sessionSignal: sessionCtrl.signal,
      });
    } else {
      await runAutoOrSse({
        channelId,
        channelLabel,
        account,
        config: cfg,
        ctx,
        initialCursor: cursor,
        forceSSE: transport === "sse",
      });
    }
  } catch (error) {
    if (!(error instanceof Error) || error.name !== "AbortError") {
      throw error;
    }
  }

  ctx.setStatus({
    accountId: account.accountId,
    running: false,
  });
}

/**
 * Auto / SSE mode:
 * 1. HEAD probe. If 404 → fallback to poll (schedule SSE recovery in 5 min).
 *    If 401/403 → stop.
 * 2. Run SSE loop until auth failure or abort.
 * 3. On auth failure → stop. On graceful end → loop around.
 *
 * In "auto" mode, when SSE is unavailable we fall back to poll AND periodically
 * re-probe SSE every 5 minutes; on recovery we abort the poll session and switch back.
 * In "sse" (forced) mode we never fall back — we keep retrying SSE.
 */
async function runAutoOrSse(params: {
  channelId: string;
  channelLabel: string;
  account: ResolvedBorgeeAccount;
  config: CoreConfig;
  ctx: ChannelGatewayContext<ResolvedBorgeeAccount>;
  initialCursor: number;
  forceSSE: boolean;
}): Promise<void> {
  const cursorRef = { value: params.initialCursor };
  const abortSignal = params.ctx.abortSignal;

  while (!abortSignal.aborted) {
    const probe = await probeSSE({
      baseUrl: params.account.baseUrl,
      apiKey: params.account.apiKey,
      signal: abortSignal,
    });

    if (probe.status === 401 || probe.status === 403) {
      console.error(
        `[borgee-plugin] SSE auth failed (${probe.status}); stopping gateway`,
      );
      return;
    }

    if (!probe.ok) {
      if (params.forceSSE) {
        console.warn(
          `[borgee-plugin] SSE probe failed (transport=sse forced); retrying in ${RETRY_MAX_MS}ms`,
        );
        try {
          await sleep(RETRY_MAX_MS, abortSignal);
        } catch {
          return;
        }
        continue;
      }

      // Auto mode: fall back to poll, run SSE recovery probe every 5 min
      console.log(
        `[borgee-plugin] SSE unavailable; falling back to poll (will retry SSE every ${SSE_RECOVERY_INTERVAL_MS / 1000}s)`,
      );
      const sessionCtrl = new AbortController();
      const onAbort = (): void => sessionCtrl.abort();
      abortSignal.addEventListener("abort", onAbort, { once: true });

      const recoveryTimer = setInterval(() => {
        void (async () => {
          const p = await probeSSE({
            baseUrl: params.account.baseUrl,
            apiKey: params.account.apiKey,
            signal: abortSignal,
          });
          if (p.ok) {
            console.log(`[borgee-plugin] SSE available again; switching from poll → SSE`);
            sessionCtrl.abort();
          }
        })();
      }, SSE_RECOVERY_INTERVAL_MS);

      try {
        await runPollLoop({
          channelId: params.channelId,
          channelLabel: params.channelLabel,
          account: params.account,
          config: params.config,
          ctx: params.ctx,
          cursorRef,
          sessionSignal: sessionCtrl.signal,
        });
      } finally {
        clearInterval(recoveryTimer);
        abortSignal.removeEventListener("abort", onAbort);
      }

      if (abortSignal.aborted) return;
      continue; // probe SSE again
    }

    // SSE is available; start the SSE loop.
    console.log(`[borgee-plugin] transport=sse (${params.account.accountId})`);
    const result = await runSSELoop({
      channelId: params.channelId,
      channelLabel: params.channelLabel,
      account: params.account,
      config: params.config,
      ctx: params.ctx,
      getLastEventId: () => {
        const c = readPersistedCursor(params.account.accountId);
        if (c > 0) cursorRef.value = c;
        return c > 0 ? c : cursorRef.value > 0 ? cursorRef.value : undefined;
      },
    });

    // SSE path only persists cursor; refresh cursorRef so a subsequent poll
    // fallback doesn't replay events from the stale bootstrap cursor.
    const latestPersisted = readPersistedCursor(params.account.accountId);
    if (latestPersisted > cursorRef.value) cursorRef.value = latestPersisted;

    if (result.reason === "auth") {
      console.error(
        `[borgee-plugin] SSE auth failed (${result.status}); stopping gateway`,
      );
      return;
    }
    if (result.reason === "aborted") return;
  }
}

async function runWsTransport(params: {
  channelId: string;
  channelLabel: string;
  account: ResolvedBorgeeAccount;
  config: CoreConfig;
  ctx: ChannelGatewayContext<ResolvedBorgeeAccount>;
}): Promise<void> {
  const { account, ctx } = params;

  const wsClient = new PluginWsClient({
    serverUrl: account.baseUrl,
    apiKey: account.apiKey,
    signal: ctx.abortSignal,
  });

  wsClient.onEvent((event, data) => {
    const payload = data as Record<string, unknown>;

    if (event === "message") {
      const isDmChannel = (payload.channel_type as string | undefined) === "dm";
      if (!isDmChannel && account.requireMention) {
        const mentions = (payload.mentions as string[] | undefined) ?? [];
        if (!mentions.includes(account.botUserId)) return;
      }
    }

    const borgeeEvent: BorgeeEvent = {
      cursor: 0,
      kind: event as BorgeeEvent["kind"],
      channel_id: (payload.channel_id as string) ?? "",
      payload: JSON.stringify(data),
      created_at: Date.now(),
    };
    void dispatchSSEEvent({
      channelId: params.channelId,
      channelLabel: params.channelLabel,
      account,
      config: params.config,
      event: borgeeEvent,
    });
  });

  wsClient.onRequest(async (data) => {
    const req = data as Record<string, unknown>;
    if (req.action === 'read_file' && typeof req.path === 'string') {
      return readFile(req.path);
    }
    return { error: 'unknown_action' };
  });

  wsClient.connect();

  // Attach the client to the account so outbound handlers can use it.
  (account as Record<string, unknown>).__wsClient = wsClient;

  await new Promise<void>((resolve) => {
    ctx.abortSignal.addEventListener("abort", () => {
      wsClient.close();
      resolve();
    }, { once: true });
    if (ctx.abortSignal.aborted) {
      wsClient.close();
      resolve();
    }
  });

  (account as Record<string, unknown>).__wsClient = undefined;
}

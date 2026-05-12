import http from "node:http";
import https from "node:https";
import type { IncomingMessage, ClientRequest } from "node:http";
import { handleBorgeeInbound, handleBorgeeReactionInbound } from "./inbound.js";
import type { ChannelGatewayContext } from "./runtime-api.js";
import { persistCursor } from "./cursor-store.js";
import type { BorgeeEvent, CoreConfig, ResolvedBorgeeAccount } from "./types.js";

// ─── SSE frame parser ───────────────────────────────────

interface SSEFrame {
  id?: string;
  event?: string;
  data: string;
}

class SSEParser {
  private buffer = "";
  private currentId: string | undefined;
  private currentEvent: string | undefined;
  private currentData: string[] = [];

  feed(chunk: string, emit: (frame: SSEFrame) => void): void {
    this.buffer += chunk;
    let idx: number;
    while ((idx = this.findLineEnd()) >= 0) {
      const line = this.buffer.slice(0, idx);
      this.buffer = this.buffer.slice(idx + (this.buffer[idx] === "\r" ? 2 : 1));
      this.handleLine(line, emit);
    }
  }

  private findLineEnd(): number {
    for (let i = 0; i < this.buffer.length; i++) {
      const c = this.buffer[i];
      if (c === "\n") return i;
      if (c === "\r" && this.buffer[i + 1] === "\n") return i;
    }
    return -1;
  }

  private handleLine(line: string, emit: (frame: SSEFrame) => void): void {
    if (line === "") {
      if (this.currentData.length > 0 || this.currentEvent || this.currentId) {
        emit({
          id: this.currentId,
          event: this.currentEvent,
          data: this.currentData.join("\n"),
        });
      }
      this.currentEvent = undefined;
      this.currentData = [];
      return;
    }
    if (line.startsWith(":")) {
      // comment / heartbeat — caller sees via raw byte observation upstream
      return;
    }
    const colon = line.indexOf(":");
    const field = colon < 0 ? line : line.slice(0, colon);
    let value = colon < 0 ? "" : line.slice(colon + 1);
    if (value.startsWith(" ")) value = value.slice(1);

    switch (field) {
      case "id":
        this.currentId = value;
        break;
      case "event":
        this.currentEvent = value;
        break;
      case "data":
        this.currentData.push(value);
        break;
      default:
        break;
    }
  }
}

// ─── SSE client ─────────────────────────────────────────

export interface SSEClientEvents {
  onOpen?: () => void;
  onMessage: (event: BorgeeEvent) => void | Promise<void>;
  onHeartbeat?: () => void;
  onError: (err: { status?: number; message: string; fatal: boolean }) => void;
  onClose?: () => void;
}

export interface SSEConnection {
  close: () => void;
}

export function connectSSE(params: {
  baseUrl: string;
  apiKey: string;
  lastEventId?: number;
  signal?: AbortSignal;
  handlers: SSEClientEvents;
}): SSEConnection {
  const url = new URL("/api/v1/stream", params.baseUrl.endsWith("/") ? params.baseUrl : params.baseUrl + "/");
  const client = url.protocol === "https:" ? https : http;

  const headers: Record<string, string> = {
    authorization: `Bearer ${params.apiKey}`,
    accept: "text/event-stream",
    "cache-control": "no-cache",
  };
  if (params.lastEventId != null && params.lastEventId > 0) {
    headers["last-event-id"] = String(params.lastEventId);
  }

  let closed = false;
  let req: ClientRequest | null = null;
  const signal = params.signal;

  const onAbort = (): void => {
    close();
  };

  const close = (): void => {
    if (closed) return;
    closed = true;
    if (signal) signal.removeEventListener("abort", onAbort);
    try {
      req?.destroy();
    } catch {
      /* ignore */
    }
    params.handlers.onClose?.();
  };

  if (signal) {
    if (signal.aborted) {
      queueMicrotask(onAbort);
      return { close };
    }
    signal.addEventListener("abort", onAbort, { once: true });
  }

  req = client.request(
    url,
    {
      method: "GET",
      headers,
    },
    (res: IncomingMessage) => {
      const status = res.statusCode ?? 0;
      if (status < 200 || status >= 300) {
        const fatal = status === 401 || status === 403;
        let body = "";
        res.setEncoding("utf8");
        res.on("data", (c: string) => {
          body += c;
          if (body.length > 1024) body = body.slice(0, 1024);
        });
        res.on("end", () => {
          params.handlers.onError({
            status,
            message: `SSE HTTP ${status}: ${body || "(no body)"}`,
            fatal,
          });
          close();
        });
        return;
      }

      params.handlers.onOpen?.();
      res.setEncoding("utf8");

      const parser = new SSEParser();

      res.on("data", (chunk: string) => {
        // Any byte arrival (including :heartbeat comment) counts as liveness
        params.handlers.onHeartbeat?.();
        parser.feed(chunk, (frame) => {
          const kind = frame.event ?? "message";
          if (kind === "heartbeat") return;
          const cursor = frame.id ? parseInt(frame.id, 10) : NaN;
          if (!Number.isFinite(cursor)) return;
          let payloadStr = frame.data;
          if (!payloadStr) return;
          let channelId = "";
          try {
            const parsed = JSON.parse(payloadStr) as Record<string, unknown>;
            const direct = parsed["channel_id"];
            if (typeof direct === "string") {
              channelId = direct;
            } else {
              const ch = parsed["channel"] as { id?: unknown } | undefined;
              if (ch && typeof ch.id === "string") channelId = ch.id;
            }
          } catch {
            /* non-JSON payload; keep channelId empty */
          }
          const event: BorgeeEvent = {
            cursor,
            kind: kind as BorgeeEvent["kind"],
            channel_id: channelId,
            payload: payloadStr,
            created_at: Date.now(),
          };
          void params.handlers.onMessage(event);
        });
      });

      res.on("end", () => {
        params.handlers.onError({
          message: "SSE stream ended by server",
          fatal: false,
        });
        close();
      });

      res.on("error", (err) => {
        params.handlers.onError({
          message: `SSE stream error: ${err.message}`,
          fatal: false,
        });
        close();
      });
    },
  );

  req.on("error", (err) => {
    if (closed) return;
    params.handlers.onError({
      message: `SSE connect error: ${err.message}`,
      fatal: false,
    });
    close();
  });

  req.end();

  return { close };
}

// ─── Inbound dispatch (same filtering as poll gateway) ────

export async function dispatchSSEEvent(params: {
  channelId: string;
  channelLabel: string;
  account: ResolvedBorgeeAccount;
  config: CoreConfig;
  event: BorgeeEvent;
}): Promise<void> {
  const { account, event } = params;

  if (event.kind !== "message" && event.kind !== "message_edited" && event.kind !== "message_deleted" && event.kind !== "reaction_update") return;

  if (event.kind === "reaction_update") {
    let payload: { message_id?: string; emoji?: string; user_id?: string; action?: string; channel_id?: string };
    try {
      payload = JSON.parse(event.payload);
    } catch {
      return;
    }
    const userId = payload.user_id;
    if (userId && userId === account.botUserId) return;
    await handleBorgeeReactionInbound({
      channelId: params.channelId,
      channelLabel: params.channelLabel,
      account: params.account,
      config: params.config,
      event,
      payload: {
        message_id: payload.message_id ?? "",
        emoji: payload.emoji ?? "",
        user_id: payload.user_id ?? "",
        action: payload.action ?? "",
      },
    });
    if (event.cursor > 0) {
      persistCursor(account.accountId, event.cursor);
    }
    return;
  }

  let payload: {
    id?: string;
    message_id?: string;
    channel_id: string;
    sender_id?: string;
    sender_name?: string;
    content?: string;
    content_type?: string;
    created_at?: number;
    mentions?: string[];
    reply_to_id?: string | null;
    channel_type?: string;
    deleted_at?: number;
  };
  try {
    payload = JSON.parse(event.payload);
  } catch {
    return;
  }

  const senderId = payload.sender_id;
  if (senderId && senderId === account.botUserId) return;

  const isDmChannel = payload.channel_type === "dm";

  if (event.kind === "message") {
    if (!isDmChannel && account.requireMention) {
      const mentions: string[] = payload.mentions ?? [];
      if (!mentions.includes(account.botUserId)) return;
    }
  }

  await handleBorgeeInbound({
    channelId: params.channelId,
    channelLabel: params.channelLabel,
    account: params.account,
    config: params.config,
    event,
    channelType: isDmChannel ? "dm" : "channel",
    message: payload as Parameters<typeof handleBorgeeInbound>[0]["message"],
  });

  if (event.cursor > 0) {
    persistCursor(account.accountId, event.cursor);
  }
}

export async function runSSEOnce(params: {
  channelId: string;
  channelLabel: string;
  account: ResolvedBorgeeAccount;
  config: CoreConfig;
  ctx: ChannelGatewayContext<ResolvedBorgeeAccount>;
  lastEventId?: number;
  heartbeatTimeoutMs?: number;
  onOpen?: () => void;
}): Promise<{ reason: "closed" | "auth" | "heartbeat"; status?: number }> {
  return await new Promise((resolve) => {
    let settled = false;
    let hbTimer: NodeJS.Timeout | null = null;
    const timeoutMs = params.heartbeatTimeoutMs ?? 30_000;

    const resetHeartbeat = (): void => {
      if (hbTimer) clearTimeout(hbTimer);
      hbTimer = setTimeout(() => {
        done({ reason: "heartbeat" });
      }, timeoutMs);
    };

    const done = (r: { reason: "closed" | "auth" | "heartbeat"; status?: number }): void => {
      if (settled) return;
      settled = true;
      if (hbTimer) clearTimeout(hbTimer);
      conn.close();
      resolve(r);
    };

    const conn = connectSSE({
      baseUrl: params.account.baseUrl,
      apiKey: params.account.apiKey,
      lastEventId: params.lastEventId,
      signal: params.ctx.abortSignal,
      handlers: {
        onOpen: () => {
          resetHeartbeat();
          params.onOpen?.();
        },
        onHeartbeat: () => {
          resetHeartbeat();
        },
        onMessage: async (event) => {
          resetHeartbeat();
          try {
            await dispatchSSEEvent({
              channelId: params.channelId,
              channelLabel: params.channelLabel,
              account: params.account,
              config: params.config,
              event,
            });
          } catch (err) {
            console.error(
              "[borgee-plugin] SSE dispatch error:",
              err instanceof Error ? err.message : err,
            );
          }
        },
        onError: (err) => {
          if (err.fatal) {
            console.error(`[borgee-plugin] SSE fatal: ${err.message}`);
            done({ reason: "auth", status: err.status });
          } else {
            done({ reason: "closed", status: err.status });
          }
        },
      },
    });
  });
}

// ─── SSE availability probe ────────────────────────────

export async function probeSSE(params: {
  baseUrl: string;
  apiKey: string;
  timeoutMs?: number;
  signal?: AbortSignal;
}): Promise<{ ok: boolean; status?: number }> {
  const url = new URL(
    "/api/v1/stream",
    params.baseUrl.endsWith("/") ? params.baseUrl : params.baseUrl + "/",
  );
  const client = url.protocol === "https:" ? https : http;
  const timeoutMs = params.timeoutMs ?? 5_000;

  return await new Promise((resolve) => {
    let settled = false;
    const sig = params.signal;
    const onAbort = (): void => {
      req.destroy();
      finish({ ok: false });
    };
    const finish = (r: { ok: boolean; status?: number }): void => {
      if (settled) return;
      settled = true;
      if (sig) sig.removeEventListener("abort", onAbort);
      resolve(r);
    };

    const req = client.request(
      url,
      {
        method: "HEAD",
        headers: {
          authorization: `Bearer ${params.apiKey}`,
          accept: "text/event-stream",
        },
        timeout: timeoutMs,
      },
      (res: IncomingMessage) => {
        const status = res.statusCode ?? 0;
        res.resume();
        // Any response (including 200/405) means server is reachable.
        // 404 means the endpoint doesn't exist — not reachable for SSE.
        // 401/403 means auth failed — fatal.
        if (status === 404) finish({ ok: false, status });
        else if (status === 401 || status === 403) finish({ ok: false, status });
        else finish({ ok: status > 0, status });
      },
    );

    req.on("error", () => finish({ ok: false }));
    req.on("timeout", () => {
      req.destroy();
      finish({ ok: false });
    });

    if (sig) {
      if (sig.aborted) {
        req.destroy();
        finish({ ok: false });
        return;
      }
      sig.addEventListener("abort", onAbort, { once: true });
    }

    req.end();
  });
}

// ─── SSE reconnect loop state ─────────────────────────

const RECONNECT_BASE_MS = 1_000;
const RECONNECT_MAX_MS = 60_000;
const STABLE_THRESHOLD_MS = 30_000;

function sleepAbortable(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(Object.assign(new Error("Aborted"), { name: "AbortError" }));
      return;
    }
    const onAbort = (): void => {
      clearTimeout(t);
      reject(Object.assign(new Error("Aborted"), { name: "AbortError" }));
    };
    const t = setTimeout(() => {
      signal.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    signal.addEventListener("abort", onAbort, { once: true });
  });
}

export interface SSELoopResult {
  reason: "auth" | "aborted";
  status?: number;
}

/**
 * Drive SSE with exponential-backoff reconnect.
 * Returns when auth fails (401/403) or ctx is aborted.
 */
export async function runSSELoop(params: {
  channelId: string;
  channelLabel: string;
  account: ResolvedBorgeeAccount;
  config: CoreConfig;
  ctx: ChannelGatewayContext<ResolvedBorgeeAccount>;
  getLastEventId: () => number | undefined;
}): Promise<SSELoopResult> {
  const signal = params.ctx.abortSignal;
  let attempt = 0;

  while (!signal.aborted) {
    // Probe the stream endpoint before reconnect attempts after the first one.
    if (attempt > 0) {
      const probe = await probeSSE({
        baseUrl: params.account.baseUrl,
        apiKey: params.account.apiKey,
        signal,
      });
      if (probe.status === 401 || probe.status === 403) {
        return { reason: "auth", status: probe.status };
      }
      if (!probe.ok) {
        const delay = Math.min(RECONNECT_BASE_MS * 2 ** attempt, RECONNECT_MAX_MS);
        try {
          await sleepAbortable(delay, signal);
        } catch {
          return { reason: "aborted" };
        }
        attempt++;
        continue;
      }
    }

    const connectedAt = Date.now();
    const result = await runSSEOnce({
      channelId: params.channelId,
      channelLabel: params.channelLabel,
      account: params.account,
      config: params.config,
      ctx: params.ctx,
      lastEventId: params.getLastEventId(),
      onOpen: () => {
        console.log(`[borgee-plugin] SSE connected (${params.account.accountId})`);
      },
    });

    if (signal.aborted) return { reason: "aborted" };

    if (result.reason === "auth") {
      return { reason: "auth", status: result.status };
    }

    const stable = Date.now() - connectedAt >= STABLE_THRESHOLD_MS;
    if (stable) attempt = 0;
    else attempt++;

    const delay = Math.min(RECONNECT_BASE_MS * 2 ** Math.max(0, attempt - 1), RECONNECT_MAX_MS);
    console.warn(
      `[borgee-plugin] SSE disconnected (${result.reason}); reconnecting in ${delay}ms`,
    );
    try {
      await sleepAbortable(delay, signal);
    } catch {
      return { reason: "aborted" };
    }
  }

  return { reason: "aborted" };
}

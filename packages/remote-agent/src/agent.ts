import WebSocket from 'ws';
import { ls, readFile, stat } from './fs-ops.js';

interface WsMessage {
  type: string;
  id?: string;
  data?: Record<string, unknown>;
  error?: string;
}

export interface RemoteAgentOptions {
  /**
   * Called once after the very first successful handshake of this process.
   * Used to persist the in-memory token to disk so subsequent restarts can
   * read it back without `--token` (#1004).
   *
   * The callback runs synchronously inside the `open` handler; errors are
   * logged but do not abort the agent — failing to persist is recoverable
   * (operator can re-pass --token), losing the connection is not.
   */
  onFirstHandshake?: (token: string) => void;

  /**
   * Called when the server closes the WS with a code that indicates the
   * token was rejected (4001 / 4003 / generic 1008 policy violation). The
   * agent stops the reconnect loop and the caller (index.ts) typically
   * exits non-zero so the operator notices.
   */
  onAuthRejected?: (code: number, reason: string) => void;
}

// WS close codes that the server uses to signal "your token is bad — do not
// retry". We treat any 4xxx close code with an auth-shaped reason as fatal;
// generic 1008 (policy violation) is also included since some server stacks
// fall back to it when the upgrade is rejected.
const AUTH_REJECTED_CODES = new Set<number>([4001, 4003, 1008]);

export class RemoteAgent {
  private ws: WebSocket | null = null;
  private reconnectDelay = 1000;
  private maxReconnectDelay = 30000;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private closed = false;
  private firstHandshakeDone = false;

  constructor(
    private serverUrl: string,
    private token: string,
    private allowedDirs: string[],
    private options: RemoteAgentOptions = {},
  ) {}

  connect(): void {
    this.closed = false;
    const url = `${this.serverUrl}/ws/remote?token=${encodeURIComponent(this.token)}`;
    console.log(`[remote-agent] Connecting to ${this.serverUrl}...`);

    this.ws = new WebSocket(url);

    this.ws.on('open', () => {
      console.log('[remote-agent] Connected');
      this.reconnectDelay = 1000;
      this.startHeartbeat();
      if (!this.firstHandshakeDone) {
        this.firstHandshakeDone = true;
        if (this.options.onFirstHandshake) {
          try {
            this.options.onFirstHandshake(this.token);
          } catch (err) {
            console.error(`[remote-agent] Failed to persist token: ${(err as Error).message}`);
          }
        }
      }
    });

    this.ws.on('message', (raw: Buffer) => {
      let msg: WsMessage;
      try {
        msg = JSON.parse(raw.toString()) as WsMessage;
      } catch {
        return;
      }
      this.handleMessage(msg);
    });

    this.ws.on('close', (code, reason) => {
      const reasonStr = reason.toString();
      console.log(`[remote-agent] Disconnected: ${code} ${reasonStr}`);
      this.stopHeartbeat();
      if (this.closed) return;
      if (this.isAuthRejected(code, reasonStr)) {
        console.error(
          `[remote-agent] Auth rejected by server (code ${code}); refusing to reconnect. ` +
          `The persisted token may have been revoked — re-run with --token <new token>.`,
        );
        this.closed = true;
        if (this.options.onAuthRejected) {
          try {
            this.options.onAuthRejected(code, reasonStr);
          } catch (err) {
            console.error(`[remote-agent] onAuthRejected callback failed: ${(err as Error).message}`);
          }
        }
        return;
      }
      this.scheduleReconnect();
    });

    this.ws.on('error', (err) => {
      console.error(`[remote-agent] Error: ${err.message}`);
    });
  }

  close(): void {
    this.closed = true;
    this.stopHeartbeat();
    this.ws?.close(1000, 'Agent shutting down');
  }

  private isAuthRejected(code: number, reason: string): boolean {
    if (AUTH_REJECTED_CODES.has(code)) return true;
    // Some WS servers emit a vanilla 1006/1011 with a reason string mentioning
    // "unauthorized" / "invalid token" — match conservatively.
    const lower = reason.toLowerCase();
    return lower.includes('unauthorized') || lower.includes('invalid token') || lower.includes('token revoked');
  }

  private handleMessage(msg: WsMessage): void {
    switch (msg.type) {
      case 'pong':
        break;
      case 'request':
        if (msg.id && msg.data) {
          void this.handleRequest(msg.id, msg.data);
        }
        break;
      default:
        break;
    }
  }

  private async handleRequest(id: string, data: Record<string, unknown>): Promise<void> {
    const action = data.action as string;
    const targetPath = data.path as string;
    let result: unknown;

    switch (action) {
      case 'ls':
        result = ls(targetPath, this.allowedDirs);
        break;
      case 'read':
        result = readFile(targetPath, this.allowedDirs);
        break;
      case 'stat':
        result = stat(targetPath, this.allowedDirs);
        break;
      default:
        result = { error: `Unknown action: ${action}` };
    }

    const hasError = result && typeof result === 'object' && 'error' in result;
    if (hasError) {
      this.send({ type: 'response', id, data: result });
    } else {
      this.send({ type: 'response', id, data: result });
    }
  }

  private send(msg: unknown): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  private startHeartbeat(): void {
    this.heartbeatTimer = setInterval(() => {
      this.send({ type: 'ping' });
    }, 30_000);
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  private scheduleReconnect(): void {
    console.log(`[remote-agent] Reconnecting in ${this.reconnectDelay}ms...`);
    setTimeout(() => {
      if (!this.closed) this.connect();
    }, this.reconnectDelay);
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
  }
}

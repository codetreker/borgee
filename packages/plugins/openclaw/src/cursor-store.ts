import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join, dirname } from "node:path";

function cursorFilePath(accountId: string): string {
  const base = process.env.OPENCLAW_DATA_DIR || process.env.HOME || ".";
  return join(base, "data", `collab-cursor-${accountId}.json`);
}

export function readPersistedCursor(accountId: string): number {
  const fp = cursorFilePath(accountId);
  try {
    if (existsSync(fp)) {
      const raw = readFileSync(fp, "utf-8");
      const parsed = JSON.parse(raw);
      if (typeof parsed.cursor === "number" && parsed.cursor > 0) {
        return parsed.cursor;
      }
    }
  } catch {
    /* Ignore a corrupt or unreadable cursor file. */
  }
  return -1;
}

export function persistCursor(accountId: string, cursor: number): void {
  const fp = cursorFilePath(accountId);
  try {
    const dir = dirname(fp);
    if (!existsSync(dir)) {
      mkdirSync(dir, { recursive: true });
    }
    writeFileSync(fp, JSON.stringify({ cursor, updatedAt: Date.now() }), "utf-8");
  } catch {
    /* Ignore cursor persistence failures; delivery can resume from server state. */
  }
}

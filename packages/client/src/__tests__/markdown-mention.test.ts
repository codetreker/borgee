// markdown-mention.test.ts — DM-2.3 (#377) §0 设计 ② lock for the
// mention render path in lib/markdown.ts (CV-1 既有 path 复用).
//
// Stance pinned: raw UUID lives in `data-mention-id` attr ONLY; text
// node renders `display_name` (or short-id fallback). Reverse grep on
// MessageList for full UUID strings stays at 0 hits.

import { describe, it, expect } from 'vitest';
import { renderMarkdown } from '../lib/markdown';

const FULL_UUID = '11111111-2222-3333-4444-555555555555';

describe('renderMarkdown — mention rendering (DM-2.3 #377)', () => {
  it('emits data-mention-id attr + display_name in text node', () => {
    const userMap = new Map<string, string>([[FULL_UUID, 'Alice']]);
    const html = renderMarkdown(`hello <@${FULL_UUID}> there`, [], userMap);

    // Attr present (data-mention-id with full UUID).
    expect(html).toContain(`data-mention-id="${FULL_UUID}"`);
    // Display name present in text node.
    expect(html).toMatch(/>@Alice</);
  });

  it('反约束: full UUID never appears as a text node — short-id fallback when display_name missing', () => {
    // No userMap entry — fallback path. Short-id (first 8 chars) renders
    // in the text node; the data-mention-id attr still carries the full
    // UUID so the click handler can resolve later.
    const userMap = new Map<string, string>(); // empty
    const html = renderMarkdown(`hi <@${FULL_UUID}>`, [], userMap);

    expect(html).toContain(`data-mention-id="${FULL_UUID}"`);
    // Sniff text-node-only portion: strip all data-* attrs and check the
    // remaining payload doesn't carry the full UUID. Cheap proxy for the
    // server-side reverse grep `[0-9a-f]{8}-[0-9a-f]{4}-...` 0 hit on
    // MessageList.tsx text content.
    const stripped = html.replace(/data-mention-id="[^"]*"/g, '');
    expect(stripped).not.toContain(FULL_UUID);
    // Short-id (first 8 chars) is acceptable in the text fallback.
    expect(stripped).toContain('11111111');
  });

  it('honors DOMPurify ALLOWED_ATTR — data-mention-id survives sanitize', () => {
    // DOMPurify strips unknown attrs; this test guards against a future
    // refactor that drops `data-mention-id` from ALLOWED_ATTR (would
    // silently break the反查 grep + click resolution path).
    const userMap = new Map<string, string>([[FULL_UUID, 'Bob']]);
    const html = renderMarkdown(`<@${FULL_UUID}>`, [], userMap);
    expect(html).toContain('data-mention-id=');
  });
});

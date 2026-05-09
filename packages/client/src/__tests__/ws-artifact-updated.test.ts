// ws-artifact-updated.test.ts — CV-1.3 client gap (mirrors
// ws-invitation.test.ts pattern).
//
// Validates the CV-1.2 push pipeline on the client side without booting
// a real WebSocket:
//   1. dispatchArtifactUpdated fires the locked window CustomEvent
//      with the 7-field frame as `detail` (envelope BPP-byte-identical
//      vs RT-1.1 #290 cursor.go::ArtifactUpdatedFrame).
//   2. Field-order discipline guard — drift here breaks server↔client
//      schema lock checked by BPP-1 #304 envelope CI lint on the server.
//   3. Event-name lock pins the literal so ArtifactPanel's listener
//      keeps subscribing to the same channel post-rename.
//
// Why mock-only: useWebSocket.ts's switch arm is a 4-line passthrough
// (case 'artifact_updated' → dispatchArtifactUpdated(data)). Once the
// dispatcher is proven, the wire integration is by inspection. The real
// WS + UI ≤3s contract is the playwright spec (cv-1-3-canvas.spec.ts).

import { describe, it, expect, vi } from 'vitest';
import { dispatchArtifactUpdated } from '../hooks/useWsHubFrames';
import {
  ARTIFACT_UPDATED_EVENT,
  type ArtifactUpdatedFrame,
} from '../types/ws-frames';

const commitFrame: ArtifactUpdatedFrame = {
  type: 'artifact_updated',
  cursor: 42,
  artifact_id: 'art-X',
  version: 7,
  channel_id: 'ch-Y',
  updated_at: 1700000000000,
  kind: 'commit',
};

const rollbackFrame: ArtifactUpdatedFrame = {
  type: 'artifact_updated',
  cursor: 43,
  artifact_id: 'art-X',
  version: 8,
  channel_id: 'ch-Y',
  updated_at: 1700000001000,
  kind: 'rollback',
};

describe('dispatchArtifactUpdated', () => {
  it('fires ARTIFACT_UPDATED_EVENT with the frame in detail', () => {
    const listener = vi.fn();
    window.addEventListener(ARTIFACT_UPDATED_EVENT, listener);
    try {
      dispatchArtifactUpdated(commitFrame);
    } finally {
      window.removeEventListener(ARTIFACT_UPDATED_EVENT, listener);
    }

    expect(listener).toHaveBeenCalledTimes(1);
    const evt = listener.mock.calls[0][0] as CustomEvent<ArtifactUpdatedFrame>;
    expect(evt.detail).toEqual(commitFrame);
  });

  it('preserves the 7-field byte-identical key order (RT-1.1 #290 lock)', () => {
    const listener = vi.fn();
    window.addEventListener(ARTIFACT_UPDATED_EVENT, listener);
    try {
      dispatchArtifactUpdated(commitFrame);
    } finally {
      window.removeEventListener(ARTIFACT_UPDATED_EVENT, listener);
    }
    const evt = listener.mock.calls[0][0] as CustomEvent<ArtifactUpdatedFrame>;
    // Drift guard: server cursor.go::ArtifactUpdatedFrame field order is
    // checked by BPP-1 #304 envelope CI lint server-side. This pins the
    // client-side mirror so type rename here breaks loud.
    expect(Object.keys(evt.detail)).toEqual([
      'type',
      'cursor',
      'artifact_id',
      'version',
      'channel_id',
      'updated_at',
      'kind',
    ]);
  });

  it('both kinds (commit / rollback) round-trip', () => {
    const listener = vi.fn();
    window.addEventListener(ARTIFACT_UPDATED_EVENT, listener);
    try {
      dispatchArtifactUpdated(commitFrame);
      dispatchArtifactUpdated(rollbackFrame);
    } finally {
      window.removeEventListener(ARTIFACT_UPDATED_EVENT, listener);
    }
    expect(listener).toHaveBeenCalledTimes(2);
    const kinds = listener.mock.calls.map(
      (c) => (c[0] as CustomEvent<ArtifactUpdatedFrame>).detail.kind,
    );
    expect(kinds).toEqual(['commit', 'rollback']);
  });

  it('reverse — frame envelope must NOT leak body or committer (设计 ⑤)', () => {
    // Push is signal-only per cv-1.md §2.5: body + committer come from
    // GET /api/v1/artifacts/:id. If a future frame schema slips body or
    // committer_kind in, this test catches it (frame keys must stay 7).
    const keys = Object.keys(commitFrame);
    expect(keys).not.toContain('body');
    expect(keys).not.toContain('committer_id');
    expect(keys).not.toContain('committer_kind');
    expect(keys.length).toBe(7);
  });
});

describe('event-name lock', () => {
  // Drift guard: ArtifactPanel.tsx subscribes via useArtifactUpdated which
  // hard-codes this constant — if the literal renames, the canvas tab
  // silently stops refreshing on commit/rollback.
  it('ARTIFACT_UPDATED_EVENT === borgee:artifact-updated', () => {
    expect(ARTIFACT_UPDATED_EVENT).toBe('borgee:artifact-updated');
  });
});

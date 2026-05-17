// useWsHubFrames.ts — RT-0 (#40) listener side.
//
// Bridges the two new /ws frames (agent_invitation_pending /
// agent_invitation_decided) into window-level CustomEvents that
// InvitationsInbox + Sidebar bell can subscribe to without coupling
// to the WebSocket hook.
//
// Why CustomEvent and not a context store: the existing precedent in
// useWebSocket is `case 'commands_updated': window.dispatchEvent(...)`.
// InvitationsInbox + Sidebar both already manage their own local
// state via REST fetch. The smallest change that removes polling is to
// fire an event and let those components re-fetch. Avoids touching the
// AppContext reducer for a Phase 2 transitional path that BPP will
// replace anyway.
//
// Phase 2 → Phase 4 contract: when BPP takes over (#40 Phase 4),
// the server change is `hub.Broadcast → bpp.SendFrame`; the
// client handler here remains unchanged because the schema is locked
// exact match (see frame_schemas.go CI lint).
//
// Wiring: useWebSocket.ts's handleMessage switch adds two cases that
// call into this module's dispatchers (kept exported for the WS hook
// to import directly — no React state needed).

import { useEffect } from 'react';
import {
  INVITATION_PENDING_EVENT,
  INVITATION_DECIDED_EVENT,
  ARTIFACT_UPDATED_EVENT,
  MENTION_PUSHED_EVENT,
  ANCHOR_COMMENT_ADDED_EVENT,
  ITERATION_STATE_CHANGED_EVENT,
  ARTIFACT_COMMENT_ADDED_EVENT,
  type AgentInvitationPendingFrame,
  type AgentInvitationDecidedFrame,
  type ArtifactUpdatedFrame,
  type MentionPushedFrame,
  type AnchorCommentAddedFrame,
  type IterationStateChangedFrame,
  type ArtifactCommentAddedFrame,
} from '../types/ws-frames';

/**
 * Fire `borgee:invitation-pending`. Called by the /ws message handler
 * (see useWebSocket.ts) when a `agent_invitation_pending` frame lands.
 */
export function dispatchInvitationPending(frame: AgentInvitationPendingFrame): void {
  window.dispatchEvent(
    new CustomEvent(INVITATION_PENDING_EVENT, { detail: frame }),
  );
}

/**
 * Fire `borgee:invitation-decided`. Called by the /ws message handler
 * when a `agent_invitation_decided` frame lands (used for cross-tab
 * sync — owner approves on phone, web refreshes).
 */
export function dispatchInvitationDecided(frame: AgentInvitationDecidedFrame): void {
  window.dispatchEvent(
    new CustomEvent(INVITATION_DECIDED_EVENT, { detail: frame }),
  );
}

/**
 * Hook that registers callbacks for invitation push events. Returns
 * void; cleans up listeners on unmount. The callbacks receive the
 * full frame so consumers can decide whether to re-fetch (cheap +
 * authoritative) or splice locally (cheaper but can diverge from REST state).
 *
 * Bell badge / inbox both opt for re-fetch in v1: server remains
 * authoritative, and push is only the wake-up signal.
 */
export function useInvitationFrames(handlers: {
  onPending?: (frame: AgentInvitationPendingFrame) => void;
  onDecided?: (frame: AgentInvitationDecidedFrame) => void;
}): void {
  const { onPending, onDecided } = handlers;

  useEffect(() => {
    if (!onPending) return;
    const listener = (e: Event) => {
      onPending((e as CustomEvent<AgentInvitationPendingFrame>).detail);
    };
    window.addEventListener(INVITATION_PENDING_EVENT, listener);
    return () => window.removeEventListener(INVITATION_PENDING_EVENT, listener);
  }, [onPending]);

  useEffect(() => {
    if (!onDecided) return;
    const listener = (e: Event) => {
      onDecided((e as CustomEvent<AgentInvitationDecidedFrame>).detail);
    };
    window.addEventListener(INVITATION_DECIDED_EVENT, listener);
    return () => window.removeEventListener(INVITATION_DECIDED_EVENT, listener);
  }, [onDecided]);
}

// ─── CV-1.2 ArtifactUpdated dispatch ────────────────────────
//
// Same precedent as invitation frames: useWebSocket.ts decodes the frame
// then calls dispatchArtifactUpdated; ArtifactPanel listens via
// useArtifactUpdated(handler) — handler decides whether to re-fetch
// (cheap + authoritative). Rule ⑤: envelope is signal-only, body comes
// from GET /api/v1/artifacts/:id (the handler call).

export function dispatchArtifactUpdated(frame: ArtifactUpdatedFrame): void {
  window.dispatchEvent(
    new CustomEvent(ARTIFACT_UPDATED_EVENT, { detail: frame }),
  );
}

export function useArtifactUpdated(
  handler: (frame: ArtifactUpdatedFrame) => void,
): void {
  useEffect(() => {
    const listener = (e: Event) => {
      handler((e as CustomEvent<ArtifactUpdatedFrame>).detail);
    };
    window.addEventListener(ARTIFACT_UPDATED_EVENT, listener);
    return () => window.removeEventListener(ARTIFACT_UPDATED_EVENT, listener);
  }, [handler]);
}

// ─── DM-2.2 MentionPushed dispatch (DM-2.3 client) ──────────
//
// Same precedent as artifact_updated: useWebSocket.ts decodes the frame
// then calls dispatchMentionPushed; MessageList listens via
// useMentionPushed(handler) — handler decides whether to refetch
// channel messages (cheap + authoritative). Rule ② envelope is
// signal-only — client MUST NOT use body_preview as message body
// (server has truncated to 80 runes for server data-trim; full body
// arrives via the existing message backfill path).

export function dispatchMentionPushed(frame: MentionPushedFrame): void {
  window.dispatchEvent(
    new CustomEvent(MENTION_PUSHED_EVENT, { detail: frame }),
  );
}

export function useMentionPushed(
  handler: (frame: MentionPushedFrame) => void,
): void {
  useEffect(() => {
    const listener = (e: Event) => {
      handler((e as CustomEvent<MentionPushedFrame>).detail);
    };
    window.addEventListener(MENTION_PUSHED_EVENT, listener);
    return () => window.removeEventListener(MENTION_PUSHED_EVENT, listener);
  }, [handler]);
}

// ─── CV-2.2 AnchorCommentAdded dispatch (CV-2.3 client) ─────
//
// Same precedent as artifact_updated / mention_pushed: useWebSocket.ts
// decodes the frame then calls dispatchAnchorCommentAdded;
// AnchorThreadPanel listens via useAnchorCommentAdded(handler) — handler
// decides whether to refetch the anchor thread via GET
// /api/v1/artifacts/:id/anchors + comments (cheap + authoritative).
// Rule ③ envelope is signal-only — frame carries no body. body arrives
// via the existing REST pull path (client must NOT use
// AnchorCommentAddedFrame to render comment text — it has no `body`).

export function dispatchAnchorCommentAdded(frame: AnchorCommentAddedFrame): void {
  window.dispatchEvent(
    new CustomEvent(ANCHOR_COMMENT_ADDED_EVENT, { detail: frame }),
  );
}

export function useAnchorCommentAdded(
  handler: (frame: AnchorCommentAddedFrame) => void,
): void {
  useEffect(() => {
    const listener = (e: Event) => {
      handler((e as CustomEvent<AnchorCommentAddedFrame>).detail);
    };
    window.addEventListener(ANCHOR_COMMENT_ADDED_EVENT, listener);
    return () => window.removeEventListener(ANCHOR_COMMENT_ADDED_EVENT, listener);
  }, [handler]);
}

// ─── CV-4.3 IterationStateChanged dispatch (CV-4.3 client) ──
//
// Same precedent as artifact_updated / anchor_comment_added: useWebSocket.ts
// decodes the frame then calls dispatchIterationStateChanged; IteratePanel
// listens via useIterationStateChanged(handler) — handler decides whether
// to refetch the iteration via GET /api/v1/artifacts/:id/iterations/:iid
// or splice locally on state change. envelope is signal-only — frame
// carries no intent_text (privacy constraint ADM-0 §1.3; intent_text is
// fetched via GET, push only carries the state signal).

export function dispatchIterationStateChanged(
  frame: IterationStateChangedFrame,
): void {
  window.dispatchEvent(
    new CustomEvent(ITERATION_STATE_CHANGED_EVENT, { detail: frame }),
  );
}

export function useIterationStateChanged(
  handler: (frame: IterationStateChangedFrame) => void,
): void {
  useEffect(() => {
    const listener = (e: Event) => {
      handler((e as CustomEvent<IterationStateChangedFrame>).detail);
    };
    window.addEventListener(ITERATION_STATE_CHANGED_EVENT, listener);
    return () => window.removeEventListener(ITERATION_STATE_CHANGED_EVENT, listener);
  }, [handler]);
}

// ─── CV-5 ArtifactCommentAdded dispatch ─────────────────────
//
// Same precedent as anchor_comment_added / iteration_state_changed:
// useWebSocket.ts decodes the frame then calls
// dispatchArtifactCommentAdded; ArtifactComments listens via
// useArtifactCommentAdded(handler) and refetches via GET
// /api/v1/artifacts/:id/comments (cheap + authoritative).
//
// Rule ② envelope is signal-only — full body arrives via REST
// pull (channel-member ACL). body_preview 80 rune (隐私 §13)
// is for badge/toast preview only, NOT the rendered comment text.

export function dispatchArtifactCommentAdded(
  frame: ArtifactCommentAddedFrame,
): void {
  window.dispatchEvent(
    new CustomEvent(ARTIFACT_COMMENT_ADDED_EVENT, { detail: frame }),
  );
}

export function useArtifactCommentAdded(
  handler: (frame: ArtifactCommentAddedFrame) => void,
): void {
  useEffect(() => {
    const listener = (e: Event) => {
      handler((e as CustomEvent<ArtifactCommentAddedFrame>).detail);
    };
    window.addEventListener(ARTIFACT_COMMENT_ADDED_EVENT, listener);
    return () => window.removeEventListener(ARTIFACT_COMMENT_ADDED_EVENT, listener);
  }, [handler]);
}

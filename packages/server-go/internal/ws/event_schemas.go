// Package ws — event_schemas.go: RT-0 (#40) source-of-truth for the
// agent invitation push frames defined in docs/blueprint/current/realtime.md
// §2.3.
//
// Phase 2 routes these via the existing /ws hub; Phase 4 BPP will swap
// the wire layer without changing the schema. The blueprint locks the
// promise that `bpp/frame_schemas.go` (Phase 4) and this file stay
// byte-identical or type-aliased — that is what makes "client handler
// 0 改" possible at the BPP cutover.
//
// Field ordering is part of the contract. The matching client TS
// interfaces live in packages/client/src/types/ws-frames.ts (PR #218):
//
//   pending : invitation_id, requester_user_id, agent_id, channel_id,
//             created_at, expires_at
//   decided : invitation_id, state, decided_at
//
// JSON tag values must equal the TS field names. Adding a field is a
// CI red unless the client side adds the same field in the same PR.
package ws

// Push frame `type` discriminator strings. These are what the client's
// `data.type` switch matches on.
const (
	FrameTypeAgentInvitationPending = "agent_invitation_pending"
	FrameTypeAgentInvitationDecided = "agent_invitation_decided"
)

// AgentInvitationPendingFrame — owner-side "someone wants to bring
// your agent into a channel" notification. Replaces the 60s polling
// loop on the bell badge per 野马 G2.4 hardline (latency ≤ 3s).
//
// Sent by the POST /api/v1/agent_invitations handler after the invite
// row is committed, addressed at the agent's owner.
type AgentInvitationPendingFrame struct {
	Type            string `json:"type"`
	InvitationID    string `json:"invitation_id"`
	RequesterUserID string `json:"requester_user_id"`
	AgentID         string `json:"agent_id"`
	ChannelID       string `json:"channel_id"`
	CreatedAt       int64  `json:"created_at"` // Unix ms
	ExpiresAt       int64  `json:"expires_at"` // Unix ms; 0 sentinel when absent (client TS marks required per PR #218)
}

// AgentInvitationDecidedFrame — cross-client sync of an invitation
// state change (owner approved/rejected on another device, or the
// server expired it). Sent to BOTH parties (requester + owner) so
// every open tab updates without polling.
type AgentInvitationDecidedFrame struct {
	Type         string `json:"type"`
	InvitationID string `json:"invitation_id"`
	State        string `json:"state"`      // "approved" | "rejected" | "expired"
	DecidedAt    int64  `json:"decided_at"` // Unix ms
}

// Package bpp — frame_schemas.go: RT-1.3 (#293) source-of-truth for
// the BPP `session.resume` / `session.resume_ack` agent-side handshake
// defined in docs/blueprint/bpp-protocol.md §session-resume.
//
// Layout contract (byte-identical with the RT-0 #237 invitation
// envelopes + the RT-1.1 #290 `artifact_updated` envelope):
//
//   - `Type` is the first JSON field — wire-layer dispatcher matches
//     on it.
//   - Semantic IDs / counters come next (`Mode`, `Since` / `Count`,
//     `Cursor`).
//   - There is NO `timestamp` field on the request/response themselves
//     because the cursor IS the order. See RT-1 spec §1 negative constraint:
//     clients MUST NOT sort by timestamp.
//
// Type aliasing: `internal/ws/event_schemas.go` and
// `internal/ws/cursor.go` already pin the wire schema for the
// server→client push frames (RT-0 + RT-1.1). This file owns the
// agent-runtime ↔ server resume handshake exclusively. When the BPP
// cutover lands, those `ws.*Frame` structs will be type-aliased here so
// the wire schema lives in one package; the resume frames live here
// from day one.

package bpp

// Frame `type` discriminator strings on the BPP wire. The matching
// runtime-side decoder lives in the agent SDK (Phase 4).
const (
	// FrameTypeSessionResume is sent by the agent runtime when the
	// plugin WS reconnects. Carries the desired replay `mode` + the
	// last cursor the runtime saw before it dropped.
	FrameTypeSessionResume = "session.resume"

	// FrameTypeSessionResumeAck is sent by the server in reply. Carries
	// the count of events the server is about to (re-)stream and the
	// max cursor the runtime should expect after the replay completes.
	FrameTypeSessionResumeAck = "session.resume_ack"
)

// ResumeMode is the replay-strategy enum carried on `session.resume`.
//
// Negative constraint (RT-1 spec §1.3 hardline): the server MUST NOT default the
// runtime into `Full`. The blueprint splits human vs. agent: agents are
// the only callers permitted to ask for `Full`, and even then they
// must do so explicitly. An empty / unknown mode falls back to
// `Incremental` (per parseResumeMode below) — never to `Full`.
type ResumeMode string

const (
	// ResumeModeIncremental — replay events strictly after `Since`.
	// Same semantics as the client REST backfill (`GET /api/v1/events
	// ?since=N`, RT-1.2 #292): server returns only `cursor > since`.
	// This is the default for any well-formed reconnect.
	ResumeModeIncremental ResumeMode = "incremental"

	// ResumeModeNone — cold start. Runtime declares it does not want a
	// replay (e.g. fresh process with no prior cursor). Server returns
	// an empty ack with `count=0` and the current high-water `cursor`.
	ResumeModeNone ResumeMode = "none"

	// ResumeModeFull — replay everything visible to the agent's channel
	// scope from cursor 0. Reserved for agent runtimes that have lost
	// their durable state and explicitly request a re-seed. Human
	// clients are forbidden from picking this mode; the negative constraint in the
	// resolver enforces this even when a malformed frame arrives.
	ResumeModeFull ResumeMode = "full"
)

// SessionResumeRequest — agent runtime → server. JSON layout is locked
// (`Type`, `Mode`, `Since` order). Adding a field is a CI red unless
// the agent-side schema in the SDK adds the same field in the same PR.
type SessionResumeRequest struct {
	Type  string     `json:"type"`
	Mode  ResumeMode `json:"mode"`
	Since int64      `json:"since"`
}

// SessionResumeAck — server → agent runtime. `Count` is the number of
// replay events about to follow on the wire; `Cursor` is the server's
// current high-water mark, which the runtime persists as its new
// last-seen cursor once the replay drains.
//
// Note: the events themselves are NOT inlined here. They follow as
// individual frames (one per event, same envelope as the live push)
// after the ack lands. This mirrors the REST backfill shape on the
// client side — ack is metadata, events are first-class frames.
type SessionResumeAck struct {
	Type   string `json:"type"`
	Count  int    `json:"count"`
	Cursor int64  `json:"cursor"`
}

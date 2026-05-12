// Package bpp — session_resume.go: RT-1.3 (#293) resolver for the
// `session.resume` / `session.resume_ack` agent handshake.
//
// Behaviour contract (RT-1 spec §1.3 hardline + review §0):
//
//   1. Three modes — `incremental` (default), `none` (cold start),
//      `full` (agent-explicit only).
//   2. Negative constraint: the server NEVER defaults a caller into `full`. An
//      empty, unknown, or malformed mode falls back to `incremental`.
//      Reverse grep `defaultReplayMode` / `= "full"` (excluding tests)
//      MUST be empty in this package; the only `Full` literal is the
//      enum constant itself.
//   3. `incremental` is byte-equivalent to the RT-1.2 client backfill
//      (`GET /api/v1/events?since=N`): server returns only events with
//      `cursor > since`, scoped to the caller's channels. The wire
//      shape on the BPP side is per-event frames following the ack;
//      this resolver returns them as `[]store.Event` so the caller
//      (plugin WS handler) can stream them out at its own pace.
//   4. `none` is the cold-start declaration. Resolver returns an empty
//      slice + the server's current high-water cursor. Used by a fresh
//      runtime process that has no prior `last_seen_cursor` and does
//      NOT want history pushed at it.
//   5. `full` requires explicit caller intent (the request literally
//      sets `mode: "full"`). Resolver replays from cursor 0 within the
//      agent's channel scope. There is no implicit fallback into this
//      branch; see ParseResumeMode below.

package bpp

import (
	"errors"

	"borgee-server/internal/store"
)

// EventLister is the narrow store surface this package needs. Concrete
// impl is `*store.Store` (queries_phase3.go); the interface keeps the
// resolver unit-testable without a SQLite fixture.
type EventLister interface {
	GetEventsSince(cursor int64, limit int, channelIDs []string) ([]store.Event, error)
	GetLatestCursor() int64
}

// DefaultResumeLimit caps the replay batch the resolver returns in one
// call. Mirrors the RT-1.2 REST backfill default (200) so a runtime
// that switched between BPP and REST sees the same window.
const DefaultResumeLimit = 200

// MaxResumeLimit is the hard ceiling the resolver enforces on a
// caller-supplied limit. Same value as the REST endpoint's clamp.
const MaxResumeLimit = 500

// ErrNoChannelScope is returned when the caller has no channel
// membership to scope the replay against. The plugin WS handler
// surfaces this as a no-op ack with count=0.
var ErrNoChannelScope = errors.New("bpp: empty channel scope")

// ParseResumeMode normalises the wire string into a ResumeMode value.
//
// Negative constraint: unknown / empty / mis-typed input MUST resolve to
// `Incremental`, NEVER to `Full`. The single `Full` branch is reachable
// ONLY when the caller's bytes are literally `"full"`. This is the
// gate the spec calls out: agents that want a full replay declare it
// explicitly; everyone else gets incremental.
func ParseResumeMode(raw string) ResumeMode {
	switch ResumeMode(raw) {
	case ResumeModeNone:
		return ResumeModeNone
	case ResumeModeFull:
		return ResumeModeFull
	case ResumeModeIncremental:
		return ResumeModeIncremental
	default:
		// Empty / unknown → incremental. Documented hardline.
		return ResumeModeIncremental
	}
}

// ResolveResume executes the resume request against the given store
// and returns (ack, events, error).
//
// `channelIDs` is the caller's permitted channel scope (per
// `Store.GetUserChannelIDs(userID)`). An empty scope short-circuits to
// an empty ack — the caller has nothing visible to replay.
//
// `limit` <= 0 falls back to DefaultResumeLimit. limit > MaxResumeLimit
// is clamped down. The clamp matches RT-1.2's REST endpoint so BPP and
// REST stay in lock-step.
func ResolveResume(es EventLister, req SessionResumeRequest, channelIDs []string, limit int) (SessionResumeAck, []store.Event, error) {
	if es == nil {
		return SessionResumeAck{}, nil, errors.New("bpp: nil event lister")
	}
	if limit <= 0 {
		limit = DefaultResumeLimit
	}
	if limit > MaxResumeLimit {
		limit = MaxResumeLimit
	}

	mode := ParseResumeMode(string(req.Mode))
	highWater := es.GetLatestCursor()

	switch mode {
	case ResumeModeNone:
		// Cold start — runtime explicitly opts out of replay. Ack with
		// the current high-water so the runtime can persist it as its
		// new last_seen_cursor on the next live frame.
		return SessionResumeAck{
			Type:   FrameTypeSessionResumeAck,
			Count:  0,
			Cursor: highWater,
		}, nil, nil

	case ResumeModeFull:
		// Explicit full replay — re-seed from cursor 0 within scope.
		// No reverse-default funnel reaches this branch; mode came
		// from the literal `"full"` parse path.
		if len(channelIDs) == 0 {
			return SessionResumeAck{
				Type:   FrameTypeSessionResumeAck,
				Count:  0,
				Cursor: highWater,
			}, nil, ErrNoChannelScope
		}
		events, err := es.GetEventsSince(0, limit, channelIDs)
		if err != nil {
			return SessionResumeAck{}, nil, err
		}
		return SessionResumeAck{
			Type:   FrameTypeSessionResumeAck,
			Count:  len(events),
			Cursor: highWater,
		}, events, nil

	default:
		// Incremental (the default). Negative `since` is treated as 0
		// so a malformed runtime can't accidentally trigger a full
		// scan via underflow.
		since := req.Since
		if since < 0 {
			since = 0
		}
		if len(channelIDs) == 0 {
			return SessionResumeAck{
				Type:   FrameTypeSessionResumeAck,
				Count:  0,
				Cursor: highWater,
			}, nil, ErrNoChannelScope
		}
		events, err := es.GetEventsSince(since, limit, channelIDs)
		if err != nil {
			return SessionResumeAck{}, nil, err
		}
		return SessionResumeAck{
			Type:   FrameTypeSessionResumeAck,
			Count:  len(events),
			Cursor: highWater,
		}, events, nil
	}
}

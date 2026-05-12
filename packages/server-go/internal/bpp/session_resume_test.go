// Package bpp — session_resume_test.go: RT-1.3 (#293) acceptance pin
// for the three-mode resume resolver + the negative assertion (no implicit `full`).
//
// Subtests map 1:1 to spec items:
//
//   §1.3 (a) `incremental` — events strictly after `since`
//             → TestResolveResumeIncremental
//   §1.3 (b) `none`        — empty replay, ack carries high-water
//             → TestResolveResumeNone
//   §1.3 (c) `full`        — replay from 0 (agent-explicit)
//             → TestResolveResumeFull
//   §1.3 (d) Negative assertion — unknown / empty / "FULL" / "Full"
//                                never resolves to Full
//             → TestParseResumeModeNeverDefaultsFull
//   §1.3 (e) byte-form     — request/ack JSON layout matches blueprint
//             → TestSessionResumeFrameFieldOrder

package bpp_test

import (
	"encoding/json"
	"strings"
	"testing"

	"borgee-server/internal/bpp"
	"borgee-server/internal/store"
)

// fakeLister is an in-memory EventLister so the resolver tests don't
// need a SQLite fixture. The resolver only calls GetEventsSince /
// GetLatestCursor.
type fakeLister struct {
	events    []store.Event
	highWater int64

	// captured args for assertions
	gotSince  int64
	gotLimit  int
	gotChans  []string
	callCount int
}

func (f *fakeLister) GetEventsSince(cursor int64, limit int, channelIDs []string) ([]store.Event, error) {
	f.gotSince = cursor
	f.gotLimit = limit
	f.gotChans = append([]string(nil), channelIDs...)
	f.callCount++
	out := make([]store.Event, 0, len(f.events))
	for _, e := range f.events {
		if e.Cursor > cursor {
			out = append(out, e)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (f *fakeLister) GetLatestCursor() int64 { return f.highWater }

func mkEvents(n int, channelID string) []store.Event {
	evs := make([]store.Event, n)
	for i := 0; i < n; i++ {
		evs[i] = store.Event{
			Cursor:    int64(i + 1),
			Kind:      "test",
			ChannelID: channelID,
		}
	}
	return evs
}

// (a) incremental — events strictly after `since`, server NEVER returns
// cursor <= since. Default fallback when mode is empty also lands here.
func TestResolveResumeIncremental(t *testing.T) {
	t.Parallel()
	lister := &fakeLister{
		events:    mkEvents(5, "ch1"),
		highWater: 5,
	}
	req := bpp.SessionResumeRequest{
		Type:  bpp.FrameTypeSessionResume,
		Mode:  bpp.ResumeModeIncremental,
		Since: 2,
	}
	ack, events, err := bpp.ResolveResume(lister, req, []string{"ch1"}, 0)
	if err != nil {
		t.Fatalf("incremental: unexpected err %v", err)
	}
	if ack.Type != bpp.FrameTypeSessionResumeAck {
		t.Fatalf("ack.Type = %q, want %q", ack.Type, bpp.FrameTypeSessionResumeAck)
	}
	if ack.Count != 3 {
		t.Fatalf("ack.Count = %d, want 3", ack.Count)
	}
	if ack.Cursor != 5 {
		t.Fatalf("ack.Cursor = %d, want 5 (high-water)", ack.Cursor)
	}
	for _, e := range events {
		if e.Cursor <= 2 {
			t.Fatalf("incremental returned cursor %d <= since=2 (negative assertion failed)", e.Cursor)
		}
	}
	if lister.gotSince != 2 {
		t.Fatalf("store called with since=%d, want 2", lister.gotSince)
	}
	if lister.gotLimit != bpp.DefaultResumeLimit {
		t.Fatalf("store called with limit=%d, want default %d", lister.gotLimit, bpp.DefaultResumeLimit)
	}
}

// (a') incremental fallback: empty / unknown mode — must take the
// incremental branch (NOT the full branch).
func TestResolveResumeUnknownModeFallsBackIncremental(t *testing.T) {
	t.Parallel()
	lister := &fakeLister{events: mkEvents(3, "ch1"), highWater: 3}
	for _, raw := range []string{"", "garbage", "FULL", "Full", "INCREMENTAL"} {
		l := *lister // shallow copy is OK; events slice is read-only here
		req := bpp.SessionResumeRequest{Mode: bpp.ResumeMode(raw), Since: 1}
		ack, events, err := bpp.ResolveResume(&l, req, []string{"ch1"}, 0)
		if err != nil {
			t.Fatalf("mode=%q: unexpected err %v", raw, err)
		}
		// Negative assertion: NEVER full. Full would return all 3 events
		// from cursor 0; incremental from since=1 returns 2 events.
		if ack.Count == 3 {
			t.Fatalf("mode=%q: negative assertion failed: unknown mode resolved to full (got count=3)", raw)
		}
		if ack.Count != 2 {
			t.Fatalf("mode=%q: incremental count = %d, want 2", raw, ack.Count)
		}
		for _, e := range events {
			if e.Cursor <= 1 {
				t.Fatalf("mode=%q: returned cursor %d <= since=1", raw, e.Cursor)
			}
		}
	}
}

// (b) none — cold start. Resolver MUST NOT touch the event store; ack
// carries the current high-water cursor and zero events.
func TestResolveResumeNone(t *testing.T) {
	t.Parallel()
	lister := &fakeLister{events: mkEvents(7, "ch1"), highWater: 7}
	req := bpp.SessionResumeRequest{Mode: bpp.ResumeModeNone, Since: 3}
	ack, events, err := bpp.ResolveResume(lister, req, []string{"ch1"}, 0)
	if err != nil {
		t.Fatalf("none: unexpected err %v", err)
	}
	if ack.Count != 0 {
		t.Fatalf("none: ack.Count = %d, want 0", ack.Count)
	}
	if len(events) != 0 {
		t.Fatalf("none: returned %d events, want 0 (cold start MUST NOT replay)", len(events))
	}
	if ack.Cursor != 7 {
		t.Fatalf("none: ack.Cursor = %d, want 7 (high-water)", ack.Cursor)
	}
	if lister.callCount != 0 {
		t.Fatalf("none: store.GetEventsSince called %d times, want 0", lister.callCount)
	}
}

// (c) full — agent-explicit. Replays from cursor 0 within scope.
func TestResolveResumeFull(t *testing.T) {
	t.Parallel()
	lister := &fakeLister{events: mkEvents(4, "ch1"), highWater: 4}
	req := bpp.SessionResumeRequest{Mode: bpp.ResumeModeFull, Since: 99 /* ignored */}
	ack, events, err := bpp.ResolveResume(lister, req, []string{"ch1"}, 0)
	if err != nil {
		t.Fatalf("full: unexpected err %v", err)
	}
	if ack.Count != 4 {
		t.Fatalf("full: ack.Count = %d, want 4", ack.Count)
	}
	if len(events) != 4 {
		t.Fatalf("full: returned %d events, want 4", len(events))
	}
	if lister.gotSince != 0 {
		t.Fatalf("full: store called with since=%d, want 0", lister.gotSince)
	}
	if events[0].Cursor != 1 {
		t.Fatalf("full: first event cursor = %d, want 1", events[0].Cursor)
	}
}

// (d) Negative assertion — ParseResumeMode never resolves anything other than the
// literal "full" string into ResumeModeFull.
func TestParseResumeModeNeverDefaultsFull(t *testing.T) {
	t.Parallel()
	// The ONLY input that yields Full.
	if got := bpp.ParseResumeMode("full"); got != bpp.ResumeModeFull {
		t.Fatalf("ParseResumeMode(\"full\") = %q, want full", got)
	}
	for _, raw := range []string{
		"",
		" ",
		"FULL",
		"Full",
		"full ",
		"none",
		"incremental",
		"unknown",
		"replay",
		"all",
		"history",
	} {
		got := bpp.ParseResumeMode(raw)
		if got == bpp.ResumeModeFull {
			t.Fatalf("negative assertion failed: ParseResumeMode(%q) = full (expected anything but full)", raw)
		}
	}
}

// (d') Reverse-grep guard inlined as a code-shape test — the resolver
// must not have a "default → full" path. We check by feeding a
// flooded set of bogus modes and asserting none of them produce the
// `full` branch's behaviour (replay from 0).
func TestResolverNeverDefaultsToFullBranch(t *testing.T) {
	t.Parallel()
	lister := &fakeLister{events: mkEvents(10, "ch1"), highWater: 10}
	for _, raw := range []string{"", "x", "FULL", "Full", "0", "1", "true"} {
		l := *lister
		req := bpp.SessionResumeRequest{Mode: bpp.ResumeMode(raw), Since: 5}
		ack, events, err := bpp.ResolveResume(&l, req, []string{"ch1"}, 0)
		if err != nil {
			t.Fatalf("mode=%q: %v", raw, err)
		}
		// Full would have count=10 (everything). Incremental from
		// since=5 has count=5. Anything yielding 10 is a regression.
		if ack.Count == 10 {
			t.Fatalf("mode=%q: resolver took full branch (count=10)", raw)
		}
		for _, e := range events {
			if e.Cursor <= 5 {
				t.Fatalf("mode=%q: leaked cursor %d <= since=5", raw, e.Cursor)
			}
		}
	}
}

// (e) frame layout — JSON field order/names match the blueprint. If the
// struct tags are reordered or renamed this test flips red.
func TestSessionResumeFrameFieldOrder(t *testing.T) {
	t.Parallel()
	req := bpp.SessionResumeRequest{
		Type:  bpp.FrameTypeSessionResume,
		Mode:  bpp.ResumeModeIncremental,
		Since: 42,
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"session.resume","mode":"incremental","since":42}`
	if string(b) != want {
		t.Fatalf("request bytes:\n  got:  %s\n  want: %s", b, want)
	}

	ack := bpp.SessionResumeAck{
		Type:   bpp.FrameTypeSessionResumeAck,
		Count:  3,
		Cursor: 99,
	}
	b, err = json.Marshal(ack)
	if err != nil {
		t.Fatal(err)
	}
	wantAck := `{"type":"session.resume_ack","count":3,"cursor":99}`
	if string(b) != wantAck {
		t.Fatalf("ack bytes:\n  got:  %s\n  want: %s", b, wantAck)
	}
}

// (f) limit clamp matches RT-1.2 REST endpoint: <=0 → 200, >500 → 500.
func TestResolveResumeLimitClamp(t *testing.T) {
	t.Parallel()
	lister := &fakeLister{events: mkEvents(5, "ch1"), highWater: 5}
	req := bpp.SessionResumeRequest{Mode: bpp.ResumeModeIncremental, Since: 0}

	_, _, _ = bpp.ResolveResume(lister, req, []string{"ch1"}, -7)
	if lister.gotLimit != bpp.DefaultResumeLimit {
		t.Fatalf("limit=-7 → store limit=%d, want %d", lister.gotLimit, bpp.DefaultResumeLimit)
	}

	_, _, _ = bpp.ResolveResume(lister, req, []string{"ch1"}, 9999)
	if lister.gotLimit != bpp.MaxResumeLimit {
		t.Fatalf("limit=9999 → store limit=%d, want %d", lister.gotLimit, bpp.MaxResumeLimit)
	}
}

// (g) empty channel scope short-circuits to ack(0, high-water) with
// ErrNoChannelScope, regardless of mode (except `none` which is already
// short-circuited before the scope check).
func TestResolveResumeEmptyScope(t *testing.T) {
	t.Parallel()
	lister := &fakeLister{highWater: 11}
	for _, mode := range []bpp.ResumeMode{bpp.ResumeModeIncremental, bpp.ResumeModeFull} {
		req := bpp.SessionResumeRequest{Mode: mode, Since: 0}
		ack, events, err := bpp.ResolveResume(lister, req, nil, 0)
		if err == nil || !strings.Contains(err.Error(), "channel scope") {
			t.Fatalf("mode=%s: err=%v, want ErrNoChannelScope", mode, err)
		}
		if ack.Count != 0 || len(events) != 0 {
			t.Fatalf("mode=%s: count=%d events=%d, want 0/0", mode, ack.Count, len(events))
		}
		if ack.Cursor != 11 {
			t.Fatalf("mode=%s: ack.Cursor=%d, want 11", mode, ack.Cursor)
		}
	}
}

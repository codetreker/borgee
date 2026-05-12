// Package ws_test — iteration_state_changed_frame_test.go: CV-4.2 envelope
// byte-identity lock and push coverage for IterationStateChangedFrame.
//
// The 9-field order is the contract per cv-4-spec.md (#365) +
// acceptance §2.4. Any reorder is caught here before merge, paired with
// BPP-1 #304 envelope CI lint.
package ws_test

import (
	"encoding/json"
	"testing"

	"borgee-server/internal/ws"
)

// TestIterationStateChangedFrameFieldOrder pins the 9-field byte-identical
// envelope order. JSON key order follows struct declaration order.
//
// Two snapshots: pending (zero-valued tail) + completed (filled tail).
// Both share the same 9-key order — confirms no omitempty branch.
func TestIterationStateChangedFrameFieldOrder(t *testing.T) {
	t.Parallel()

	pending := ws.IterationStateChangedFrame{
		Type:        ws.FrameTypeIterationStateChanged,
		Cursor:      42,
		IterationID: "it-A",
		ArtifactID:  "art-X",
		ChannelID:   "ch-Y",
		State:       ws.IterationStatePending,
		// ErrorReason / CreatedArtifactVersionID / CompletedAt zero-valued.
	}
	b, err := json.Marshal(&pending)
	if err != nil {
		t.Fatal(err)
	}
	wantPending := `{"type":"iteration_state_changed","cursor":42,"iteration_id":"it-A","artifact_id":"art-X","channel_id":"ch-Y","state":"pending","error_reason":"","created_artifact_version_id":0,"completed_at":0}`
	if string(b) != wantPending {
		t.Fatalf("IterationStateChanged pending envelope byte-identity broken:\n got: %s\nwant: %s", string(b), wantPending)
	}

	completed := ws.IterationStateChangedFrame{
		Type:                     ws.FrameTypeIterationStateChanged,
		Cursor:                   43,
		IterationID:              "it-A",
		ArtifactID:               "art-X",
		ChannelID:                "ch-Y",
		State:                    ws.IterationStateCompleted,
		ErrorReason:              "",
		CreatedArtifactVersionID: 7,
		CompletedAt:              1700000000001,
	}
	b, err = json.Marshal(&completed)
	if err != nil {
		t.Fatal(err)
	}
	wantCompleted := `{"type":"iteration_state_changed","cursor":43,"iteration_id":"it-A","artifact_id":"art-X","channel_id":"ch-Y","state":"completed","error_reason":"","created_artifact_version_id":7,"completed_at":1700000000001}`
	if string(b) != wantCompleted {
		t.Fatalf("IterationStateChanged completed envelope byte-identity broken:\n got: %s\nwant: %s", string(b), wantCompleted)
	}

	failed := ws.IterationStateChangedFrame{
		Type:        ws.FrameTypeIterationStateChanged,
		Cursor:      44,
		IterationID: "it-B",
		ArtifactID:  "art-X",
		ChannelID:   "ch-Y",
		State:       ws.IterationStateFailed,
		ErrorReason: "runtime_not_registered",
		CompletedAt: 1700000000002,
	}
	b, err = json.Marshal(&failed)
	if err != nil {
		t.Fatal(err)
	}
	wantFailed := `{"type":"iteration_state_changed","cursor":44,"iteration_id":"it-B","artifact_id":"art-X","channel_id":"ch-Y","state":"failed","error_reason":"runtime_not_registered","created_artifact_version_id":0,"completed_at":1700000000002}`
	if string(b) != wantFailed {
		t.Fatalf("IterationStateChanged failed envelope byte-identity broken:\n got: %s\nwant: %s", string(b), wantFailed)
	}
}

// TestPushIterationStateChanged checks that a fresh emit returns sent=true with a
// fresh cursor strictly above the prior artifact-updated head (same sequence,
// no separate channel — same pattern as anchor_comment_frame_test).
func TestPushIterationStateChanged(t *testing.T) {
	t.Parallel()
	hub, _ := setupTestHub(t)

	c1, sent1 := hub.PushArtifactUpdated("art-1", 1, "ch-1", 1700000000000, "commit")
	if !sent1 {
		t.Fatal("seed artifact push failed")
	}
	c2, sent2 := hub.PushIterationStateChanged(
		"it-1", "art-1", "ch-1", "running", "", 0, 0,
	)
	if !sent2 || c2 == 0 {
		t.Fatalf("iteration push must broadcast fresh frame; sent=%v cursor=%d", sent2, c2)
	}
	if c2 <= c1 {
		t.Fatalf("iteration cursor must be strictly above prior; c1=%d c2=%d", c1, c2)
	}
}

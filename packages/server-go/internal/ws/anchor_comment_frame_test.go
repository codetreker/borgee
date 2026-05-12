// Package ws_test — anchor_comment_frame_test.go: CV-2.2 envelope
// byte-identity lock + push smoke for AnchorCommentAddedFrame.
//
// The 10-field order is the contract per cv-2-spec.md §0 point 3 +
// v2 changelog (字段名 `author_kind` 不复用 CV-1 `committer_kind`). Any
// reorder caught here pre-merge — paired with BPP-1 #304 envelope CI lint.
package ws_test

import (
	"encoding/json"
	"testing"

	"borgee-server/internal/ws"
)

// TestAnchorCommentAddedFrameFieldOrder pins the 10-field byte-identical
// envelope order. JSON key order follows struct declaration order.
func TestAnchorCommentAddedFrameFieldOrder(t *testing.T) {
	t.Parallel()
	frame := ws.AnchorCommentAddedFrame{
		Type:              ws.FrameTypeAnchorCommentAdded,
		Cursor:            42,
		AnchorID:          "anc-A",
		CommentID:         100,
		ArtifactID:        "art-X",
		ArtifactVersionID: 7,
		ChannelID:         "ch-Y",
		AuthorID:          "u-1",
		AuthorKind:        "human",
		CreatedAt:         1700000000000,
	}
	b, err := json.Marshal(&frame)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"anchor_comment_added","cursor":42,"anchor_id":"anc-A","comment_id":100,"artifact_id":"art-X","artifact_version_id":7,"channel_id":"ch-Y","author_id":"u-1","author_kind":"human","created_at":1700000000000}`
	if string(b) != want {
		t.Fatalf("AnchorCommentAdded envelope byte-identity broken:\n got: %s\nwant: %s", string(b), want)
	}
}

// TestPushAnchorCommentAdded smoke: fresh emit returns sent=true with a
// fresh cursor strictly above the artifact-updated head.
func TestPushAnchorCommentAdded(t *testing.T) {
	t.Parallel()
	hub, _ := setupTestHub(t)

	// Seed an artifact_updated push first so the cursor moves and we can
	// confirm anchor_comment_added picks up the next slot (same sequence).
	c1, sent1 := hub.PushArtifactUpdated("art-1", 1, "ch-1", 1700000000000, "commit")
	if !sent1 {
		t.Fatal("seed artifact push failed")
	}
	c2, sent2 := hub.PushAnchorCommentAdded(
		"anc-1", 100, "art-1", 7, "ch-1", "u-1", "human", 1700000000001,
	)
	if !sent2 || c2 == 0 {
		t.Fatalf("anchor push must broadcast fresh frame; sent=%v cursor=%d", sent2, c2)
	}
	if c2 <= c1 {
		t.Fatalf("anchor cursor must be strictly above prior; c1=%d c2=%d", c1, c2)
	}
}

// Package ws_test — mention_pushed_frame_test.go: DM-2.2 envelope
// byte-identity lock + push smoke + body_preview rune-safe truncation.
//
// The 8-field order is the contract per dm-2-spec.md §0 point 2 + #362
// spec brief envelope. Any reorder caught here pre-merge — paired with
// BPP-1 #304 envelope CI lint.
package ws_test

import (
	"encoding/json"
	"strings"
	"testing"

	"borgee-server/internal/ws"
)

// TestMentionPushedFrameFieldOrder pins the 8-field byte-identical
// envelope order (#362 spec §0). JSON key order follows struct
// declaration order; drift breaks DM-2.3 client switch + BPP lint.
func TestMentionPushedFrameFieldOrder(t *testing.T) {
	t.Parallel()
	frame := ws.MentionPushedFrame{
		Type:            ws.FrameTypeMentionPushed,
		Cursor:          42,
		MessageID:       "msg-A",
		ChannelID:       "ch-Y",
		SenderID:        "u-sender",
		MentionTargetID: "u-target",
		BodyPreview:     "hello @u-target",
		CreatedAt:       1700000000000,
	}
	b, err := json.Marshal(&frame)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"mention_pushed","cursor":42,"message_id":"msg-A","channel_id":"ch-Y","sender_id":"u-sender","mention_target_id":"u-target","body_preview":"hello @u-target","created_at":1700000000000}`
	if string(b) != want {
		t.Fatalf("MentionPushed envelope byte-identity broken:\n got: %s\nwant: %s", string(b), want)
	}
}

// TestMentionPushedFrame_NoOwnerField — negative acceptance §1.0.e + spec
// §0 point 3: marshalled frame MUST NOT contain owner_id / target_owner /
// fanout_to_owner — mention never copies owner via this frame (offline owner
// fallback uses system DM, not this envelope).
func TestMentionPushedFrame_NoOwnerField(t *testing.T) {
	t.Parallel()
	frame := ws.MentionPushedFrame{
		Type:            ws.FrameTypeMentionPushed,
		MessageID:       "msg-A",
		ChannelID:       "ch-Y",
		SenderID:        "u-sender",
		MentionTargetID: "u-target",
		BodyPreview:     "x",
		CreatedAt:       1,
	}
	b, _ := json.Marshal(&frame)
	got := string(b)
	for _, forbidden := range []string{
		"owner_id",
		"target_owner",
		"fanout_to_owner",
		"cc_owner",
	} {
		if strings.Contains(got, forbidden) {
			t.Errorf("MentionPushed envelope contains forbidden field %q — 反约束 broken (立场 ③ 不抄送 owner): %s", forbidden, got)
		}
	}
}

// TestTruncateBodyPreview pins the 80-rune cap (UTF-8 rune-safe). Privacy
// stance §13: full body does not enter mention frame, only an 80-character
// preview; prevents full raw body leakage through this frame.
func TestTruncateBodyPreview(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		input    string
		wantLen  int // rune count
		wantSame bool
	}{
		{"short ascii", "hello @x", 8, true},
		{"exactly 80 ascii", strings.Repeat("a", 80), 80, true},
		{"81 ascii truncated to 80", strings.Repeat("a", 81), 80, false},
		{"200 ascii truncated to 80", strings.Repeat("a", 200), 80, false},
		// 100 multibyte runes (CJK) — rune count, not byte count, drives the cap.
		{"100 cjk truncated to 80 runes", strings.Repeat("中", 100), 80, false},
		{"empty", "", 0, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := ws.TruncateBodyPreview(tc.input)
			gotRunes := len([]rune(got))
			if gotRunes != tc.wantLen {
				t.Errorf("rune count: got %d want %d (input %q → %q)", gotRunes, tc.wantLen, tc.input, got)
			}
			if tc.wantSame && got != tc.input {
				t.Errorf("expected unchanged: got %q want %q", got, tc.input)
			}
		})
	}
}

// TestPushMentionPushed smoke: fresh emit returns sent=true with cursor
// > prior pushes (shares sequence with ArtifactUpdated / AnchorCommentAdded — RT-1 §1.1).
func TestPushMentionPushed(t *testing.T) {
	t.Parallel()
	hub, _ := setupTestHub(t)

	c1, sent1 := hub.PushArtifactUpdated("art-1", 1, "ch-1", 1700000000000, "commit")
	if !sent1 {
		t.Fatal("seed artifact push failed")
	}
	c2, sent2 := hub.PushMentionPushed(
		"msg-1", "ch-1", "u-sender", "u-target", "preview", 1700000000001,
	)
	if !sent2 || c2 == 0 {
		t.Fatalf("mention push must broadcast fresh frame; sent=%v cursor=%d", sent2, c2)
	}
	if c2 <= c1 {
		t.Fatalf("mention cursor must be strictly above prior; c1=%d c2=%d", c1, c2)
	}
}

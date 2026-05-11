// Package api — dm_10_pin_dberror_test.go: DM-10 #6 coverage bump (C),
// TestDM_HandlePin_DBError_500. This file uses `package api` so it can access
// setupFullTestServer and exerciseAuthedHandler, matching the
// TestClosedStoreInternalErrorBranches closed-store 500 pattern.

package api

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
)

// TestDM_HandlePin_DBError_500 — SQLite query_only=ON before
// SetMessagePinnedAt → INSERT/UPDATE fail while SELECT (gateDM lookups)
// still succeed, forcing the handler through the "Failed to pin message" 500
// branch.
//
// State-based fault injection, matching TestClosedStoreInternalErrorBranches:
// SQLite read-only / missing-table driver error paths.
func TestDM_HandlePin_DBError_500(t *testing.T) {
	t.Parallel()
	ts, s, cfg := setupFullTestServer(t)
	memberToken := loginAs(t, ts.URL, "member@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")
	if owner == nil || member == nil {
		t.Skip("missing fixture users")
	}

	// Open DM channel + post a message before closing store.
	ch := &store.Channel{
		Name: "dm-pin-dberror-test",
		Type: "dm", Visibility: "private",
		CreatedBy: member.ID,
		Position:  store.GenerateInitialRank(),
		OrgID:     member.OrgID,
	}
	if err := s.CreateChannel(ch); err != nil {
		t.Fatalf("create DM channel: %v", err)
	}
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: ch.ID, UserID: member.ID}); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: ch.ID, UserID: owner.ID}); err != nil {
		t.Fatalf("add owner: %v", err)
	}
	msg := &store.Message{
		ChannelID: ch.ID,
		SenderID:  member.ID,
		Content:   "to-pin",
	}
	if err := s.CreateMessage(msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	pattern := "POST /api/v1/channels/{channelId}/messages/{messageId}/pin"
	target := "/api/v1/channels/" + ch.ID + "/messages/" + msg.ID + "/pin"
	handler := (&MessagePinHandler{Store: s, Logger: testLogger()}).handlePin

	rec := exerciseAuthedHandler(t, s, cfg, memberToken, pattern, "POST", target, nil,
		func(w http.ResponseWriter, r *http.Request) {
			// PRAGMA query_only=ON — gateDM SELECTs still pass, but
			// SetMessagePinnedAt UPDATE fails → handler reaches the
			// "Failed to pin message" 500 branch.
			s.DB().Exec("PRAGMA query_only = ON")
			defer s.DB().Exec("PRAGMA query_only = OFF")
			handler(w, r)
		})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("DBError_500: expected 500 (Failed to pin message), got %d body=%s",
			rec.Code, rec.Body.String())
	}
}

// TestDM_HandleUnpin_DBError_500 — same query_only trick for unpin path.
//
// State-based fault injection, matching TestClosedStoreInternalErrorBranches:
// SQLite read-only / missing-table driver error paths.
func TestDM_HandleUnpin_DBError_500(t *testing.T) {
	t.Parallel()
	ts, s, cfg := setupFullTestServer(t)
	memberToken := loginAs(t, ts.URL, "member@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")
	if owner == nil || member == nil {
		t.Skip("missing fixture users")
	}
	ch := &store.Channel{
		Name: "dm-unpin-dberror-test",
		Type: "dm", Visibility: "private",
		CreatedBy: member.ID,
		Position:  store.GenerateInitialRank(),
		OrgID:     member.OrgID,
	}
	if err := s.CreateChannel(ch); err != nil {
		t.Fatalf("create DM channel: %v", err)
	}
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: ch.ID, UserID: member.ID}); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: ch.ID, UserID: owner.ID}); err != nil {
		t.Fatalf("add owner: %v", err)
	}
	msg := &store.Message{ChannelID: ch.ID, SenderID: member.ID, Content: "x"}
	if err := s.CreateMessage(msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	pattern := "DELETE /api/v1/channels/{channelId}/messages/{messageId}/pin"
	target := "/api/v1/channels/" + ch.ID + "/messages/" + msg.ID + "/pin"
	handler := (&MessagePinHandler{Store: s, Logger: testLogger()}).handleUnpin

	rec := exerciseAuthedHandler(t, s, cfg, memberToken, pattern, "DELETE", target, nil,
		func(w http.ResponseWriter, r *http.Request) {
			s.DB().Exec("PRAGMA query_only = ON")
			defer s.DB().Exec("PRAGMA query_only = OFF")
			handler(w, r)
		})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Unpin_DBError: expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestDM_HandleListPinned_DBError_500 — drop messages table after
// gateDM passes → ListPinnedMessages SELECT fails → handler reaches
// "Failed to list pinned messages" 500 branch.
//
// State-based fault injection, matching TestClosedStoreInternalErrorBranches:
// SQLite read-only / missing-table driver error paths.
func TestDM_HandleListPinned_DBError_500(t *testing.T) {
	t.Parallel()
	ts, s, cfg := setupFullTestServer(t)
	memberToken := loginAs(t, ts.URL, "member@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")
	if owner == nil || member == nil {
		t.Skip("missing fixture users")
	}
	ch := &store.Channel{
		Name: "dm-list-dberror-test",
		Type: "dm", Visibility: "private",
		CreatedBy: member.ID,
		Position:  store.GenerateInitialRank(),
		OrgID:     member.OrgID,
	}
	if err := s.CreateChannel(ch); err != nil {
		t.Fatalf("create DM channel: %v", err)
	}
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: ch.ID, UserID: member.ID}); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: ch.ID, UserID: owner.ID}); err != nil {
		t.Fatalf("add owner: %v", err)
	}

	pattern := "GET /api/v1/channels/{channelId}/messages/pinned"
	target := "/api/v1/channels/" + ch.ID + "/messages/pinned"
	handler := (&MessagePinHandler{Store: s, Logger: testLogger()}).handleListPinned

	rec := exerciseAuthedHandler(t, s, cfg, memberToken, pattern, "GET", target, nil,
		func(w http.ResponseWriter, r *http.Request) {
			// Drop the messages table — gateDM only needs channels table
			// (passed already in middleware). ListPinnedMessages SELECTs
			// from messages → SQL error → handler 500 branch.
			s.DB().Exec("DROP TABLE messages")
			handler(w, r)
		})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("List_DBError: expected 500 (Failed to list pinned messages), got %d body=%s",
			rec.Code, rec.Body.String())
	}
}

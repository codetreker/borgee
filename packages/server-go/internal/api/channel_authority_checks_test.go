package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/testutil"
)

func TestChannelAuthorityChecks(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	owner := mustUserByEmail(t, s, "owner@test.com")
	member := mustUserByEmail(t, s, "member@test.com")

	t.Run("creator cannot leave through leave endpoint", func(t *testing.T) {
		ch := testutil.CreateChannel(t, ts.URL, ownerToken, "owner-leave-guard", "public")
		chID := stringField(t, ch, "id")

		resp, body := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+chID+"/leave", ownerToken, nil)
		requireStatus(t, resp, http.StatusBadRequest, body)
		if !s.IsChannelMember(chID, owner.ID) {
			t.Fatalf("creator membership was removed after rejected leave")
		}
	})

	t.Run("non-member cannot leave public channel", func(t *testing.T) {
		ch := testutil.CreateChannel(t, ts.URL, ownerToken, "non-member-leave-guard", "public")
		chID := stringField(t, ch, "id")
		if s.IsChannelMember(chID, member.ID) {
			t.Fatalf("member fixture unexpectedly joined creator-only channel")
		}

		resp, body := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+chID+"/leave", memberToken, nil)
		requireStatus(t, resp, http.StatusForbidden, body)
	})

	t.Run("non-creator cannot delete channel despite broad permissions", func(t *testing.T) {
		ch := testutil.CreateChannel(t, ts.URL, ownerToken, "delete-owner-guard", "public")
		chID := stringField(t, ch, "id")

		resp, body := testutil.JSON(t, http.MethodDelete, ts.URL+"/api/v1/channels/"+chID, memberToken, nil)
		requireStatus(t, resp, http.StatusForbidden, body)
		if _, err := s.GetChannelByID(chID); err != nil {
			t.Fatalf("channel was deleted after rejected non-creator delete: %v", err)
		}
	})

	t.Run("non-creator cannot archive channel despite broad permissions", func(t *testing.T) {
		ch := testutil.CreateChannel(t, ts.URL, ownerToken, "archive-owner-guard", "public")
		chID := stringField(t, ch, "id")

		resp, body := testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/channels/"+chID, memberToken, map[string]any{"archived": true})
		requireStatus(t, resp, http.StatusForbidden, body)
		got, err := s.GetChannelByID(chID)
		if err != nil {
			t.Fatalf("get channel after rejected archive: %v", err)
		}
		if got.ArchivedAt != nil {
			t.Fatalf("channel was archived after rejected non-creator archive")
		}
	})

	t.Run("membership management cannot remove channel creator", func(t *testing.T) {
		ch := testutil.CreateChannel(t, ts.URL, ownerToken, "remove-owner-guard", "public")
		chID := stringField(t, ch, "id")
		resp, body := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+chID+"/join", memberToken, nil)
		requireStatus(t, resp, http.StatusOK, body)

		resp, body = testutil.JSON(t, http.MethodDelete, ts.URL+"/api/v1/channels/"+chID+"/members/"+owner.ID, memberToken, nil)
		requireStatus(t, resp, http.StatusBadRequest, body)
		if !s.IsChannelMember(chID, owner.ID) {
			t.Fatalf("creator membership was removed by non-creator")
		}
	})
}

package api_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"borgee-server/internal/testutil"
)

func TestP2ConcurrentHumanAgentMessages(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	channelID := testutil.GetGeneralChannelID(t, ts.URL, adminToken)

	agent := testutil.CreateAgent(t, ts.URL, memberToken, "P2 Relay Bot")
	agentKey := stringField(t, agent, "api_key")
	agentID := stringField(t, agent, "id")

	root := testutil.PostMessage(t, ts.URL, memberToken, channelID, "human starts a thread")
	rootID := stringField(t, root, "id")

	type postCase struct {
		token       string
		content     string
		contentType string
		replyToID   string
	}

	cases := []postCase{
		{token: memberToken, content: "human note for <@" + agentID + ">", contentType: "text", replyToID: rootID},
		{token: agentKey, content: "agent command result for @Member", contentType: "command", replyToID: rootID},
		{token: memberToken, content: "human image payload", contentType: "image"},
		{token: agentKey, content: "agent plain response"},
	}

	var wg sync.WaitGroup
	errs := make(chan string, len(cases))
	for i, tc := range cases {
		wg.Add(1)
		go func(i int, tc postCase) {
			defer wg.Done()
			body := map[string]any{"content": fmt.Sprintf("%s #%d", tc.content, i)}
			if tc.contentType != "" {
				body["content_type"] = tc.contentType
			}
			if tc.replyToID != "" {
				body["reply_to_id"] = tc.replyToID
			}
			resp, data := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+channelID+"/messages", tc.token, body)
			if resp.StatusCode != http.StatusCreated {
				errs <- fmt.Sprintf("post %d status %d body %v", i, resp.StatusCode, data)
				return
			}
			msg := data["message"].(map[string]any)
			if msg["channel_id"] != channelID {
				errs <- fmt.Sprintf("post %d channel mismatch: %v", i, msg)
			}
		}(i, tc)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	resp, data := testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/channels/"+channelID+"/messages/search?q=agent", memberToken, nil)
	requireStatus(t, resp, http.StatusOK, data)
	if len(data["messages"].([]any)) == 0 {
		t.Fatalf("expected search to find agent messages, got %v", data)
	}
}

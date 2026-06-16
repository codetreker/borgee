package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/testutil"
)

func TestP1RemoteNodeBasics(t *testing.T) {
	t.Parallel()
	ts, store, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")
	channelID := testutil.GetGeneralChannelID(t, ts.URL, token)

	resp, data := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/remote/nodes", token, map[string]string{"machine_name": "ci-node"})
	requireStatus(t, resp, http.StatusCreated, data)
	nodeID := stringField(t, data["node"].(map[string]any), "id")
	node, err := store.GetRemoteNode(nodeID)
	if err != nil {
		t.Fatalf("get remote node: %v", err)
	}

	remote := testutil.DialWS(t, ts.URL, "/ws/remote", node.ConnectionToken)
	testutil.WSWriteJSON(t, remote, map[string]string{"type": "ping"})
	if msg := testutil.WSReadUntil(t, remote, "pong"); msg["type"] != "pong" {
		t.Fatalf("expected remote pong, got %v", msg)
	}

	resp, data = testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/remote/nodes/"+nodeID+"/status", token, nil)
	requireStatus(t, resp, http.StatusOK, data)
	if data["online"] != true {
		t.Fatalf("expected remote node online: %v", data)
	}

	resp, data = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/remote/nodes/"+nodeID+"/bindings", token, map[string]string{"channel_id": channelID, "path": "/repo", "label": "Repo"})
	requireStatus(t, resp, http.StatusCreated, data)
	bindingID := stringField(t, data["binding"].(map[string]any), "id")

	done := make(chan struct{})
	go func() {
		request := testutil.WSReadUntil(t, remote, "request")
		payload := request["data"].(map[string]any)
		if payload["action"] != "ls" {
			t.Errorf("expected ls proxy request, got %v", request)
		}
		testutil.WSWriteJSON(t, remote, map[string]any{
			"type": "response",
			"id":   request["id"],
			"data": map[string]any{"entries": []map[string]any{{"name": "README.md", "type": "file"}}},
		})
		close(done)
	}()

	resp, data = testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/remote/nodes/"+nodeID+"/ls?path=/repo", token, nil)
	requireStatus(t, resp, http.StatusOK, data)
	entries := data["entries"].([]any)
	if entries[0].(map[string]any)["name"] != "README.md" {
		t.Fatalf("unexpected remote ls response: %v", data)
	}
	<-done

	resp, data = testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/channels/"+channelID+"/remote-bindings", token, nil)
	requireStatus(t, resp, http.StatusOK, data)
	if !containsObjectWithID(data["bindings"].([]any), bindingID) {
		t.Fatalf("channel binding missing: %v", data)
	}

	resp, data = testutil.JSON(t, http.MethodDelete, ts.URL+"/api/v1/remote/nodes/"+nodeID+"/bindings/"+bindingID, token, nil)
	requireStatus(t, resp, http.StatusOK, data)
	resp, data = testutil.JSON(t, http.MethodDelete, ts.URL+"/api/v1/remote/nodes/"+nodeID, token, nil)
	requireStatus(t, resp, http.StatusOK, data)
}

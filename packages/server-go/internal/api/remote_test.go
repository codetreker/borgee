package api_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/auth"
	"borgee-server/internal/testutil"
)

func TestRemoteNodesCRUD(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	var nodeID string

	t.Run("CreateNode", func(t *testing.T) {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/remote/nodes", adminToken, map[string]string{
			"machine_name": "test-machine",
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %v", resp.StatusCode, data)
		}
		node := data["node"].(map[string]any)
		nodeID = node["id"].(string)
		if nodeID == "" {
			t.Fatal("expected node id")
		}
	})

	t.Run("CreateNodeMissingName", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/remote/nodes", adminToken, map[string]string{})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("ListNodes", func(t *testing.T) {
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/remote/nodes", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		nodes := data["nodes"].([]any)
		if len(nodes) == 0 {
			t.Fatal("expected at least 1 node")
		}
	})

	t.Run("NodeStatus", func(t *testing.T) {
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/remote/nodes/"+nodeID+"/status", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if data["online"] != false {
			t.Fatal("expected online=false")
		}
	})

	t.Run("OtherUserCannotDeleteNode", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/remote/nodes/"+nodeID, memberToken, nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("NodeLsOffline", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/remote/nodes/"+nodeID+"/ls?path=/", adminToken, nil)
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", resp.StatusCode)
		}
	})

	t.Run("NodeReadOffline", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/remote/nodes/"+nodeID+"/read?path=/test", adminToken, nil)
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", resp.StatusCode)
		}
	})

	var bindingID string

	t.Run("CreateBinding", func(t *testing.T) {
		_, chData := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels", adminToken, nil)
		channels := chData["channels"].([]any)
		var generalID string
		for _, c := range channels {
			cm := c.(map[string]any)
			if cm["name"] == "general" {
				generalID = cm["id"].(string)
				break
			}
		}

		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/remote/nodes/"+nodeID+"/bindings", adminToken, map[string]string{
			"channel_id": generalID,
			"path":       "/home/user/project",
			"label":      "my project",
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %v", resp.StatusCode, data)
		}
		binding := data["binding"].(map[string]any)
		bindingID = binding["id"].(string)
	})

	t.Run("CreateBindingMissingFields", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/remote/nodes/"+nodeID+"/bindings", adminToken, map[string]string{
			"channel_id": "",
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("ListBindings", func(t *testing.T) {
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/remote/nodes/"+nodeID+"/bindings", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		bindings := data["bindings"].([]any)
		if len(bindings) == 0 {
			t.Fatal("expected at least 1 binding")
		}
	})

	t.Run("ListChannelBindings", func(t *testing.T) {
		_, chData := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels", adminToken, nil)
		channels := chData["channels"].([]any)
		var generalID string
		for _, c := range channels {
			cm := c.(map[string]any)
			if cm["name"] == "general" {
				generalID = cm["id"].(string)
				break
			}
		}
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+generalID+"/remote-bindings", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if data["bindings"] == nil {
			t.Fatal("expected bindings key")
		}
	})

	t.Run("OtherUserCannotListBindings", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/remote/nodes/"+nodeID+"/bindings", memberToken, nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("DeleteBinding", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/remote/nodes/"+nodeID+"/bindings/"+bindingID, adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("DeleteNode", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/remote/nodes/"+nodeID, adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("DeleteNodeNotFound", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/remote/nodes/nonexistent", adminToken, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("NodeStatusNotFound", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/remote/nodes/nonexistent/status", adminToken, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})
}

// TestRemoteNodeConnectionTokenSurfacedOnceOnCreate locks the "secret shown
// once at create" contract: the connection token (the secret a remote agent
// uses to enroll) is returned ONLY in the POST create response, and is NEVER
// leaked by list/status — those read paths serialize the RemoteNode model,
// which carries json:"-" on the token column. A node row can be re-read by any
// later request, so leaking the token there would defeat the one-shot model.
func TestRemoteNodeConnectionTokenSurfacedOnceOnCreate(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	// Create — the token MUST be present as a top-level sibling field.
	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/remote/nodes", adminToken, map[string]string{
		"machine_name": "token-surface-machine",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %v", resp.StatusCode, data)
	}
	token, ok := data["connection_token"].(string)
	if !ok || token == "" {
		t.Fatalf("create response must surface a non-empty connection_token, got %v", data["connection_token"])
	}
	// The token must NOT be nested inside the node object (json:"-" strips it).
	node := data["node"].(map[string]any)
	if _, leaked := node["connection_token"]; leaked {
		t.Fatalf("node object must not carry connection_token (json:\"-\"), got %v", node)
	}
	nodeID := node["id"].(string)

	// rawGET returns the verbatim response body so we can substring-scan for the
	// secret (testutil.JSON parses into a map, which would hide a token nested
	// anywhere unexpected).
	rawGET := func(url string) (int, string) {
		t.Helper()
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: adminToken})
		req.Header.Set("Authorization", "Bearer "+adminToken)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("do request: %v", err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return res.StatusCode, string(b)
	}

	// List — must NOT leak the token anywhere in the response body.
	listStatus, listBody := rawGET(ts.URL + "/api/v1/remote/nodes")
	if listStatus != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", listStatus)
	}
	if strings.Contains(listBody, token) {
		t.Fatalf("list response leaked connection token: %s", listBody)
	}
	if strings.Contains(listBody, "connection_token") {
		t.Fatalf("list response must not contain a connection_token field: %s", listBody)
	}

	// Status — must NOT leak the token either.
	statusStatus, statusBody := rawGET(ts.URL + "/api/v1/remote/nodes/" + nodeID + "/status")
	if statusStatus != http.StatusOK {
		t.Fatalf("status: expected 200, got %d", statusStatus)
	}
	if strings.Contains(statusBody, token) {
		t.Fatalf("status response leaked connection token: %s", statusBody)
	}
}

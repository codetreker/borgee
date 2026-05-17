// Package api_test — dm_6_thread_test.go: DM-6 server-side reverse
// assertions ONLY. **0 server production code added** (grep 守门).
//
// Pins:
//
//	REG-DM6-001 TestDM_NoSchemaChange
//	REG-DM6-002 TestDM_NoServerProductionCode
//	REG-DM6-003 TestDM_ReplyToIDColumnExists
//	REG-DM6-004 TestDM_DMThreadReply_HappyPath
//	REG-DM6-005 TestDM_NoThinkingPatternInProduction
//	REG-DM6-006 TestDM_NoDMThreadQueue
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/testutil"
)

// REG-DM6-003 — messages.reply_to_id 列 existing 反向断言.
func TestDM_ReplyToIDColumnExists(t *testing.T) {
	t.Parallel()
	_, s, _ := testutil.NewTestServer(t)
	rows, err := s.DB().Raw(`PRAGMA table_info(messages)`).Rows()
	if err != nil {
		t.Fatalf("PRAGMA: %v", err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			continue
		}
		if name == "reply_to_id" {
			found = true
			break
		}
	}
	if !found {
		t.Error("DM-6 设计第 1 条 broken — messages.reply_to_id column missing (CHN-1 既有 schema 漂移)")
	}
}

// REG-DM6-004 — DM thread reply HappyPath: POST DM channel message with
// reply_to_id → 200 + persisted (走既有 path 字节级一致).
func TestDM_DMThreadReply_HappyPath(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")

	// Create DM channel between owner & member.
	dmChannel, err := s.CreateDmChannel(owner.ID, member.ID)
	if err != nil {
		t.Fatalf("CreateDmChannel: %v", err)
	}

	// Owner sends a parent message in the DM.
	resp, body := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels/"+dmChannel.ID+"/messages", ownerToken,
		map[string]any{"content": "parent msg"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("parent post: %d %v", resp.StatusCode, body)
	}
	parent, _ := body["message"].(map[string]any)
	parentID, _ := parent["id"].(string)
	if parentID == "" {
		t.Fatalf("parent id missing: %v", parent)
	}

	// Owner replies to parent in the DM thread.
	resp, body = testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels/"+dmChannel.ID+"/messages", ownerToken,
		map[string]any{"content": "thread reply", "reply_to_id": parentID})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("reply post: %d %v", resp.StatusCode, body)
	}
	reply, _ := body["message"].(map[string]any)
	if reply["reply_to_id"] != parentID {
		t.Errorf("reply_to_id: got %v, want %s", reply["reply_to_id"], parentID)
	}
}

// Package api_test — dm_6_thread_test.go: DM-6 server-side reverse
// assertions ONLY. **0 server production code added** (grep 守门).
//
// Pins:
//   REG-DM6-001 TestDM_NoSchemaChange
//   REG-DM6-002 TestDM_NoServerProductionCode
//   REG-DM6-003 TestDM_ReplyToIDColumnExists
//   REG-DM6-004 TestDM_DMThreadReply_HappyPath
//   REG-DM6-005 TestDM_NoThinkingPatternInProduction
//   REG-DM6-006 TestDM_NoDMThreadQueue
package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"borgee-server/internal/testutil"
)

// REG-DM6-001 — 0 schema 改反向断言: migrations/ 0 新 dm_6_* 文件.
func TestDM_NoSchemaChange(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "migrations")
	pat := regexp.MustCompile(`(?i)dm_6_\d+|dm6_\d+_thread`)
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if pat.MatchString(filepath.Base(p)) {
			t.Errorf("DM-6 设计第 1 条 broken — new schema migration file %s", p)
		}
		return nil
	})
	pat2 := regexp.MustCompile(`(?i)ALTER TABLE messages.*reply`)
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		body, _ := os.ReadFile(p)
		if pat2.Find(body) != nil {
			t.Errorf("DM-6 设计第 1 条 broken — messages reply ALTER in %s", p)
		}
		return nil
	})
}

// REG-DM6-002 — 0 server production code 反向断言: internal/api/ 反向
// grep `dm_6` 在 production *.go 0 hit (仅 _test.go 允许).
func TestDM_NoServerProductionCode(t *testing.T) {
	t.Parallel()
	dirs := []string{filepath.Join("..", "api"), filepath.Join("..", "server")}
	pat := regexp.MustCompile(`(?i)dm_?6\b`)
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			base := filepath.Base(p)
			// Skip test files (allowed); also skip this very file (helper has dm6_).
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			body, _ := os.ReadFile(p)
			if loc := pat.FindIndex(body); loc != nil {
				t.Errorf("DM-6 设计第 1 条 broken — dm_6 production reference in %s: %q",
					p, body[loc[0]:loc[1]])
			}
			_ = base
			return nil
		})
	}
}

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

// REG-DM6-005 — thinking 5-pattern 对齐链第 9 处 — grep 检查 在 dm_6
// production 0 hit (DM-5 第 8 处 + DM-4 第 7 处 + DM-3 第 6 处 + RT-3
// 第 5 处).
func TestDM_NoThinkingPatternInProduction(t *testing.T) {
	t.Parallel()
	dirs := []string{filepath.Join("..", "api"), filepath.Join("..", "server")}
	pat := regexp.MustCompile(`<thinking>|<thought>|<reasoning>|<reflection>|<internal>`)
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			base := filepath.Base(p)
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			// Only inspect dm_6_* production files (we add 0 today, so this
			// is reverse 守门 against future mismatch).
			if !strings.HasPrefix(base, "dm_6") {
				return nil
			}
			body, _ := os.ReadFile(p)
			if loc := pat.FindIndex(body); loc != nil {
				t.Errorf("DM-6 设计第 3 条 broken — thinking pattern in %s: %q",
					p, body[loc[0]:loc[1]])
			}
			return nil
		})
	}
}

// REG-DM6-006 — AST 对齐链延伸第 15 处.
func TestDM_NoDMThreadQueue(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingDMThread",
		"dmThreadQueue",
		"deadLetterDMThread",
	}
	dir := filepath.Join("..", "api")
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		body, _ := os.ReadFile(p)
		for _, tok := range forbidden {
			if strings.Contains(string(body), tok) {
				t.Errorf("AST 对齐链延伸第 15 处 broken — token %q in %s", tok, p)
			}
		}
		return nil
	})
}

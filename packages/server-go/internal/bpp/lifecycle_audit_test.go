// Package bpp — lifecycle_audit_test.go: BPP-8.2 unit tests.
package bpp

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"borgee-server/internal/agent/reasons"
)

// stubLifecycleStore captures InsertAdminAction calls for assertions.
type stubLifecycleStore struct {
	mu    sync.Mutex
	calls []stubAdminAction
	err   error
}

type stubAdminAction struct {
	actorID, targetUserID, action, metadata string
}

func (s *stubLifecycleStore) InsertAdminAction(actorID, targetUserID, action, metadata string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return "", s.err
	}
	s.calls = append(s.calls, stubAdminAction{actorID, targetUserID, action, metadata})
	return "fake-id", nil
}

// TestBPP_RecordConnect — acceptance §2.1.
func TestBPP_RecordConnect(t *testing.T) {
	t.Parallel()
	st := &stubLifecycleStore{}
	a := NewAdminActionsLifecycleAuditor(st, nil)
	a.RecordConnect("plugin-1", "agent-1")
	if len(st.calls) != 1 {
		t.Fatalf("expected 1 InsertAdminAction call, got %d", len(st.calls))
	}
	c := st.calls[0]
	if c.actorID != "system" {
		t.Errorf("actor: got %q, want system", c.actorID)
	}
	if c.targetUserID != "agent-1" {
		t.Errorf("target: got %q, want agent-1", c.targetUserID)
	}
	if c.action != "plugin_connect" {
		t.Errorf("action: got %q, want plugin_connect", c.action)
	}
	if !strings.Contains(c.metadata, `"plugin_id":"plugin-1"`) {
		t.Errorf("metadata missing plugin_id: %s", c.metadata)
	}
}

// TestBPP_RecordDisconnect — acceptance §2.1.
func TestBPP_RecordDisconnect(t *testing.T) {
	t.Parallel()
	st := &stubLifecycleStore{}
	a := NewAdminActionsLifecycleAuditor(st, nil)
	a.RecordDisconnect("plugin-1", "agent-1", "client_close")
	if len(st.calls) != 1 || st.calls[0].action != "plugin_disconnect" {
		t.Errorf("disconnect: %+v", st.calls)
	}
	if !strings.Contains(st.calls[0].metadata, `"reason":"client_close"`) {
		t.Errorf("metadata missing reason: %s", st.calls[0].metadata)
	}
}

// TestBPP_RecordReconnect — acceptance §2.1.
func TestBPP_RecordReconnect(t *testing.T) {
	t.Parallel()
	st := &stubLifecycleStore{}
	a := NewAdminActionsLifecycleAuditor(st, nil)
	a.RecordReconnect("plugin-1", "agent-1", 12345)
	if len(st.calls) != 1 || st.calls[0].action != "plugin_reconnect" {
		t.Errorf("reconnect: %+v", st.calls)
	}
	if !strings.Contains(st.calls[0].metadata, `"last_known_cursor":12345`) {
		t.Errorf("metadata missing cursor: %s", st.calls[0].metadata)
	}
}

// TestBPP_RecordColdStart_ReasonRuntimeCrashed — acceptance §2.1
// 设计 ② AL-1a 锁链第 13 处.
//
// reason 字面必须 byte-identical=reasons.RuntimeCrashed (跟 BPP-6 +
// BPP-7 SDK ColdStart 同源). 反向断言 hardcode "runtime_crashed" 字符串
// 0 hit (强制走 reasons.* 引用).
func TestBPP_RecordColdStart_ReasonRuntimeCrashed(t *testing.T) {
	t.Parallel()
	st := &stubLifecycleStore{}
	a := NewAdminActionsLifecycleAuditor(st, nil)
	// caller passes reasons.RuntimeCrashed (byte-identical 跟 BPP-6/BPP-7).
	a.RecordColdStart("plugin-1", "agent-1", reasons.RuntimeCrashed)
	if len(st.calls) != 1 || st.calls[0].action != "plugin_cold_start" {
		t.Errorf("cold_start: %+v", st.calls)
	}
	if !strings.Contains(st.calls[0].metadata, `"restart_reason":"runtime_crashed"`) {
		t.Errorf("metadata missing reason: %s", st.calls[0].metadata)
	}
	// AL-1a 锁链第 13 处 — direct const literal lock.
	if reasons.RuntimeCrashed != "runtime_crashed" {
		t.Errorf("reasons.RuntimeCrashed drift: got %q, want runtime_crashed (锁链第 13 处)", reasons.RuntimeCrashed)
	}
}

// TestBPP_RecordHeartbeatTimeout_ReasonNetworkUnreachable — acceptance §2.1.
// reason 字面 byte-identical=reasons.NetworkUnreachable (AL-1a 锁链第 13 处).
func TestBPP_RecordHeartbeatTimeout_ReasonNetworkUnreachable(t *testing.T) {
	t.Parallel()
	st := &stubLifecycleStore{}
	a := NewAdminActionsLifecycleAuditor(st, nil)
	a.RecordHeartbeatTimeout("plugin-1", "agent-1")
	if len(st.calls) != 1 || st.calls[0].action != "plugin_heartbeat_timeout" {
		t.Errorf("heartbeat_timeout: %+v", st.calls)
	}
	if !strings.Contains(st.calls[0].metadata, `"reason":"network_unreachable"`) {
		t.Errorf("metadata missing reason: %s", st.calls[0].metadata)
	}
	if reasons.NetworkUnreachable != "network_unreachable" {
		t.Errorf("reasons.NetworkUnreachable drift: got %q (锁链第 13 处)", reasons.NetworkUnreachable)
	}
}

// TestBPP_NilSafeCtor — boot bug (acceptance §2.2).
func TestBPP_NilSafeCtor(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("expected panic on nil store")
		}
	}()
	NewAdminActionsLifecycleAuditor(nil, nil)
}

// TestBPP_BestEffort_FireAndForget — acceptance §3.1 设计 ⑥.
//
// On InsertAdminAction error, RecordX must log warn but NOT panic /
// return error (handler must continue).
func TestBPP_BestEffort_FireAndForget(t *testing.T) {
	t.Parallel()
	st := &stubLifecycleStore{err: errors.New("db down")}
	a := NewAdminActionsLifecycleAuditor(st, nil)
	// All 5 methods must not panic.
	a.RecordConnect("p", "a")
	a.RecordDisconnect("p", "a", "x")
	a.RecordReconnect("p", "a", 0)
	a.RecordColdStart("p", "a", reasons.RuntimeCrashed)
	a.RecordHeartbeatTimeout("p", "a")
	// store.calls should be 0 because err triggered early return.
	if len(st.calls) != 0 {
		t.Errorf("expected 0 successful inserts (err mode), got %d", len(st.calls))
	}
}

// TestBPP_LifecycleAuditor_SingleGate — acceptance §2.2 设计 ④.
//
// grep 检查 `"plugin_*"` 字面 在 production *.go 路径 — single-gate
// 排除 lifecycle_audit.go (write path) + bpp_8_lifecycle_list.go (read-only
// filter switch, 5 字面同源跟 migration v=31 CHECK + auditor const).
func TestBPP_LifecycleAuditor_SingleGate(t *testing.T) {
	t.Parallel()
	dirs := []string{".", "../api"}
	whitelist := map[string]bool{
		"lifecycle_audit.go":        true, // write-side single-gate
		"plugin_list.go":   true, // read-only filter (isPluginLifecycleAction switch)
	}
	hits := []string{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			if strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			if whitelist[e.Name()] {
				continue
			}
			path := filepath.Join(dir, e.Name())
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			content := string(b)
			for _, action := range []string{
				`"plugin_connect"`, `"plugin_disconnect"`, `"plugin_reconnect"`,
				`"plugin_cold_start"`, `"plugin_heartbeat_timeout"`,
			} {
				if strings.Contains(content, action) {
					hits = append(hits, path+":"+action)
				}
			}
		}
	}
	if len(hits) > 0 {
		t.Errorf("BPP-8 设计 ④ broken: plugin_* action literals outside whitelisted files (single-gate violation): %v", hits)
	}
}

// TestBPP_NoLifecycleQueueOrAuditTable — acceptance §3.1 设计 ⑥
// best-effort 锁链延伸第 5 处.
func TestBPP_NoLifecycleQueueOrAuditTable(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingLifecycleAudit",
		"lifecycleQueue",
		"deadLetterLifecycle",
	}
	dir := "."
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	fset := token.NewFileSet()
	hits := []string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			for _, bad := range forbidden {
				if strings.Contains(ident.Name, bad) {
					hits = append(hits, path+":"+ident.Name)
				}
			}
			return true
		})
	}
	if len(hits) > 0 {
		t.Errorf("BPP-8 设计 ⑥ broken: forbidden lifecycle-queue identifiers in internal/bpp/ "+
			"(best-effort 锁链延伸第 5 处, 跟 BPP-4/5/6/7 同模式): %v", hits)
	}
}

// TestBPP_LifecycleSystemActor_ByteIdentical — acceptance §3.2 设计 ⑦.
func TestBPP_LifecycleSystemActor_ByteIdentical(t *testing.T) {
	t.Parallel()
	if LifecycleSystemActor != "system" {
		t.Errorf("LifecycleSystemActor drift: got %q, want system (跟 BPP-4 watchdog + AP-2 sweeper actor='system' 跨五 milestone byte-identical)",
			LifecycleSystemActor)
	}
}

// TestBPP_AdminGodModeNotMounted — acceptance §3.2 设计 ⑦ ADM-0 §1.3.
func TestBPP_AdminGodModeNotMounted(t *testing.T) {
	t.Parallel()
	dir := "../api"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	literals := []string{
		"admin/plugin/lifecycle",
		"admin/plugins/lifecycle",
		"AdminPluginLifecycle",
		"AdminBPP8",
		"adminPluginLifecycle",
	}
	hits := []string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "admin") {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(b)
		for _, bad := range literals {
			if strings.Contains(content, bad) {
				hits = append(hits, path+":"+bad)
			}
		}
	}
	if len(hits) > 0 {
		t.Errorf("BPP-8 设计 ⑦ broken: admin god-mode references plugin lifecycle (ADM-0 §1.3 红线): %v", hits)
	}
}

// Test surfaces the formatColdStartReason internal helper for direct
// assertion (file-internal export, exercised once for coverage).
func TestBPP_FormatColdStartReason(t *testing.T) {
	t.Parallel()
	if got, want := formatColdStartReason(), "runtime_crashed"; got != want {
		t.Errorf("formatColdStartReason drift: got %q, want %q", got, want)
	}
}

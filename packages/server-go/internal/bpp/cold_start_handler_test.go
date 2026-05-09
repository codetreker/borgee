// Package bpp — cold_start_handler_test.go: BPP-6.2 server handler unit
// tests (acceptance §1+§2+§3 验收 + stance §1+§2+§3+§4 守门 + AST scan
// cold-start-* 反约束 跟 BPP-4/BPP-5 锁链延伸第 3 处).
package bpp

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"borgee-server/internal/store"
)

// ---- fixtures ----

type bpp6StubStateAppender struct {
	mu          sync.Mutex
	appendCalls []bpp6AppendCall
	listLog     []store.AgentStateLogRow
	listErr     error
	appendErr   error
}

type bpp6AppendCall struct {
	agentID string
	from    store.AgentState
	to      store.AgentState
	reason  string
	taskID  string
}

func (s *bpp6StubStateAppender) AppendAgentStateTransition(agentID string,
	from, to store.AgentState, reason, taskID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.appendErr != nil {
		return 0, s.appendErr
	}
	s.appendCalls = append(s.appendCalls, bpp6AppendCall{
		agentID: agentID, from: from, to: to, reason: reason, taskID: taskID,
	})
	s.listLog = append([]store.AgentStateLogRow{{
		AgentID:   agentID,
		FromState: string(from),
		ToState:   string(to),
		Reason:    reason,
	}}, s.listLog...)
	return int64(len(s.appendCalls)), nil
}

func (s *bpp6StubStateAppender) ListAgentStateLog(agentID string, limit int) ([]store.AgentStateLogRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := []store.AgentStateLogRow{}
	for _, r := range s.listLog {
		if r.AgentID == agentID {
			out = append(out, r)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

type bpp6StubOwner struct {
	owners map[string]string
	err    error
}

func (s *bpp6StubOwner) OwnerOf(agentID string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if v, ok := s.owners[agentID]; ok {
		return v, nil
	}
	return "", errors.New("agent not found")
}

type bpp6StubClearer struct {
	mu      sync.Mutex
	cleared []string
}

func (s *bpp6StubClearer) Clear(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleared = append(s.cleared, agentID)
}

func bpp6NewHandler(t *testing.T, st *bpp6StubStateAppender,
	owner *bpp6StubOwner, clearer *bpp6StubClearer) *ColdStartHandler {
	t.Helper()
	return NewColdStartHandler(st, owner, clearer, nil)
}

func bpp6Frame(agentID string) json.RawMessage {
	b, _ := json.Marshal(ColdStartHandshakeFrame{
		Type:          FrameTypeBPPColdStartHandshake,
		PluginID:      "plugin-1",
		AgentID:       agentID,
		RestartAt:     1700000000000,
		RestartReason: "sigkill",
	})
	return b
}

// ---- §1 frame schema (3 项) ----

// TestBPP_FieldOrder — acceptance §1.1 byte-identical 5 字段.
func TestBPP_FieldOrder(t *testing.T) {
	t.Parallel()
	want := []struct{ name, tag string }{
		{"Type", "type"},
		{"PluginID", "plugin_id"},
		{"AgentID", "agent_id"},
		{"RestartAt", "restart_at"},
		{"RestartReason", "restart_reason"},
	}
	typ := reflect.TypeOf(ColdStartHandshakeFrame{})
	if got := typ.NumField(); got != len(want) {
		t.Fatalf("field count: got %d, want %d", got, len(want))
	}
	for i, w := range want {
		f := typ.Field(i)
		if f.Name != w.name {
			t.Errorf("field %d name: got %q, want %q", i, f.Name, w.name)
		}
		if got := f.Tag.Get("json"); got != w.tag {
			t.Errorf("field %d json tag: got %q, want %q", i, got, w.tag)
		}
	}
}

// TestBPP_DirectionLock — acceptance §1.2 plugin→server.
func TestBPP_DirectionLock(t *testing.T) {
	t.Parallel()
	f := ColdStartHandshakeFrame{}
	if got, want := f.FrameDirection(), DirectionPluginToServer; got != want {
		t.Errorf("direction: got %q, want %q", got, want)
	}
	if got, want := f.FrameType(), FrameTypeBPPColdStartHandshake; got != want {
		t.Errorf("frame type: got %q, want %q", got, want)
	}
}

// TestBPP_FrameSet_NoReconnectFields — acceptance §1.3 字段集与
// ReconnectHandshakeFrame 互斥反断 (spec §0.1 设计守门).
func TestBPP_FrameSet_NoReconnectFields(t *testing.T) {
	t.Parallel()
	forbidden := []string{"LastKnownCursor", "DisconnectAt", "ReconnectAt"}
	typ := reflect.TypeOf(ColdStartHandshakeFrame{})
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		for _, bad := range forbidden {
			if name == bad {
				t.Errorf("ColdStartHandshakeFrame must not have field %q (字段集与 ReconnectHandshakeFrame 互斥, spec §0.1)", name)
			}
		}
	}
}

// ---- §2 server handler (3 项) ----

func TestBPP_Handler_TransitionsToOnline_FromInitial(t *testing.T) {
	t.Parallel()
	st := &bpp6StubStateAppender{}
	owner := &bpp6StubOwner{owners: map[string]string{"agent-1": "user-1"}}
	clearer := &bpp6StubClearer{}
	h := bpp6NewHandler(t, st, owner, clearer)

	if err := h.Dispatch(bpp6Frame("agent-1"), PluginSessionContext{OwnerUserID: "user-1"}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(st.appendCalls) != 1 {
		t.Fatalf("expected 1 append, got %d", len(st.appendCalls))
	}
	c := st.appendCalls[0]
	if c.from != store.AgentStateInitial {
		t.Errorf("from: got %q, want initial", c.from)
	}
	if c.to != store.AgentStateOnline {
		t.Errorf("to: got %q, want online", c.to)
	}
	if c.reason != "runtime_crashed" {
		t.Errorf("reason: got %q, want runtime_crashed (AL-1a SSOT byte-identical)", c.reason)
	}
	if len(clearer.cleared) != 1 || clearer.cleared[0] != "agent-1" {
		t.Errorf("Tracker.Clear not invoked once for agent-1: %v", clearer.cleared)
	}
}

func TestBPP_Handler_TransitionsToOnline_FromError(t *testing.T) {
	t.Parallel()
	st := &bpp6StubStateAppender{
		listLog: []store.AgentStateLogRow{{
			AgentID: "agent-1", FromState: "online", ToState: "error", Reason: "network_unreachable",
		}},
	}
	owner := &bpp6StubOwner{owners: map[string]string{"agent-1": "user-1"}}
	clearer := &bpp6StubClearer{}
	h := bpp6NewHandler(t, st, owner, clearer)

	if err := h.Dispatch(bpp6Frame("agent-1"), PluginSessionContext{OwnerUserID: "user-1"}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if got := st.appendCalls[0].from; got != store.AgentStateError {
		t.Errorf("from: got %q, want error", got)
	}
}

func TestBPP_Handler_TransitionsToOnline_FromOffline(t *testing.T) {
	t.Parallel()
	st := &bpp6StubStateAppender{
		listLog: []store.AgentStateLogRow{{
			AgentID: "agent-1", FromState: "online", ToState: "offline", Reason: "",
		}},
	}
	owner := &bpp6StubOwner{owners: map[string]string{"agent-1": "user-1"}}
	clearer := &bpp6StubClearer{}
	h := bpp6NewHandler(t, st, owner, clearer)

	if err := h.Dispatch(bpp6Frame("agent-1"), PluginSessionContext{OwnerUserID: "user-1"}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if got := st.appendCalls[0].from; got != store.AgentStateOffline {
		t.Errorf("from: got %q, want offline", got)
	}
}

func TestColdStartHandler_Handler_CrossOwnerReject(t *testing.T) {
	t.Parallel()
	st := &bpp6StubStateAppender{}
	owner := &bpp6StubOwner{owners: map[string]string{"agent-1": "OTHER"}}
	clearer := &bpp6StubClearer{}
	h := bpp6NewHandler(t, st, owner, clearer)

	err := h.Dispatch(bpp6Frame("agent-1"), PluginSessionContext{OwnerUserID: "user-1"})
	if err == nil || !IsColdStartCrossOwnerReject(err) {
		t.Fatalf("expected cross-owner reject, got %v", err)
	}
	if len(st.appendCalls) != 0 {
		t.Errorf("must not append state on cross-owner reject")
	}
	if len(clearer.cleared) != 0 {
		t.Errorf("must not clear tracker on cross-owner reject")
	}
}

func TestBPP_Handler_NilSafeCtor(t *testing.T) {
	t.Parallel()
	st := &bpp6StubStateAppender{}
	owner := &bpp6StubOwner{}
	clearer := &bpp6StubClearer{}
	cases := []struct {
		name string
		fn   func()
	}{
		{"nil state", func() { NewColdStartHandler(nil, owner, clearer, nil) }},
		{"nil owner", func() { NewColdStartHandler(st, nil, clearer, nil) }},
		{"nil clearer", func() { NewColdStartHandler(st, owner, nil, nil) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("expected panic on %s", tc.name)
				}
			}()
			tc.fn()
		})
	}
}

// ---- §3 restart count derive + AST 兜底 (3 项) ----

// TestBPP_RestartCount_DerivedFromStateLog — acceptance §3.1 设计 ③
// restart 计数走 state-log COUNT(WHERE to_state='online' AND
// reason='runtime_crashed') 反向 derive (不另开 plugin_restart_count 列).
func TestBPP_RestartCount_DerivedFromStateLog(t *testing.T) {
	t.Parallel()
	st := &bpp6StubStateAppender{}
	owner := &bpp6StubOwner{owners: map[string]string{"agent-1": "user-1"}}
	clearer := &bpp6StubClearer{}
	h := bpp6NewHandler(t, st, owner, clearer)

	for i := 0; i < 3; i++ {
		// Force prior state to error so handler fires fresh
		// runtime_crashed transition (online→online would no-op).
		st.mu.Lock()
		st.listLog = append([]store.AgentStateLogRow{{
			AgentID: "agent-1", FromState: "online", ToState: "error", Reason: "runtime_crashed",
		}}, st.listLog...)
		st.mu.Unlock()
		if err := h.Dispatch(bpp6Frame("agent-1"), PluginSessionContext{OwnerUserID: "user-1"}); err != nil {
			t.Fatalf("dispatch %d: %v", i, err)
		}
	}

	count := 0
	for _, c := range st.appendCalls {
		if c.agentID == "agent-1" && c.to == store.AgentStateOnline && c.reason == "runtime_crashed" {
			count++
		}
	}
	if count != 3 {
		t.Errorf("derived restart count: got %d, want 3 (3 cold-start dispatches → 3 online+runtime_crashed rows)", count)
	}
}

// TestBPP_Handler_DoesNotInvokeResolveResume — acceptance §2.3 设计 ②
// 不重放历史 — handler 源 AST identifier scan 不 reference ResolveResume /
// SessionResumeRequest (注释里说明设计沿用 OK, 实际 ident 调用必 0 hit).
func TestBPP_Handler_DoesNotInvokeResolveResume(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"ResolveResume",
		"SessionResumeRequest",
		"ResumeModeIncremental",
		"DefaultResumeLimit",
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "cold_start_handler.go", nil, 0)
	if err != nil {
		t.Fatalf("parse cold_start_handler.go: %v", err)
	}
	hits := []string{}
	ast.Inspect(f, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		for _, bad := range forbidden {
			if ident.Name == bad {
				hits = append(hits, ident.Name)
			}
		}
		return true
	})
	if len(hits) > 0 {
		t.Errorf("cold_start_handler.go references %v — BPP-6 spec §0.2 设计: "+
			"cold-start 是 fresh start, 不重放历史 (反向 BPP-5)", hits)
	}
}

// TestBPP_NoColdStartQueueInBPPPackage — acceptance §3.3 设计 ⑥
// best-effort 锁链延伸第 3 处 (BPP-4 dead_letter_test +
// BPP-5 reconnect_handler_test 同模式).
func TestBPP_NoColdStartQueueInBPPPackage(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingColdStart",
		"coldStartQueue",
		"deadLetterColdStart",
		"plugin_restart_count",
		"coldStartCount",
		"restartCounter",
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
	sort.Strings(hits)
	if len(hits) > 0 {
		t.Errorf("BPP-6 stance §4 反约束: forbidden cold-start-queue / restart-count "+
			"identifiers in internal/bpp/ source (best-effort 设计 + count 反向 derive 设计, "+
			"跟 BPP-4/BPP-5 锁链延伸第 3 处): %v", hits)
	}
}

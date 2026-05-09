// Package bpp — reconnect_handler_test.go: BPP-5.2 server handler unit
// tests (acceptance §1+§2 验收 + stance §1+§2+§3+§4 守门 + AST scan
// reconnect-* 反约束 跟 BPP-4 dead_letter_test 锁链延伸).
package bpp

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
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

type bpp5StubEventLister struct {
	highWater int64
	calls     []bpp5ResumeCall
	mu        sync.Mutex
}

type bpp5ResumeCall struct {
	since      int64
	limit      int
	channelIDs []string
}

func (s *bpp5StubEventLister) GetEventsSince(cursor int64, limit int, channelIDs []string) ([]store.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, bpp5ResumeCall{since: cursor, limit: limit, channelIDs: channelIDs})
	return nil, nil
}

func (s *bpp5StubEventLister) GetLatestCursor() int64 { return s.highWater }

type bpp5StubScope struct {
	ids map[string][]string
	err error
}

func (s *bpp5StubScope) ChannelIDsForOwner(owner string) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	if v, ok := s.ids[owner]; ok {
		return v, nil
	}
	return []string{}, nil
}

type bpp5StubOwner struct {
	owners map[string]string
	err    error
}

func (s *bpp5StubOwner) OwnerOf(agentID string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if v, ok := s.owners[agentID]; ok {
		return v, nil
	}
	return "", errors.New("agent not found")
}

type bpp5StubClearer struct {
	mu     sync.Mutex
	cleared []string
}

func (s *bpp5StubClearer) Clear(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleared = append(s.cleared, agentID)
}

// ---- §1 frame schema (4 项) ----

func TestBPP_ReconnectHandshakeFrame_FieldOrder(t *testing.T) {
	t.Parallel()
	want := []struct{ name, tag string }{
		{"Type", "type"},
		{"PluginID", "plugin_id"},
		{"AgentID", "agent_id"},
		{"LastKnownCursor", "last_known_cursor"},
		{"DisconnectAt", "disconnect_at"},
		{"ReconnectAt", "reconnect_at"},
	}
	typ := reflect.TypeOf(ReconnectHandshakeFrame{})
	if typ.NumField() != len(want) {
		t.Fatalf("field count drift: got %d, want %d", typ.NumField(), len(want))
	}
	for i, w := range want {
		f := typ.Field(i)
		if f.Name != w.name {
			t.Errorf("field[%d] name=%q, want %q", i, f.Name, w.name)
		}
		if got := f.Tag.Get("json"); got != w.tag {
			t.Errorf("field[%d] tag=%q, want %q", i, got, w.tag)
		}
	}
}

func TestBPP_ReconnectHandshake_DirectionLock(t *testing.T) {
	t.Parallel()
	if got := (ReconnectHandshakeFrame{}).FrameDirection(); got != DirectionPluginToServer {
		t.Errorf("direction drift: got %q, want plugin_to_server", got)
	}
	if got := (ReconnectHandshakeFrame{}).FrameType(); got != FrameTypeBPPReconnectHandshake {
		t.Errorf("type drift: got %q, want %q", got, FrameTypeBPPReconnectHandshake)
	}
}

func TestBPP_ConnectFrame_NoReconnectFields(t *testing.T) {
	t.Parallel()
	// 反约束: connect ≠ reconnect — 字段集不交.
	typ := reflect.TypeOf(ConnectFrame{})
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		if name == "LastKnownCursor" || name == "DisconnectAt" || name == "ReconnectAt" {
			t.Errorf("ConnectFrame must NOT contain reconnect-only field %q", name)
		}
	}
}

// ---- §2 server handler (5 项) ----

func TestBPP_Handler_CallsResolveResumeIncremental(t *testing.T) {
	t.Parallel()
	events := &bpp5StubEventLister{highWater: 100}
	scope := &bpp5StubScope{ids: map[string][]string{"owner-1": {"ch1", "ch2"}}}
	owner := &bpp5StubOwner{owners: map[string]string{"agent-1": "owner-1"}}
	clearer := &bpp5StubClearer{}
	h := NewReconnectHandler(events, scope, owner, clearer, nil)

	frame := ReconnectHandshakeFrame{
		Type: FrameTypeBPPReconnectHandshake, PluginID: "p1", AgentID: "agent-1",
		LastKnownCursor: 42, DisconnectAt: 1, ReconnectAt: 2,
	}
	raw, _ := json.Marshal(frame)
	if err := h.Dispatch(raw, PluginSessionContext{OwnerUserID: "owner-1"}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(events.calls) != 1 {
		t.Fatalf("expected 1 ResolveResume call, got %d", len(events.calls))
	}
	if events.calls[0].since != 42 {
		t.Errorf("AfterCursor mismatch: got %d, want 42", events.calls[0].since)
	}
	if !reflect.DeepEqual(events.calls[0].channelIDs, []string{"ch1", "ch2"}) {
		t.Errorf("channel scope mismatch: got %v", events.calls[0].channelIDs)
	}
}

func TestBPP_Handler_ClearsAgentError(t *testing.T) {
	t.Parallel()
	events := &bpp5StubEventLister{highWater: 100}
	scope := &bpp5StubScope{ids: map[string][]string{"o": {"c1"}}}
	owner := &bpp5StubOwner{owners: map[string]string{"a": "o"}}
	clearer := &bpp5StubClearer{}
	h := NewReconnectHandler(events, scope, owner, clearer, nil)

	raw, _ := json.Marshal(ReconnectHandshakeFrame{
		Type: FrameTypeBPPReconnectHandshake, AgentID: "a",
	})
	_ = h.Dispatch(raw, PluginSessionContext{OwnerUserID: "o"})
	if len(clearer.cleared) != 1 || clearer.cleared[0] != "a" {
		t.Errorf("expected Clear(a), got %v", clearer.cleared)
	}
}

func TestReconnectHandler_Handler_CrossOwnerReject(t *testing.T) {
	t.Parallel()
	events := &bpp5StubEventLister{highWater: 100}
	scope := &bpp5StubScope{}
	owner := &bpp5StubOwner{owners: map[string]string{"a": "real-owner"}}
	clearer := &bpp5StubClearer{}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	h := NewReconnectHandler(events, scope, owner, clearer, logger)

	raw, _ := json.Marshal(ReconnectHandshakeFrame{
		Type: FrameTypeBPPReconnectHandshake, AgentID: "a",
	})
	err := h.Dispatch(raw, PluginSessionContext{OwnerUserID: "different-owner"})
	if err == nil || !IsReconnectCrossOwnerReject(err) {
		t.Errorf("expected cross-owner reject, got %v", err)
	}
	if !strings.Contains(buf.String(), ReconnectErrCodeCrossOwnerReject) {
		t.Errorf("missing log key: %q", buf.String())
	}
	if len(clearer.cleared) != 0 {
		t.Errorf("must NOT clear on reject")
	}
}

func TestBPP_Handler_CursorRegression_TrustButLog(t *testing.T) {
	t.Parallel()
	// frame.LastKnownCursor > server high-water → log warn but do NOT reject.
	events := &bpp5StubEventLister{highWater: 50}
	scope := &bpp5StubScope{ids: map[string][]string{"o": {"c1"}}}
	owner := &bpp5StubOwner{owners: map[string]string{"a": "o"}}
	clearer := &bpp5StubClearer{}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	h := NewReconnectHandler(events, scope, owner, clearer, logger)

	raw, _ := json.Marshal(ReconnectHandshakeFrame{
		Type: FrameTypeBPPReconnectHandshake, AgentID: "a",
		LastKnownCursor: 999, // > highWater
	})
	if err := h.Dispatch(raw, PluginSessionContext{OwnerUserID: "o"}); err != nil {
		t.Fatalf("expected success (trust-but-log), got %v", err)
	}
	if !strings.Contains(buf.String(), "bpp.reconnect_cursor_regression") {
		t.Errorf("missing regression warn log: %q", buf.String())
	}
	if len(clearer.cleared) != 1 {
		t.Errorf("expected Clear after trust-but-log path")
	}
}

func TestBPP_Handler_PanicsOnNilDeps(t *testing.T) {
	t.Parallel()
	events := &bpp5StubEventLister{}
	scope := &bpp5StubScope{}
	owner := &bpp5StubOwner{}
	clearer := &bpp5StubClearer{}
	cases := []struct {
		name string
		fn   func()
	}{
		{"nil events", func() { NewReconnectHandler(nil, scope, owner, clearer, nil) }},
		{"nil scope", func() { NewReconnectHandler(events, nil, owner, clearer, nil) }},
		{"nil owner", func() { NewReconnectHandler(events, scope, nil, clearer, nil) }},
		{"nil clearer", func() { NewReconnectHandler(events, scope, owner, nil, nil) }},
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

// ---- §4 反约束 AST scan (BPP-4 dead_letter_test 锁链延伸) ----

func TestBPP_NoReconnectQueueInBPPPackage(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingReconnects",
		"reconnectQueue",
		"deadLetterReconnect",
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
		t.Errorf("BPP-5 stance §4 反约束: forbidden reconnect-queue identifiers "+
			"found in internal/bpp/ source (best-effort 设计代码层守门, 跟 "+
			"BPP-4 dead_letter_test::TestBPP4_NoRetryQueueInBPPPackage 锁链延伸): %v", hits)
	}
}

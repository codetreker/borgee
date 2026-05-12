package ipc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"borgee-helper/internal/acl"
	"borgee-helper/internal/audit"
	"borgee-helper/internal/grants"
)

func newTestHandler(t *testing.T) (*Handler, *bytes.Buffer, *grants.MemoryConsumer) {
	t.Helper()
	mc := grants.NewMemoryConsumer()
	mc.SetNowFn(func() int64 { return 100 })
	g := acl.New(mc)
	buf := &bytes.Buffer{}
	a := audit.New(buf)
	return New(g, a), buf, mc
}

// startServe feeds data into Serve and captures responses using a single net.Conn simulation.
func startServe(t *testing.T, h *Handler, in []byte) []byte {
	t.Helper()
	c1, c2 := net.Pipe()
	done := make(chan struct{})
	go func() {
		_ = h.Serve(context.Background(), c2)
		close(done)
	}()
	go func() {
		_, _ = c1.Write(in)
	}()
	out := &bytes.Buffer{}
	r := bufio.NewReader(c1)
	deadline := time.Now().Add(2 * time.Second)
	_ = c1.SetReadDeadline(deadline)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			out.Write(line)
		}
		if err != nil {
			break
		}
		if time.Now().After(deadline) {
			break
		}
	}
	_ = c1.Close()
	<-done
	return out.Bytes()
}

func TestHB25_HandshakeAndReadFileHappyPath(t *testing.T) {
	t.Parallel()
	h, _, mc := newTestHandler(t)
	// v0(D) real IO — seed a real file under t.TempDir, scope=fs:<dir>.
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "hello.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	mc.Put(grants.Grant{AgentID: "a1", Scope: "fs:" + filePath, TTLUntil: 9999})
	in := []byte(`{"agent_id":"a1"}` + "\n" +
		`{"request_id":"r1","action":"read_file","agent_id":"a1","params":{"path":"` + filePath + `"}}` + "\n")
	out := startServe(t, h, in)
	if !strings.Contains(string(out), `"status":"ok"`) {
		t.Errorf("expected ok response, got: %s", out)
	}
	if !strings.Contains(string(out), `"request_id":"r1"`) {
		t.Errorf("missing request_id correlation: %s", out)
	}
}

func TestHB25_CrossAgentRejected(t *testing.T) {
	t.Parallel()
	h, _, mc := newTestHandler(t)
	mc.Put(grants.Grant{AgentID: "a1", Scope: "fs:/x", TTLUntil: 9999})
	in := []byte(`{"agent_id":"a1"}` + "\n" +
		`{"request_id":"r1","action":"read_file","agent_id":"a2","params":{"path":"/x"}}` + "\n")
	out := startServe(t, h, in)
	var resp Response
	for _, line := range bytes.Split(bytes.TrimSpace(out), []byte("\n")) {
		_ = json.Unmarshal(line, &resp)
		if resp.RequestID == "r1" {
			break
		}
	}
	if resp.Status != "rejected" || resp.Reason != "cross_agent_reject" {
		t.Errorf("cross-agent: status=%q reason=%q", resp.Status, resp.Reason)
	}
}

func TestHB25_AuditWrittenForReject(t *testing.T) {
	t.Parallel()
	h, auditBuf, _ := newTestHandler(t)
	in := []byte(`{"agent_id":"a1"}` + "\n" +
		`{"request_id":"r1","action":"read_file","agent_id":"a1","params":{"path":"/missing"}}` + "\n")
	_ = startServe(t, h, in)
	if auditBuf.Len() == 0 {
		t.Fatal("audit log empty after reject (反向约束 #5 要求每次 IPC call 含 reject 写 audit)")
	}
	var ev audit.Event
	_ = json.Unmarshal(bytes.TrimSpace(auditBuf.Bytes()), &ev)
	if ev.Actor != "a1" || ev.Action != "read_file" {
		t.Errorf("audit event 脱节: %+v", ev)
	}
}

func TestHB25_HandshakeMissingAgentRejected(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler(t)
	in := []byte(`{}` + "\n")
	out := startServe(t, h, in)
	if len(out) != 0 {
		t.Errorf("handshake without agent_id should close conn, got: %s", out)
	}
}

func TestHB25_MultipleRequestsMultiplexed(t *testing.T) {
	t.Parallel()
	h, _, mc := newTestHandler(t)
	mc.Put(grants.Grant{AgentID: "a1", Scope: "fs:/x", TTLUntil: 9999})
	in := []byte(`{"agent_id":"a1"}` + "\n" +
		`{"request_id":"r1","action":"read_file","agent_id":"a1","params":{"path":"/x"}}` + "\n" +
		`{"request_id":"r2","action":"list_files","agent_id":"a1","params":{"path":"/x"}}` + "\n" +
		`{"request_id":"r3","action":"write_file","agent_id":"a1","params":{"path":"/x"}}` + "\n")
	out := startServe(t, h, in)
	for _, id := range []string{"r1", "r2", "r3"} {
		if !strings.Contains(string(out), `"request_id":"`+id+`"`) {
			t.Errorf("missing response for %s: %s", id, out)
		}
	}
	// r3 是写类 → rejected
	if !strings.Contains(string(out), `"status":"rejected"`) {
		t.Errorf("write action should be rejected: %s", out)
	}
}

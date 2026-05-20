package acl

import (
	"context"
	"testing"

	"borgee/internal/grants"
	"borgee/internal/reasons"
)

func newGate(t *testing.T) (*Gate, *grants.MemoryConsumer) {
	t.Helper()
	mc := grants.NewMemoryConsumer()
	mc.SetNowFn(func() int64 { return 100 })
	return New(mc), mc
}

func TestHB24_ReadFileHappyPath(t *testing.T) {
	t.Parallel()
	g, mc := newGate(t)
	mc.Put(grants.Grant{AgentID: "a1", Scope: "fs:/Users/me/projects", TTLUntil: 9999})
	d := g.Decide(context.Background(), "a1", "a1", ActionReadFile, "/Users/me/projects")
	if !d.Allow || d.Reason != reasons.OK {
		t.Errorf("read happy: allow=%v reason=%s", d.Allow, d.Reason)
	}
}

func TestHB24_PathTraversalRejected(t *testing.T) {
	t.Parallel()
	g, mc := newGate(t)
	mc.Put(grants.Grant{AgentID: "a1", Scope: "fs:/etc", TTLUntil: 9999})
	cases := []string{
		"/Users/me/../../../etc/passwd",
		"/etc/../etc/shadow",
		"relative/path",
		"",
		"/path\x00with-nul",
	}
	for _, p := range cases {
		d := g.Decide(context.Background(), "a1", "a1", ActionReadFile, p)
		if d.Allow {
			t.Errorf("traversal %q allowed (反向约束 #2 break)", p)
		}
		if d.Reason != reasons.PathOutsideGrants {
			t.Errorf("traversal %q reason 脱节: %s (want path_outside_grants)", p, d.Reason)
		}
	}
}

func TestHB24_CrossAgentRejected(t *testing.T) {
	t.Parallel()
	g, mc := newGate(t)
	mc.Put(grants.Grant{AgentID: "a1", Scope: "fs:/x", TTLUntil: 9999})
	d := g.Decide(context.Background(), "a1", "a2", ActionReadFile, "/x")
	if d.Allow || d.Reason != reasons.CrossAgentReject {
		t.Errorf("cross-agent: allow=%v reason=%s (want cross_agent_reject)", d.Allow, d.Reason)
	}
}

func TestHB24_GrantNotFound(t *testing.T) {
	t.Parallel()
	g, _ := newGate(t)
	d := g.Decide(context.Background(), "a1", "a1", ActionReadFile, "/missing")
	if d.Allow || d.Reason != reasons.GrantNotFound {
		t.Errorf("grant_not_found: allow=%v reason=%s", d.Allow, d.Reason)
	}
}

func TestHB24_GrantExpired(t *testing.T) {
	t.Parallel()
	g, mc := newGate(t)
	mc.Put(grants.Grant{AgentID: "a1", Scope: "fs:/x", TTLUntil: 50}) // < now=100
	d := g.Decide(context.Background(), "a1", "a1", ActionReadFile, "/x")
	if d.Allow || d.Reason != reasons.GrantExpired {
		t.Errorf("grant_expired: allow=%v reason=%s", d.Allow, d.Reason)
	}
}

// TestHB24_WriteActions100PercentRejected — negative constraint #7 reverse-enumeration source.
func TestHB24_WriteActions100PercentRejected(t *testing.T) {
	t.Parallel()
	g, mc := newGate(t)
	mc.Put(grants.Grant{AgentID: "a1", Scope: "fs:/x", TTLUntil: 9999})
	writes := []Action{
		"write_file", "delete_file", "chmod", "chown",
		"mkdir", "rmdir", "mv", "cp", "exec",
		"shell", "rename", "truncate",
	}
	for _, w := range writes {
		d := g.Decide(context.Background(), "a1", "a1", w, "/x")
		if d.Allow {
			t.Errorf("write action %q allowed (anti-constraint #7 100%% reject break)", w)
		}
		if IsReadOnly(w) {
			t.Errorf("IsReadOnly(%q)=true 污染读类白名单", w)
		}
	}
}

func TestHB24_NetworkEgressHappyPath(t *testing.T) {
	t.Parallel()
	g, mc := newGate(t)
	mc.Put(grants.Grant{AgentID: "a1", Scope: "egress:api.example.com", TTLUntil: 9999})
	d := g.Decide(context.Background(), "a1", "a1", ActionNetworkEgress, "api.example.com")
	if !d.Allow {
		t.Errorf("egress happy: allow=%v reason=%s", d.Allow, d.Reason)
	}
}

func TestHB24_HandshakeAgentEmpty(t *testing.T) {
	t.Parallel()
	g, mc := newGate(t)
	mc.Put(grants.Grant{AgentID: "a1", Scope: "fs:/x", TTLUntil: 9999})
	d := g.Decide(context.Background(), "", "a1", ActionReadFile, "/x")
	if d.Allow || d.Reason != reasons.CrossAgentReject {
		t.Errorf("empty handshake should reject: %v", d)
	}
}

package grants

import (
	"context"
	"testing"
)

func TestHB23_GrantLookupHappyPath(t *testing.T) {
	t.Parallel()
	c := NewMemoryConsumer()
	c.SetNowFn(func() int64 { return 1000 })
	c.Put(Grant{AgentID: "a1", Scope: "fs:/x", TTLUntil: 2000, GrantedAt: 500})
	g, ok, err := c.Lookup(context.Background(), "a1", "fs:/x")
	if err != nil || !ok {
		t.Fatalf("lookup miss: ok=%v err=%v", ok, err)
	}
	if g.Scope != "fs:/x" {
		t.Errorf("scope 脱节: %q", g.Scope)
	}
}

func TestHB23_GrantNotFound(t *testing.T) {
	t.Parallel()
	c := NewMemoryConsumer()
	_, ok, err := c.Lookup(context.Background(), "a1", "fs:/missing")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Error("expected not-found")
	}
	_, exists, expired, _ := c.LookupRaw(context.Background(), "a1", "fs:/missing")
	if exists || expired {
		t.Error("LookupRaw not_found should yield exists=false expired=false")
	}
}

func TestHB23_GrantExpired(t *testing.T) {
	t.Parallel()
	c := NewMemoryConsumer()
	c.SetNowFn(func() int64 { return 5000 })
	c.Put(Grant{AgentID: "a1", Scope: "fs:/x", TTLUntil: 1000, GrantedAt: 0})
	_, ok, err := c.Lookup(context.Background(), "a1", "fs:/x")
	if err != nil || ok {
		t.Errorf("expected expired (ok=false), got ok=%v err=%v", ok, err)
	}
	_, exists, expired, _ := c.LookupRaw(context.Background(), "a1", "fs:/x")
	if !exists || !expired {
		t.Errorf("expected exists=true expired=true, got %v %v", exists, expired)
	}
}

// TestHB23_RevocationLessThan100ms 反向断言: 撤销后下次 Lookup 立即拒绝
// (HB-4 §1.5 release gate 第 5 行 < 100ms; grep 检查 grantsCache 0 hit).
func TestHB23_RevocationImmediate(t *testing.T) {
	t.Parallel()
	c := NewMemoryConsumer()
	c.SetNowFn(func() int64 { return 100 })
	c.Put(Grant{AgentID: "a1", Scope: "fs:/x", TTLUntil: 9999})
	_, ok, _ := c.Lookup(context.Background(), "a1", "fs:/x")
	if !ok {
		t.Fatal("setup: grant missing pre-revoke")
	}
	c.Delete("a1", "fs:/x")
	_, ok, _ = c.Lookup(context.Background(), "a1", "fs:/x")
	if ok {
		t.Error("revocation 不立即生效 (grants cache 反向约束 break)")
	}
}

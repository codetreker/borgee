package store

// TEST-FIX-3-COV: deterministic SetChannelReadonly cov 真补 (was 0%).
//
// 设计: 真补 deterministic cov 让 baseline 重回 ≥85% — 0 race scheduler 依赖.

import (
	"testing"
)

func TestSetChannelReadonly_HappyPath(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := createUser(t, s, "ro_owner", "member")
	ch := createChannel(t, s, "ro-ch", "public", owner.ID)

	// 关 → 开
	if _, err := s.SetChannelReadonly(ch.ID, true); err != nil {
		t.Fatalf("set readonly true: %v", err)
	}
	on, err := s.GetChannelReadonly(ch.ID)
	if err != nil {
		t.Fatalf("is readonly: %v", err)
	}
	if !on {
		t.Fatal("expected readonly=true after SetChannelReadonly(true)")
	}

	// 开 → 关
	if _, err := s.SetChannelReadonly(ch.ID, false); err != nil {
		t.Fatalf("set readonly false: %v", err)
	}
	off, err := s.GetChannelReadonly(ch.ID)
	if err != nil {
		t.Fatalf("is readonly off: %v", err)
	}
	if off {
		t.Fatal("expected readonly=false after SetChannelReadonly(false)")
	}
}

func TestSetChannelReadonly_EmptyChannelID(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	if _, err := s.SetChannelReadonly("", true); err == nil {
		t.Fatal("expected error for empty channelID")
	}
}

func TestSetChannelReadonly_UnknownChannel(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	if _, err := s.SetChannelReadonly("nonexistent-channel-id", true); err == nil {
		t.Fatal("expected error for unknown channelID")
	}
}

package store

import (
	"errors"
	"testing"
)

func TestRequireMentionPolicyResolutionAndOwnerCeiling(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := createUser(t, s, "policy_owner", "member")
	agent := createUser(t, s, "policy_agent", "agent")
	agent.OwnerID = &owner.ID
	if err := s.UpdateUser(agent.ID, map[string]any{"owner_id": owner.ID, "require_mention": true}); err != nil {
		t.Fatalf("set agent owner policy: %v", err)
	}
	ch := createChannel(t, s, "policy-room", "public", owner.ID)
	if err := s.AddChannelMember(&ChannelMember{ChannelID: ch.ID, UserID: owner.ID}); err != nil {
		t.Fatalf("add owner member: %v", err)
	}
	if err := s.AddChannelMember(&ChannelMember{ChannelID: ch.ID, UserID: agent.ID}); err != nil {
		t.Fatalf("add agent member: %v", err)
	}

	state, err := s.GetChannelMemberRequireMentionState(ch.ID, agent.ID)
	if err != nil {
		t.Fatalf("default state: %v", err)
	}
	if state.RequireMentionPolicy != RequireMentionPolicyInherit || !state.EffectiveRequireMention {
		t.Fatalf("default state = %#v, want inherit/effective true", state)
	}

	state, err = s.SetChannelMemberRequireMentionPolicy(ch.ID, agent.ID, RequireMentionPolicyOn)
	if err != nil {
		t.Fatalf("set on: %v", err)
	}
	if state.RequireMentionPolicy != RequireMentionPolicyOn || !state.EffectiveRequireMention {
		t.Fatalf("on state = %#v, want on/effective true", state)
	}

	if _, err := s.SetChannelMemberRequireMentionPolicy(ch.ID, agent.ID, RequireMentionPolicyOff); !errors.Is(err, ErrRequireMentionPolicyOwnerCeiling) {
		t.Fatalf("set off with owner ceiling err = %v, want ErrRequireMentionPolicyOwnerCeiling", err)
	}
	state, err = s.GetChannelMemberRequireMentionState(ch.ID, agent.ID)
	if err != nil {
		t.Fatalf("state after rejected off: %v", err)
	}
	if state.RequireMentionPolicy != RequireMentionPolicyOn || !state.EffectiveRequireMention {
		t.Fatalf("rejected off mutated state = %#v, want on/effective true", state)
	}

	if err := s.UpdateUser(agent.ID, map[string]any{"require_mention": false}); err != nil {
		t.Fatalf("owner permits broader delivery: %v", err)
	}
	state, err = s.SetChannelMemberRequireMentionPolicy(ch.ID, agent.ID, RequireMentionPolicyOff)
	if err != nil {
		t.Fatalf("set off after global opt-out: %v", err)
	}
	if state.RequireMentionPolicy != RequireMentionPolicyOff || state.EffectiveRequireMention {
		t.Fatalf("off state = %#v, want off/effective false", state)
	}
}

func TestRequireMentionPolicyRejectsInvalidAndHumanTargets(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := createUser(t, s, "policy_human_owner", "member")
	human := createUser(t, s, "policy_human_target", "member")
	ch := createChannel(t, s, "policy-human-room", "public", owner.ID)
	if err := s.AddChannelMember(&ChannelMember{ChannelID: ch.ID, UserID: human.ID}); err != nil {
		t.Fatalf("add human member: %v", err)
	}

	if _, err := s.SetChannelMemberRequireMentionPolicy(ch.ID, human.ID, RequireMentionPolicyOn); !errors.Is(err, ErrRequireMentionPolicyTargetNotAgent) {
		t.Fatalf("human target err = %v, want ErrRequireMentionPolicyTargetNotAgent", err)
	}
	if _, err := s.SetChannelMemberRequireMentionPolicy(ch.ID, human.ID, "sometimes"); !errors.Is(err, ErrInvalidRequireMentionPolicy) {
		t.Fatalf("invalid policy err = %v, want ErrInvalidRequireMentionPolicy", err)
	}
	if _, err := s.SetChannelMemberRequireMentionPolicy(ch.ID, "missing-agent", RequireMentionPolicyOn); !errors.Is(err, ErrRequireMentionPolicyTargetNotMember) {
		t.Fatalf("missing member err = %v, want ErrRequireMentionPolicyTargetNotMember", err)
	}
}

func TestListChannelAgentsAllowedWithoutMention(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := createUser(t, s, "policy_delivery_owner", "member")
	sender := createUser(t, s, "policy_delivery_sender", "member")
	allowed := createUser(t, s, "policy_delivery_allowed", "agent")
	blocked := createUser(t, s, "policy_delivery_blocked", "agent")
	explicit := createUser(t, s, "policy_delivery_explicit", "agent")
	for _, u := range []*User{allowed, blocked, explicit} {
		u.OwnerID = &owner.ID
		if err := s.UpdateUser(u.ID, map[string]any{"owner_id": owner.ID, "require_mention": false}); err != nil {
			t.Fatalf("set agent %s owner policy: %v", u.ID, err)
		}
	}
	ch := createChannel(t, s, "policy-delivery-room", "public", sender.ID)
	for _, uid := range []string{sender.ID, allowed.ID, blocked.ID, explicit.ID} {
		if err := s.AddChannelMember(&ChannelMember{ChannelID: ch.ID, UserID: uid}); err != nil {
			t.Fatalf("add member %s: %v", uid, err)
		}
	}
	if _, err := s.SetChannelMemberRequireMentionPolicy(ch.ID, allowed.ID, RequireMentionPolicyOff); err != nil {
		t.Fatalf("set allowed off: %v", err)
	}
	if _, err := s.SetChannelMemberRequireMentionPolicy(ch.ID, blocked.ID, RequireMentionPolicyOn); err != nil {
		t.Fatalf("set blocked on: %v", err)
	}
	if _, err := s.SetChannelMemberRequireMentionPolicy(ch.ID, explicit.ID, RequireMentionPolicyOff); err != nil {
		t.Fatalf("set explicit off: %v", err)
	}

	got, err := s.ListChannelAgentsAllowedWithoutMention(ch.ID, sender.ID, []string{explicit.ID})
	if err != nil {
		t.Fatalf("list non-mention agents: %v", err)
	}
	if len(got) != 1 || got[0] != allowed.ID {
		t.Fatalf("non-mention agents = %v, want [%s]", got, allowed.ID)
	}
}

package store

import "testing"

func TestListEveryoneMentionTargetsUsesChannelMembership(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	sender := createUser(t, s, "everyone_sender", "member")
	human := createUser(t, s, "everyone_human", "member")
	agent := createUser(t, s, "everyone_agent", "agent")
	stranger := createUser(t, s, "everyone_stranger", "member")
	deleted := createUser(t, s, "everyone_deleted", "member")
	deletedAt := int64(1_700_000_001_000)
	if err := s.UpdateUser(deleted.ID, map[string]any{"deleted_at": deletedAt}); err != nil {
		t.Fatalf("mark deleted: %v", err)
	}

	ch := createChannel(t, s, "everyone-room", "private", sender.ID)
	for _, uid := range []string{sender.ID, human.ID, agent.ID, deleted.ID} {
		if err := s.AddChannelMember(&ChannelMember{ChannelID: ch.ID, UserID: uid}); err != nil {
			t.Fatalf("add member %s: %v", uid, err)
		}
	}
	_ = stranger

	targets, err := s.ListEveryoneMentionTargets(ch.ID, sender.ID)
	if err != nil {
		t.Fatalf("list everyone targets: %v", err)
	}
	if len(targets) != 2 || targets[0] != human.ID || targets[1] != agent.ID {
		t.Fatalf("targets = %v, want [%s %s]", targets, human.ID, agent.ID)
	}
}

func TestCreateMessageFullDoesNotTreatEveryoneAsDisplayNameMention(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	sender := createUser(t, s, "everyone_reserved_sender", "member")
	everyoneNamedUser := createUser(t, s, "Everyone", "member")
	ch := createChannel(t, s, "everyone-reserved-room", "public", sender.ID)
	for _, uid := range []string{sender.ID, everyoneNamedUser.ID} {
		if err := s.AddChannelMember(&ChannelMember{ChannelID: ch.ID, UserID: uid}); err != nil {
			t.Fatalf("add member %s: %v", uid, err)
		}
	}

	msg, err := s.CreateMessageFull(ch.ID, sender.ID, "hello @Everyone", "text", nil, nil)
	if err != nil {
		t.Fatalf("create message: %v", err)
	}
	if len(msg.Mentions) != 0 {
		t.Fatalf("legacy mentions = %v, want none for reserved @Everyone token", msg.Mentions)
	}
}

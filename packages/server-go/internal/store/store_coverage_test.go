package store

import (
	"encoding/json"
	"testing"
)

func migratedStore(t *testing.T) *Store {
	t.Helper()
	// TEST-FIX-3-COV: 走 MigratedStoreFromTemplate (1.24ms) 替代 testStore +
	// Migrate (26ms), ~20x 加速. byte-identical schema (template 跑同一份
	// Migrate). 反约束: testStore + Migrate 仍存在 (TestMigrate 真测 Migrate),
	// 仅业务 test 走 template 路径.
	return MigratedStoreFromTemplate(t)
}

func createUser(t *testing.T, s *Store, name, role string) *User {
	t.Helper()
	email := name + "@test.com"
	u := &User{DisplayName: name, Role: role, Email: &email, PasswordHash: "hash"}
	if err := s.CreateUser(u); err != nil {
		t.Fatal(err)
	}
	return u
}

func TestChannelCRUDStore(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "user1", "admin")

	ch := &Channel{Name: "test-ch", Visibility: "public", CreatedBy: u.ID, Type: "channel", Position: GenerateInitialRank()}
	if err := s.CreateChannel(ch); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetChannelByID(ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "test-ch" {
		t.Fatalf("expected test-ch, got %s", got.Name)
	}

	byName, err := s.GetChannelByName("test-ch")
	if err != nil {
		t.Fatal(err)
	}
	if byName.ID != ch.ID {
		t.Fatal("ID mismatch")
	}

	if err := s.UpdateChannel(ch.ID, map[string]any{"topic": "new topic"}); err != nil {
		t.Fatal(err)
	}
	updated, _ := s.GetChannelByID(ch.ID)
	if updated.Topic != "new topic" {
		t.Fatal("topic not updated")
	}

	channels, err := s.ListChannels()
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) == 0 {
		t.Fatal("expected channels")
	}

	if err := s.SoftDeleteChannel(ch.ID); err != nil {
		t.Fatal(err)
	}
	_, err = s.GetChannelByID(ch.ID)
	if err == nil {
		t.Fatal("expected error for deleted channel")
	}

	deleted, err := s.GetChannelIncludingDeleted(ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if deleted.DeletedAt == nil {
		t.Fatal("expected DeletedAt to be set")
	}
}

func TestChannelMemberOps(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "m1", "member")
	u2 := createUser(t, s, "m2", "member")

	ch := &Channel{Name: "mem-ch", Visibility: "private", CreatedBy: u.ID, Type: "channel", Position: GenerateInitialRank()}
	s.CreateChannel(ch)

	s.AddChannelMember(&ChannelMember{ChannelID: ch.ID, UserID: u.ID})
	if !s.IsChannelMember(ch.ID, u.ID) {
		t.Fatal("expected member")
	}

	if s.IsChannelMember(ch.ID, u2.ID) {
		t.Fatal("u2 should not be member")
	}

	if !s.CanAccessChannel(ch.ID, u.ID) {
		t.Fatal("should access")
	}

	members, err := s.ListChannelMembers(ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}

	s.RemoveChannelMember(ch.ID, u.ID)
	if s.IsChannelMember(ch.ID, u.ID) {
		t.Fatal("should not be member after removal")
	}

	s.MarkChannelRead(ch.ID, u.ID)
}

func TestMessageOps(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "sender", "member")
	ch := &Channel{Name: "msg-ch", Visibility: "public", CreatedBy: u.ID, Type: "channel", Position: GenerateInitialRank()}
	s.CreateChannel(ch)
	s.AddChannelMember(&ChannelMember{ChannelID: ch.ID, UserID: u.ID})

	msg, err := s.CreateMessageFull(ch.ID, u.ID, "hello world", "text", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg.ID == "" {
		t.Fatal("expected message ID")
	}
	if msg.SenderName != "sender" {
		t.Fatalf("expected sender name 'sender', got %s", msg.SenderName)
	}

	got, err := s.GetMessageByID(msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "hello world" {
		t.Fatal("content mismatch")
	}

	updated, err := s.UpdateMessage(msg.ID, "edited content")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Content != "edited content" {
		t.Fatal("update failed")
	}
	if updated.EditedAt == nil {
		t.Fatal("expected edited_at")
	}

	msgs, hasMore, err := s.ListChannelMessages(ch.ID, nil, nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) == 0 {
		t.Fatal("expected messages")
	}
	if hasMore {
		t.Fatal("unexpected has_more")
	}

	results, err := s.SearchMessages(ch.ID, "edited", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}

	deletedAt, err := s.SoftDeleteMessage(msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if deletedAt == 0 {
		t.Fatal("expected deletedAt")
	}
}

func TestMessageMentions(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "mentioner", "member")
	u2 := createUser(t, s, "mentioned", "member")
	ch := &Channel{Name: "mention-ch", Visibility: "public", CreatedBy: u.ID, Type: "channel", Position: GenerateInitialRank()}
	s.CreateChannel(ch)
	s.AddChannelMember(&ChannelMember{ChannelID: ch.ID, UserID: u.ID})

	msg, err := s.CreateMessageFull(ch.ID, u.ID, "hello <@"+u2.ID+">", "text", nil, []string{u2.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.Mentions) == 0 {
		t.Fatal("expected mentions")
	}
}

func TestDmChannelOps(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u1 := createUser(t, s, "dm1", "member")
	u2 := createUser(t, s, "dm2", "member")

	ch, err := s.CreateDmChannel(u1.ID, u2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Type != "dm" {
		t.Fatal("expected dm type")
	}

	ch2, err := s.CreateDmChannel(u2.ID, u1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ch2.ID != ch.ID {
		t.Fatal("DM should be idempotent")
	}

	dms, err := s.ListDmChannelsForUser(u1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(dms) == 0 {
		t.Fatal("expected DM channels")
	}
}

func TestChannelGroupOps(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "grouper", "admin")

	pos := GenerateRankBetween(s.GetLastGroupPosition(), "")
	g := &ChannelGroup{Name: "Test Group", Position: pos, CreatedBy: u.ID}
	if err := s.CreateChannelGroup(g); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetChannelGroup(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Test Group" {
		t.Fatal("name mismatch")
	}

	groups, err := s.ListChannelGroups()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) == 0 {
		t.Fatal("expected groups")
	}

	s.UpdateChannelGroup(g.ID, "Updated Group")
	updated, _ := s.GetChannelGroup(g.ID)
	if updated.Name != "Updated Group" {
		t.Fatal("update failed")
	}

	ch := &Channel{Name: "grp-ch", Visibility: "public", CreatedBy: u.ID, Type: "channel", Position: GenerateInitialRank(), GroupID: &g.ID}
	s.CreateChannel(ch)

	ungrouped, err := s.UngroupChannels(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ungrouped) == 0 {
		t.Fatal("expected ungrouped channels")
	}

	s.DeleteChannelGroup(g.ID)
	_, err = s.GetChannelGroup(g.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestPositionHelpers(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "poshelper", "admin")

	ch1 := &Channel{Name: "pos-1", Visibility: "public", CreatedBy: u.ID, Type: "channel", Position: GenerateInitialRank()}
	s.CreateChannel(ch1)

	ch2 := &Channel{Name: "pos-2", Visibility: "public", CreatedBy: u.ID, Type: "channel", Position: GenerateRankBetween(ch1.Position, "")}
	s.CreateChannel(ch2)

	lastPos := s.GetLastChannelPosition()
	if lastPos == "" {
		t.Fatal("expected last position")
	}

	before, after, err := s.GetAdjacentChannelPositions(&ch1.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if before == "" {
		t.Fatal("expected before position")
	}
	_ = after

	before2, after2, err := s.GetAdjacentChannelPositions(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = before2
	if after2 == "" {
		t.Fatal("expected after position")
	}

	s.UpdateChannelPosition(ch1.ID, GenerateRankBetween(ch2.Position, ""), nil)
}

func TestGroupPositionHelpers(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "gposhelper", "admin")

	g1 := &ChannelGroup{Name: "GP1", Position: GenerateInitialRank(), CreatedBy: u.ID}
	s.CreateChannelGroup(g1)
	g2 := &ChannelGroup{Name: "GP2", Position: GenerateRankBetween(g1.Position, ""), CreatedBy: u.ID}
	s.CreateChannelGroup(g2)

	lastPos := s.GetLastGroupPosition()
	if lastPos == "" {
		t.Fatal("expected last group position")
	}

	before, after, err := s.GetAdjacentGroupPositions(&g1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before == "" {
		t.Fatal("expected before")
	}
	_ = after

	before2, after2, err := s.GetAdjacentGroupPositions(nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = before2
	if after2 == "" {
		t.Fatal("expected after")
	}

	s.UpdateGroupPosition(g1.ID, GenerateRankBetween(g2.Position, ""))
}

func TestReactionOps(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "reactor", "member")
	u2 := createUser(t, s, "reactor2", "member")
	ch := &Channel{Name: "rxn-ch", Visibility: "public", CreatedBy: u.ID, Type: "channel", Position: GenerateInitialRank()}
	s.CreateChannel(ch)
	s.AddChannelMember(&ChannelMember{ChannelID: ch.ID, UserID: u.ID})

	msg, _ := s.CreateMessageFull(ch.ID, u.ID, "react to this", "text", nil, nil)

	s.AddReaction(msg.ID, u.ID, "👍")
	s.AddReaction(msg.ID, u2.ID, "❤️")

	reactions, err := s.GetReactionsByMessage(msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reactions) != 2 {
		t.Fatalf("expected 2 reactions, got %d", len(reactions))
	}

	s.RemoveReaction(msg.ID, u.ID, "👍")
	reactions2, _ := s.GetReactionsByMessage(msg.ID)
	if len(reactions2) != 1 {
		t.Fatalf("expected 1 reaction, got %d", len(reactions2))
	}
}

func TestWorkspaceOps(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "wsuser", "member")
	ch := &Channel{Name: "ws-ch", Visibility: "public", CreatedBy: u.ID, Type: "channel", Position: GenerateInitialRank()}
	s.CreateChannel(ch)

	wf := &WorkspaceFile{UserID: u.ID, ChannelID: ch.ID, Name: "test.txt", MimeType: "text/plain", SizeBytes: 100, Source: "upload"}
	result, err := s.InsertWorkspaceFile(wf)
	if err != nil {
		t.Fatal(err)
	}
	if result.ID == "" {
		t.Fatal("expected ID")
	}

	got, err := s.GetWorkspaceFile(result.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "test.txt" {
		t.Fatal("name mismatch")
	}

	files, err := s.ListWorkspaceFiles(u.ID, ch.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("expected files")
	}

	renamed, err := s.RenameWorkspaceFile(result.ID, "renamed.txt")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Name != "renamed.txt" {
		t.Fatal("rename failed")
	}

	s.UpdateWorkspaceFileSize(result.ID, 200)

	dir, err := s.MkdirWorkspace(u.ID, ch.ID, nil, "test-dir")
	if err != nil {
		t.Fatal(err)
	}
	if !dir.IsDirectory {
		t.Fatal("expected directory")
	}

	moved, err := s.MoveWorkspaceFile(result.ID, &dir.ID)
	if err != nil {
		t.Fatal(err)
	}
	if moved.ParentID == nil || *moved.ParentID != dir.ID {
		t.Fatal("move failed")
	}

	allFiles, err := s.GetAllWorkspaceFiles(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(allFiles) < 2 {
		t.Fatalf("expected at least 2 files, got %d", len(allFiles))
	}

	siblings, err := s.GetSiblingNames(u.ID, ch.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = siblings

	s.DeleteWorkspaceFile(dir.ID)
}

func TestResolveConflictFunc(t *testing.T) {
	t.Parallel()
	result := ResolveConflict("test.txt", []string{"test.txt"})
	if result != "test (1).txt" {
		t.Fatalf("expected test (1).txt, got %s", result)
	}

	result2 := ResolveConflict("test.txt", []string{})
	if result2 != "test.txt" {
		t.Fatalf("expected test.txt, got %s", result2)
	}

	result3 := ResolveConflict("test.txt", []string{"test.txt", "test (1).txt"})
	if result3 != "test (2).txt" {
		t.Fatalf("expected test (2).txt, got %s", result3)
	}
}

func TestRemoteNodeOps(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "remoteuser", "member")

	node, err := s.CreateRemoteNode(u.ID, "my-machine")
	if err != nil {
		t.Fatal(err)
	}
	if node.ID == "" {
		t.Fatal("expected node ID")
	}
	if node.ConnectionToken == "" {
		t.Fatal("expected connection token")
	}

	nodes, err := s.ListRemoteNodes(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected nodes")
	}

	got, err := s.GetRemoteNode(node.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.MachineName != "my-machine" {
		t.Fatal("name mismatch")
	}

	byToken, err := s.GetRemoteNodeByToken(node.ConnectionToken)
	if err != nil {
		t.Fatal(err)
	}
	if byToken.ID != node.ID {
		t.Fatal("token lookup mismatch")
	}

	s.UpdateRemoteNodeLastSeen(node.ID)

	ch := &Channel{Name: "binding-ch", Visibility: "public", CreatedBy: u.ID, Type: "channel", Position: GenerateInitialRank()}
	s.CreateChannel(ch)

	binding, err := s.CreateRemoteBinding(node.ID, ch.ID, "/home/user", "project")
	if err != nil {
		t.Fatal(err)
	}

	bindings, err := s.ListRemoteBindings(node.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) == 0 {
		t.Fatal("expected bindings")
	}

	gotBinding, err := s.GetRemoteBinding(binding.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotBinding.Path != "/home/user" {
		t.Fatal("path mismatch")
	}

	chBindings, err := s.ListChannelRemoteBindings(ch.ID, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(chBindings) == 0 {
		t.Fatal("expected channel bindings")
	}

	s.DeleteRemoteBinding(binding.ID)
	s.DeleteRemoteNode(node.ID)

	_, err = s.GetRemoteNode(node.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestEventOps(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "evtuser", "member")
	ch := &Channel{Name: "evt-ch", Visibility: "public", CreatedBy: u.ID, Type: "channel", Position: GenerateInitialRank()}
	s.CreateChannel(ch)
	s.AddChannelMember(&ChannelMember{ChannelID: ch.ID, UserID: u.ID})

	s.CreateEvent(&Event{Kind: "test_event", ChannelID: ch.ID, Payload: `{"test":true}`})

	cursor := s.GetLatestCursor()
	if cursor == 0 {
		t.Fatal("expected non-zero cursor")
	}

	events, err := s.GetEventsSince(0, 10, []string{ch.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("expected events")
	}

	eventsWithChanges, err := s.GetEventsSinceWithChanges(0, 10, []string{ch.ID}, []string{"test_event"})
	if err != nil {
		t.Fatal(err)
	}
	if len(eventsWithChanges) == 0 {
		t.Fatal("expected events")
	}

	evt, err := s.GetEventByCursor(cursor)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Kind != "test_event" {
		t.Fatalf("expected test_event, got %s", evt.Kind)
	}

	ids := s.GetUserChannelIDs(u.ID)
	if len(ids) == 0 {
		t.Fatal("expected channel IDs")
	}
}

func TestCreateMessageFullEventPayloadIncludesChannelDisplayMetadata(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "evtmeta", "member")
	ch := &Channel{Name: "product-room", Visibility: "public", CreatedBy: u.ID, Type: "channel", Position: GenerateInitialRank()}
	if err := s.CreateChannel(ch); err != nil {
		t.Fatal(err)
	}

	if _, err := s.CreateMessageFull(ch.ID, u.ID, "hello", "text", nil, nil); err != nil {
		t.Fatal(err)
	}

	events, err := s.GetEventsSince(0, 10, []string{ch.ID})
	if err != nil {
		t.Fatal(err)
	}
	var messageEvent *Event
	for i := range events {
		if events[i].Kind == "message" {
			messageEvent = &events[i]
			break
		}
	}
	if messageEvent == nil {
		t.Fatalf("expected message event, got %#v", events)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(messageEvent.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	if got := payload["channel_type"]; got != "channel" {
		t.Fatalf("channel_type = %v, want channel", got)
	}
	if got := payload["channel_name"]; got != "product-room" {
		t.Fatalf("channel_name = %v, want product-room", got)
	}
}

func TestAdminOps(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "adminops", "member")
	m := createUser(t, s, "memberops", "member")

	users, err := s.ListAdminUsers()
	if err != nil {
		t.Fatal(err)
	}
	if len(users) < 2 {
		t.Fatal("expected users")
	}

	s.UpdateUser(m.ID, map[string]any{"display_name": "Updated"})
	updated, _ := s.GetUserByID(m.ID)
	if updated.DisplayName != "Updated" {
		t.Fatal("update failed")
	}

	apiKey, err := GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	s.SetAPIKey(m.ID, apiKey)

	byKey, err := s.GetUserByAPIKey(apiKey)
	if err != nil {
		t.Fatal(err)
	}
	if byKey.ID != m.ID {
		t.Fatal("API key lookup failed")
	}

	s.ClearAPIKey(m.ID)

	ic, err := s.CreateInviteCode(u.ID, nil, "test note")
	if err != nil {
		t.Fatal(err)
	}

	invites, err := s.ListInviteCodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(invites) == 0 {
		t.Fatal("expected invites")
	}

	deleted, err := s.DeleteInviteCode(ic.Code)
	if err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("expected deleted")
	}

	deleted2, _ := s.DeleteInviteCode("nonexistent")
	if deleted2 {
		t.Fatal("should not delete nonexistent")
	}

	ch := &Channel{Name: "admin-ch", Visibility: "public", CreatedBy: u.ID, Type: "channel", Position: GenerateInitialRank()}
	s.CreateChannel(ch)

	adminChs, err := s.ListAllChannelsAdmin()
	if err != nil {
		t.Fatal(err)
	}
	if len(adminChs) == 0 {
		t.Fatal("expected channels")
	}

	s.ForceDeleteChannel(ch.ID)

	s.SoftDeleteUser(m.ID)
	_, err = s.GetUserByID(m.ID)
	if err == nil {
		t.Fatal("expected error for deleted user")
	}
}

func TestAgentOps(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := createUser(t, s, "agentowner", "member")

	apiKey, _ := GenerateAPIKey()
	agent := &User{DisplayName: "Bot", Role: "agent", OwnerID: &owner.ID, APIKey: &apiKey}
	s.CreateUser(agent)

	agents, err := s.ListAgentsByOwner(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) == 0 {
		t.Fatal("expected agents")
	}

	allAgents, err := s.ListAllAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(allAgents) == 0 {
		t.Fatal("expected all agents")
	}

	got, err := s.GetAgent(agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "Bot" {
		t.Fatal("name mismatch")
	}
}

func TestPermissionOps(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "permuser", "member")

	s.GrantDefaultPermissions(u.ID, "member")
	perms, err := s.ListUserPermissions(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	// AP-0: humans default to a single (*, *) row.
	if len(perms) != 1 {
		t.Fatalf("expected 1 perm for member, got %d", len(perms))
	}
	if perms[0].Permission != "*" || perms[0].Scope != "*" {
		t.Fatalf("expected (*, *), got (%s, %s)", perms[0].Permission, perms[0].Scope)
	}

	s.GrantCreatorPermissions(u.ID, "member", "ch-123", nil)
	perms2, _ := s.ListUserPermissions(u.ID)
	if len(perms2) <= len(perms) {
		t.Fatal("expected more perms after creator grant")
	}

	s.DeletePermissionsByScope("channel:ch-123")
	s.DeletePermissionsByUserID(u.ID)

	perms3, _ := s.ListUserPermissions(u.ID)
	if len(perms3) != 0 {
		t.Fatalf("expected 0 perms, got %d", len(perms3))
	}
}

func TestChannelQueries(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "chquery", "admin")

	ch := &Channel{Name: "query-ch", Visibility: "public", CreatedBy: u.ID, Type: "channel", Position: GenerateInitialRank()}
	s.CreateChannel(ch)
	s.AddChannelMember(&ChannelMember{ChannelID: ch.ID, UserID: u.ID})

	s.CreateMessageFull(ch.ID, u.ID, "test msg for query", "text", nil, nil)

	cwc, err := s.GetChannelWithCounts(ch.ID, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cwc.MemberCount == 0 {
		t.Fatal("expected member count")
	}

	detail, err := s.GetChannelDetail(ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail) == 0 {
		t.Fatal("expected detail")
	}

	public, err := s.ListChannelsPublic()
	if err != nil {
		t.Fatal(err)
	}
	if len(public) == 0 {
		t.Fatal("expected public channels")
	}

	withUnread, err := s.ListChannelsWithUnread(u.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(withUnread) == 0 {
		t.Fatal("expected channels with unread")
	}

	adminList, err := s.ListAllChannelsForAdmin(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(adminList) == 0 {
		t.Fatal("expected admin channel list")
	}

	preview, err := s.GetPreviewMessages(ch.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	_ = preview

	s.AddAllUsersToChannel(ch.ID)
	s.AddUserToPublicChannels(u.ID)
}

func TestOnlineUsers(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "onlineuser", "member")
	s.UpdateLastSeen(u.ID)

	users, err := s.GetOnlineUsers()
	if err != nil {
		t.Fatal(err)
	}
	if len(users) == 0 {
		t.Fatal("expected online users")
	}
}

func TestGetUserByDisplayName(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	createUser(t, s, "findme", "member")

	u, err := s.GetUserByDisplayName("findme")
	if err != nil {
		t.Fatal(err)
	}
	if u.DisplayName != "findme" {
		t.Fatal("name mismatch")
	}

	_, err = s.GetUserByDisplayName("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInviteCodeConsume(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "inviter", "admin")
	u2 := createUser(t, s, "consumer", "member")

	ic, err := s.CreateInviteCode(u.ID, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.ConsumeInviteCode(ic.Code, u2.ID); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetInviteCode(ic.Code)
	if err != nil {
		t.Fatal(err)
	}
	if got.UsedBy == nil {
		t.Fatal("expected used_by")
	}

	if err := s.ConsumeInviteCode(ic.Code, u2.ID); err == nil {
		t.Fatal("expected error consuming used code")
	}
}

func TestDefaultPermissionsAgent(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "agentperm", "agent")
	s.GrantDefaultPermissions(u.ID, "agent")
	perms, _ := s.ListUserPermissions(u.ID)
	// AP-0-bis (R3 决议 #1, 2026-04-28): agent default capability set is
	// locked at [message.send, message.read].
	if len(perms) != 2 {
		t.Fatalf("expected 2 perms for agent (send + read), got %d", len(perms))
	}
	got := map[string]bool{}
	for _, p := range perms {
		got[p.Permission] = true
	}
	for _, want := range []string{"message.send", "message.read"} {
		if !got[want] {
			t.Fatalf("agent default missing %q (got %v)", want, got)
		}
	}
}

func TestDefaultPermissionsAdmin(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	u := createUser(t, s, "adminperm", "admin")
	s.GrantDefaultPermissions(u.ID, "admin")
	perms, _ := s.ListUserPermissions(u.ID)
	if len(perms) != 0 {
		t.Fatalf("expected 0 perms for admin (implicit), got %d", len(perms))
	}
}

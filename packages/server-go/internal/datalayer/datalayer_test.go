// DL-1 — datalayer unit tests (12 cases, 4 interface × 3 happy/empty/err).
//
// This follows spec §0: interface behavior stays unchanged, and factory-based
// dependency injection supplies handlers.
// Tests use real SQLite (in-memory) + real PresenceTracker (empty db) to verify
// the interface contract remains unchanged.

package datalayer

import (
	"context"
	"errors"
	"testing"

	"borgee-server/internal/presence"
	"borgee-server/internal/store"
)

func newTestDataLayer(t *testing.T) *DataLayer {
	t.Helper()
	s := store.MigratedStoreFromTemplate(t)
	t.Cleanup(func() { _ = s.Close() })
	pt, err := presence.NewSessionsTracker(s.DB())
	if err != nil {
		t.Fatalf("presence.NewSessionsTracker: %v", err)
	}
	return NewDataLayer(s, pt, nil)
}

func newTestUser(t *testing.T, dl *DataLayer, displayName, email string) *store.User {
	t.Helper()
	em := email
	u := &store.User{
		DisplayName:  displayName,
		Role:         "member",
		Email:        &em,
		PasswordHash: "hash",
	}
	if err := dl.UserRepo.Create(context.Background(), u); err != nil {
		t.Fatalf("UserRepo.Create: %v", err)
	}
	return u
}

// ----- UserRepository (3 cases) -----

func TestUserRepository_GetByID_Happy(t *testing.T) {
	t.Parallel()
	dl := newTestDataLayer(t)
	u := newTestUser(t, dl, "alice", "alice@example.com")
	got, err := dl.UserRepo.GetByID(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != u.ID || got.DisplayName != "alice" {
		t.Fatalf("mismatch: %+v", got)
	}
}

func TestUserRepository_GetByEmail_NotFound(t *testing.T) {
	t.Parallel()
	dl := newTestDataLayer(t)
	_, err := dl.UserRepo.GetByEmail(context.Background(), "ghost@nope")
	if !errors.Is(err, ErrRepositoryNotFound) {
		t.Fatalf("want ErrRepositoryNotFound, got %v", err)
	}
}

func TestUserRepository_GetByDisplayName_Empty(t *testing.T) {
	t.Parallel()
	dl := newTestDataLayer(t)
	_, err := dl.UserRepo.GetByDisplayName(context.Background(), "")
	if !errors.Is(err, ErrRepositoryNotFound) {
		t.Fatalf("want ErrRepositoryNotFound for empty name, got %v", err)
	}
}

// ----- ChannelRepository (3 cases) -----

func TestChannelRepository_CreateAndGet_Happy(t *testing.T) {
	t.Parallel()
	dl := newTestDataLayer(t)
	u := newTestUser(t, dl, "ch-creator", "ch@example.com")
	ch := &store.Channel{
		Name:      "general",
		CreatedBy: u.ID,
	}
	if err := dl.ChannelRepo.Create(context.Background(), ch); err != nil {
		t.Fatalf("ChannelRepo.Create: %v", err)
	}
	if ch.ID == "" {
		t.Fatal("expected ID set after Create")
	}
	got, err := dl.ChannelRepo.GetByID(context.Background(), ch.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "general" {
		t.Fatalf("name mismatch: %+v", got)
	}
}

func TestChannelRepository_GetByID_NotFound(t *testing.T) {
	t.Parallel()
	dl := newTestDataLayer(t)
	_, err := dl.ChannelRepo.GetByID(context.Background(), "no-such-id")
	if !errors.Is(err, ErrRepositoryNotFound) {
		t.Fatalf("want ErrRepositoryNotFound, got %v", err)
	}
}

func TestChannelRepository_GetByNameInOrg_Empty(t *testing.T) {
	t.Parallel()
	dl := newTestDataLayer(t)
	_, err := dl.ChannelRepo.GetByNameInOrg(context.Background(), "", "")
	if !errors.Is(err, ErrRepositoryNotFound) {
		t.Fatalf("want ErrRepositoryNotFound, got %v", err)
	}
}

// ----- MessageRepository (3 cases) -----

func TestMessageRepository_CreateAndGet_Happy(t *testing.T) {
	t.Parallel()
	dl := newTestDataLayer(t)
	u := newTestUser(t, dl, "msg-sender", "ms@example.com")
	ch := &store.Channel{Name: "msgs", CreatedBy: u.ID}
	if err := dl.ChannelRepo.Create(context.Background(), ch); err != nil {
		t.Fatalf("ch.Create: %v", err)
	}
	msg := &store.Message{
		ChannelID: ch.ID,
		SenderID:  u.ID,
		Content:   "hi",
	}
	if err := dl.MessageRepo.Create(context.Background(), msg); err != nil {
		t.Fatalf("MessageRepo.Create: %v", err)
	}
	got, err := dl.MessageRepo.GetByID(context.Background(), msg.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Content != "hi" {
		t.Fatalf("content mismatch: %+v", got)
	}
}

func TestMessageRepository_GetByID_NotFound(t *testing.T) {
	t.Parallel()
	dl := newTestDataLayer(t)
	_, err := dl.MessageRepo.GetByID(context.Background(), "missing")
	if !errors.Is(err, ErrRepositoryNotFound) {
		t.Fatalf("want ErrRepositoryNotFound, got %v", err)
	}
}

func TestMessageRepository_GetByID_Empty(t *testing.T) {
	t.Parallel()
	dl := newTestDataLayer(t)
	_, err := dl.MessageRepo.GetByID(context.Background(), "")
	if !errors.Is(err, ErrRepositoryNotFound) {
		t.Fatalf("want ErrRepositoryNotFound, got %v", err)
	}
}

// ----- PresenceStore + Storage + EventBus (3 cases combined for non-Repo interfaces) -----

func TestPresenceStore_IsOnline_OfflineUser(t *testing.T) {
	t.Parallel()
	dl := newTestDataLayer(t)
	online, err := dl.Presence.IsOnline(context.Background(), "no-such-user")
	if err != nil {
		t.Fatalf("IsOnline: %v", err)
	}
	if online {
		t.Fatal("expected offline for unknown user")
	}
	sess, err := dl.Presence.Sessions(context.Background(), "no-such-user")
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(sess) != 0 {
		t.Fatalf("expected empty sessions, got %v", sess)
	}
}

func TestStorage_GetURL_HappyAndEmpty(t *testing.T) {
	t.Parallel()
	dl := newTestDataLayer(t)
	ctx := context.Background()
	url, err := dl.Storage.GetURL(ctx, "abc")
	if err != nil {
		t.Fatalf("GetURL: %v", err)
	}
	if url != "db://artifact/abc" {
		t.Fatalf("url mismatch: %q", url)
	}
	if _, err := dl.Storage.GetURL(ctx, ""); !errors.Is(err, ErrStorageKeyNotFound) {
		t.Fatalf("want ErrStorageKeyNotFound for empty key, got %v", err)
	}
	if err := dl.Storage.PutBlob(ctx, "", []byte("x")); !errors.Is(err, ErrStorageKeyNotFound) {
		t.Fatalf("PutBlob empty: want ErrStorageKeyNotFound, got %v", err)
	}
	if err := dl.Storage.Delete(ctx, ""); !errors.Is(err, ErrStorageKeyNotFound) {
		t.Fatalf("Delete empty: want ErrStorageKeyNotFound, got %v", err)
	}
}

func TestEventBus_PubSub_Roundtrip(t *testing.T) {
	t.Parallel()
	dl := newTestDataLayer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := dl.EventBus.Subscribe(ctx, "test.topic")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := dl.EventBus.Publish(ctx, "test.topic", []byte("payload")); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	select {
	case ev := <-ch:
		if ev.Topic != "test.topic" || string(ev.Payload) != "payload" {
			t.Fatalf("unexpected event: %+v", ev)
		}
	default:
		t.Fatal("expected event delivered to buffered channel")
	}

	// Publish to a topic with no subscribers — best-effort, no error.
	if err := dl.EventBus.Publish(ctx, "no.subs", []byte("x")); err != nil {
		t.Fatalf("Publish to empty topic: %v", err)
	}
}

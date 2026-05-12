// DL-1 — Repository interfaces (blueprint §4 B item 4).
//
// Principle ① (DL-1 spec §0): 4 typed Repository interfaces wrap the existing
// store.Store behavior. The v1 SQLiteRepository implementation delegates to
// store.Store gorm queries without changing behavior.
//
// Note: blueprint §4 B lists 4 typed repos (User / Channel / Message /
// Artifact). In v1, only User/Channel/Message have store package models and
// CRUD methods; Artifact still uses direct gorm in internal/api/artifacts.go.
// ArtifactRepo remains a v1.5 follow-up for when store.Artifact is extracted,
// matching the spec §3 progressive-migration policy.
//
// Implementation swap path (v3+, triggered by DL-3 threshold monitor):
//   - SQLiteRepository (v1) → store.Store wrap
//   - PostgresRepository    → standard SQL (blueprint §4 C #10 forbids ORM)
package datalayer

import (
	"context"
	"errors"

	"borgee-server/internal/store"
)

// ErrRepositoryNotFound is returned by Repository methods when the entity
// has no matching row.
var ErrRepositoryNotFound = errors.New("datalayer: repository entity not found")

// UserRepository is the canonical interface for user CRUD ops.
// v1 wraps store.Store user methods without changing behavior.
type UserRepository interface {
	GetByID(ctx context.Context, id string) (*store.User, error)
	GetByEmail(ctx context.Context, email string) (*store.User, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*store.User, error)
	GetByDisplayName(ctx context.Context, displayName string) (*store.User, error)
	Create(ctx context.Context, user *store.User) error
}

// ChannelRepository is the canonical interface for channel CRUD ops.
type ChannelRepository interface {
	GetByID(ctx context.Context, id string) (*store.Channel, error)
	GetByName(ctx context.Context, name string) (*store.Channel, error)
	GetByNameInOrg(ctx context.Context, orgID, name string) (*store.Channel, error)
	Create(ctx context.Context, ch *store.Channel) error
}

// MessageRepository is the canonical interface for message CRUD ops.
type MessageRepository interface {
	GetByID(ctx context.Context, id string) (*store.Message, error)
	Create(ctx context.Context, msg *store.Message) error
}

// DL-1 — Repository interfaces (蓝图 §4 B 第 4 条).
//
// 设计 ① (DL-1 spec §0): 4 typed Repository wrap 既有 store.Store 字面.
// v1 实现 SQLiteRepository 走 store.Store gorm 直查 byte-identical 不破.
//
// Note (战马D): 蓝图 §4 B 列了 4 typed Repo (User / Channel / Message /
// Artifact); v1 现状只有 User/Channel/Message 在 store 包真有 model + CRUD,
// Artifact 走 internal/api/artifacts.go 直 gorm. ArtifactRepo 留 v1.5 后续
// 当 store.Artifact model 抽出时再补 (跟 spec §3 "渐进迁移" 设计沿用).
//
// 切换路径 (留 v3+, DL-3 阈值哨触发):
//   - SQLiteRepository (v1) → store.Store wrap
//   - PostgresRepository    → standard SQL (蓝图 §4 C #10 字面禁 ORM)
package datalayer

import (
	"context"
	"errors"

	"borgee-server/internal/store"
)

// ErrRepositoryNotFound is returned by Repository methods when the entity
// has no matching row.
var ErrRepositoryNotFound = errors.New("datalayer: repository entity not found")

// UserRepository is the SSOT interface for user CRUD ops.
// v1 wrap store.Store user methods byte-identical.
type UserRepository interface {
	GetByID(ctx context.Context, id string) (*store.User, error)
	GetByEmail(ctx context.Context, email string) (*store.User, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*store.User, error)
	GetByDisplayName(ctx context.Context, displayName string) (*store.User, error)
	Create(ctx context.Context, user *store.User) error
}

// ChannelRepository is the SSOT interface for channel CRUD ops.
type ChannelRepository interface {
	GetByID(ctx context.Context, id string) (*store.Channel, error)
	GetByName(ctx context.Context, name string) (*store.Channel, error)
	GetByNameInOrg(ctx context.Context, orgID, name string) (*store.Channel, error)
	Create(ctx context.Context, ch *store.Channel) error
}

// MessageRepository is the SSOT interface for message CRUD ops.
type MessageRepository interface {
	GetByID(ctx context.Context, id string) (*store.Message, error)
	Create(ctx context.Context, msg *store.Message) error
}

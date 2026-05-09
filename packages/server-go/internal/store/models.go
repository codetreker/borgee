package store

type Channel struct {
	ID         string  `gorm:"primaryKey;size:36" json:"id"`
	Name       string  `gorm:"not null;size:100" json:"name"`
	Topic      string  `gorm:"not null;default:'';size:500" json:"topic"`
	Visibility string  `gorm:"not null;default:public;size:20" json:"visibility"`
	CreatedAt  int64   `gorm:"not null" json:"created_at"`
	CreatedBy  string  `gorm:"not null;size:36;index" json:"created_by"`
	Type       string  `gorm:"not null;default:channel;size:20" json:"type"`
	DeletedAt  *int64  `gorm:"index" json:"deleted_at,omitempty"`
	Position   string  `gorm:"not null;default:0|aaaaaa;size:50;index" json:"position"`
	GroupID    *string `gorm:"size:36" json:"group_id"`
	// OrgID is the channel's organization (CM-3.1). Stamped at create time
	// from creator.OrgID. Column added by migration cm_1_1_organizations
	// (NOT NULL DEFAULT ''); v=9 backfills legacy rows. Blueprint §1.1 forbids
	// UI exposure → json:"-".
	OrgID string `gorm:"column:org_id;not null;default:'';size:36;index" json:"-"`
	// ArchivedAt is the soft-archive marker (CHN-1.1, migration v=11). nil = active;
	// non-nil = archived (channel is read-only, hidden from default lists).
	// Distinct from DeletedAt — archive preserves history per channel-model §2 不变量。
	ArchivedAt *int64 `gorm:"column:archived_at" json:"archived_at,omitempty"`
	// DescriptionEditHistory is a JSON array of edit-history entries appended
	// by UpdateChannelDescription each time channel.topic changes via CHN-10
	// owner-only PUT path (CHN-14.2 SSOT 跟 DM-7 messages.edit_history 同模式
	// byte-identical). NULL = 无历史 / 老 channel 行 byte-identical 不动.
	// Migration v=44 (chn_14_1_channels_description_edit_history).
	DescriptionEditHistory *string `gorm:"column:description_edit_history" json:"description_edit_history,omitempty"`
}

type ChannelGroup struct {
	ID        string `gorm:"primaryKey;size:36" json:"id"`
	Name      string `gorm:"not null;size:100" json:"name"`
	Position  string `gorm:"not null;size:50;index" json:"position"`
	CreatedBy string `gorm:"not null;size:36;index" json:"created_by"`
	CreatedAt int64  `gorm:"not null" json:"created_at"`
}

type User struct {
	ID             string  `gorm:"primaryKey;size:36" json:"id"`
	DisplayName    string  `gorm:"not null;size:100" json:"display_name"`
	Role           string  `gorm:"not null;default:member;size:20" json:"role"`
	AvatarURL      string  `gorm:"size:500" json:"avatar_url"`
	APIKey         *string `gorm:"uniqueIndex;size:128" json:"-"`
	CreatedAt      int64   `gorm:"not null" json:"created_at"`
	Email          *string `gorm:"uniqueIndex:idx_users_email;size:255" json:"email,omitempty"`
	PasswordHash   string  `gorm:"size:255" json:"-"`
	LastSeenAt     *int64  `json:"last_seen_at,omitempty"`
	RequireMention bool    `gorm:"not null;default:true" json:"require_mention"`
	OwnerID        *string `gorm:"size:36;index" json:"owner_id,omitempty"`
	DeletedAt      *int64  `gorm:"index" json:"deleted_at,omitempty"`
	Disabled       bool    `gorm:"not null;default:false" json:"disabled"`
	// OrgID is the user's organization (CM-1.2). Blueprint §1.1 forbids UI
	// exposure, hence json:"-" — every API serializer is hand-built map and
	// must NOT include org_id. Column added by migration cm_1_1_organizations
	// (NOT NULL DEFAULT '').
	OrgID string `gorm:"column:org_id;not null;default:'';size:36;index" json:"-"`
}

// Organization is the data-layer container for a person's resources
// (CM-1.2, blueprint concept-model §1.1 + §2). 1 person = 1 org in v0; UI
// permanently does not expose org_id.
type Organization struct {
	ID        string `gorm:"primaryKey;size:36" json:"id"`
	Name      string `gorm:"not null;size:100" json:"name"`
	CreatedAt int64  `gorm:"not null" json:"created_at"`
}

type Message struct {
	ID          string  `gorm:"primaryKey;size:36" json:"id"`
	ChannelID   string  `gorm:"not null;size:36;index:idx_messages_channel_time,priority:1" json:"channel_id"`
	SenderID    string  `gorm:"not null;size:36;index" json:"sender_id"`
	Content     string  `gorm:"not null" json:"content"`
	ContentType string  `gorm:"not null;default:text;size:20" json:"content_type"`
	ReplyToID   *string `gorm:"size:36;index" json:"reply_to_id"`
	CreatedAt   int64   `gorm:"not null;index:idx_messages_channel_time,priority:2,sort:desc" json:"created_at"`
	EditedAt    *int64  `json:"edited_at"`
	DeletedAt   *int64  `gorm:"index" json:"deleted_at"`
	// QuickAction is a JSON-encoded `{kind, label, action}` payload attached
	// to system messages (CM-onboarding migration v=7). Nil/empty for plain
	// chat messages. The client decodes and renders a button when set.
	QuickAction *string `gorm:"column:quick_action" json:"quick_action,omitempty"`
	// OrgID is the message's organization (CM-3.1). Stamped at INSERT from
	// sender.OrgID. Column added by migration cm_1_1_organizations.
	OrgID string `gorm:"column:org_id;not null;default:'';size:36;index" json:"-"`
	// EditHistory is a JSON array of edit-history entries appended by
	// UpdateMessage when the content changes. NULL = no edits (DM-7
	// 设计 ①). Format: [{old_content, ts, reason}].
	EditHistory *string `gorm:"column:edit_history" json:"edit_history,omitempty"`
	// PinnedAt is Unix ms when the message was pinned (DM-10.1). NULL =
	// unpinned. DM scope only — server REJECTS pin on non-DM channels
	// (跟 chn_7_mute DM-only mirror, 设计 ②). Sparse partial idx
	// `idx_messages_pinned_at WHERE pinned_at IS NOT NULL` (跟 AL-7.1
	// archived_at + HB-5.1 archived_at sparse 同模式).
	PinnedAt *int64 `gorm:"column:pinned_at;index:,where:pinned_at IS NOT NULL" json:"pinned_at,omitempty"`
}

type ChannelMember struct {
	ChannelID  string `gorm:"primaryKey;size:36" json:"channel_id"`
	UserID     string `gorm:"primaryKey;size:36;index" json:"user_id"`
	JoinedAt   int64  `gorm:"not null" json:"joined_at"`
	LastReadAt *int64 `json:"last_read_at,omitempty"`
	// Silent (CHN-1.1, migration v=11): when true, the member does not
	// auto-broadcast on lifecycle events. Default 0 for humans; backfilled
	// to 1 for agents. concept-model §1.4 — agent = colleague, not chatter.
	Silent bool `gorm:"column:silent;not null;default:0" json:"silent"`
	// OrgIDAtJoin (CHN-1.1): audit-only snapshot of the user's OrgID at the
	// time of join. Not used in the read path — kept for cross-org history.
	OrgIDAtJoin string `gorm:"column:org_id_at_join;not null;default:''" json:"-"`
}

type Mention struct {
	ID        string `gorm:"primaryKey;size:36" json:"id"`
	MessageID string `gorm:"not null;size:36;index" json:"message_id"`
	UserID    string `gorm:"not null;size:36;index:idx_mentions_user,priority:1" json:"user_id"`
	ChannelID string `gorm:"not null;size:36;index:idx_mentions_user,priority:2" json:"channel_id"`
}

type Event struct {
	Cursor    int64  `gorm:"primaryKey;autoIncrement" json:"cursor"`
	Kind      string `gorm:"not null;size:50;index" json:"kind"`
	ChannelID string `gorm:"not null;size:36;index" json:"channel_id"`
	Payload   string `gorm:"not null" json:"payload"`
	CreatedAt int64  `gorm:"not null;index" json:"created_at"`
}

type UserPermission struct {
	ID         uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     string  `gorm:"not null;size:36;index:idx_user_permissions_lookup" json:"user_id"`
	Permission string  `gorm:"not null;size:100" json:"permission"`
	Scope      string  `gorm:"not null;default:*;size:255" json:"scope"`
	GrantedBy  *string `gorm:"size:36" json:"granted_by,omitempty"`
	GrantedAt  int64   `gorm:"not null" json:"granted_at"`
	// AP-1.1 (v=24): expires_at is a SCHEMA-only slot per spec §5
	// (蓝图 auth-permissions.md §1.2 字面 "v1 schema 保留, UI/runtime 不做").
	// 字段保留以备 v2+ 业务化, server 端不读 (HasCapability 不消费).
	ExpiresAt *int64 `gorm:"column:expires_at" json:"expires_at,omitempty"`
	// AP-2 #ap-2 (v=30): revoked_at is the soft-delete sentinel — sweeper
	// goroutine writes this when expires_at < now (forward-only audit, 跟
	// AL-1 #492 state_log + ADM-2.1 #484 admin_actions forward-only 同精神,
	// row 不真删). NULL = active. ListUserPermissions 排除 NOT NULL 行
	// (改 = 改 queries.go WHERE 一处, AP-1 SSOT 同精神).
	RevokedAt *int64 `gorm:"column:revoked_at" json:"revoked_at,omitempty"`
}

type InviteCode struct {
	Code      string  `gorm:"primaryKey;size:128" json:"code"`
	CreatedBy string  `gorm:"not null;size:36;index" json:"created_by"`
	CreatedAt int64   `gorm:"not null" json:"created_at"`
	ExpiresAt *int64  `gorm:"index" json:"expires_at,omitempty"`
	UsedBy    *string `gorm:"size:36;index" json:"used_by,omitempty"`
	UsedAt    *int64  `json:"used_at,omitempty"`
	Note      string  `gorm:"size:500" json:"note"`
}

type MessageReaction struct {
	ID        string `gorm:"primaryKey;size:36" json:"id"`
	MessageID string `gorm:"not null;size:36;index" json:"message_id"`
	UserID    string `gorm:"not null;size:36;index" json:"user_id"`
	Emoji     string `gorm:"not null;size:64" json:"emoji"`
	CreatedAt int64  `gorm:"not null" json:"created_at"`
}

type WorkspaceFile struct {
	ID              string  `gorm:"primaryKey;size:36" json:"id"`
	UserID          string  `gorm:"not null;size:36;index" json:"user_id"`
	ChannelID       string  `gorm:"not null;size:36;index" json:"channel_id"`
	ParentID        *string `gorm:"size:36;index" json:"parent_id,omitempty"`
	Name            string  `gorm:"not null;size:255" json:"name"`
	IsDirectory     bool    `gorm:"not null;default:false" json:"is_directory"`
	MimeType        string  `gorm:"size:255" json:"mime_type"`
	SizeBytes       int64   `gorm:"not null;default:0" json:"size_bytes"`
	Source          string  `gorm:"not null;default:upload;size:50" json:"source"`
	SourceMessageID *string `gorm:"size:36;index" json:"source_message_id,omitempty"`
	CreatedAt       int64   `gorm:"not null" json:"created_at"`
	UpdatedAt       int64   `gorm:"not null" json:"updated_at"`
	// OrgID is the file's organization (CM-3.1). Stamped at INSERT from
	// uploader.OrgID. Column added by migration cm_1_1_organizations.
	OrgID string `gorm:"column:org_id;not null;default:'';size:36;index" json:"-"`
}

type RemoteNode struct {
	ID              string `gorm:"primaryKey;size:36" json:"id"`
	UserID          string `gorm:"not null;size:36;index" json:"user_id"`
	MachineName     string `gorm:"not null;size:255" json:"machine_name"`
	ConnectionToken string `gorm:"not null;uniqueIndex;size:255" json:"-"`
	LastSeenAt      *int64 `gorm:"index" json:"last_seen_at,omitempty"`
	CreatedAt       int64  `gorm:"not null" json:"created_at"`
	// OrgID is the node's organization (CM-3.1). Stamped at INSERT from
	// registrant.OrgID. Column added by migration cm_1_1_organizations.
	OrgID string `gorm:"column:org_id;not null;default:'';size:36;index" json:"-"`
}

type RemoteBinding struct {
	ID        string `gorm:"primaryKey;size:36" json:"id"`
	NodeID    string `gorm:"not null;size:36;index" json:"node_id"`
	ChannelID string `gorm:"not null;size:36;index" json:"channel_id"`
	Path      string `gorm:"not null;size:1000" json:"path"`
	Label     string `gorm:"size:255" json:"label"`
	CreatedAt int64  `gorm:"not null" json:"created_at"`
}

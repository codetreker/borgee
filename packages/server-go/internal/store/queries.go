package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"
	"unicode"

	"borgee-server/internal/idgen"
	"gorm.io/gorm"
)

type ChannelWithCounts struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Topic         string  `json:"topic"`
	Visibility    string  `json:"visibility"`
	CreatedAt     int64   `json:"created_at"`
	CreatedBy     string  `json:"created_by"`
	Type          string  `json:"type"`
	DeletedAt     *int64  `json:"deleted_at,omitempty"`
	ArchivedAt    *int64  `json:"archived_at,omitempty"`
	Position      string  `json:"position"`
	GroupID       *string `json:"group_id"`
	MemberCount   int     `json:"member_count"`
	UnreadCount   int     `json:"unread_count"`
	LastMessageAt *int64  `json:"last_message_at,omitempty"`
	IsMember      bool    `json:"is_member"`
}

type ChannelMemberInfo struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	AvatarURL   string `json:"avatar_url"`
	JoinedAt    int64  `json:"joined_at"`
	Silent      bool   `json:"silent"`
}

type PreviewMessage struct {
	ID          string  `json:"id"`
	Content     string  `json:"content"`
	ContentType string  `json:"content_type"`
	CreatedAt   int64   `json:"created_at"`
	SenderID    string  `json:"sender_id"`
	SenderName  string  `json:"sender_name"`
	ReplyToID   *string `json:"reply_to_id,omitempty"`
}

type DmChannelInfo struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	CreatedAt   int64          `json:"created_at"`
	Peer        DmPeer         `json:"peer"`
	UnreadCount int            `json:"unread_count"`
	LastMessage *DmLastMessage `json:"last_message,omitempty"`
}

type DmPeer struct {
	ID             string `json:"id"`
	DisplayName    string `json:"display_name"`
	AvatarURL      string `json:"avatar_url"`
	Role           string `json:"role"`
	State          string `json:"state,omitempty"`
	Reason         string `json:"reason,omitempty"`
	StateUpdatedAt int64  `json:"state_updated_at,omitempty"`
	Disabled       bool   `json:"-"`
}

type DmLastMessage struct {
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at"`
	SenderID  string `json:"sender_id"`
}

var mentionIDRegex = regexp.MustCompile(`<@([^>]+)>`)

// parseMentionIDs extracts user IDs from <@userId> tokens in content.
func parseMentionIDs(content string) []string {
	matches := mentionIDRegex.FindAllStringSubmatch(content, -1)
	var ids []string
	for _, m := range matches {
		if len(m) > 1 {
			ids = append(ids, m[1])
		}
	}
	return ids
}

// parseMentionNames extracts user IDs from @displayName tokens in content.
func (s *Store) parseMentionNames(content string) []string {
	var ids []string
	// Simple scanner: find @word sequences
	i := 0
	for i < len(content) {
		if content[i] == '@' && (i == 0 || content[i-1] == ' ' || content[i-1] == '\n') {
			j := i + 1
			for j < len(content) {
				r := rune(content[j])
				if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
					j++
				} else {
					break
				}
			}
			name := content[i+1 : j]
			if name != "" {
				if user, err := s.GetUserByDisplayName(name); err == nil {
					ids = append(ids, user.ID)
				}
			}
			i = j
		} else {
			i++
		}
	}
	return ids
}

// dedup returns a deduplicated slice preserving order.
func dedup(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func (s *Store) GetUserByEmail(email string) (*User, error) {
	var user User
	err := s.db.Where("email = ? AND deleted_at IS NULL", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) GetUserByID(id string) (*User, error) {
	var user User
	err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) GetUserByAPIKey(apiKey string) (*User, error) {
	var user User
	err := s.db.Where("api_key = ? AND deleted_at IS NULL", apiKey).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) CreateUser(user *User) error {
	if user.ID == "" {
		user.ID = idgen.NewID()
	}
	if user.CreatedAt == 0 {
		user.CreatedAt = time.Now().UnixMilli()
	}
	return s.db.Create(user).Error
}

// CreateOrgForUser creates a new Organization and links it to the given user
// (CM-1.2). Used by the register path and admin-created humans: 1 person =
// 1 org in v0. The user.OrgID field is updated in-place on success.
//
// Blueprint: docs/architecture/concept-model.md §CM-1.2; concept §1.1.
//
// Atomicity: org row + users.org_id update happen in a single transaction so
// a failure cannot leave a user with an empty org_id (the column is NOT NULL
// DEFAULT '' but app-layer contract is that registered users always have a
// non-empty org_id — verified by tests). If you call this for a user that
// already has org_id != "", the call is a no-op and returns nil.
func (s *Store) CreateOrgForUser(user *User, orgName string) (*Organization, error) {
	if user == nil || user.ID == "" {
		return nil, errors.New("user with ID is required")
	}
	if user.OrgID != "" {
		// Already has an org; idempotent no-op so callers can be tolerant.
		var existing Organization
		if err := s.db.Where("id = ?", user.OrgID).First(&existing).Error; err == nil {
			return &existing, nil
		}
		return nil, nil
	}
	org := &Organization{
		ID:        idgen.NewID(),
		Name:      orgName,
		CreatedAt: time.Now().UnixMilli(),
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(org).Error; err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", user.ID).Update("org_id", org.ID).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	user.OrgID = org.ID
	return org, nil
}

func (s *Store) ListUsers() ([]User, error) {
	var users []User
	err := s.db.Where("deleted_at IS NULL").Find(&users).Error
	return users, err
}

func (s *Store) GetInviteCode(code string) (*InviteCode, error) {
	var ic InviteCode
	err := s.db.Where("code = ?", code).First(&ic).Error
	if err != nil {
		return nil, err
	}
	return &ic, nil
}

func (s *Store) ConsumeInviteCode(code string, userID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var ic InviteCode
		if err := tx.Where("code = ?", code).First(&ic).Error; err != nil {
			return err
		}
		if ic.UsedBy != nil {
			return errors.New("invite code already used")
		}
		now := time.Now().UnixMilli()
		if ic.ExpiresAt != nil && *ic.ExpiresAt < now {
			return errors.New("invite code expired")
		}
		return tx.Model(&InviteCode{}).Where("code = ?", code).Updates(map[string]any{
			"used_by": userID,
			"used_at": now,
		}).Error
	})
}

func (s *Store) ListUserPermissions(userID string) ([]UserPermission, error) {
	var perms []UserPermission
	// AP-2 #ap-2 原则 ⑥: revoked_at IS NOT NULL 行被排除 (sweeper 软删
	// 路径; AP-1 central query pattern, 改 = 改此处一处, HasCapability 路径自动
	// 返 false 对 revoked 行). NULL = active (跟 expires_at NULL = 永久
	// + org_id NULL = 历史 同精神).
	err := s.db.Where("user_id = ? AND revoked_at IS NULL", userID).Find(&perms).Error
	return perms, err
}

func (s *Store) GrantPermission(perm *UserPermission) error {
	if perm.GrantedAt == 0 {
		perm.GrantedAt = time.Now().UnixMilli()
	}
	return s.db.
		Where("user_id = ? AND permission = ? AND scope = ?", perm.UserID, perm.Permission, perm.Scope).
		FirstOrCreate(perm).Error
}

func (s *Store) CreateChannel(ch *Channel) error {
	if ch.ID == "" {
		ch.ID = idgen.NewID()
	}
	if ch.CreatedAt == 0 {
		ch.CreatedAt = time.Now().UnixMilli()
	}
	return s.db.Create(ch).Error
}

func (s *Store) ListChannels() ([]Channel, error) {
	var channels []Channel
	err := s.db.Where("deleted_at IS NULL").Find(&channels).Error
	return channels, err
}

func (s *Store) GetChannelByID(id string) (*Channel, error) {
	var ch Channel
	err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&ch).Error
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

func (s *Store) ListChannelMembers(channelID string) ([]ChannelMember, error) {
	var members []ChannelMember
	err := s.db.Where("channel_id = ?", channelID).Find(&members).Error
	return members, err
}

func (s *Store) AddChannelMember(member *ChannelMember) error {
	if member.JoinedAt == 0 {
		member.JoinedAt = time.Now().UnixMilli()
	}
	// CHN-1.1: stamp org_id_at_join from the user's current org when not
	// explicitly supplied. Audit field; no read path consumes it.
	if member.OrgIDAtJoin == "" && member.UserID != "" {
		var u User
		if err := s.db.Select("org_id, role").Where("id = ?", member.UserID).First(&u).Error; err == nil {
			member.OrgIDAtJoin = u.OrgID
			// CHN-1.1: agent默认 silent (concept-model §1.4 — 同事不主动说话)。
			// Existing rows preserved; only stamps on first-create.
			if u.Role == "agent" && !member.Silent {
				member.Silent = true
			}
		}
	}
	return s.db.
		Where("channel_id = ? AND user_id = ?", member.ChannelID, member.UserID).
		FirstOrCreate(member).Error
}

func (s *Store) RemoveChannelMember(channelID, userID string) error {
	return s.db.Where("channel_id = ? AND user_id = ?", channelID, userID).Delete(&ChannelMember{}).Error
}

func (s *Store) CreateMessage(msg *Message) error {
	if msg.ID == "" {
		msg.ID = idgen.NewID()
	}
	if msg.CreatedAt == 0 {
		msg.CreatedAt = time.Now().UnixMilli()
	}
	return s.db.Create(msg).Error
}

func (s *Store) GetMessageByID(id string) (*Message, error) {
	var msg Message
	err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&msg).Error
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

func (s *Store) CreateEvent(event *Event) error {
	if event.CreatedAt == 0 {
		event.CreatedAt = time.Now().UnixMilli()
	}
	return s.db.Create(event).Error
}

func (s *Store) CreateMention(mention *Mention) error {
	if mention.ID == "" {
		mention.ID = idgen.NewID()
	}
	return s.db.Create(mention).Error
}

func (s *Store) AddUserToPublicChannels(userID string) error {
	var channels []Channel
	if err := s.db.Where("visibility = ? AND deleted_at IS NULL", "public").Find(&channels).Error; err != nil {
		return err
	}
	// Route through AddChannelMember so the agent-default silent stamp
	// (concept-model §1.4) is applied uniformly. CHN-1.3 e2e regression:
	// raw db.FirstOrCreate bypassed the silent fan-out and broke the
	// "agent silent default = true" 约定 in PR #288.
	for _, ch := range channels {
		_ = s.AddChannelMember(&ChannelMember{ChannelID: ch.ID, UserID: userID})
	}
	return nil
}

// GrantDefaultPermissions writes the role's default permission grants. Per
// AP-0 (Phase 1) + AP-0-bis (Phase 2 R3 决议) the rules are:
//
//   - role "member": one row (`*`, `*`) — humans default to full capability,
//     UI bundles will narrow this in AP-2 (no role names).
//   - role "agent":  two rows (`message.send`, `*`) + (`message.read`, `*`) —
//     agents need read to ingest channel context; send is the side-effect axis.
//     R3 决议 #1 (2026-04-28): default capability set is locked at
//     [message.send, message.read]. Owners grant additional capabilities
//     explicitly per blueprint §3.
//   - any other role (incl. "admin"): nothing. Admin authority lives on the
//     /admin-api/* rail behind admin.RequireAdmin (ADM-0.2). users.role='admin'
//     no longer shortcuts the user-rail (ADM-0.2 §4); explicit grants would
//     just shadow nothing.
//
// Blueprint: docs/architecture/auth-permissions.md §AP-0-bis; §3.
func (s *Store) GrantDefaultPermissions(userID string, role string) error {
	var perms []string
	switch role {
	case "member":
		// AP-0: human full-capability default, single (*, *) row.
		perms = []string{"*"}
	case "agent":
		// AP-0-bis: agent minimum default = [message.send, message.read].
		// Order is informational; rows are enforced individually downstream.
		perms = []string{"message.send", "message.read"}
	default:
		return nil
	}
	now := time.Now().UnixMilli()
	for _, p := range perms {
		perm := &UserPermission{UserID: userID, Permission: p, Scope: "*", GrantedAt: now}
		if err := s.GrantPermission(perm); err != nil {
			return err
		}
	}
	return nil
}

// ─── Messages ───────────────────────────────────────────

// MessageWithSender is a Message with the sender's display name attached.
type MessageWithSender struct {
	Message
	SenderName string   `gorm:"column:sender_name" json:"sender_name"`
	Mentions   []string `gorm:"-" json:"mentions"`
}

// ListChannelMessages returns messages for a channel with cursor-based pagination.
// Uses created_at as cursor (before/after are epoch-ms timestamps).
// Returns limit+1 rows to detect has_more; caller trims.
func (s *Store) ListChannelMessages(channelID string, before, after *int64, limit int) ([]MessageWithSender, bool, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	actualLimit := limit + 1

	var msgs []MessageWithSender
	q := s.db.Table("messages m").
		Select("m.*, u.display_name AS sender_name").
		Joins("JOIN users u ON u.id = m.sender_id").
		Where("m.channel_id = ? AND m.deleted_at IS NULL", channelID)

	if after != nil {
		q = q.Where("m.created_at > ?", *after).Order("m.created_at ASC")
	} else if before != nil {
		q = q.Where("m.created_at < ?", *before).Order("m.created_at DESC")
	} else {
		q = q.Order("m.created_at DESC")
	}

	if err := q.Limit(actualLimit).Find(&msgs).Error; err != nil {
		return nil, false, err
	}

	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}

	// For before/default (DESC order), reverse so output is ASC
	if after == nil {
		for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
			msgs[i], msgs[j] = msgs[j], msgs[i]
		}
	}

	// Attach mentions and mask deleted
	s.attachMentions(msgs)
	maskDeletedMessages(msgs)

	return msgs, hasMore, nil
}

// SearchMessages performs a LIKE search on message content in a channel.
func (s *Store) SearchMessages(channelID string, query string, limit int) ([]MessageWithSender, error) {
	if limit <= 0 || limit > 50 {
		limit = 50
	}

	var msgs []MessageWithSender
	err := s.db.Table("messages m").
		Select("m.*, u.display_name AS sender_name").
		Joins("JOIN users u ON u.id = m.sender_id").
		Where("m.channel_id = ? AND m.content LIKE ? AND m.deleted_at IS NULL", channelID, "%"+query+"%").
		Order("m.created_at DESC").
		Limit(limit).
		Find(&msgs).Error
	if err != nil {
		return nil, err
	}

	s.attachMentions(msgs)
	maskDeletedMessages(msgs)
	return msgs, nil
}

// CreateMessageFull creates a message, resolves mentions, inserts mention records and events.
// Returns the complete message with sender_name.
func (s *Store) CreateMessageFull(channelID, senderID, content, contentType string, replyToID *string, clientMentionIDs []string) (*MessageWithSender, error) {
	msgID := idgen.NewID()
	now := time.Now().UnixMilli()

	// Parse <@userId> from content
	parsedIDs := parseMentionIDs(content)

	// Parse @displayName from content
	parsedNameIDs := s.parseMentionNames(content)

	// Merge all mention IDs, deduplicate
	allMentions := dedup(append(append(clientMentionIDs, parsedIDs...), parsedNameIDs...))

	// Validate mentioned user IDs exist
	var validMentions []string
	for _, uid := range allMentions {
		var count int64
		s.db.Model(&User{}).Where("id = ?", uid).Count(&count)
		if count > 0 {
			validMentions = append(validMentions, uid)
		}
	}

	// Get sender display name + org_id (CM-3.1: stamp message org_id from sender).
	var sender User
	senderName := "Unknown"
	if err := s.db.Select("display_name, org_id").Where("id = ?", senderID).First(&sender).Error; err == nil {
		senderName = sender.DisplayName
	}

	// Get channel type for event payload
	var ch Channel
	chType := "channel"
	chName := ""
	if err := s.db.Select("type, name").Where("id = ?", channelID).First(&ch).Error; err == nil {
		if ch.Type != "" {
			chType = ch.Type
		}
		chName = ch.Name
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		msg := &Message{
			ID:          msgID,
			ChannelID:   channelID,
			SenderID:    senderID,
			Content:     content,
			ContentType: contentType,
			ReplyToID:   replyToID,
			CreatedAt:   now,
			OrgID:       sender.OrgID, // CM-3.1
		}
		if err := tx.Create(msg).Error; err != nil {
			return err
		}

		for _, uid := range validMentions {
			m := &Mention{
				ID:        idgen.NewID(),
				MessageID: msgID,
				UserID:    uid,
				ChannelID: channelID,
			}
			if err := tx.Create(m).Error; err != nil {
				return err
			}
		}

		// Write message event
		eventPayload, _ := json.Marshal(map[string]any{
			"id":           msgID,
			"channel_id":   channelID,
			"channel_name": chName,
			"sender_id":    senderID,
			"sender_name":  senderName,
			"content":      content,
			"content_type": contentType,
			"reply_to_id":  replyToID,
			"created_at":   now,
			"mentions":     validMentions,
			"channel_type": chType,
		})
		evt := &Event{Kind: "message", ChannelID: channelID, Payload: string(eventPayload), CreatedAt: now}
		if err := tx.Create(evt).Error; err != nil {
			return err
		}

		// Write mention events
		for _, uid := range validMentions {
			payload, _ := json.Marshal(map[string]any{
				"message":           map[string]any{"id": msgID, "channel_id": channelID, "sender_id": senderID, "sender_name": senderName, "content": content},
				"mentioned_user_id": uid,
			})
			me := &Event{Kind: "mention", ChannelID: channelID, Payload: string(payload), CreatedAt: now}
			if err := tx.Create(me).Error; err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	result := &MessageWithSender{
		Message: Message{
			ID:          msgID,
			ChannelID:   channelID,
			SenderID:    senderID,
			Content:     content,
			ContentType: contentType,
			ReplyToID:   replyToID,
			CreatedAt:   now,
		},
		SenderName: senderName,
		Mentions:   validMentions,
	}
	if result.Mentions == nil {
		result.Mentions = []string{}
	}

	return result, nil
}

// UpdateMessage updates a message's content and sets edited_at.
func (s *Store) UpdateMessage(messageID, content string) (*MessageWithSender, error) {
	now := time.Now().UnixMilli()
	// DM-7.2 原则 ②: SELECT old content + edit_history FIRST so we can
	// append a new history entry. UpdateMessage is the central update path —
	// DM-4 #553 既有 PATCH caller unchanged. AL-1a reason alignment point 18
	// (复用 reasons.Unknown literal 跟 AL-7 SweeperReason / HB-5
	// HeartbeatSweeperReason shared dictionary — DM-7 不另起 reason 字典).
	var existing Message
	if err := s.db.Select("content", "edit_history").
		Where("id = ?", messageID).First(&existing).Error; err != nil {
		return nil, err
	}
	updates := map[string]any{
		"content":   content,
		"edited_at": now,
	}
	// Skip history append when content is unchanged (idempotent
	// PATCH — 原则 ② "重复 PATCH 同 content 不重复入 history").
	if existing.Content != content {
		entry := map[string]any{
			"old_content": existing.Content,
			"ts":          now,
			// AL-1a reason alignment point 18 — 字面 "unknown" matches
			// reasons.Unknown / AL-7 SweeperReason / HB-5.
			"reason": "unknown",
		}
		var arr []map[string]any
		if existing.EditHistory != nil && *existing.EditHistory != "" {
			_ = json.Unmarshal([]byte(*existing.EditHistory), &arr)
		}
		arr = append(arr, entry)
		if jsonBytes, err := json.Marshal(arr); err == nil {
			updates["edit_history"] = string(jsonBytes)
		}
	}
	if err := s.db.Model(&Message{}).Where("id = ?", messageID).
		Updates(updates).Error; err != nil {
		return nil, err
	}

	var msg MessageWithSender
	err := s.db.Table("messages m").
		Select("m.*, u.display_name AS sender_name").
		Joins("JOIN users u ON u.id = m.sender_id").
		Where("m.id = ?", messageID).
		First(&msg).Error
	if err != nil {
		return nil, err
	}

	s.attachMentions([]MessageWithSender{msg})
	return &msg, nil
}

// SoftDeleteMessage marks a message as deleted. Returns the deleted_at timestamp.
// Idempotent: if already deleted, returns the existing deleted_at.
func (s *Store) SoftDeleteMessage(messageID string) (int64, error) {
	now := time.Now().UnixMilli()
	s.db.Model(&Message{}).Where("id = ? AND deleted_at IS NULL", messageID).Update("deleted_at", now)

	var msg Message
	if err := s.db.Select("deleted_at").Where("id = ?", messageID).First(&msg).Error; err != nil {
		return 0, err
	}
	if msg.DeletedAt != nil {
		return *msg.DeletedAt, nil
	}
	return now, nil
}

// CanAccessChannel checks if a user can access a channel.
func (s *Store) CanAccessChannel(channelID, userID string) bool {
	ch, err := s.GetChannelByID(channelID)
	if err != nil {
		return false
	}
	if ch.Visibility != "private" {
		return true
	}
	// Check membership
	var count int64
	s.db.Model(&ChannelMember{}).Where("channel_id = ? AND user_id = ?", channelID, userID).Count(&count)
	if count > 0 {
		return true
	}
	// ADM-0.3: no role short-circuit. Admin-rail channel access goes
	// through admin.RequireAdmin (admin_sessions cookie) and bypasses
	// this gate entirely. Members must be explicit channel members on
	// private channels.
	return false
}

// IsChannelMember checks if a user is a member of a channel.
func (s *Store) IsChannelMember(channelID, userID string) bool {
	var count int64
	s.db.Model(&ChannelMember{}).Where("channel_id = ? AND user_id = ?", channelID, userID).Count(&count)
	return count > 0
}

// OrgStatsRow is one row of the admin stats `by_org` aggregation
// (CM-1.3, blueprint §2 — organizations as data-layer first-class).
type OrgStatsRow struct {
	OrgID        string `json:"org_id"`
	UserCount    int64  `json:"user_count"`
	ChannelCount int64  `json:"channel_count"`
}

// StatsByOrg aggregates user/channel counts grouped by org_id.
// Excludes soft-deleted rows. Empty org_id ('') is folded into a single
// "" bucket — v0 does not backfill, so 历史 rows surface as their own
// group rather than being silently dropped.
func (s *Store) StatsByOrg() ([]OrgStatsRow, error) {
	type userAgg struct {
		OrgID string
		C     int64
	}
	type chanAgg struct {
		OrgID string
		C     int64
	}

	var users []userAgg
	if err := s.db.Model(&User{}).
		Select("org_id, COUNT(*) AS c").
		Where("deleted_at IS NULL").
		Group("org_id").
		Scan(&users).Error; err != nil {
		return nil, err
	}

	var channels []chanAgg
	if err := s.db.Model(&Channel{}).
		Select("org_id, COUNT(*) AS c").
		Where("deleted_at IS NULL").
		Group("org_id").
		Scan(&channels).Error; err != nil {
		return nil, err
	}

	merged := map[string]*OrgStatsRow{}
	for _, u := range users {
		row := merged[u.OrgID]
		if row == nil {
			row = &OrgStatsRow{OrgID: u.OrgID}
			merged[u.OrgID] = row
		}
		row.UserCount = u.C
	}
	for _, c := range channels {
		row := merged[c.OrgID]
		if row == nil {
			row = &OrgStatsRow{OrgID: c.OrgID}
			merged[c.OrgID] = row
		}
		row.ChannelCount = c.C
	}

	out := make([]OrgStatsRow, 0, len(merged))
	for _, r := range merged {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OrgID < out[j].OrgID })
	return out, nil
}

// GetOnlineUsers returns users whose last_seen_at is within the last 5 minutes.
func (s *Store) GetOnlineUsers() ([]User, error) {
	cutoff := time.Now().UnixMilli() - 5*60*1000
	var users []User
	err := s.db.Where("last_seen_at > ? AND deleted_at IS NULL AND disabled = 0", cutoff).Find(&users).Error
	return users, err
}

// UpdateLastSeen updates a user's last_seen_at to now.
func (s *Store) UpdateLastSeen(userID string) error {
	now := time.Now().UnixMilli()
	return s.db.Model(&User{}).Where("id = ?", userID).Update("last_seen_at", now).Error
}

// GetUserByDisplayName finds a user by their display_name.
func (s *Store) GetUserByDisplayName(displayName string) (*User, error) {
	var user User
	err := s.db.Where("display_name = ? AND deleted_at IS NULL", displayName).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ─── Helpers ────────────────────────────────────────────

func (s *Store) attachMentions(msgs []MessageWithSender) {
	for i := range msgs {
		var mentions []Mention
		s.db.Where("message_id = ?", msgs[i].ID).Find(&mentions)
		ids := make([]string, len(mentions))
		for j, m := range mentions {
			ids[j] = m.UserID
		}
		msgs[i].Mentions = ids
	}
}

func maskDeletedMessages(msgs []MessageWithSender) {
	for i := range msgs {
		if msgs[i].DeletedAt != nil {
			msgs[i].Content = ""
		}
	}
}

// ─── Channel queries ──────────────────────────────────

func (s *Store) ListChannelsPublic() ([]ChannelWithCounts, error) {
	var results []ChannelWithCounts
	err := s.db.Raw(`
		SELECT c.*,
			(SELECT COUNT(*) FROM channel_members cm WHERE cm.channel_id = c.id) AS member_count,
			0 AS unread_count,
			(SELECT MAX(m.created_at) FROM messages m WHERE m.channel_id = c.id AND m.deleted_at IS NULL) AS last_message_at,
			0 AS is_member
		FROM channels c
		WHERE c.deleted_at IS NULL AND c.visibility = 'public' AND c.type = 'channel'
		ORDER BY c.position ASC, c.created_at ASC
	`).Scan(&results).Error
	return results, err
}

// ListChannelsWithUnread returns the channels visible to userID:
//   - all channels where the user is a member (any visibility, any org), AND
//   - same-org public channels (so non-members can discover them).
//
// CHN-1.2 锁定: cross-org public channels are NOT visible (蓝图 §2 — 跨 org
// 必须显式邀请). Archived channels (archived_at IS NOT NULL) are filtered
// out unless the user is still a member, so a creator who archives a
// channel can still see it but org peers stop seeing it.
//
// CHN-13 设计 ②: optional `q` filter — 空 q 走原 SQL unchanged;
// q != "" 加 `AND c.name LIKE '%' || ? || '%' COLLATE NOCASE` 子串
// 大小写不敏感. 既有 ordering (position ASC, created_at ASC) 不变.
func (s *Store) ListChannelsWithUnread(userID, q string) ([]ChannelWithCounts, error) {
	var results []ChannelWithCounts
	// CHN-13 设计 ④: 空 q 路径 reuses the existing SQL shape.
	if q == "" {
		err := s.db.Raw(`
			SELECT c.*,
				(SELECT COUNT(*) FROM channel_members cm2 WHERE cm2.channel_id = c.id) AS member_count,
				(SELECT COUNT(*) FROM messages m
					WHERE m.channel_id = c.id AND m.deleted_at IS NULL
					AND m.sender_id != ?
					AND m.created_at > COALESCE((SELECT cm3.last_read_at FROM channel_members cm3 WHERE cm3.channel_id = c.id AND cm3.user_id = ?), 0)
				) AS unread_count,
				(SELECT MAX(m2.created_at) FROM messages m2 WHERE m2.channel_id = c.id AND m2.deleted_at IS NULL) AS last_message_at,
				CASE WHEN cm.user_id IS NOT NULL THEN 1 ELSE 0 END AS is_member
			FROM channels c
			LEFT JOIN channel_members cm ON cm.channel_id = c.id AND cm.user_id = ?
			LEFT JOIN users u ON u.id = ?
			WHERE c.deleted_at IS NULL AND c.type IN ('channel', 'system')
				AND (
					cm.user_id IS NOT NULL
					OR (c.visibility = 'public' AND c.org_id = u.org_id AND c.archived_at IS NULL)
				)
			ORDER BY c.position ASC, c.created_at ASC
		`, userID, userID, userID, userID).Scan(&results).Error
		return results, err
	}
	// CHN-13 设计 ②: q != "" 加 LIKE 子句 (COLLATE NOCASE 大小写不敏感).
	err := s.db.Raw(`
		SELECT c.*,
			(SELECT COUNT(*) FROM channel_members cm2 WHERE cm2.channel_id = c.id) AS member_count,
			(SELECT COUNT(*) FROM messages m
				WHERE m.channel_id = c.id AND m.deleted_at IS NULL
				AND m.sender_id != ?
				AND m.created_at > COALESCE((SELECT cm3.last_read_at FROM channel_members cm3 WHERE cm3.channel_id = c.id AND cm3.user_id = ?), 0)
			) AS unread_count,
			(SELECT MAX(m2.created_at) FROM messages m2 WHERE m2.channel_id = c.id AND m2.deleted_at IS NULL) AS last_message_at,
			CASE WHEN cm.user_id IS NOT NULL THEN 1 ELSE 0 END AS is_member
		FROM channels c
		LEFT JOIN channel_members cm ON cm.channel_id = c.id AND cm.user_id = ?
		LEFT JOIN users u ON u.id = ?
		WHERE c.deleted_at IS NULL AND c.type IN ('channel', 'system')
			AND c.name LIKE '%' || ? || '%' COLLATE NOCASE
			AND (
				cm.user_id IS NOT NULL
				OR (c.visibility = 'public' AND c.org_id = u.org_id AND c.archived_at IS NULL)
			)
		ORDER BY c.position ASC, c.created_at ASC
	`, userID, userID, userID, userID, q).Scan(&results).Error
	return results, err
}

// SetMuteBit toggles a single bit in user_channel_layout.collapsed for
// (user, channel) without disturbing other bits. Returns the new
// collapsed value so handlers can echo it. CHN-7 设计 ① (0 schema 改).
//
// bitMask is normally CHN-7 api.MuteBit (=2); other bits (CHN-3 collapse
// state at bit 0) are preserved. Caller validates membership/DM in
// the handler layer (this function only writes).
func (s *Store) SetMuteBit(userID, channelID string, bitMask int64, set bool) (int64, error) {
	var current int64
	if err := s.db.Raw(`SELECT COALESCE(collapsed, 0)
		FROM user_channel_layout
		WHERE user_id = ? AND channel_id = ?`,
		userID, channelID).Row().Scan(&current); err != nil {
		// no row → start from 0 (the bit will be applied below).
		current = 0
	}
	var next int64
	if set {
		next = current | bitMask
	} else {
		next = current &^ bitMask
	}
	var pos float64
	_ = s.db.Raw(`SELECT COALESCE(position, 0)
		FROM user_channel_layout
		WHERE user_id = ? AND channel_id = ?`,
		userID, channelID).Row().Scan(&pos)
	now := time.Now().UnixMilli()
	if err := s.db.Exec(`INSERT INTO user_channel_layout
		(user_id, channel_id, collapsed, position, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, channel_id) DO UPDATE SET
		  collapsed  = excluded.collapsed,
		  updated_at = excluded.updated_at`,
		userID, channelID, next, pos, now, now).Error; err != nil {
		return 0, err
	}
	return next, nil
}

// SetNotifPrefBits writes the notification preference for (user, channel)
// into user_channel_layout.collapsed bits at the given shift/mask, while
// preserving every other bit. Returns the new collapsed value so handlers
// can echo it. CHN-8 设计 ① + ③.
//
// Caller validates membership/DM in the handler layer.
func (s *Store) SetNotifPrefBits(userID, channelID string, shift, mask, pref int64) (int64, error) {
	var current int64
	if err := s.db.Raw(`SELECT COALESCE(collapsed, 0)
		FROM user_channel_layout
		WHERE user_id = ? AND channel_id = ?`,
		userID, channelID).Row().Scan(&current); err != nil {
		current = 0
	}
	// Clear the pref slot, then OR in the new pref shifted into position.
	next := (current &^ (mask << shift)) | ((pref & mask) << shift)
	var pos float64
	_ = s.db.Raw(`SELECT COALESCE(position, 0)
		FROM user_channel_layout
		WHERE user_id = ? AND channel_id = ?`,
		userID, channelID).Row().Scan(&pos)
	now := time.Now().UnixMilli()
	if err := s.db.Exec(`INSERT INTO user_channel_layout
		(user_id, channel_id, collapsed, position, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, channel_id) DO UPDATE SET
		  collapsed  = excluded.collapsed,
		  updated_at = excluded.updated_at`,
		userID, channelID, next, pos, now, now).Error; err != nil {
		return 0, err
	}
	return next, nil
}

// IsMutedForUser reports whether (user, channel) has the CHN-7 mute bit
// set. Used by DL-4 push notifier (best-effort skip) — not by RT-3
// fan-out / WS frame / messages 表 (设计 ③ mute 不 drop messages).
//
// MuteBit is sourced from internal/api package to avoid a cycle; caller
// must pass the const literal value (= 2, matching the client).
func (s *Store) IsMutedForUser(userID, channelID string, muteBit int64) (bool, error) {
	var collapsed int64
	err := s.db.Raw(`SELECT COALESCE(collapsed, 0)
		FROM user_channel_layout
		WHERE user_id = ? AND channel_id = ?`,
		userID, channelID).Row().Scan(&collapsed)
	if err != nil {
		// no row → not muted (default 0).
		return false, nil
	}
	return collapsed&muteBit != 0, nil
}

// ListArchivedChannelsForUser returns the user's archived channels —
// only channels where the user is a member AND archived_at IS NOT NULL.
// CHN-5 原则 ② owner-only (cm.user_id = ? 跟 ListChannelsWithUnread 同精
// 神 + 跨 org 不可见 跟 CM-3 #208 一致). 无 admin god-mode 路径.
func (s *Store) ListArchivedChannelsForUser(userID string) ([]ChannelWithCounts, error) {
	var results []ChannelWithCounts
	err := s.db.Raw(`
		SELECT c.*,
			(SELECT COUNT(*) FROM channel_members cm2 WHERE cm2.channel_id = c.id) AS member_count,
			0 AS unread_count,
			(SELECT MAX(m2.created_at) FROM messages m2 WHERE m2.channel_id = c.id AND m2.deleted_at IS NULL) AS last_message_at,
			1 AS is_member
		FROM channels c
		INNER JOIN channel_members cm ON cm.channel_id = c.id AND cm.user_id = ?
		WHERE c.deleted_at IS NULL
			AND c.archived_at IS NOT NULL
			AND c.type IN ('channel', 'system')
		ORDER BY c.archived_at DESC
	`, userID).Scan(&results).Error
	return results, err
}

// ListAllArchivedChannelsForAdmin returns every archived channel across
// all orgs — admin-rail readonly 视图 (CHN-5 设计 ④). No PATCH/PUT/DELETE
// path on admin handler — admin god-mode 不挂直接改 (ADM-0 §1.3 红线).
func (s *Store) ListAllArchivedChannelsForAdmin() ([]ChannelWithCounts, error) {
	var results []ChannelWithCounts
	err := s.db.Raw(`
		SELECT c.*,
			(SELECT COUNT(*) FROM channel_members cm2 WHERE cm2.channel_id = c.id) AS member_count,
			0 AS unread_count,
			(SELECT MAX(m2.created_at) FROM messages m2 WHERE m2.channel_id = c.id AND m2.deleted_at IS NULL) AS last_message_at,
			0 AS is_member
		FROM channels c
		WHERE c.deleted_at IS NULL
			AND c.archived_at IS NOT NULL
			AND c.type IN ('channel', 'system')
		ORDER BY c.archived_at DESC
	`).Scan(&results).Error
	return results, err
}

// PinChannelLayout upserts user_channel_layout with the supplied
// pinned position (CHN-6 原则 ③: caller stamps `-(nowMs)` for ASC-asc
// sort). Uses the same upsert pattern as CHN-3.2 PUT /me/layout.
//
// 反向约束 (chn-6-spec.md §0 原则 ②): user_id 必传 (per-user pin); caller
// 在 handler 层走 IsChannelMember + DM reject 红线; 此函数仅写入.
func (s *Store) PinChannelLayout(userID, channelID string, position float64, nowMs int64) error {
	return s.db.Exec(`INSERT INTO user_channel_layout
		(user_id, channel_id, collapsed, position, created_at, updated_at)
		VALUES (?, ?, 0, ?, ?, ?)
		ON CONFLICT(user_id, channel_id) DO UPDATE SET
		  position   = excluded.position,
		  updated_at = excluded.updated_at`,
		userID, channelID, position, nowMs, nowMs).Error
}

// UnpinChannelLayout returns the (user, channel) row to the non-pinned
// section by writing `position = max(positive)+1.0` (跟 CHN-3.3 #415
// client MIN-1.0 单调小数模式互补; new pos always > 0 = unpinned).
// Returns the new position so handler can echo it.
func (s *Store) UnpinChannelLayout(userID, channelID string, nowMs int64) (float64, error) {
	// Find current max non-pinned position for this user. No row → 0.0.
	var maxPos float64
	if err := s.db.Raw(`SELECT COALESCE(MAX(position), 0.0)
		FROM user_channel_layout
		WHERE user_id = ? AND position >= 0`, userID).Row().Scan(&maxPos); err != nil {
		return 0, err
	}
	newPos := maxPos + 1.0
	if err := s.db.Exec(`INSERT INTO user_channel_layout
		(user_id, channel_id, collapsed, position, created_at, updated_at)
		VALUES (?, ?, 0, ?, ?, ?)
		ON CONFLICT(user_id, channel_id) DO UPDATE SET
		  position   = excluded.position,
		  updated_at = excluded.updated_at`,
		userID, channelID, newPos, nowMs, nowMs).Error; err != nil {
		return 0, err
	}
	return newPos, nil
}

// GetNotifPrefForUser returns the current notification preference for
// (user, channel) as a int (0/1/2). Used by DL-4 push notifier (设计 ③
// best-effort skip). No row → returns 0 (NotifPrefAll = 现网行为零变).
func (s *Store) GetNotifPrefForUser(userID, channelID string, shift, mask int64) (int64, error) {
	var collapsed int64
	err := s.db.Raw(`SELECT COALESCE(collapsed, 0)
		FROM user_channel_layout
		WHERE user_id = ? AND channel_id = ?`,
		userID, channelID).Row().Scan(&collapsed)
	if err != nil {
		return 0, nil
	}
	return (collapsed >> shift) & mask, nil
}

// GetCollapsedForUser returns the raw collapsed bitmap for a user/channel
// pair (used by acceptance tests to verify bitmap isolation across CHN-3
// + CHN-7 + CHN-8 bits without coupling to internal/api package consts).
func (s *Store) GetCollapsedForUser(userID, channelID string) (int64, error) {
	var collapsed int64
	err := s.db.Raw(`SELECT COALESCE(collapsed, 0)
		FROM user_channel_layout
		WHERE user_id = ? AND channel_id = ?`,
		userID, channelID).Row().Scan(&collapsed)
	if err != nil {
		return 0, nil
	}
	return collapsed, nil
}

func (s *Store) ListAllChannelsForAdmin(userID string) ([]ChannelWithCounts, error) {
	var results []ChannelWithCounts
	err := s.db.Raw(`
		SELECT c.*,
			(SELECT COUNT(*) FROM channel_members cm2 WHERE cm2.channel_id = c.id) AS member_count,
			(SELECT COUNT(*) FROM messages m
				WHERE m.channel_id = c.id AND m.deleted_at IS NULL
				AND m.sender_id != ?
				AND m.created_at > COALESCE((SELECT cm3.last_read_at FROM channel_members cm3 WHERE cm3.channel_id = c.id AND cm3.user_id = ?), 0)
			) AS unread_count,
			(SELECT MAX(m2.created_at) FROM messages m2 WHERE m2.channel_id = c.id AND m2.deleted_at IS NULL) AS last_message_at,
			CASE WHEN cm.user_id IS NOT NULL THEN 1 ELSE 0 END AS is_member
		FROM channels c
		LEFT JOIN channel_members cm ON cm.channel_id = c.id AND cm.user_id = ?
		WHERE c.deleted_at IS NULL
			AND (c.type = 'channel' OR (c.type = 'system' AND cm.user_id IS NOT NULL))
		ORDER BY c.position ASC, c.created_at ASC
	`, userID, userID, userID).Scan(&results).Error
	return results, err
}

func (s *Store) GetChannelWithCounts(channelID, userID string) (*ChannelWithCounts, error) {
	var result ChannelWithCounts
	err := s.db.Raw(`
		SELECT c.*,
			(SELECT COUNT(*) FROM channel_members cm2 WHERE cm2.channel_id = c.id) AS member_count,
			(SELECT COUNT(*) FROM messages m
				WHERE m.channel_id = c.id AND m.deleted_at IS NULL
				AND m.sender_id != ?
				AND m.created_at > COALESCE((SELECT cm3.last_read_at FROM channel_members cm3 WHERE cm3.channel_id = c.id AND cm3.user_id = ?), 0)
			) AS unread_count,
			(SELECT MAX(m2.created_at) FROM messages m2 WHERE m2.channel_id = c.id AND m2.deleted_at IS NULL) AS last_message_at,
			CASE WHEN cm.user_id IS NOT NULL THEN 1 ELSE 0 END AS is_member
		FROM channels c
		LEFT JOIN channel_members cm ON cm.channel_id = c.id AND cm.user_id = ?
		WHERE c.id = ? AND c.deleted_at IS NULL
	`, userID, userID, userID, channelID).Scan(&result).Error
	if err != nil {
		return nil, err
	}
	if result.ID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	return &result, nil
}

func (s *Store) GetChannelDetail(channelID string) ([]ChannelMemberInfo, error) {
	var members []ChannelMemberInfo
	err := s.db.Raw(`
		SELECT cm.user_id, u.display_name, u.role, u.avatar_url, cm.joined_at, cm.silent
		FROM channel_members cm
		JOIN users u ON u.id = cm.user_id
		WHERE cm.channel_id = ?
		ORDER BY cm.joined_at ASC
	`, channelID).Scan(&members).Error
	return members, err
}

func (s *Store) GetChannelByName(name string) (*Channel, error) {
	var ch Channel
	err := s.db.Where("name = ? AND deleted_at IS NULL", name).First(&ch).Error
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

// GetChannelByNameInOrg looks up a channel by (org_id, name) — the per-org
// uniqueness scope locked by CHN-1.1 (migration v=11). Cross-org same-name
// channels are independent rows and do NOT collide. Excludes soft-deleted.
func (s *Store) GetChannelByNameInOrg(orgID, name string) (*Channel, error) {
	var ch Channel
	err := s.db.Where("org_id = ? AND name = ? AND deleted_at IS NULL", orgID, name).First(&ch).Error
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

// ArchiveChannel sets archived_at = now (CHN-1.2 soft退役). Idempotent — if
// already archived, returns the existing timestamp without overwrite.
func (s *Store) ArchiveChannel(channelID string) (int64, error) {
	var ch Channel
	if err := s.db.Where("id = ? AND deleted_at IS NULL", channelID).First(&ch).Error; err != nil {
		return 0, err
	}
	if ch.ArchivedAt != nil {
		return *ch.ArchivedAt, nil
	}
	now := time.Now().UnixMilli()
	if err := s.db.Model(&Channel{}).Where("id = ?", channelID).Update("archived_at", now).Error; err != nil {
		return 0, err
	}
	return now, nil
}

func (s *Store) GetChannelIncludingDeleted(id string) (*Channel, error) {
	var ch Channel
	err := s.db.Where("id = ?", id).First(&ch).Error
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

func (s *Store) UpdateChannel(id string, updates map[string]any) error {
	return s.db.Model(&Channel{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateChannelDescription is the CHN-14.2 canonical wrapper for description
// edits — SELECT old topic + edit_history → JSON append `{old_content, ts,
// reason='unknown'}` → UPDATE topic + description_edit_history. CHN-10
// #561 owner-only PUT /channels/:id/description path (chn_10_description.
// go::handlePut) 调用此包装代替泛通用 UpdateChannel; 同 owner-only ACL +
// length cap 500 path unchanged.
//
// 原则 (chn-14-spec.md §0):
//   - ② central path: SELECT old → JSON append → UPDATE in one wrapper (反向 grep inline
//     UPDATE channels.*topic 在 chn_10/chn_14 之外 production 0 hit).
//   - ⑤ AL-1a reason alignment point HB-6 #19 — reason='unknown' literal matches
//     DM-7 #558 / AL-7 SweeperReason / HB-5 HeartbeatSweeperReason.
//   - ② idempotent — same-content PUT 不重复入 history (跟 DM-7 同精神).
func (s *Store) UpdateChannelDescription(channelID, newDescription string) error {
	now := time.Now().UnixMilli()
	var existing Channel
	if err := s.db.Select("topic", "description_edit_history").
		Where("id = ?", channelID).First(&existing).Error; err != nil {
		return err
	}
	updates := map[string]any{"topic": newDescription}
	// Skip history append when description is unchanged (idempotent
	// PUT — 原则 ② "重复 PUT 同 description 不重复入 history").
	if existing.Topic != newDescription {
		entry := map[string]any{
			"old_content": existing.Topic,
			"ts":          now,
			// AL-1a reason alignment point HB-6 #19 — 字面 "unknown" matches
			// DM-7 #558 reasons.Unknown / AL-7 SweeperReason / HB-5.
			"reason": "unknown",
		}
		var arr []map[string]any
		if existing.DescriptionEditHistory != nil && *existing.DescriptionEditHistory != "" {
			_ = json.Unmarshal([]byte(*existing.DescriptionEditHistory), &arr)
		}
		arr = append(arr, entry)
		if jsonBytes, err := json.Marshal(arr); err == nil {
			updates["description_edit_history"] = string(jsonBytes)
		}
	}
	return s.db.Model(&Channel{}).Where("id = ?", channelID).Updates(updates).Error
}

// GetChannelDescriptionHistory returns the parsed JSON history array.
// Returns empty slice when description_edit_history is NULL / empty.
func (s *Store) GetChannelDescriptionHistory(channelID string) ([]map[string]any, error) {
	var ch Channel
	if err := s.db.Select("description_edit_history").
		Where("id = ?", channelID).First(&ch).Error; err != nil {
		return nil, err
	}
	var arr []map[string]any
	if ch.DescriptionEditHistory != nil && *ch.DescriptionEditHistory != "" {
		if err := json.Unmarshal([]byte(*ch.DescriptionEditHistory), &arr); err != nil {
			return nil, err
		}
	}
	if arr == nil {
		arr = []map[string]any{}
	}
	return arr, nil
}

func (s *Store) SoftDeleteChannel(id string) error {
	now := time.Now().UnixMilli()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Channel{}).Where("id = ?", id).Update("deleted_at", now).Error; err != nil {
			return err
		}

		var messages []Message
		if err := tx.Select("id").Where("channel_id = ?", id).Find(&messages).Error; err != nil {
			return err
		}
		messageIDs := make([]string, 0, len(messages))
		for _, msg := range messages {
			messageIDs = append(messageIDs, msg.ID)
		}
		if len(messageIDs) > 0 {
			if err := tx.Model(&Message{}).Where("id IN ? AND deleted_at IS NULL", messageIDs).Update("deleted_at", now).Error; err != nil {
				return err
			}
			if err := tx.Where("message_id IN ?", messageIDs).Delete(&MessageReaction{}).Error; err != nil {
				return err
			}
			if err := tx.Where("message_id IN ?", messageIDs).Delete(&Mention{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("channel_id = ?", id).Delete(&ChannelMember{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (s *Store) MarkChannelRead(channelID, userID string) error {
	now := time.Now().UnixMilli()
	return s.db.Model(&ChannelMember{}).
		Where("channel_id = ? AND user_id = ?", channelID, userID).
		Update("last_read_at", now).Error
}

func (s *Store) AddAllUsersToChannel(channelID string) error {
	var users []User
	if err := s.db.Where("deleted_at IS NULL AND disabled = 0").Find(&users).Error; err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	for _, u := range users {
		member := &ChannelMember{ChannelID: channelID, UserID: u.ID, JoinedAt: now}
		s.db.Where("channel_id = ? AND user_id = ?", channelID, u.ID).FirstOrCreate(member)
	}
	return nil
}

func (s *Store) GetPreviewMessages(channelID string, limit int) ([]PreviewMessage, error) {
	cutoff := time.Now().UnixMilli() - 24*60*60*1000
	var msgs []PreviewMessage
	err := s.db.Raw(`
		SELECT m.id, m.content, m.content_type, m.created_at, m.sender_id, u.display_name AS sender_name, m.reply_to_id
		FROM messages m
		JOIN users u ON u.id = m.sender_id
		WHERE m.channel_id = ? AND m.deleted_at IS NULL AND m.created_at > ?
		ORDER BY m.created_at DESC
		LIMIT ?
	`, channelID, cutoff, limit).Scan(&msgs).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

func (s *Store) UpdateChannelPosition(channelID, position string, groupID *string) error {
	updates := map[string]any{"position": position}
	if groupID != nil {
		updates["group_id"] = *groupID
	} else {
		updates["group_id"] = nil
	}
	return s.db.Model(&Channel{}).Where("id = ?", channelID).Updates(updates).Error
}

func (s *Store) GetAdjacentChannelPositions(afterID *string, groupID *string) (before string, after string, err error) {
	if afterID != nil && *afterID != "" {
		var ch Channel
		if err := s.db.Select("position").Where("id = ?", *afterID).First(&ch).Error; err != nil {
			return "", "", err
		}
		before = ch.Position

		q := s.db.Model(&Channel{}).Select("position").
			Where("deleted_at IS NULL AND position > ?", before)
		if groupID != nil {
			q = q.Where("group_id = ?", *groupID)
		} else {
			q = q.Where("group_id IS NULL")
		}
		var next Channel
		if err := q.Order("position ASC").First(&next).Error; err == nil {
			after = next.Position
		}
	} else {
		q := s.db.Model(&Channel{}).Select("position").Where("deleted_at IS NULL")
		if groupID != nil {
			q = q.Where("group_id = ?", *groupID)
		} else {
			q = q.Where("group_id IS NULL")
		}
		var first Channel
		if err := q.Order("position ASC").First(&first).Error; err == nil {
			after = first.Position
		}
	}
	return before, after, nil
}

func (s *Store) GetLastChannelPosition() string {
	var ch Channel
	if err := s.db.Model(&Channel{}).Select("position").
		Where("deleted_at IS NULL AND type = 'channel'").
		Order("position DESC").First(&ch).Error; err != nil {
		return ""
	}
	return ch.Position
}

func (s *Store) GrantCreatorPermissions(creatorID, creatorRole, channelID string, ownerIDIfAgent *string) error {
	scope := fmt.Sprintf("channel:%s", channelID)
	now := time.Now().UnixMilli()
	perms := []string{"channel.delete", "channel.manage_members", "channel.manage_visibility"}

	targetID := creatorID
	if creatorRole == "agent" && ownerIDIfAgent != nil {
		targetID = *ownerIDIfAgent
	}

	for _, p := range perms {
		perm := &UserPermission{
			UserID:     targetID,
			Permission: p,
			Scope:      scope,
			GrantedBy:  &creatorID,
			GrantedAt:  now,
		}
		if err := s.GrantPermission(perm); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeletePermissionsByScope(scope string) error {
	return s.db.Where("scope = ?", scope).Delete(&UserPermission{}).Error
}

// ─── Channel Groups ───────────────────────────────────

func (s *Store) CreateChannelGroup(group *ChannelGroup) error {
	if group.ID == "" {
		group.ID = idgen.NewID()
	}
	if group.CreatedAt == 0 {
		group.CreatedAt = time.Now().UnixMilli()
	}
	return s.db.Create(group).Error
}

func (s *Store) GetChannelGroup(id string) (*ChannelGroup, error) {
	var g ChannelGroup
	err := s.db.Where("id = ?", id).First(&g).Error
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *Store) UpdateChannelGroup(id, name string) error {
	return s.db.Model(&ChannelGroup{}).Where("id = ?", id).Update("name", name).Error
}

func (s *Store) DeleteChannelGroup(id string) error {
	return s.db.Where("id = ?", id).Delete(&ChannelGroup{}).Error
}

func (s *Store) ListChannelGroups() ([]ChannelGroup, error) {
	var groups []ChannelGroup
	err := s.db.Order("position ASC, created_at ASC").Find(&groups).Error
	return groups, err
}

func (s *Store) GetLastGroupPosition() string {
	var g ChannelGroup
	if err := s.db.Model(&ChannelGroup{}).Select("position").
		Order("position DESC").First(&g).Error; err != nil {
		return ""
	}
	return g.Position
}

func (s *Store) GetAdjacentGroupPositions(afterID *string) (before string, after string, err error) {
	if afterID != nil && *afterID != "" {
		var g ChannelGroup
		if err := s.db.Select("position").Where("id = ?", *afterID).First(&g).Error; err != nil {
			return "", "", err
		}
		before = g.Position

		var next ChannelGroup
		if err := s.db.Model(&ChannelGroup{}).Select("position").
			Where("position > ?", before).Order("position ASC").First(&next).Error; err == nil {
			after = next.Position
		}
	} else {
		var first ChannelGroup
		if err := s.db.Model(&ChannelGroup{}).Select("position").
			Order("position ASC").First(&first).Error; err == nil {
			after = first.Position
		}
	}
	return before, after, nil
}

func (s *Store) UpdateGroupPosition(groupID, position string) error {
	return s.db.Model(&ChannelGroup{}).Where("id = ?", groupID).Update("position", position).Error
}

func (s *Store) UngroupChannels(groupID string) ([]string, error) {
	var channels []Channel
	if err := s.db.Select("id").Where("group_id = ? AND deleted_at IS NULL", groupID).Find(&channels).Error; err != nil {
		return nil, err
	}
	ids := make([]string, len(channels))
	for i, ch := range channels {
		ids[i] = ch.ID
	}
	if len(ids) > 0 {
		s.db.Model(&Channel{}).Where("group_id = ?", groupID).Update("group_id", nil)
	}
	return ids, nil
}

// ─── DM queries ───────────────────────────────────────

func (s *Store) CreateDmChannel(userID1, userID2 string) (*Channel, error) {
	ids := []string{userID1, userID2}
	sort.Strings(ids)
	name := fmt.Sprintf("dm:%s_%s", ids[0], ids[1])

	existing, err := s.GetChannelByName(name)
	if err == nil {
		return existing, nil
	}

	ch := &Channel{
		ID:         idgen.NewID(),
		Name:       name,
		Type:       "dm",
		Visibility: "private",
		CreatedBy:  userID1,
		CreatedAt:  time.Now().UnixMilli(),
		Position:   GenerateInitialRank(),
	}
	if err := s.db.Create(ch).Error; err != nil {
		if existing, err2 := s.GetChannelByName(name); err2 == nil {
			return existing, nil
		}
		return nil, err
	}

	now := time.Now().UnixMilli()
	for _, uid := range ids {
		s.db.Create(&ChannelMember{ChannelID: ch.ID, UserID: uid, JoinedAt: now})
	}

	return ch, nil
}

func (s *Store) ListDmChannelsForUser(userID string) ([]DmChannelInfo, error) {
	type rawRow struct {
		ID             string  `gorm:"column:id"`
		Name           string  `gorm:"column:name"`
		CreatedAt      int64   `gorm:"column:created_at"`
		PeerID         string  `gorm:"column:peer_id"`
		PeerName       string  `gorm:"column:peer_name"`
		PeerAvatar     string  `gorm:"column:peer_avatar"`
		PeerRole       string  `gorm:"column:peer_role"`
		PeerDisabled   bool    `gorm:"column:peer_disabled"`
		UnreadCount    int     `gorm:"column:unread_count"`
		LastMsgContent *string `gorm:"column:last_msg_content"`
		LastMsgAt      *int64  `gorm:"column:last_msg_at"`
		LastMsgSender  *string `gorm:"column:last_msg_sender"`
	}

	var rows []rawRow
	err := s.db.Raw(`
		SELECT c.id, c.name, c.created_at,
			peer.id AS peer_id, peer.display_name AS peer_name, peer.avatar_url AS peer_avatar, peer.role AS peer_role, peer.disabled AS peer_disabled,
			(SELECT COUNT(*) FROM messages m
				WHERE m.channel_id = c.id AND m.deleted_at IS NULL
				AND m.sender_id != ?
				AND m.created_at > COALESCE(cm.last_read_at, 0)
			) AS unread_count,
			lm.content AS last_msg_content, lm.created_at AS last_msg_at, lm.sender_id AS last_msg_sender
		FROM channels c
		JOIN channel_members cm ON cm.channel_id = c.id AND cm.user_id = ?
		JOIN channel_members cm2 ON cm2.channel_id = c.id AND cm2.user_id != ?
		JOIN users peer ON peer.id = cm2.user_id
		LEFT JOIN messages lm ON lm.id = (
			SELECT m2.id FROM messages m2 WHERE m2.channel_id = c.id AND m2.deleted_at IS NULL ORDER BY m2.created_at DESC LIMIT 1
		)
		WHERE c.type = 'dm' AND c.deleted_at IS NULL
		ORDER BY COALESCE(lm.created_at, c.created_at) DESC
	`, userID, userID, userID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]DmChannelInfo, len(rows))
	for i, r := range rows {
		info := DmChannelInfo{
			ID:          r.ID,
			Name:        r.Name,
			CreatedAt:   r.CreatedAt,
			Peer:        DmPeer{ID: r.PeerID, DisplayName: r.PeerName, AvatarURL: r.PeerAvatar, Role: r.PeerRole, Disabled: r.PeerDisabled},
			UnreadCount: r.UnreadCount,
		}
		if r.LastMsgContent != nil && r.LastMsgAt != nil && r.LastMsgSender != nil {
			info.LastMessage = &DmLastMessage{Content: *r.LastMsgContent, CreatedAt: *r.LastMsgAt, SenderID: *r.LastMsgSender}
		}
		result[i] = info
	}
	return result, nil
}

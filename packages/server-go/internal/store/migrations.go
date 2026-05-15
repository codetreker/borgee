package store

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"borgee-server/internal/migrations"
)

func (s *Store) Migrate() error {
	// Disable FK constraints during migration to avoid issues with table recreation
	if err := s.execMigrationSQL("disable foreign keys", "PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}

	if err := s.createSchema(); err != nil {
		s.db.Exec("PRAGMA foreign_keys = ON")
		return err
	}

	if err := s.applyColumnMigrations(); err != nil {
		s.db.Exec("PRAGMA foreign_keys = ON")
		return err
	}

	if err := s.createSchemaIndexes(); err != nil {
		s.db.Exec("PRAGMA foreign_keys = ON")
		return err
	}

	// Re-enable FK constraints after migration
	if err := s.execMigrationSQL("enable foreign keys", "PRAGMA foreign_keys = ON"); err != nil {
		return err
	}

	if err := s.backfillDefaultPermissions(); err != nil {
		return fmt.Errorf("backfill permissions: %w", err)
	}

	if err := s.backfillCreatorChannelPermissions(); err != nil {
		return fmt.Errorf("backfill creator perms: %w", err)
	}

	if err := s.backfillAgentOwnerID(); err != nil {
		return fmt.Errorf("backfill agent owner: %w", err)
	}

	if err := s.backfillPositions(); err != nil {
		return fmt.Errorf("backfill positions: %w", err)
	}

	if err := s.cleanupDuplicateDMs(); err != nil {
		return fmt.Errorf("cleanup duplicate DMs: %w", err)
	}

	if err := s.cleanupDMExtraMembers(); err != nil {
		return fmt.Errorf("cleanup DM members: %w", err)
	}

	// Run forward-only registry migrations (CM-1.1+) after legacy createSchema
	// so columns like users.org_id exist for app-layer code (CM-1.2). The
	// engine is idempotent — already-applied versions are skipped via
	// schema_migrations. cmd/migrate also runs this after Migrate(); having
	// it here keeps in-process boot (cmd/collab) and tests on the same path.
	if err := migrations.Default(s.db).Run(0); err != nil {
		return fmt.Errorf("forward-only migrations: %w", err)
	}

	return nil
}

func (s *Store) createSchema() error {
	return s.execMigrationSQL("create schema", `
CREATE TABLE IF NOT EXISTS channels (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL UNIQUE,
  topic       TEXT DEFAULT '',
  visibility  TEXT DEFAULT 'public' CHECK(visibility IN ('public','private')),
  created_at  INTEGER NOT NULL,
  created_by  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
  id           TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  role         TEXT DEFAULT 'member',
  avatar_url   TEXT,
  api_key      TEXT UNIQUE,
  created_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS messages (
  id            TEXT PRIMARY KEY,
  channel_id    TEXT NOT NULL REFERENCES channels(id),
  sender_id     TEXT NOT NULL REFERENCES users(id),
  content       TEXT NOT NULL,
  content_type  TEXT DEFAULT 'text',
  reply_to_id   TEXT REFERENCES messages(id),
  created_at    INTEGER NOT NULL,
  edited_at     INTEGER
);

CREATE TABLE IF NOT EXISTS channel_members (
  channel_id    TEXT NOT NULL REFERENCES channels(id),
  user_id       TEXT NOT NULL REFERENCES users(id),
  joined_at     INTEGER NOT NULL,
  last_read_at  INTEGER,
  require_mention_policy TEXT NOT NULL DEFAULT 'inherit' CHECK (require_mention_policy IN ('inherit','on','off')),
  PRIMARY KEY (channel_id, user_id)
);

CREATE TABLE IF NOT EXISTS mentions (
  id          TEXT PRIMARY KEY,
  message_id  TEXT NOT NULL REFERENCES messages(id),
  user_id     TEXT NOT NULL REFERENCES users(id),
  channel_id  TEXT NOT NULL REFERENCES channels(id)
);

CREATE TABLE IF NOT EXISTS events (
  cursor      INTEGER PRIMARY KEY AUTOINCREMENT,
  kind        TEXT NOT NULL,
  channel_id  TEXT NOT NULL,
  payload     TEXT NOT NULL,
  created_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS user_permissions (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  permission  TEXT NOT NULL,
  scope       TEXT NOT NULL DEFAULT '*',
  granted_by  TEXT REFERENCES users(id),
  granted_at  INTEGER NOT NULL,
  UNIQUE(user_id, permission, scope)
);

CREATE TABLE IF NOT EXISTS invite_codes (
  code        TEXT PRIMARY KEY,
  created_by  TEXT NOT NULL,
  created_at  INTEGER NOT NULL,
  expires_at  INTEGER,
  used_by     TEXT REFERENCES users(id),
  used_at     INTEGER,
  note        TEXT
);

CREATE TABLE IF NOT EXISTS message_reactions (
  id          TEXT PRIMARY KEY,
  message_id  TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  user_id     TEXT NOT NULL REFERENCES users(id),
  emoji       TEXT NOT NULL,
  created_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS workspace_files (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  channel_id TEXT NOT NULL REFERENCES channels(id),
  parent_id TEXT REFERENCES workspace_files(id),
  name TEXT NOT NULL,
  is_directory INTEGER NOT NULL DEFAULT 0,
  mime_type TEXT,
  size_bytes INTEGER DEFAULT 0,
  source TEXT DEFAULT 'upload',
  source_message_id TEXT,
  created_at INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL DEFAULT 0,
  UNIQUE(user_id, channel_id, parent_id, name)
);

CREATE TABLE IF NOT EXISTS remote_nodes (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  machine_name TEXT NOT NULL,
  connection_token TEXT NOT NULL UNIQUE,
  last_seen_at INTEGER,
  created_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS remote_bindings (
  id TEXT PRIMARY KEY,
  node_id TEXT NOT NULL REFERENCES remote_nodes(id) ON DELETE CASCADE,
  channel_id TEXT NOT NULL REFERENCES channels(id),
  path TEXT NOT NULL,
  label TEXT,
  created_at INTEGER NOT NULL DEFAULT 0,
  UNIQUE(node_id, channel_id, path)
);

CREATE TABLE IF NOT EXISTS channel_groups (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  position    TEXT NOT NULL,
  created_by  TEXT NOT NULL REFERENCES users(id),
  created_at  INTEGER NOT NULL
);
`)
}

func (s *Store) applyColumnMigrations() error {
	columns := []struct {
		table string
		name  string
		ddl   string
		label string
	}{
		{"channel_members", "last_read_at", "ALTER TABLE channel_members ADD COLUMN last_read_at INTEGER", "add channel_members.last_read_at"},
		{"users", "email", "ALTER TABLE users ADD COLUMN email TEXT", "add users.email"},
		{"users", "password_hash", "ALTER TABLE users ADD COLUMN password_hash TEXT", "add users.password_hash"},
		{"users", "last_seen_at", "ALTER TABLE users ADD COLUMN last_seen_at INTEGER", "add users.last_seen_at"},
		{"users", "require_mention", "ALTER TABLE users ADD COLUMN require_mention INTEGER DEFAULT 1", "add users.require_mention"},
		{"channels", "type", "ALTER TABLE channels ADD COLUMN type TEXT DEFAULT 'channel'", "add channels.type"},
		{"channels", "visibility", "ALTER TABLE channels ADD COLUMN visibility TEXT DEFAULT 'public'", "add channels.visibility"},
		{"channels", "deleted_at", "ALTER TABLE channels ADD COLUMN deleted_at INTEGER", "add channels.deleted_at"},
		{"users", "owner_id", "ALTER TABLE users ADD COLUMN owner_id TEXT REFERENCES users(id)", "add users.owner_id"},
		{"users", "deleted_at", "ALTER TABLE users ADD COLUMN deleted_at INTEGER", "add users.deleted_at"},
		{"users", "disabled", "ALTER TABLE users ADD COLUMN disabled INTEGER DEFAULT 0", "add users.disabled"},
		{"messages", "deleted_at", "ALTER TABLE messages ADD COLUMN deleted_at INTEGER", "add messages.deleted_at"},
		{"channels", "position", "ALTER TABLE channels ADD COLUMN position TEXT DEFAULT '0|aaaaaa'", "add channels.position"},
		{"channels", "group_id", "ALTER TABLE channels ADD COLUMN group_id TEXT REFERENCES channel_groups(id) ON DELETE SET NULL", "add channels.group_id"},
		{"channel_members", "require_mention_policy", "ALTER TABLE channel_members ADD COLUMN require_mention_policy TEXT NOT NULL DEFAULT 'inherit' CHECK (require_mention_policy IN ('inherit','on','off'))", "add channel_members.require_mention_policy"},
	}

	for _, col := range columns {
		exists, err := s.columnExists(col.table, col.name)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if err := s.execMigrationSQL(col.label, col.ddl); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) createSchemaIndexes() error {
	return s.execMigrationSQL("create schema indexes", `
CREATE INDEX IF NOT EXISTS idx_messages_channel_time ON messages(channel_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender_id);
CREATE INDEX IF NOT EXISTS idx_mentions_user ON mentions(user_id, channel_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE email IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_owner_id ON users(owner_id);
CREATE INDEX IF NOT EXISTS idx_user_permissions_user ON user_permissions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_permissions_lookup ON user_permissions(user_id, permission, scope);
CREATE INDEX IF NOT EXISTS idx_invite_codes_used ON invite_codes(used_by);
CREATE UNIQUE INDEX IF NOT EXISTS idx_reactions_unique ON message_reactions(message_id, user_id, emoji);
CREATE INDEX IF NOT EXISTS idx_reactions_message ON message_reactions(message_id);
CREATE INDEX IF NOT EXISTS idx_workspace_files_user_channel ON workspace_files(user_id, channel_id);
CREATE INDEX IF NOT EXISTS idx_workspace_files_parent ON workspace_files(parent_id);
CREATE INDEX IF NOT EXISTS idx_remote_nodes_user ON remote_nodes(user_id);
CREATE INDEX IF NOT EXISTS idx_channels_position ON channels(position);
CREATE INDEX IF NOT EXISTS idx_channel_groups_position ON channel_groups(position);
CREATE INDEX IF NOT EXISTS idx_channels_group ON channels(group_id);
`)
}

func (s *Store) columnExists(table, name string) (bool, error) {
	var cols []struct {
		Name string `gorm:"column:name"`
	}
	if err := s.db.Raw("PRAGMA table_info(" + table + ")").Scan(&cols).Error; err != nil {
		return false, fmt.Errorf("inspect %s columns: %w", table, err)
	}
	for _, col := range cols {
		if col.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) execMigrationSQL(label, sql string) error {
	if err := s.db.Exec(sql).Error; err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	return nil
}

func (s *Store) backfillDefaultPermissions() error {
	// AP-0 (Phase 1) + AP-0-bis (Phase 2 R3 决议 #1, 2026-04-28): humans default
	// to a single (*, *) row; agents to (message.send, *) + (message.read, *).
	// Older v0 dev DBs may carry the legacy
	// (channel.create / message.send / agent.manage) triple — we leave those
	// rows alone (UNIQUE-guarded FirstOrCreate is additive only) so the boot
	// path stays "delete db and rebuild" friendly without surprise reductions.
	//
	// Production-side legacy backfill happens via the versioned migration
	// engine (migrations/ap_0_bis_message_read.go v=8); this loop is the
	// legacy `Store.Migrate()` boot path mirror — kept consistent so a brand-
	// new v0 dev rebuild matches what production has after migration v=8.
	memberPerms := []string{"*"}
	agentPerms := []string{"message.send", "message.read"}

	var members []User
	s.db.Where("role = ? AND deleted_at IS NULL", "member").Find(&members)

	now := time.Now().UnixMilli()
	for _, u := range members {
		for _, p := range memberPerms {
			perm := UserPermission{UserID: u.ID, Permission: p, Scope: "*", GrantedAt: now}
			s.db.Where("user_id = ? AND permission = ? AND scope = ?", u.ID, p, "*").FirstOrCreate(&perm)
		}
	}

	var agents []User
	s.db.Where("role = ? AND deleted_at IS NULL", "agent").Find(&agents)

	for _, u := range agents {
		for _, p := range agentPerms {
			perm := UserPermission{UserID: u.ID, Permission: p, Scope: "*", GrantedAt: now}
			s.db.Where("user_id = ? AND permission = ? AND scope = ?", u.ID, p, "*").FirstOrCreate(&perm)
		}
	}

	return nil
}

func (s *Store) backfillCreatorChannelPermissions() error {
	var channels []Channel
	s.db.Where("deleted_at IS NULL").Find(&channels)

	now := time.Now().UnixMilli()
	for _, ch := range channels {
		for _, p := range []string{"channel.delete", "channel.manage_members", "channel.manage_visibility"} {
			scope := "channel:" + ch.ID
			perm := UserPermission{UserID: ch.CreatedBy, Permission: p, Scope: scope, GrantedAt: now}
			s.db.Where("user_id = ? AND permission = ? AND scope = ?", ch.CreatedBy, p, scope).FirstOrCreate(&perm)
		}
	}

	return nil
}

func (s *Store) backfillAgentOwnerID() error {
	return nil
}

func (s *Store) backfillPositions() error {
	var channels []Channel
	s.db.Where("deleted_at IS NULL AND (position = ? OR position = ?)", "0|aaaaaa", "").Find(&channels)

	if len(channels) == 0 {
		return nil
	}

	items := make([]RankItem, len(channels))
	for i, ch := range channels {
		items[i] = RankItem{ID: ch.ID, Rank: ch.Position}
	}

	results := Rebalance(items)
	for _, r := range results {
		s.db.Model(&Channel{}).Where("id = ?", r.ID).Update("position", r.NewRank)
	}

	return nil
}

func (s *Store) cleanupDuplicateDMs() error {
	var dmChannels []Channel
	s.db.Where("type = ? AND deleted_at IS NULL", "dm").Order("created_at ASC").Find(&dmChannels)

	seen := map[string]string{}
	for _, ch := range dmChannels {
		normalizedName := normalizeDMName(ch.Name)
		if _, exists := seen[normalizedName]; exists {
			s.db.Where("channel_id = ?", ch.ID).Delete(&Message{})
			s.db.Where("channel_id = ?", ch.ID).Delete(&ChannelMember{})
			s.db.Where("channel_id = ?", ch.ID).Delete(&Mention{})
			s.db.Where("channel_id = ?", ch.ID).Delete(&Event{})
			now := time.Now().UnixMilli()
			s.db.Model(&Channel{}).Where("id = ?", ch.ID).Update("deleted_at", now)
		} else {
			seen[normalizedName] = ch.ID
		}
	}

	return nil
}

func (s *Store) cleanupDMExtraMembers() error {
	var dmChannels []Channel
	s.db.Where("type = ? AND deleted_at IS NULL", "dm").Find(&dmChannels)

	for _, ch := range dmChannels {
		uids := parseDMUserIDs(ch.Name)
		if len(uids) != 2 {
			continue
		}

		allowed := map[string]bool{uids[0]: true, uids[1]: true}

		var members []ChannelMember
		s.db.Where("channel_id = ?", ch.ID).Find(&members)

		for _, m := range members {
			if !allowed[m.UserID] {
				s.db.Where("channel_id = ? AND user_id = ?", ch.ID, m.UserID).Delete(&ChannelMember{})
			}
		}
	}

	return nil
}

func normalizeDMName(name string) string {
	parts := parseDMUserIDs(name)
	if len(parts) != 2 {
		return name
	}
	sort.Strings(parts)
	return "dm:" + parts[0] + "_" + parts[1]
}

func parseDMUserIDs(name string) []string {
	if !strings.HasPrefix(name, "dm:") {
		return nil
	}
	rest := strings.TrimPrefix(name, "dm:")
	parts := strings.SplitN(rest, "_", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil
	}
	return parts
}

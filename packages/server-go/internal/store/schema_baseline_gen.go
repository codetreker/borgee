// Code generated from internal/store/testdata/schema_golden.json by
// schema_baseline_gen_test.go (TestGenerateBaseline, GEN_BASELINE=1). DO NOT EDIT.
//
// This is the one-time re-baselined schema: the verbatim sqlite_master `sql`
// text of every non-auto object (tables, real indexes, view, triggers, FTS5
// virtual table) as it stood after the legacy baseline + all forward-only
// migrations. Auto objects (sqlite_sequence, FTS shadow tables,
// sqlite_autoindex_*) are omitted — sqlite materializes them automatically.
package store

// schemaBaselineStatements holds the consolidated CREATE statements in
// dependency order: tables, then the FTS5 virtual table, then indexes, then
// the view, then triggers. Each entry is the verbatim golden `sql`; the exec
// path (withIfNotExists) injects IF NOT EXISTS so re-running on an existing DB
// is a no-op while the text here stays byte-identical to the golden for AC-1.
var schemaBaselineStatements = []string{
	`CREATE TABLE _migrations_marker (
  version INTEGER PRIMARY KEY,
  note    TEXT
)`,
	`CREATE TABLE admin_sessions (
  token       TEXT PRIMARY KEY,
  admin_id    TEXT NOT NULL,
  created_at  INTEGER NOT NULL,
  expires_at  INTEGER NOT NULL
)`,
	`CREATE TABLE admins (
  id            TEXT PRIMARY KEY,
  login         TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at    INTEGER NOT NULL
)`,
	`CREATE TABLE agent_configs (
  agent_id       TEXT    NOT NULL,
  schema_version INTEGER NOT NULL,
  blob           TEXT    NOT NULL,
  created_at     INTEGER NOT NULL,
  updated_at     INTEGER NOT NULL,
  PRIMARY KEY (agent_id)
)`,
	`CREATE TABLE agent_invitations (
  id           TEXT PRIMARY KEY,
  channel_id   TEXT NOT NULL,
  agent_id     TEXT NOT NULL,
  requested_by TEXT NOT NULL,
  state        TEXT NOT NULL DEFAULT 'pending'
                 CHECK (state IN ('pending','approved','rejected','expired')),
  created_at   INTEGER NOT NULL,
  decided_at   INTEGER,
  expires_at   INTEGER
)`,
	`CREATE TABLE agent_runtimes (
  id                 TEXT    PRIMARY KEY,
  agent_id           TEXT    NOT NULL UNIQUE,
  endpoint_url       TEXT    NOT NULL,
  process_kind       TEXT    NOT NULL CHECK (process_kind IN ('openclaw','hermes')),
  status             TEXT    NOT NULL CHECK (status IN ('registered','running','stopped','error')),
  last_error_reason  TEXT,
  last_heartbeat_at  INTEGER,
  created_at         INTEGER NOT NULL,
  updated_at         INTEGER NOT NULL
)`,
	`CREATE TABLE agent_state_log (
  id          INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  agent_id    TEXT    NOT NULL,
  from_state  TEXT    NOT NULL,
  to_state    TEXT    NOT NULL,
  reason      TEXT    NOT NULL DEFAULT '',
  task_id     TEXT    NOT NULL DEFAULT '',
  ts          INTEGER NOT NULL
, archived_at INTEGER)`,
	`CREATE TABLE agent_status (
  agent_id              TEXT    PRIMARY KEY,
  state                 TEXT    NOT NULL CHECK (state IN ('busy','idle')),
  last_task_id          TEXT,
  last_task_started_at  INTEGER,
  last_task_finished_at INTEGER,
  created_at            INTEGER NOT NULL,
  updated_at            INTEGER NOT NULL
)`,
	`CREATE TABLE anchor_comments (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  anchor_id   TEXT    NOT NULL,
  body        TEXT    NOT NULL DEFAULT '',
  author_kind TEXT    NOT NULL CHECK (author_kind IN ('agent','human')),
  author_id   TEXT    NOT NULL,
  created_at  INTEGER NOT NULL
)`,
	`CREATE TABLE artifact_anchors (
  id                  TEXT    PRIMARY KEY,
  artifact_id         TEXT    NOT NULL,
  artifact_version_id INTEGER NOT NULL,
  start_offset        INTEGER NOT NULL,
  end_offset          INTEGER NOT NULL,
  created_by          TEXT    NOT NULL,
  created_at          INTEGER NOT NULL,
  resolved_at         INTEGER,
  CHECK (end_offset >= start_offset)
)`,
	`CREATE TABLE artifact_iterations (
  id                          TEXT    PRIMARY KEY,
  artifact_id                 TEXT    NOT NULL,
  requested_by                TEXT    NOT NULL,
  intent_text                 TEXT    NOT NULL,
  target_agent_id             TEXT    NOT NULL,
  state                       TEXT    NOT NULL CHECK (state IN ('pending','running','completed','failed')),
  created_artifact_version_id INTEGER,
  error_reason                TEXT,
  created_at                  INTEGER NOT NULL,
  completed_at                INTEGER
)`,
	`CREATE TABLE artifact_versions (
  id                       INTEGER PRIMARY KEY AUTOINCREMENT,
  artifact_id              TEXT    NOT NULL,
  version                  INTEGER NOT NULL,
  body                     TEXT    NOT NULL DEFAULT '',
  committer_kind           TEXT    NOT NULL CHECK (committer_kind IN ('agent','human')),
  committer_id             TEXT    NOT NULL,
  created_at               INTEGER NOT NULL,
  rolled_back_from_version INTEGER,
  UNIQUE(artifact_id, version)
)`,
	`CREATE TABLE "artifacts" (
  id                  TEXT    PRIMARY KEY,
  channel_id          TEXT    NOT NULL,
  type                TEXT    NOT NULL CHECK (type IN ('markdown','code','image_link','video_link','pdf_link')),
  title               TEXT    NOT NULL,
  body                TEXT    NOT NULL DEFAULT '',
  current_version     INTEGER NOT NULL DEFAULT 1,
  created_at          INTEGER NOT NULL,
  archived_at         INTEGER,
  lock_holder_user_id TEXT,
  lock_acquired_at    INTEGER,
  preview_url         TEXT
, thumbnail_url TEXT)`,
	`CREATE TABLE "audit_events" (
  id              TEXT    NOT NULL PRIMARY KEY,
  actor_id        TEXT    NOT NULL,
  target_user_id  TEXT    NOT NULL,
  action          TEXT    NOT NULL CHECK (action IN ('delete_channel','suspend_user','change_role','reset_password','start_impersonation','permission_expired','plugin_connect','plugin_disconnect','plugin_reconnect','plugin_cold_start','plugin_heartbeat_timeout','audit_retention_override')),
  metadata        TEXT    NOT NULL DEFAULT '',
  created_at      INTEGER NOT NULL,
  archived_at     INTEGER
)`,
	`CREATE TABLE channel_events (
  lex_id          TEXT    NOT NULL PRIMARY KEY,
  channel_id      TEXT    NOT NULL,
  kind            TEXT    NOT NULL,
  payload         TEXT    NOT NULL DEFAULT '',
  created_at      INTEGER NOT NULL,
  retention_days  INTEGER
)`,
	`CREATE TABLE channel_groups (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  position    TEXT NOT NULL,
  created_by  TEXT NOT NULL REFERENCES users(id),
  created_at  INTEGER NOT NULL
)`,
	`CREATE TABLE channel_members (
  channel_id    TEXT NOT NULL REFERENCES channels(id),
  user_id       TEXT NOT NULL REFERENCES users(id),
  joined_at     INTEGER NOT NULL,
  last_read_at  INTEGER,
  require_mention_policy TEXT NOT NULL DEFAULT 'inherit' CHECK (require_mention_policy IN ('inherit','on','off')), silent INTEGER NOT NULL DEFAULT 0, org_id_at_join TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (channel_id, user_id)
)`,
	`CREATE TABLE "channels" ("id" TEXT PRIMARY KEY, "name" TEXT NOT NULL, "topic" TEXT DEFAULT '', "visibility" TEXT DEFAULT 'public', "created_at" INTEGER NOT NULL, "created_by" TEXT NOT NULL, "type" TEXT DEFAULT 'channel', "deleted_at" INTEGER, "position" TEXT DEFAULT '0|aaaaaa', "group_id" TEXT, "org_id" TEXT NOT NULL DEFAULT '', "archived_at" INTEGER, description_edit_history TEXT)`,
	`CREATE TABLE events (
  cursor      INTEGER PRIMARY KEY AUTOINCREMENT,
  kind        TEXT NOT NULL,
  channel_id  TEXT NOT NULL,
  payload     TEXT NOT NULL,
  created_at  INTEGER NOT NULL
)`,
	`CREATE TABLE global_events (
  lex_id          TEXT    NOT NULL PRIMARY KEY,
  kind            TEXT    NOT NULL,
  payload         TEXT    NOT NULL DEFAULT '',
  created_at      INTEGER NOT NULL,
  retention_days  INTEGER
)`,
	`CREATE TABLE impersonation_grants (
  id          TEXT    NOT NULL PRIMARY KEY,
  user_id     TEXT    NOT NULL,
  granted_at  INTEGER NOT NULL,
  expires_at  INTEGER NOT NULL,
  revoked_at  INTEGER NULL
)`,
	`CREATE TABLE invite_codes (
  code        TEXT PRIMARY KEY,
  created_by  TEXT NOT NULL,
  created_at  INTEGER NOT NULL,
  expires_at  INTEGER,
  used_by     TEXT REFERENCES users(id),
  used_at     INTEGER,
  note        TEXT
)`,
	`CREATE TABLE mentions (
  id          TEXT PRIMARY KEY,
  message_id  TEXT NOT NULL REFERENCES messages(id),
  user_id     TEXT NOT NULL REFERENCES users(id),
  channel_id  TEXT NOT NULL REFERENCES channels(id)
)`,
	`CREATE TABLE message_mentions (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  message_id     TEXT    NOT NULL,
  target_user_id TEXT    NOT NULL,
  created_at     INTEGER NOT NULL,
  UNIQUE(message_id, target_user_id)
)`,
	`CREATE TABLE message_reactions (
  id          TEXT PRIMARY KEY,
  message_id  TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  user_id     TEXT NOT NULL REFERENCES users(id),
  emoji       TEXT NOT NULL,
  created_at  INTEGER NOT NULL
)`,
	`CREATE TABLE messages (
  id            TEXT PRIMARY KEY,
  channel_id    TEXT NOT NULL REFERENCES channels(id),
  sender_id     TEXT NOT NULL REFERENCES users(id),
  content       TEXT NOT NULL,
  content_type  TEXT DEFAULT 'text',
  reply_to_id   TEXT REFERENCES messages(id),
  created_at    INTEGER NOT NULL,
  edited_at     INTEGER
, deleted_at INTEGER, org_id TEXT NOT NULL DEFAULT '', quick_action TEXT, edit_history TEXT, pinned_at INTEGER)`,
	`CREATE TABLE organizations (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  created_at INTEGER NOT NULL
)`,
	`CREATE TABLE presence_sessions (
  id                INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id        TEXT    NOT NULL UNIQUE,
  user_id           TEXT    NOT NULL,
  agent_id          TEXT,
  connected_at      INTEGER NOT NULL,
  last_heartbeat_at INTEGER NOT NULL
)`,
	`CREATE TABLE remote_bindings (
  id TEXT PRIMARY KEY,
  node_id TEXT NOT NULL REFERENCES remote_nodes(id) ON DELETE CASCADE,
  channel_id TEXT NOT NULL REFERENCES channels(id),
  path TEXT NOT NULL,
  label TEXT,
  created_at INTEGER NOT NULL DEFAULT 0,
  UNIQUE(node_id, channel_id, path)
)`,
	`CREATE TABLE remote_nodes (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  machine_name TEXT NOT NULL,
  connection_token TEXT NOT NULL UNIQUE,
  last_seen_at INTEGER,
  created_at INTEGER NOT NULL DEFAULT 0
, org_id TEXT NOT NULL DEFAULT '')`,
	`CREATE TABLE schema_migrations (
  version    INTEGER PRIMARY KEY,
  applied_at INTEGER NOT NULL,
  name       TEXT NOT NULL
)`,
	`CREATE TABLE user_channel_layout (
  user_id    TEXT    NOT NULL,
  channel_id TEXT    NOT NULL,
  collapsed  INTEGER NOT NULL DEFAULT 0,
  position   REAL    NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  PRIMARY KEY (user_id, channel_id)
)`,
	`CREATE TABLE user_permissions (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  permission  TEXT NOT NULL,
  scope       TEXT NOT NULL DEFAULT '*',
  granted_by  TEXT REFERENCES users(id),
  granted_at  INTEGER NOT NULL, expires_at INTEGER, org_id TEXT, revoked_at INTEGER,
  UNIQUE(user_id, permission, scope)
)`,
	`CREATE TABLE users (
  id           TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  role         TEXT DEFAULT 'member',
  avatar_url   TEXT,
  api_key      TEXT UNIQUE,
  created_at   INTEGER NOT NULL
, email TEXT, password_hash TEXT, last_seen_at INTEGER, require_mention INTEGER DEFAULT 1, owner_id TEXT REFERENCES users(id), deleted_at INTEGER, disabled INTEGER DEFAULT 0, org_id TEXT NOT NULL DEFAULT '')`,
	`CREATE TABLE web_push_subscriptions (
  id            TEXT    NOT NULL PRIMARY KEY,
  user_id       TEXT    NOT NULL,
  endpoint      TEXT    NOT NULL UNIQUE,
  p256dh_key    TEXT    NOT NULL,
  auth_key      TEXT    NOT NULL,
  user_agent    TEXT    NOT NULL DEFAULT '',
  created_at    INTEGER NOT NULL,
  last_used_at  INTEGER NULL
)`,
	`CREATE TABLE workspace_files (
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
  updated_at INTEGER NOT NULL DEFAULT 0, org_id TEXT NOT NULL DEFAULT '',
  UNIQUE(user_id, channel_id, parent_id, name)
)`,
	`CREATE VIRTUAL TABLE artifacts_fts USING fts5(
			title, body,
			content='artifacts',
			content_rowid='rowid',
			tokenize='unicode61 remove_diacritics 2'
		)`,
	`CREATE INDEX idx_admin_actions_actor_id_created_at
			ON "audit_events"(actor_id, created_at DESC)`,
	`CREATE INDEX idx_admin_actions_archived_at
			ON "audit_events"(archived_at) WHERE archived_at IS NOT NULL`,
	`CREATE INDEX idx_admin_actions_target_user_id_created_at
			ON "audit_events"(target_user_id, created_at DESC)`,
	`CREATE INDEX idx_admin_sessions_admin_id ON admin_sessions(admin_id)`,
	`CREATE INDEX idx_admin_sessions_expires_at ON admin_sessions(expires_at)`,
	`CREATE UNIQUE INDEX idx_admins_login ON admins(login)`,
	`CREATE INDEX idx_agent_configs_agent_id
			ON agent_configs(agent_id)`,
	`CREATE INDEX idx_agent_invitations_agent_state
   ON agent_invitations(agent_id, state)`,
	`CREATE INDEX idx_agent_invitations_channel_state
   ON agent_invitations(channel_id, state)`,
	`CREATE INDEX idx_agent_invitations_requested_by
   ON agent_invitations(requested_by)`,
	`CREATE INDEX idx_agent_runtimes_agent_id
			ON agent_runtimes(agent_id)`,
	`CREATE INDEX idx_agent_state_log_agent_id_ts
			ON agent_state_log(agent_id, ts DESC)`,
	`CREATE INDEX idx_agent_state_log_archived_at
			ON agent_state_log(archived_at) WHERE archived_at IS NOT NULL`,
	`CREATE INDEX idx_agent_status_state
			ON agent_status(state)`,
	`CREATE INDEX idx_anchor_comments_anchor
			ON anchor_comments(anchor_id)`,
	`CREATE INDEX idx_anchors_artifact_version
			ON artifact_anchors(artifact_version_id)`,
	`CREATE INDEX idx_artifact_versions_artifact_id
			ON artifact_versions(artifact_id)`,
	`CREATE INDEX idx_artifacts_channel_id
			ON artifacts(channel_id)`,
	`CREATE INDEX idx_channel_events_channel_lex
				ON channel_events(channel_id, lex_id DESC)`,
	`CREATE INDEX idx_channel_events_kind_created
				ON channel_events(kind, created_at)`,
	`CREATE INDEX idx_channel_groups_position ON channel_groups(position)`,
	`CREATE INDEX idx_channel_members_org_at_join
			ON channel_members(org_id_at_join)`,
	`CREATE INDEX idx_channels_group ON channels(group_id)`,
	`CREATE INDEX idx_channels_org_id        ON channels(org_id)`,
	`CREATE UNIQUE INDEX idx_channels_org_id_name
				ON channels(org_id, name) WHERE deleted_at IS NULL`,
	`CREATE INDEX idx_channels_position ON channels(position)`,
	`CREATE INDEX idx_global_events_created
				ON global_events(created_at)`,
	`CREATE INDEX idx_global_events_kind_lex
				ON global_events(kind, lex_id DESC)`,
	`CREATE INDEX idx_impersonation_grants_user_id_expires
			ON impersonation_grants(user_id, expires_at DESC)`,
	`CREATE INDEX idx_invite_codes_used ON invite_codes(used_by)`,
	`CREATE INDEX idx_iterations_artifact_id_state
			ON artifact_iterations(artifact_id, state)`,
	`CREATE INDEX idx_iterations_target_agent
			ON artifact_iterations(target_agent_id)`,
	`CREATE INDEX idx_mentions_user ON mentions(user_id, channel_id)`,
	`CREATE INDEX idx_message_mentions_target_user_id
			ON message_mentions(target_user_id)`,
	`CREATE INDEX idx_messages_channel_time ON messages(channel_id, created_at DESC)`,
	`CREATE INDEX idx_messages_org_id        ON messages(org_id)`,
	`CREATE INDEX idx_messages_pinned_at
			ON messages(channel_id, pinned_at DESC) WHERE pinned_at IS NOT NULL`,
	`CREATE INDEX idx_messages_sender ON messages(sender_id)`,
	`CREATE INDEX idx_presence_sessions_agent_id
			ON presence_sessions(agent_id) WHERE agent_id IS NOT NULL`,
	`CREATE INDEX idx_presence_sessions_user_id
			ON presence_sessions(user_id)`,
	`CREATE INDEX idx_reactions_message ON message_reactions(message_id)`,
	`CREATE UNIQUE INDEX idx_reactions_unique ON message_reactions(message_id, user_id, emoji)`,
	`CREATE INDEX idx_remote_nodes_org_id    ON remote_nodes(org_id)`,
	`CREATE INDEX idx_remote_nodes_user ON remote_nodes(user_id)`,
	`CREATE INDEX idx_user_channel_layout_user_id
			ON user_channel_layout(user_id)`,
	`CREATE INDEX idx_user_permissions_expires
			ON user_permissions(expires_at) WHERE expires_at IS NOT NULL`,
	`CREATE INDEX idx_user_permissions_lookup ON user_permissions(user_id, permission, scope)`,
	`CREATE INDEX idx_user_permissions_org_id
			ON user_permissions(org_id) WHERE org_id IS NOT NULL`,
	`CREATE INDEX idx_user_permissions_revoked
				ON user_permissions(revoked_at) WHERE revoked_at IS NOT NULL`,
	`CREATE INDEX idx_user_permissions_user ON user_permissions(user_id)`,
	`CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE email IS NOT NULL`,
	`CREATE INDEX idx_users_org_id           ON users(org_id)`,
	`CREATE INDEX idx_users_owner_id ON users(owner_id)`,
	`CREATE INDEX idx_web_push_subscriptions_user_id
			ON web_push_subscriptions(user_id)`,
	`CREATE INDEX idx_workspace_files_org_id ON workspace_files(org_id)`,
	`CREATE INDEX idx_workspace_files_parent ON workspace_files(parent_id)`,
	`CREATE INDEX idx_workspace_files_user_channel ON workspace_files(user_id, channel_id)`,
	`CREATE VIEW admin_actions AS SELECT * FROM audit_events`,
	`CREATE TRIGGER admin_actions_insert
			INSTEAD OF INSERT ON admin_actions
			BEGIN
				INSERT INTO audit_events (id, actor_id, target_user_id, action, metadata, created_at, archived_at)
				VALUES (NEW.id, NEW.actor_id, NEW.target_user_id, NEW.action, NEW.metadata, NEW.created_at, NEW.archived_at);
			END`,
	`CREATE TRIGGER admin_actions_update
			INSTEAD OF UPDATE ON admin_actions
			BEGIN
				UPDATE audit_events SET archived_at = NEW.archived_at WHERE id = NEW.id;
			END`,
	`CREATE TRIGGER artifacts_ad
			AFTER DELETE ON artifacts BEGIN
			INSERT INTO artifacts_fts(artifacts_fts, rowid, title, body)
			VALUES('delete', old.rowid, old.title, old.body);
		END`,
	`CREATE TRIGGER artifacts_ai
			AFTER INSERT ON artifacts BEGIN
			INSERT INTO artifacts_fts(rowid, title, body) VALUES (new.rowid, new.title, new.body);
		END`,
	`CREATE TRIGGER artifacts_au
			AFTER UPDATE ON artifacts BEGIN
			INSERT INTO artifacts_fts(artifacts_fts, rowid, title, body)
			VALUES('delete', old.rowid, old.title, old.body);
			INSERT INTO artifacts_fts(rowid, title, body) VALUES (new.rowid, new.title, new.body);
		END`,
}

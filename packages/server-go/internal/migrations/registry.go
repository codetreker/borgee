package migrations

import "gorm.io/gorm"

// All is the canonical, ordered list of versioned migrations applied by the
// server on startup.
//
// Rules:
//   - Version is strictly increasing. Never reuse or renumber.
//   - Once a migration is on main, its body is immutable. To change schema,
//     append a new migration.
//   - Phase 0 / INFRA-1a ships with one "dummy" migration that proves the
//     framework end-to-end. Real Phase 1 schema (organizations, users.org_id,
//     ...) lands as version 2+.
var All = []Migration{
	{
		Version: 1,
		Name:    "infra_1a_dummy_marker",
		Up: func(tx *gorm.DB) error {
			// G0.1 acceptance: prove the engine can run a migration that
			// touches schema. Creating an inert marker table is enough to
			// demonstrate forward-only behavior end-to-end without polluting
			// the production schema. Subsequent migrations replace this with
			// real DDL.
			return tx.Exec(`CREATE TABLE IF NOT EXISTS _migrations_marker (
  version INTEGER PRIMARY KEY,
  note    TEXT
)`).Error
		},
	},
	organizations,
	agentInvitations,
	admins,
	adminSessions,
	// v=6 was originally reserved for ADM-0.3 but the slot was skipped after
	// CM-onboarding (v=7) / AP-0-bis (v=8) / CM-3 (v=9) landed sequentially;
	// ADM-0.3 took v=10 to keep the registry strictly increasing.
	cmOnboardingWelcome,
	bisMessageRead,
	orgIDBackfill,
	usersRoleCollapse,
	channelsOrgScoped,
	presenceSessions,
	artifacts,
	anchorComments,
	messageMentions,
	agentRuntimes,
	artifactKinds,
	artifactIterations,
	userChannelLayout,
	al2a1AgentConfigs,
	adminActions,
	impersonationGrants,
	al1b1AgentStatus,
	userPermissionsExpires,
	agentStateLog,
	webPushSubscriptions,
	hostGrants,
	cv2v2MediaPreview,
	userPermissionsOrg,
	userPermissionsRevoked,
	adminActionsPluginActions,
	cv3v2ArtifactThumbnail,
	adminActionsArchivedAt,
	messagesEditHistory,
	agentStateLogArchivedAt,
	artifactsFTS,
	auditEventsRename,
	channelsDescriptionEditHistory,
	messagesPinnedAt,
	channelEvents,
	globalEvents,
	capabilitiesDotNotationBackfill,
	helperEnrollments,
	helperCredentialRotation,
	helperJobs,
	channelMemberRequireMentionPolicy,
}

// Default returns an Engine wired to db with All registered.
func Default(db *gorm.DB) *Engine {
	e := New(db)
	e.RegisterAll(All)
	return e
}

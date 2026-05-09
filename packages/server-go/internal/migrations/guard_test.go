// Package migrations — guard_test.go: cover the "early-return when prerequisite
// table missing" branches that 7 forward-only migrations share. Each
// guard returns nil without touching schema (跟 Phase-6 partial-rollout
// 设计 — 不裂表, 不假装存在).
package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// TestPrereqMissingMigrationsAreNoOp registers each guarded migration on a
// fresh DB without seeding its prerequisite table. Each Up MUST return
// nil and not record any side-effect schema. We assert the prerequisite
// table is still absent after Run.
func TestPrereqMissingMigrationsAreNoOp(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		migration Migration
		prereq    string // prerequisite table the migration guards on
	}{
		{"al_7_1", adminActionsArchivedAt, "admin_actions"},
		{"bpp_8_1", adminActionsPluginActions, "admin_actions"},
		{"hb_5_1", agentStateLogArchivedAt, "agent_state_log"},
		{"chn_14_1", channelsDescriptionEditHistory, "channels"},
		{"ap_2_1", userPermissionsRevoked, "user_permissions"},
		{"adm_3_1", auditEventsRename, "audit_events"},
		{"dm_7_1", messagesEditHistory, "messages"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := openMem(t)
			e := New(db)
			e.Register(tc.migration)
			if err := e.Run(0); err != nil {
				t.Fatalf("%s Run on empty DB returned err: %v", tc.name, err)
			}
			// Prerequisite never created → migration short-circuits but is
			// recorded as applied (engine commits the row even if Up was a no-op).
			if exists, err := tableExists(db, tc.prereq); err != nil {
				t.Fatalf("%s table check: %v", tc.name, err)
			} else if exists {
				t.Fatalf("%s should not have created prereq table %q", tc.name, tc.prereq)
			}
		})
	}
}

func tableExists(db *gorm.DB, name string) (bool, error) {
	var n int64
	if err := db.Raw(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name,
	).Row().Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// TestADM_ExecError_BadPrior covers the err-return inside the stmts
// loop — pre-existing `admins` table without the `login` column makes
// the UNIQUE INDEX statement fail.
func TestADM_ExecError_BadPrior(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	if err := db.Exec(`CREATE TABLE admins (id TEXT PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	e := New(db)
	e.Register(admins)
	if err := e.Run(0); err == nil {
		t.Error("expected error from index on missing login column")
	}
}

// TestADM_ExecError_BadPrior — same pattern for adm_0_2_admin_sessions.
func TestADM_ExecError_BadPrior_Sessions(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	if err := db.Exec(`CREATE TABLE admin_sessions (id TEXT PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	e := New(db)
	e.Register(adminSessions)
	if err := e.Run(0); err == nil {
		t.Error("expected error from index on missing column")
	}
}

// TestCM_ExecError_BadPrior — same pattern for cm_1_1_organizations.
func TestCM_ExecError_BadPrior(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	if err := db.Exec(`CREATE TABLE organizations (id TEXT PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	e := New(db)
	e.Register(organizations)
	if err := e.Run(0); err == nil {
		t.Error("expected error from missing slug column")
	}
}

// TestCM_ExecError_BadPrior — agent_invitations missing required cols.
func TestCM_ExecError_BadPrior_2(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	if err := db.Exec(`CREATE TABLE agent_invitations (id TEXT PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	e := New(db)
	e.Register(agentInvitations)
	if err := e.Run(0); err == nil {
		t.Error("expected error from index on missing column")
	}
}

// TestDL_ExecError_BadPrior — web_push_subscriptions missing column.
func TestDL_ExecError_BadPrior(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	if err := db.Exec(`CREATE TABLE web_push_subscriptions (id TEXT PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	e := New(db)
	e.Register(webPushSubscriptions)
	if err := e.Run(0); err == nil {
		t.Error("expected error from index on missing column")
	}
}

// TestCHN_ExecError_BadPrior — user_channel_layout missing column.
func TestCHN_ExecError_BadPrior(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	if err := db.Exec(`CREATE TABLE user_channel_layout (id TEXT PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	e := New(db)
	e.Register(userChannelLayout)
	if err := e.Run(0); err == nil {
		t.Error("expected error from missing column")
	}
}

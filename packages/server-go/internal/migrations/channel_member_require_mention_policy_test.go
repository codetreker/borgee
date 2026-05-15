package migrations

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestChannelMemberRequireMentionPolicyMigration(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	if err := db.Exec(`CREATE TABLE channel_members (
  channel_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  joined_at INTEGER NOT NULL,
  PRIMARY KEY(channel_id, user_id)
)`).Error; err != nil {
		t.Fatalf("seed channel_members: %v", err)
	}
	if err := db.Exec(`INSERT INTO channel_members (channel_id, user_id, joined_at) VALUES ('ch-1', 'agent-1', 1)`).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}

	e := New(db)
	e.Register(channelMemberRequireMentionPolicy)
	if err := e.Run(0); err != nil {
		t.Fatalf("run require mention policy migration: %v", err)
	}

	columns := tableColumns(t, db, "channel_members")
	if !columns["require_mention_policy"] {
		t.Fatalf("channel_members missing require_mention_policy; columns=%v", columns)
	}

	var policy string
	if err := db.Raw(`SELECT require_mention_policy FROM channel_members WHERE channel_id = 'ch-1' AND user_id = 'agent-1'`).Scan(&policy).Error; err != nil {
		t.Fatalf("read default policy: %v", err)
	}
	if policy != "inherit" {
		t.Fatalf("legacy member default policy = %q, want inherit", policy)
	}

	ddl := sqliteTableDDL(t, db, "channel_members")
	for _, literal := range []string{"'inherit'", "'on'", "'off'"} {
		if !strings.Contains(ddl, literal) {
			t.Fatalf("require_mention_policy CHECK missing %s in DDL: %s", literal, ddl)
		}
	}
	if err := db.Exec(`INSERT INTO channel_members (channel_id, user_id, joined_at, require_mention_policy) VALUES ('ch-1', 'bad', 2, 'sometimes')`).Error; err == nil {
		t.Fatal("expected invalid require_mention_policy literal to fail")
	}
}

func TestMigrationRegistryIncludesChannelMemberRequireMentionAfterHelperJobs(t *testing.T) {
	t.Parallel()
	last := All[len(All)-1]
	if last.Version != 52 || last.Name != "channel_member_require_mention_policy" {
		t.Fatalf("last migration = v%d %q, want v52 channel_member_require_mention_policy", last.Version, last.Name)
	}
	prev := -1
	for i, m := range All {
		if m.Version == 51 {
			prev = i
		}
		if m.Version == 52 && prev < 0 {
			t.Fatalf("require mention policy v52 appears before helper jobs v51")
		}
	}
}

func tableColumns(t *testing.T, db *gorm.DB, table string) map[string]bool {
	t.Helper()
	rows, err := db.Raw(`PRAGMA table_info(` + table + `)`).Rows()
	if err != nil {
		t.Fatalf("PRAGMA table_info(%s): %v", table, err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan table_info(%s): %v", table, err)
		}
		out[name] = true
	}
	return out
}

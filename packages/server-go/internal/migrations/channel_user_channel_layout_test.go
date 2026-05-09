package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runCHN31 runs migration v=19 (CHN-3.1) on a memory DB. v=19 is a clean
// CREATE — user_channel_layout logical-FKs into users / channels, but
// SQLite FK enforcement is off, so we don't seed upstream tables. Tests
// that exercise real layout REST behaviour live in CHN-3.2 (server path),
// not here (acceptance §1.* only).
func runCHN31(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(userChannelLayout)
	if err := e.Run(0); err != nil {
		t.Fatalf("run chn_3_1: %v", err)
	}
}

// TestCHN_CreatesUserChannelLayoutTable pins acceptance §1.1: the
// table has the contract columns with the right NOT NULL shape. Drift
// here breaks CHN-3.2 GET/PUT /me/layout or 设计 ③ pin=position 单调
// 小数 implementation. 跟 CV-4.1 #399
// TestCV41_CreatesArtifactIterationsTable 同模式.
func TestCHN_CreatesUserChannelLayoutTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCHN31(t, db)

	cols := pragmaColumns(t, db, "user_channel_layout")
	if len(cols) == 0 {
		t.Fatal("user_channel_layout table not created")
	}

	for _, name := range []string{
		"user_id",
		"channel_id",
		"collapsed",
		"position",
		"created_at",
		"updated_at",
	} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("user_channel_layout missing %q (have %v)", name, keys(cols))
		}
		if !c.notNull {
			t.Errorf("user_channel_layout.%s must be NOT NULL", name)
		}
	}

	// 设计 ② PK 复合 (user_id, channel_id) — 本人偏好按 (user_id,
	// channel_id) 唯一; pragma reports pk position 1+ for each PK column.
	if userIDCol := cols["user_id"]; !userIDCol.pk {
		t.Error("user_channel_layout.user_id must be part of PRIMARY KEY")
	}
	if channelIDCol := cols["channel_id"]; !channelIDCol.pk {
		t.Error("user_channel_layout.channel_id must be part of PRIMARY KEY")
	}
}

// TestCHN_NoDomainBleed pins acceptance §1.5 反约束 — 列名反向断言
// hidden / muted / pinned / is_pinned / group_id / cursor 全无.
// 字面沿用野马 #366 stance 7 条原则 黑名单 ①②③ + 设计 ⑥.
func TestCHN_NoDomainBleed(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCHN31(t, db)

	cols := pragmaColumns(t, db, "user_channel_layout")
	for _, forbidden := range []string{
		// 设计 ② 个人偏好两维 (collapsed + position 仅), 反约束:
		// hidden / muted 留 v3+ / Phase 5+ (#366 黑名单 ②).
		"hidden",
		"muted",
		"is_hidden",
		"is_muted",
		// 设计 ③ pin = position 单调小数, 反约束: pinned BOOL 双源排序
		// (#366 设计 ③ + 黑名单 ③).
		"pinned",
		"is_pinned",
		// 设计 ① 物理拆死 — 不裂 group 关系到个人偏好 (作者权,
		// 蓝图 §1.4 字面). 个人不能 reorganize group.
		"group_id",
		// RT-1 envelope cursor 拆死 (跟 al_3_1 / al_4_1 / cv_1_1 /
		// cv_2_1 / dm_2_1 / cv_4_1 同模式 — frame 路径, 不下沉 schema).
		"cursor",
		// 设计 ⑥ ordering client 端 — server 不算偏好排序 fanout.
		"sort_index",
		"order_index",
	} {
		if _, has := cols[forbidden]; has {
			t.Errorf("user_channel_layout.%s exists — 反约束 broken (acceptance §1.5 + spec §0 设计 ②③⑥ + #366 黑名单)", forbidden)
		}
	}
}

// TestCHN_PKEnforcesUniqueRowPerUserChannel pins acceptance §1.2 +
// 设计 ② — duplicate (user_id, channel_id) INSERT must reject.
func TestCHN_PKEnforcesUniqueRowPerUserChannel(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCHN31(t, db)

	insert := func(userID, channelID string, position float64) error {
		return db.Exec(`INSERT INTO user_channel_layout
			(user_id, channel_id, collapsed, position, created_at, updated_at)
			VALUES (?, ?, 0, ?, 1700000000000, 1700000000000)`,
			userID, channelID, position).Error
	}

	if err := insert("u1", "c1", 1.0); err != nil {
		t.Fatalf("first insert should succeed: %v", err)
	}
	// Same (user_id, channel_id) → reject by PK.
	if err := insert("u1", "c1", 2.0); err == nil {
		t.Fatal("duplicate (user_id, channel_id) should reject — PK violation")
	}
	// Different channel_id for same user → OK.
	if err := insert("u1", "c2", 2.0); err != nil {
		t.Errorf("different channel_id should succeed: %v", err)
	}
	// Different user_id for same channel → OK (本人偏好独立).
	if err := insert("u2", "c1", 1.0); err != nil {
		t.Errorf("different user_id should succeed: %v", err)
	}
}

// TestCHN_AcceptsPositionMonotonicDecimals pins 设计 ③ — pin = MIN-1.0
// 单调小数. INSERT REAL position values; schema 不挂 CHECK constraint
// (留 server 校验 reasonable bound), 仅断言 REAL 列受值.
func TestCHN_AcceptsPositionMonotonicDecimals(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCHN31(t, db)

	insert := func(userID, channelID string, position float64) error {
		return db.Exec(`INSERT INTO user_channel_layout
			(user_id, channel_id, collapsed, position, created_at, updated_at)
			VALUES (?, ?, 0, ?, 1700000000000, 1700000000000)`,
			userID, channelID, position).Error
	}
	for _, p := range []float64{1.0, 2.0, 1.5, 0.5, -1.0, -100.5, 1e6} {
		userID := "u1"
		channelID := ""
		// distinct rows.
		switch p {
		case 1.0:
			channelID = "c-a"
		case 2.0:
			channelID = "c-b"
		case 1.5:
			channelID = "c-c"
		case 0.5:
			channelID = "c-d"
		case -1.0:
			channelID = "c-e"
		case -100.5:
			channelID = "c-f"
		case 1e6:
			channelID = "c-g"
		}
		if err := insert(userID, channelID, p); err != nil {
			t.Errorf("position=%v rejected: %v", p, err)
		}
	}
}

// TestCHN_HasUserIDIndex pins acceptance §1.3 — 显式命名 idx_user_
// channel_layout_user_id (本人 GET /me/layout 热路径). 跟 AL-4.1 #398
// TestAL41_HasAgentIDIndex 同模式.
func TestCHN_HasUserIDIndex(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCHN31(t, db)

	var name string
	err := db.Raw(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`,
		"idx_user_channel_layout_user_id").Scan(&name).Error
	if err != nil || name != "idx_user_channel_layout_user_id" {
		t.Errorf("missing index idx_user_channel_layout_user_id (got %q, err=%v)", name, err)
	}
}

// TestChannelUserChannelLayout_Idempotent pins acceptance §1.4 forward-only safety: re-running
// v=19 is no-op (CREATE TABLE IF NOT EXISTS + CREATE INDEX IF NOT EXISTS
// guards). Same as every migration body in the registry.
func TestChannelUserChannelLayout_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCHN31(t, db)
	e := New(db)
	e.Register(userChannelLayout)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run chn_3_1: %v", err)
	}
}

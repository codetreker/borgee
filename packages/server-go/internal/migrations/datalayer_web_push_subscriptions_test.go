package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runDL41 runs migration v=24 (DL-4 web_push_subscriptions) on a memory DB.
// Logical FK to users; SQLite FK enforcement off, no upstream seed needed.
func runDL41(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(webPushSubscriptions)
	if err := e.Run(0); err != nil {
		t.Fatalf("run dl_4_1: %v", err)
	}
}

// TestDL_CreatesWebPushSubscriptionsTable pins schema 8 列 — id PK +
// user_id NOT NULL + endpoint NOT NULL UNIQUE + p256dh_key NOT NULL +
// auth_key NOT NULL + user_agent NOT NULL DEFAULT '' + created_at NOT
// NULL + last_used_at NULL.
func TestDL_CreatesWebPushSubscriptionsTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runDL41(t, db)

	cols := pragmaColumns(t, db, "web_push_subscriptions")
	if len(cols) == 0 {
		t.Fatal("web_push_subscriptions table not created")
	}

	// 7 NOT NULL + 1 nullable.
	for _, name := range []string{"id", "user_id", "endpoint", "p256dh_key", "auth_key", "user_agent", "created_at"} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("web_push_subscriptions missing %q (have %v)", name, keys(cols))
		}
		if !c.notNull {
			t.Errorf("web_push_subscriptions.%s must be NOT NULL", name)
		}
	}
	if c, ok := cols["last_used_at"]; !ok || c.notNull {
		t.Error("web_push_subscriptions.last_used_at must exist and be nullable")
	}

	if idCol := cols["id"]; !idCol.pk {
		t.Error("web_push_subscriptions.id must be PRIMARY KEY")
	}
}

// TestDL_EndpointUNIQUE pins blueprint web-push 字面 — 同 endpoint 二次
// INSERT 必失败 (UNIQUE 严闭防 web-push 库重复加密浪费配额).
func TestDL_EndpointUNIQUE(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runDL41(t, db)

	if err := db.Exec(`INSERT INTO web_push_subscriptions
		(id, user_id, endpoint, p256dh_key, auth_key, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"sub-1", "user-A", "https://fcm.googleapis.com/fcm/send/abc", "p256dh-1", "auth-1", "Mozilla/5.0", 1700000000000).Error; err != nil {
		t.Fatalf("first INSERT: %v", err)
	}

	// Second INSERT with same endpoint MUST fail (UNIQUE constraint).
	err := db.Exec(`INSERT INTO web_push_subscriptions
		(id, user_id, endpoint, p256dh_key, auth_key, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"sub-2", "user-B", "https://fcm.googleapis.com/fcm/send/abc", "p256dh-2", "auth-2", "Chrome/120", 1700000000001).Error
	if err == nil {
		t.Error("duplicate endpoint INSERT succeeded — UNIQUE constraint missing")
	}
}

// TestDL_NoDomainBleed pins blueprint client-shape.md §1.4 隐私 + DL-4
// spec §2 — secret / device_id / cursor / org_id 等不在此表.
func TestDL_NoDomainBleed(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runDL41(t, db)

	cols := pragmaColumns(t, db, "web_push_subscriptions")
	for _, forbidden := range []string{
		// VAPID 私钥 / 任何 secret 在 server env, 不入表
		"vapid_secret",
		"vapid_private",
		"api_key",
		"token",
		"session_token",
		// device 维度走 user_agent hint, 不开 device_id 路由 (跟 al_3_1
		// presence multi-session last-wins 设计沿用)
		"device_id",
		"device_kind",
		"device_type",
		// org_id 通过 users.org_id 派生 SSOT 不冗余 (跟 al_2a_1 / chn_3_1 /
		// al_1b_1 / adm_2_1 / adm_2_2 同模式)
		"org_id",
		// cursor 走 hub.cursors RT-1/CV-2/DM-2/CV-4/AL-2b/RT-3 6 frame 共序
		// sequence, push 是 fire-and-forget 不下沉 schema
		"cursor",
		// 退订 = DELETE row 单源, 不开 enabled/paused/muted 双源
		"enabled",
		"paused",
		"muted",
	} {
		if _, has := cols[forbidden]; has {
			t.Errorf("web_push_subscriptions.%s exists — 反约束 broken (DL-4 spec §2)", forbidden)
		}
	}
}

// TestDL_HasUserIDIndex pins fan-out 热路径 — server 收
// mention/agent_task_state_changed 派生 → 查 user 全设备 N row.
func TestDL_HasUserIDIndex(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runDL41(t, db)

	var name string
	err := db.Raw(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`,
		"idx_web_push_subscriptions_user_id").Scan(&name).Error
	if err != nil || name != "idx_web_push_subscriptions_user_id" {
		t.Errorf("expected idx_web_push_subscriptions_user_id index, got %q (err=%v)", name, err)
	}
}

// TestDL_Idempotent pins forward-only stance — second Run() 不报错
// (IF NOT EXISTS 守).
func TestDL_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runDL41(t, db)
	// Second run must be a no-op.
	e := New(db)
	e.Register(webPushSubscriptions)
	if err := e.Run(0); err != nil {
		t.Fatalf("second run failed (forward-only idempotency broken): %v", err)
	}
}

// TestDL_VersionIs26 pins migration sequencing — DL-4.1 = v=26,
// continues from AL-1.4 v=25 (#492 merged).
func TestDL_VersionIs26(t *testing.T) {
	t.Parallel()
	if webPushSubscriptions.Version != 26 {
		t.Errorf("webPushSubscriptions.Version = %d, want 26 (registry sequencing post-#492)",
			webPushSubscriptions.Version)
	}
	if webPushSubscriptions.Name != "dl_4_1_web_push_subscriptions" {
		t.Errorf("webPushSubscriptions.Name = %q, want %q",
			webPushSubscriptions.Name, "dl_4_1_web_push_subscriptions")
	}
}

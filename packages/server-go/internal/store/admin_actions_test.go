package store

import (
	"strings"
	"testing"
	"time"
)

// runStoreWithMigrations bootstraps a store + applies all migrations (incl
// v=22 admin_actions + v=23 impersonation_grants).
func runStoreWithMigrations(t *testing.T) *Store {
	t.Helper()
	s := testStore(t)
	if err := s.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}

// TestInsertAdminAction_HappyPath pins acceptance §行为不变量 4.1.a — INSERT
// 1 行 (5 action 之一) successfully + 返非空 id.
func TestInsertAdminAction_HappyPath(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	for _, action := range []string{
		"delete_channel", "suspend_user", "change_role",
		"reset_password", "start_impersonation",
	} {
		id, err := s.InsertAdminAction("admin-1", "user-1", action, `{"k":"v"}`)
		if err != nil {
			t.Errorf("InsertAdminAction action=%q: %v", action, err)
		}
		if id == "" {
			t.Errorf("InsertAdminAction action=%q returned empty id", action)
		}
	}
}

// TestInsertAdminAction_RejectsEmptyRequiredFields pins 设计 — actor_id /
// target_user_id / action 三必填, server-side gate (跟 schema NOT NULL 双锁).
func TestInsertAdminAction_RejectsEmptyRequiredFields(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	cases := []struct {
		name                                 string
		actorID, targetUserID, action string
	}{
		{"empty actor_id", "", "u1", "delete_channel"},
		{"empty target_user_id", "a1", "", "delete_channel"},
		{"empty action", "a1", "u1", ""},
	}
	for _, c := range cases {
		if _, err := s.InsertAdminAction(c.actorID, c.targetUserID, c.action, ""); err == nil {
			t.Errorf("%s should reject", c.name)
		}
	}
}

// TestInsertAdminAction_RejectsUnknownActionViaCHECK pins acceptance §数据契约
// row 2 — schema CHECK 5 字面 enum + server insert path 复用 (反向: 同义词 /
// 大小写漂移 / 字典外 全 reject).
func TestInsertAdminAction_RejectsUnknownActionViaCHECK(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	for _, bad := range []string{
		"Delete_Channel", "DELETE_CHANNEL",
		"remove_channel", "ban_user", "update_role", "password_reset",
		"impersonate", "force_impersonate", "create_user",
	} {
		if _, err := s.InsertAdminAction("a1", "u1", bad, ""); err == nil {
			t.Errorf("action=%q should reject by CHECK", bad)
		}
	}
}

// TestListAdminActionsForTargetUser_ScopedToUser pins acceptance §行为不变量
// 4.1.c — user 只见自己 (反向: 跨 user 的行不返).
func TestListAdminActionsForTargetUser_ScopedToUser(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	// 3 行 for u1, 2 行 for u2.
	for i := 0; i < 3; i++ {
		if _, err := s.InsertAdminAction("a1", "u1", "delete_channel", ""); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := s.InsertAdminAction("a1", "u2", "suspend_user", ""); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := s.ListAdminActionsForTargetUser("u1", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Errorf("u1 expected 3 rows, got %d", len(rows))
	}
	for _, r := range rows {
		if r.TargetUserID != "u1" {
			t.Errorf("row leaked: target_user_id=%q (expect u1)", r.TargetUserID)
		}
	}
	// u2 仍只见 u2 的.
	u2Rows, _ := s.ListAdminActionsForTargetUser("u2", 50)
	if len(u2Rows) != 2 {
		t.Errorf("u2 expected 2 rows, got %d", len(u2Rows))
	}
}

// TestListAdminActionsForAdmin_FullVisibility pins acceptance §行为不变量
// 4.1.d — admin 之间互可见 (无 WHERE 默认).
func TestListAdminActionsForAdmin_FullVisibility(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	s.InsertAdminAction("admin-A", "u1", "delete_channel", "")
	s.InsertAdminAction("admin-B", "u2", "suspend_user", "")
	s.InsertAdminAction("admin-A", "u3", "change_role", "")

	rows, err := s.ListAdminActionsForAdmin(AdminActionListFilters{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Errorf("expected 3 rows (admin 互可见), got %d", len(rows))
	}

	// Filter by actor_id.
	aRows, _ := s.ListAdminActionsForAdmin(AdminActionListFilters{ActorID: "admin-A"}, 100)
	if len(aRows) != 2 {
		t.Errorf("filter actor=A expected 2 rows, got %d", len(aRows))
	}
	// Filter by action.
	suspendRows, _ := s.ListAdminActionsForAdmin(AdminActionListFilters{Action: "suspend_user"}, 100)
	if len(suspendRows) != 1 {
		t.Errorf("filter action=suspend_user expected 1 row, got %d", len(suspendRows))
	}
}

// TestRenderAdminActionDMBody_ByteIdentical pins content-lock §1 5 模板字面
// byte-identical (蓝图 §1.4 红线 1 + admin-model.md §4.1 R3 admin_username
// 非 UUID 兑现).
func TestRenderAdminActionDMBody_ByteIdentical(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 4, 29, 14, 32, 0, 0, time.Local)
	cases := []struct {
		action string
		ctx    AdminActionDMContext
		want   string
	}{
		{
			"delete_channel",
			AdminActionDMContext{ChannelName: "#demo"},
			`你的 channel #demo 被 admin alice 于 2026-04-29 14:32 删除。详情见设置页"隐私 → 影响记录"。`,
		},
		{
			"suspend_user",
			AdminActionDMContext{Reason: "违反社区规范"},
			`你的账号被 admin alice 于 2026-04-29 14:32 暂停: 违反社区规范。详情见设置页"隐私 → 影响记录"。`,
		},
		{
			"change_role",
			AdminActionDMContext{OldRole: "member", NewRole: "agent"},
			`你的账号角色被 admin alice 于 2026-04-29 14:32 从 member 调整为 agent。详情见设置页"隐私 → 影响记录"。`,
		},
		{
			"reset_password",
			AdminActionDMContext{},
			`你的登录密码被 admin alice 于 2026-04-29 14:32 重置, 请重新生成。详情见设置页"隐私 → 影响记录"。`,
		},
	}
	for _, c := range cases {
		got := RenderAdminActionDMBody("alice", c.action, ts, c.ctx)
		if got != c.want {
			t.Errorf("action=%s\nwant: %q\ngot:  %q", c.action, c.want, got)
		}
	}
	// start_impersonation has variable expires_at — assert structure only.
	imp := RenderAdminActionDMBody("alice", "start_impersonation", ts,
		AdminActionDMContext{ExpiresAt: ts.Add(24 * time.Hour).UnixMilli()})
	if !strings.Contains(imp, "admin alice 已对你的账号开启 24h impersonate") ||
		!strings.Contains(imp, "可在设置页随时撤销") {
		t.Errorf("start_impersonation body missing literal anchor: %q", imp)
	}
}

// TestRenderAdminActionDMBody_NeverContainsRawUUID pins stance §2
// ADM2-NEG-001 反向断言 — body 不渲染 raw UUID 字面 (admin_username 走
// admins.Login 具体名).
func TestRenderAdminActionDMBody_NeverContainsRawUUID(t *testing.T) {
	t.Parallel()
	ts := time.Now()
	uuidLike := "deadbeef-1234-5678-90ab-cdef00112233"
	for _, action := range []string{"delete_channel", "suspend_user", "change_role", "reset_password", "start_impersonation"} {
		body := RenderAdminActionDMBody("alice", action, ts, AdminActionDMContext{ChannelName: "#x", Reason: "r", OldRole: "member", NewRole: "agent"})
		if strings.Contains(body, uuidLike) {
			t.Errorf("action=%s body contains raw UUID-like string", action)
		}
		// And critically, never contains the placeholder template literal.
		for _, neg := range []string{"{admin_id}", "{actor_id}", "${adminId}"} {
			if strings.Contains(body, neg) {
				t.Errorf("action=%s body contains template placeholder %q (stance §2 ADM2-NEG-001 broken)", action, neg)
			}
		}
	}
}

// TestGrantImpersonation_24hExpiry pins acceptance §4.2.a — expires_at =
// granted_at + 24h, server 固定不接受 client 传.
func TestGrantImpersonation_24hExpiry(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	g, err := s.GrantImpersonation("u1")
	if err != nil {
		t.Fatal(err)
	}
	if g.ExpiresAt-g.GrantedAt != 24*60*60*1000 {
		t.Errorf("expires_at - granted_at = %d ms, expected 24h (%d ms)",
			g.ExpiresAt-g.GrantedAt, 24*60*60*1000)
	}
	if g.RevokedAt != nil {
		t.Errorf("new grant should not be revoked")
	}
}

// TestGrantImpersonation_RejectsActiveDuplicate pins 设计 ⑦ — 业主 cooldown
// 防重复 grant (24h 期内 grant 已存在 → 409).
func TestGrantImpersonation_RejectsActiveDuplicate(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	if _, err := s.GrantImpersonation("u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GrantImpersonation("u1"); err == nil {
		t.Error("duplicate active grant should reject")
	} else if !strings.Contains(err.Error(), "grant_already_active") {
		t.Errorf("expected grant_already_active error code, got %q", err.Error())
	}
	// Different user can grant.
	if _, err := s.GrantImpersonation("u2"); err != nil {
		t.Errorf("u2 grant should succeed: %v", err)
	}
}

// TestRevokeImpersonation_ClearsActiveGrant pins acceptance §4.2.a 业主撤销.
func TestRevokeImpersonation_ClearsActiveGrant(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	g, _ := s.GrantImpersonation("u1")
	if err := s.RevokeImpersonation("u1"); err != nil {
		t.Fatal(err)
	}

	active, err := s.ActiveImpersonationGrant("u1")
	if err != nil {
		t.Fatal(err)
	}
	if active != nil {
		t.Errorf("after revoke, ActiveImpersonationGrant should return nil; got %v", active)
	}

	// After revoke, can re-grant (cooldown released).
	g2, err := s.GrantImpersonation("u1")
	if err != nil {
		t.Errorf("re-grant after revoke should succeed: %v", err)
	}
	if g2.ID == g.ID {
		t.Error("re-grant should produce new id")
	}
}

// TestActiveImpersonationGrant_ReturnsNilWhenNone pins ActiveImpersonationGrant
// 设计 — 无 grant 返 (nil, nil), 不返 sql.ErrNoRows.
func TestActiveImpersonationGrant_ReturnsNilWhenNone(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	g, err := s.ActiveImpersonationGrant("u-no-grant")
	if err != nil {
		t.Errorf("expected nil err for no grant, got %v", err)
	}
	if g != nil {
		t.Errorf("expected nil grant, got %v", g)
	}
}

// TestRenderAdminActionDMBody_UnknownActionReturnsEmpty covers default branch.
func TestRenderAdminActionDMBody_UnknownActionReturnsEmpty(t *testing.T) {
	t.Parallel()
	got := RenderAdminActionDMBody("alice", "unknown_action", time.Now(), AdminActionDMContext{})
	if got != "" {
		t.Errorf("unknown action should return empty, got %q", got)
	}
}

// TestRenderAdminActionDMBody_SuspendUserDefaultReason covers empty Reason
// fallback "(未提供原因)".
func TestRenderAdminActionDMBody_SuspendUserDefaultReason(t *testing.T) {
	t.Parallel()
	got := RenderAdminActionDMBody("alice", "suspend_user", time.Now(), AdminActionDMContext{})
	if !strings.Contains(got, "(未提供原因)") {
		t.Errorf("expected default reason fallback, got %q", got)
	}
}

// TestEmitAdminActionAudit_RejectsEmpty covers EmitAdminActionAudit error path
// (delegated to InsertAdminAction).
func TestEmitAdminActionAudit_RejectsEmpty(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	if _, err := s.EmitAdminActionAudit("", "alice", "u1", "delete_channel", "", AdminActionDMContext{}); err == nil {
		t.Error("empty actor_id should reject")
	}
}

// TestEmitAdminActionSystemDM_RejectsEmptyArgs covers RejectsEmpty branch.
func TestEmitAdminActionSystemDM_RejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	if err := s.EmitAdminActionSystemDM("", "u1", "delete_channel", AdminActionDMContext{}); err == nil {
		t.Error("empty actor_login should reject")
	}
	if err := s.EmitAdminActionSystemDM("alice", "", "delete_channel", AdminActionDMContext{}); err == nil {
		t.Error("empty target_user_id should reject")
	}
}

// TestEmitAdminActionSystemDM_NoSystemChannelDegrades covers the graceful
// degradation when target user has no #welcome channel — returns nil err
// (not failure) because audit row is the 100% guarantee.
func TestEmitAdminActionSystemDM_NoSystemChannelDegrades(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	// User exists but has no #welcome channel.
	err := s.EmitAdminActionSystemDM("alice", "u-no-channel", "delete_channel", AdminActionDMContext{ChannelName: "#x"})
	if err != nil {
		t.Errorf("expected nil err for no-channel degraded path, got %v", err)
	}
}

// TestEmitAdminActionSystemDM_UnknownActionNoOp covers RenderAdminActionDMBody
// returning empty for unknown action.
func TestEmitAdminActionSystemDM_UnknownActionNoOp(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	err := s.EmitAdminActionSystemDM("alice", "u1", "unknown_action_name", AdminActionDMContext{})
	if err != nil {
		t.Errorf("unknown action should silently no-op, got %v", err)
	}
}

// TestListAdminActionsForTargetUser_RejectsEmpty covers error branch.
func TestListAdminActionsForTargetUser_RejectsEmpty(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	if _, err := s.ListAdminActionsForTargetUser("", 50); err == nil {
		t.Error("empty user_id should reject")
	}
}

// TestListAdminActions_LimitDefaults covers limit clamping branches.
func TestListAdminActions_LimitDefaults(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	// limit <= 0 → default 50; > 200 → 200.
	rows, err := s.ListAdminActionsForTargetUser("u1", -1)
	if err != nil {
		t.Fatal(err)
	}
	_ = rows
	rows2, _ := s.ListAdminActionsForTargetUser("u1", 9999)
	_ = rows2
	// Admin variant.
	rows3, _ := s.ListAdminActionsForAdmin(AdminActionListFilters{}, -1)
	_ = rows3
	rows4, _ := s.ListAdminActionsForAdmin(AdminActionListFilters{}, 9999)
	_ = rows4
}

// TestGrantImpersonation_RejectsEmpty + TestRevokeImpersonation_RejectsEmpty
// + TestActiveImpersonationGrant_RejectsEmpty cover error paths.
func TestGrantImpersonation_RejectsEmpty(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	if _, err := s.GrantImpersonation(""); err == nil {
		t.Error("empty user_id should reject")
	}
}
func TestRevokeImpersonation_RejectsEmpty(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	if err := s.RevokeImpersonation(""); err == nil {
		t.Error("empty user_id should reject")
	}
}
func TestActiveImpersonationGrant_RejectsEmpty(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	if _, err := s.ActiveImpersonationGrant(""); err == nil {
		t.Error("empty user_id should reject")
	}
}

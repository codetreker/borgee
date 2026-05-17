// Package store — ADM-2.2 admin_actions audit + impersonate_grants helpers.
//
// Blueprint: docs/blueprint/current/admin-model.md §1.4 (谁能看到什么 + 三红线) +
// §2 不变量 (受影响者必感知 + Audit 100% 留痕 + 分层可见).
// Spec: docs/implementation/modules/adm-2-spec.md §2.
// Content lock: docs/qa/adm-2-content-lock.md §1 (5 system DM 模板).
//
// Public surface (跟 ADM-1 deferred 2 行兑现锚):
//   - InsertAdminAction(actorID, targetUserID, action, metadata) — 写一行 audit
//   - EmitAdminActionSystemDM(actorLogin, targetUserID, action, ts, ctx) — 受
//     影响者必收 system DM, body text matches content-lock §1
//   - ListAdminActionsForTargetUser(userID, limit) — user 侧 GET
//     /api/v1/me/admin-actions 走此 (WHERE target_user_id=)
//   - ListAdminActionsForAdmin(filters, limit) — admin 侧 GET
//     /admin-api/v1/audit-log 走此 (无 WHERE 默认全可见)
//
// 设计反查 (约定 §1 7 设计):
//   ① 每写必留痕 — 此 helper 是写动作 wrap 后的唯一 audit 入口, 反向 grep
//      `skip_audit\|noAudit\|bypassAudit` count==0
//   ② 受影响者必感知 — EmitAdminActionSystemDM 强制下发, body 含 actorLogin
//      (admin.Login, admins 表) 非 raw UUID
//   ⑤ forward-only — InsertAdminAction 不返 row.ID for update; UPDATE/DELETE
//      路径不存在 (反向 grep `UPDATE admin_actions\|DELETE FROM admin_actions`
//      count==0 除 migration)
//   ⑥ admin ∉ 业务路径 — actorID 是 admins.id (独立表, 跟 ADM-0 红线对齐)
package store

import (
	"errors"
	"fmt"
	"time"


	"borgee-server/internal/idgen"
)

// AdminAction is one row of admin_actions table (ADM-2.1 schema v=22).
type AdminAction struct {
	ID           string `gorm:"column:id;primaryKey"`
	ActorID      string `gorm:"column:actor_id"`
	TargetUserID string `gorm:"column:target_user_id"`
	Action       string `gorm:"column:action"`
	Metadata     string `gorm:"column:metadata"`
	CreatedAt    int64  `gorm:"column:created_at"`
	// ADMIN-SPA-SHAPE-FIX D4 (走 A): AL-7.1 schema 加了 archived_at 列 + sparse idx
	// (WHERE archived_at IS NOT NULL), 但 Go struct 之前 0 surfaced. 加 nullable
	// pointer 字段 + json:"-" (不直 marshal — 走 sanitizeAdminAction nil-safe surface).
	ArchivedAt *int64 `gorm:"column:archived_at" json:"-"`
}

// TableName is required because gorm pluralizes by default — table name is
// `admin_actions` (matches migration v=22 字面).
func (AdminAction) TableName() string { return "admin_actions" }

// AdminActionListFilters narrows the admin-side audit log query.
// All fields optional. Empty string = no filter on that axis.
//
// AL-8 additive filter (ADM-2.2 既有 3 字段 unchanged, 顺位
// append 新字段): Since/Until int64 ms epoch (BETWEEN created_at, nil =
// no filter); ArchivedView 三态 ("" or "active" = archived_at IS NULL 默认
// / "archived" = archived_at IS NOT NULL 走 AL-7.1 sparse idx /
// "all" = 无 WHERE on archived_at); Actions 多值 (IN slice, 跟 单值 Action
// 二选一 — 调用方先 collect Actions slice 再单字段 backward-compat).
//
// 约束 (al-8-spec.md §0 设计 ①): 既有 3 字段顺序不动 (ActorID/Action/
// TargetUserID), AL-8 新字段顺位 append.
type AdminActionListFilters struct {
	ActorID      string
	Action       string
	TargetUserID string
	// AL-8 additive filter — 顺位 append, 既有 3 字段不动.
	Since        *int64
	Until        *int64
	ArchivedView string // "" / "active" / "archived" / "all"
	Actions      []string
}

// InsertAdminAction writes one audit row. action must be in the 5-字面
// whitelist (delete_channel / suspend_user / change_role / reset_password /
// start_impersonation) — schema CHECK enforces; this is just the insert.
//
// 设计 ⑤ forward-only: returns no update handle. Errors only on db.
func (s *Store) InsertAdminAction(actorID, targetUserID, action, metadata string) (string, error) {
	if actorID == "" || targetUserID == "" || action == "" {
		return "", errors.New("actor_id, target_user_id, action all required (蓝图 §1.4 红线 1 受影响者必有)")
	}
	row := AdminAction{
		ID:           idgen.NewID(),
		ActorID:      actorID,
		TargetUserID: targetUserID,
		Action:       action,
		Metadata:     metadata, // server-validated JSON, schema 不挂 CHECK
		CreatedAt:    time.Now().UnixMilli(),
	}
	if err := s.db.Create(&row).Error; err != nil {
		return "", err
	}
	return row.ID, nil
}

// ListAdminActionsForTargetUser returns the most recent admin_actions rows
// where target_user_id = userID. Used by GET /api/v1/me/admin-actions
// (user cookie). 设计 ④ user 只见自己.
//
// 约束: this is the ONLY query path for user-side audit; ?target_user_id
// inject 防线在 handler 层忽略 (走 current user_id 不接受参数覆写).
func (s *Store) ListAdminActionsForTargetUser(userID string, limit int) ([]AdminAction, error) {
	if userID == "" {
		return nil, errors.New("user_id required")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []AdminAction
	err := s.db.Where("target_user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// ListAdminActionsForAdmin returns the most recent admin_actions rows,
// optionally filtered. Used by GET /admin-api/v1/audit-log (admin cookie).
// 设计 ③ admin 之间互可见: 默认无 WHERE 全可见, filter 只是 UI 收敛.
//
// AL-8 additive filter (al-8-spec.md §0 设计 ③④⑤): Since/Until int64 ms
// epoch BETWEEN; ArchivedView 三态 ("" or "active" = archived_at IS NULL
// 默认 / "archived" 走 AL-7.1 sparse idx / "all" 无 WHERE on archived_at);
// Actions 多值 IN slice — 跟单值 Action 二选一 (Actions 优先, 反向 reject
// 由 handler 层做).
func (s *Store) ListAdminActionsForAdmin(f AdminActionListFilters, limit int) ([]AdminAction, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := s.db.Model(&AdminAction{})
	if f.ActorID != "" {
		q = q.Where("actor_id = ?", f.ActorID)
	}
	if len(f.Actions) > 0 {
		q = q.Where("action IN ?", f.Actions)
	} else if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	if f.TargetUserID != "" {
		q = q.Where("target_user_id = ?", f.TargetUserID)
	}
	if f.Since != nil {
		q = q.Where("created_at >= ?", *f.Since)
	}
	if f.Until != nil {
		q = q.Where("created_at <= ?", *f.Until)
	}
	// 设计 ③ archived 三态 — 默认 active (跟 AL-7.1 sparse idx 反向同源).
	switch f.ArchivedView {
	case "", "active":
		q = q.Where("archived_at IS NULL")
	case "archived":
		q = q.Where("archived_at IS NOT NULL")
	case "all":
		// no WHERE on archived_at
	}
	var rows []AdminAction
	err := q.Order("created_at DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// AdminActionDMContext carries the per-action substitution data for system
// DM body rendering (跟 content-lock §1 5 模板对齐).
type AdminActionDMContext struct {
	ChannelName string // for delete_channel: "#foo"
	Reason      string // for suspend_user
	OldRole     string // for change_role
	NewRole     string // for change_role
	ExpiresAt   int64  // for start_impersonation (Unix ms)
}

// RenderAdminActionDMBody returns the system DM body for the given action.
// Matches docs/qa/adm-2-content-lock.md §1 5 templates.
//
// 约束 (约定 §2 ADM2-NEG-001 + ADM2-NEG-009):
//   - actorLogin 必须是 admins.Login (具体名), 调用方传; 此函数不接受 admin
//     UUID, 反向 grep `\{admin_id\}|\{actor_id\}` 在 body literal count==0
//   - ts 走 time.Format("2006-01-02 15:04") 本地化; 不渲染 epoch ms 字面
func RenderAdminActionDMBody(actorLogin, action string, ts time.Time, ctx AdminActionDMContext) string {
	tsStr := ts.Format("2006-01-02 15:04")
	switch action {
	case "delete_channel":
		return fmt.Sprintf("你的 channel %s 被 admin %s 于 %s 删除。", ctx.ChannelName, actorLogin, tsStr)
	case "suspend_user":
		reason := ctx.Reason
		if reason == "" {
			reason = "(未提供原因)"
		}
		return fmt.Sprintf("你的账号被 admin %s 于 %s 暂停: %s。", actorLogin, tsStr, reason)
	case "change_role":
		return fmt.Sprintf("你的账号角色被 admin %s 于 %s 从 %s 调整为 %s。", actorLogin, tsStr, ctx.OldRole, ctx.NewRole)
	case "reset_password":
		return fmt.Sprintf("你的登录密码被 admin %s 于 %s 重置, 请重新生成。", actorLogin, tsStr)
	case "start_impersonation":
		expStr := time.UnixMilli(ctx.ExpiresAt).Format("2006-01-02 15:04")
		return fmt.Sprintf("admin %s 已对你的账号开启 24h impersonate, 起于 %s, 至 %s。", actorLogin, tsStr, expStr)
	default:
		return ""
	}
}

// ImpersonationGrant is one row of impersonation_grants table (ADM-2.2).
//
// Schema (本 PR 同期落, 跟 ADM-2.1 admin_actions 共享 ADM-2 milestone — 新协议
// 一 milestone 一 PR):
//   id           TEXT PK (UUID)
//   user_id      TEXT NOT NULL (FK users.id 业主自己 grant)
//   granted_at   INTEGER NOT NULL (Unix ms)
//   expires_at   INTEGER NOT NULL (granted_at + 24h)
//   revoked_at   INTEGER NULL (业主主动撤销; NULL 表示有效)
type ImpersonationGrant struct {
	ID        string `gorm:"column:id;primaryKey"`
	UserID    string `gorm:"column:user_id"`
	GrantedAt int64  `gorm:"column:granted_at"`
	ExpiresAt int64  `gorm:"column:expires_at"`
	RevokedAt *int64 `gorm:"column:revoked_at"`
}

func (ImpersonationGrant) TableName() string { return "impersonation_grants" }

// GrantImpersonation creates a 24h grant. Returns 409-style error if a non-
// expired non-revoked grant already exists (业主 cooldown 防重复 grant).
//
// 设计 ⑦ impersonate 显眼: grant 期 24h 固定 (约束: 不接受 client 传期限);
// 业主撤销走 RevokeImpersonation (UPDATE revoked_at 唯一允许的写, 不删行
// — 留 audit 痕跡, 跟设计 ⑤ forward-only 同精神).
func (s *Store) GrantImpersonation(userID string) (*ImpersonationGrant, error) {
	if userID == "" {
		return nil, errors.New("user_id required")
	}
	now := time.Now().UnixMilli()
	// Reject duplicate active grant (cooldown).
	var existing ImpersonationGrant
	err := s.db.Where("user_id = ? AND expires_at > ? AND revoked_at IS NULL", userID, now).
		First(&existing).Error
	if err == nil {
		return nil, errors.New("impersonate.grant_already_active")
	}
	g := &ImpersonationGrant{
		ID:        idgen.NewID(),
		UserID:    userID,
		GrantedAt: now,
		ExpiresAt: now + 24*60*60*1000, // 24h
	}
	if err := s.db.Create(g).Error; err != nil {
		return nil, err
	}
	return g, nil
}

// RevokeImpersonation marks the active grant for userID as revoked. No-op if
// no active grant.
func (s *Store) RevokeImpersonation(userID string) error {
	if userID == "" {
		return errors.New("user_id required")
	}
	now := time.Now().UnixMilli()
	return s.db.Model(&ImpersonationGrant{}).
		Where("user_id = ? AND expires_at > ? AND revoked_at IS NULL", userID, now).
		Update("revoked_at", now).Error
}

// ActiveImpersonationGrant returns the user's currently-active grant or nil.
// 设计 ⑦ admin 写动作前 server 校验 grant 存在: server-side gate, plug 入
// admin handler.
func (s *Store) ActiveImpersonationGrant(userID string) (*ImpersonationGrant, error) {
	if userID == "" {
		return nil, errors.New("user_id required")
	}
	now := time.Now().UnixMilli()
	var g ImpersonationGrant
	err := s.db.Where("user_id = ? AND expires_at > ? AND revoked_at IS NULL", userID, now).
		First(&g).Error
	if err != nil {
		return nil, nil // no active grant — caller treats as 403 if needed
	}
	return &g, nil
}

// EmitAdminActionSystemDM writes the system-DM into the target user's
// existing #welcome channel (CM-onboarding type='system' channel; created
// at registration). Body matches content-lock §1 5 templates; 设计 ②
// "受影响者必感知 + admin_username 非 raw UUID".
//
// Returns nil even when target user has no system channel — system DM is
// best-effort (acceptance §4.1.b "强制下发不依赖前端订阅" is enforced by the
// admin_actions audit row, not the DM rendering); the audit row is the
// 100% guarantee, the DM is the user-visible surface that may degrade
// gracefully (cf. CM-onboarding welcome system message OK=false branch).
func (s *Store) EmitAdminActionSystemDM(actorLogin, targetUserID, action string, ctx AdminActionDMContext) error {
	if actorLogin == "" || targetUserID == "" || action == "" {
		return errors.New("actor_login, target_user_id, action all required")
	}
	body := RenderAdminActionDMBody(actorLogin, action, time.Now(), ctx)
	if body == "" {
		return nil // unknown action — silently no-op (CHECK at insert path is the gate)
	}
	// Find the target user's #welcome (type='system') channel.
	var ch Channel
	err := s.db.Where("created_by = ? AND type = ? AND deleted_at IS NULL", targetUserID, "system").
		First(&ch).Error
	if err != nil {
		// No system channel — degrade gracefully (audit row already written).
		return nil
	}
	now := time.Now().UnixMilli()
	msgID := idgen.NewID()
	return s.db.Exec(`
		INSERT INTO messages (id, channel_id, sender_id, content, content_type, created_at)
		VALUES (?, ?, 'system', ?, 'text', ?)
	`, msgID, ch.ID, body, now).Error
}

// EmitAdminActionAudit is the joint helper: write audit row + emit system DM.
// Wraps the two store calls; admin handler audit hook uses one helper call. 设计
// ① 每写必留痕 + 设计 ② 受影响者必感知 同时兑现.
func (s *Store) EmitAdminActionAudit(actorID, actorLogin, targetUserID, action, metadata string, ctx AdminActionDMContext) (string, error) {
	id, err := s.InsertAdminAction(actorID, targetUserID, action, metadata)
	if err != nil {
		return "", err
	}
	// DM emit is best-effort — failure to render DM does NOT roll back the
	// audit row (蓝图 §2 "Audit 100% 留痕" 不变量优先).
	_ = s.EmitAdminActionSystemDM(actorLogin, targetUserID, action, ctx)
	return id, nil
}

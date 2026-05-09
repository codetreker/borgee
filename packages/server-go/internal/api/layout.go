// Package api — layout.go: CHN-3.2 user_channel_layout REST endpoints.
//
// Spec: docs/implementation/modules/chn-3-spec.md §0 (3 条原则) + §1
// CHN-3.2 段.
// Stance: docs/qa/chn-3-stance-checklist.md (#366, 7 条原则).
// Acceptance: docs/qa/acceptance-templates/chn-3.md §2.* (CHN-3.2
// server-side验收: GET 本人 / PUT 批量 / DM reject / admin reject /
// non-member reject).
// Content lock: docs/qa/chn-3-content-lock.md §1 ④⑤ (DM 反约束错码
// `layout.dm_not_grouped` byte-identical 5 源 + 失败 toast 文案锁
// "侧栏顺序保存失败, 请重试" 跟 client #371 / acceptance §3.5 / 文案锁
// ④ 三源).
//
// Endpoint surface:
//   - GET /api/v1/me/layout            return [{channel_id, collapsed,
//     position, updated_at}]
//   - PUT /api/v1/me/layout            batch upsert body:
//     {layout: [{channel_id, collapsed,
//     position}, ...]}
//
// Stance reverse-grep targets:
//   - 设计 ① 物理拆死: 不动 channels / channel_groups (此文件不 import
//     channel_groups package; 反约束 grep `channel_groups` count==0).
//   - 设计 ② 个人偏好两维 collapsed + position: 入参字段反约束断言 (反向
//     reject hidden / muted / pinned / group_id 字段).
//   - 设计 ③ pin = position 单调小数: server 不算 MIN-1.0 (client 端事,
//     设计 ⑥). server 仅 store; reject DM channel (设计 ④).
//   - 设计 ④ DM 永不参与分组: channel.type IN ('private','public') 校验
//     → 400 `layout.dm_not_grouped` byte-identical (#357 spec / #353
//     acceptance §2 / #366 ④ / #402 ⑤ 5 源).
//   - 设计 ⑤ ADM-0 红线: admin god-mode endpoint **不返回**
//     user_channel_layout 行 (本文件不挂 admin 路径; 反约束 grep
//     `admin.*user_channel_layout` 在 admin*.go count==0).
//   - 设计 ⑥ ordering client 端: server 不算偏好排序, 也不 push fanout
//     LayoutChangedFrame (本文件无 hub.Broadcast 调用; 反约束 grep
//     `WSEnvelope.*position|push.*frame.*position|fanout.*user_channel_layout`
//     在 ws/ count==0 + 本文件 count==0).
//   - 设计 ⑦ lazy 清理: 作者删 group → 不级联清理 user_channel_layout
//     (本文件不订阅 channel.delete event; 反约束 grep
//     `cascade.*delete.*user_channel_layout` count==0).
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"borgee-server/internal/store"
)

// LayoutHandler handles personal-layout-preference endpoints.
type LayoutHandler struct {
	Store  *store.Store
	Logger *slog.Logger
	Now    func() time.Time // injectable clock for tests; defaults to time.Now.
}

func (h *LayoutHandler) now() int64 {
	if h.Now != nil {
		return h.Now().UnixMilli()
	}
	return time.Now().UnixMilli()
}

func (h *LayoutHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }
	mux.Handle("GET /api/v1/me/layout", wrap(h.handleGetMyLayout))
	mux.Handle("PUT /api/v1/me/layout", wrap(h.handlePutMyLayout))
}

// userChannelLayoutRow is the storage shape (mirrors migration v=19).
type userChannelLayoutRow struct {
	UserID    string  `gorm:"column:user_id"     json:"-"`
	ChannelID string  `gorm:"column:channel_id"  json:"channel_id"`
	Collapsed int64   `gorm:"column:collapsed"   json:"collapsed"`
	Position  float64 `gorm:"column:position"    json:"position"`
	CreatedAt int64   `gorm:"column:created_at"  json:"created_at"`
	UpdatedAt int64   `gorm:"column:updated_at"  json:"updated_at"`
}

// ----- GET /api/v1/me/layout -----
//
// Returns the caller's personal layout rows. ACL: 本人写本人读, no
// admin path (设计 ⑤ ADM-0 §1.3 红线 — 此 endpoint 不接受 admin token,
// admin SPA 用 /admin-api/* 走另一条 mux, 本路径 admin 401 by mw).
//
// Acceptance §2.1: 200 with `{"layout": [...]}` (空数组 if 无偏好);
// fallback ordering 是 client 端事 (设计 ⑥), server 不补全缺失行.
func (h *LayoutHandler) handleGetMyLayout(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	var rows []userChannelLayoutRow
	if err := h.Store.DB().Raw(
		`SELECT user_id, channel_id, collapsed, position, created_at, updated_at
		 FROM user_channel_layout
		 WHERE user_id = ?
		 ORDER BY position ASC`, user.ID).Scan(&rows).Error; err != nil {
		h.logErr("layout get", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to load layout")
		return
	}
	if rows == nil {
		rows = []userChannelLayoutRow{}
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"layout": rows})
}

// ----- PUT /api/v1/me/layout -----
//
// Batch upsert. Body shape:
//
//	{"layout": [{channel_id, collapsed, position}, ...]}
//
// 反约束 grep 锚 (#402 ⑤ + #366 ④ + #357 §1 设计 ② + #353 acceptance
// §2.3 5 源 byte-identical):
//   - DM channel_id → 400 with code `layout.dm_not_grouped` (字面禁
//     "升级为频道" / "Convert to channel" / "升级 DM" 同义词漂; 错码
//     `layout.dm_not_grouped` 5 源 byte-identical).
//   - non-member channel_id → 403 (设计 ⑦ + CHN-1 channel ACL 同源).
//   - 输入字段反约束 (设计 ②): 仅接受 channel_id / collapsed / position
//     三字段; hidden / muted / pinned / group_id 字面忽略 (不接受写入,
//     反约束 lint 锚: 无 alias 字段名).
//
// Failure surface (acceptance §3.5 + 文案锁 ④):
//   - 400 `layout.invalid_payload` (空 body / 非 JSON / 字段缺失)
//   - 400 `layout.dm_not_grouped` (DM channel)
//   - 403 (non-member channel — 跟 CHN-1 ACL 同源, 不暴露细节)
//   - 500 with msg "侧栏顺序保存失败, 请重试" byte-identical
//     (#371 / acceptance §3.5 / #402 ④ 三源 toast 文案锁)
const layoutSaveErrorMsg = "侧栏顺序保存失败, 请重试"

type layoutPutRow struct {
	ChannelID string  `json:"channel_id"`
	Collapsed int64   `json:"collapsed"`
	Position  float64 `json:"position"`
}

type layoutPutRequest struct {
	Layout []layoutPutRow `json:"layout"`
}

func (h *LayoutHandler) handlePutMyLayout(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	var req layoutPutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorCode(w, http.StatusBadRequest, "layout.invalid_payload", "invalid JSON body")
		return
	}
	if req.Layout == nil {
		writeJSONErrorCode(w, http.StatusBadRequest, "layout.invalid_payload", "layout field required")
		return
	}

	// Pre-validate every row BEFORE writing — atomic accept-or-reject so
	// partial writes don't leak cross-row drift. acceptance §2.4 字面.
	// REFACTOR-1 R1.1: per-row 4-step preamble 走 requireChannelMember
	// helper-1 (RejectDM=true + member-only). channel_id="" 仍 user-rail
	// 直查 (helper 不接 empty path; layout PUT 设计 ⑤ invalid_payload 字面).
	for _, row := range req.Layout {
		if row.ChannelID == "" {
			writeJSONErrorCode(w, http.StatusBadRequest, "layout.invalid_payload", "channel_id required")
			return
		}
		if _, _, ok := requireChannelMember(w, r, h.Store, row.ChannelID, ChannelACLOpts{RejectDM: true}); !ok {
			return
		}
	}

	now := h.now()
	// Per-row UPSERT — SQLite ON CONFLICT(user_id, channel_id) DO UPDATE.
	// position 是 REAL (REAL_TYPE), client 算 MIN-1.0 (设计 ③ pin =
	// 单调小数, server 不算; 设计 ⑥ ordering client 端).
	for _, row := range req.Layout {
		if err := h.Store.DB().Exec(`INSERT INTO user_channel_layout
			(user_id, channel_id, collapsed, position, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(user_id, channel_id) DO UPDATE SET
			  collapsed = excluded.collapsed,
			  position  = excluded.position,
			  updated_at = excluded.updated_at`,
			user.ID, row.ChannelID, row.Collapsed, row.Position, now, now).Error; err != nil {
			h.logErr("layout upsert", err)
			// 设计 ⑥ 文案锁 — toast 文案 byte-identical 跟 #371 / acceptance
			// §3.5 / #402 ④ 三源.
			writeJSONError(w, http.StatusInternalServerError, layoutSaveErrorMsg)
			return
		}
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *LayoutHandler) logErr(op string, err error) {
	if h.Logger != nil {
		h.Logger.Error("layout error", "op", op, "err", err)
	}
}

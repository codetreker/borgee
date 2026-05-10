// Package api — admin_audit_query.go: ADM-3 multi-source audit 合并查询.
//
// Spec: docs/implementation/modules/adm-3-spec.md §1 ADM3.1.
// Blueprint: admin-model.md §1.4 来源透明 (人/agent/admin/混合).
//
// 设计 (跟 DL-2 #615 events 双流 + ADM-2 #484 audit_events 同精神一致):
//   - 4 source enum 单一来源 (`AuditSourceServer/Plugin/HostBridge/Agent`),
//     反 inline 字面脱节 (跟 reasons.IsValid #496 / NAMING-1 / DL-2
//     mustPersistKinds 同精神一致).
//   - 0 schema 改 — UNION ALL 跨 4 源既有表 (audit_events / channel_events
//     / global_events / host_bridge placeholder), query 层合并不拆表.
//   - admin god-mode 路径独立 — 仅 /admin-api/v1/audit/multi-source 暴露,
//     反 user-rail 脱节 (ADM-0 §1.3 红线).
//   - LIMIT 100 + ORDER BY ts DESC v1 简单兜底, 反 N+1 / 反 cross-source
//     scan 脱节.

package api

import (
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"

	"borgee-server/internal/admin"
	"borgee-server/internal/store"
)

// AuditSource 4 类 enum 单一来源 (蓝图 §1.4 来源透明 byte-identical).
// 改这里 = 改 client i18n key + content-lock §1 字面三处.
const (
	AuditSourceServer     = "server"
	AuditSourcePlugin     = "plugin"
	AuditSourceHostBridge = "host_bridge"
	AuditSourceAgent      = "agent"
)

// AuditSources is the canonical 4-element ordering used by client filters
// + i18n. Reverse grep guard: grep `AuditSources` count==1 (单一来源).
var AuditSources = []string{
	AuditSourceServer,
	AuditSourcePlugin,
	AuditSourceHostBridge,
	AuditSourceAgent,
}

// MultiSourceAuditRow 是合并后的 单一来源 响应 shape (4 字段, 跟 5-field
// audit JSON-line schema 同精神 — actor/action/target/when/scope 蓝图
// §1.4 byte-identical).
type MultiSourceAuditRow struct {
	Source    string `json:"source"`     // 4 enum 之一 (反字面脱节)
	TS        int64  `json:"ts"`         // Unix ms epoch
	Actor     string `json:"actor"`      // actor_id / kind / topic 来源 string
	Action    string `json:"action"`     // action / kind 字面
	Payload   string `json:"payload"`    // metadata / payload JSON string
}

// MultiSourceAuditFilter 收 query string (?source / ?since / ?until /
// ?limit). source="" → 全 4 源 UNION ALL.
type MultiSourceAuditFilter struct {
	Source string // "" or one of AuditSources
	Since  *int64 // ms epoch
	Until  *int64
	Limit  int
}

// AdminAuditMultiSourceHandler hosts the admin-rail multi-source audit query.
// 复用 ADM-2 既有 admin_endpoints.go AdminFromContext + adminMw 模式.
type AdminAuditMultiSourceHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterAdminRoutes wires GET /admin-api/v1/audit/multi-source behind adminMw.
// 设计 ③ admin god-mode 路径独立 (反 user-rail 脱节).
func (h *AdminAuditMultiSourceHandler) RegisterAdminRoutes(mux *http.ServeMux, adminMw func(http.Handler) http.Handler) {
	mux.Handle("GET /admin-api/v1/audit/multi-source", adminMw(http.HandlerFunc(h.handle)))
}

func (h *AdminAuditMultiSourceHandler) handle(w http.ResponseWriter, r *http.Request) {
	// admin gate (走 adminMw, 此处再 read context defense-in-depth, 跟 ADM-2
	// handleAdminAuditLog 同模式一致).
	a := admin.AdminFromContext(r.Context())
	if a == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	q := r.URL.Query()
	filter := MultiSourceAuditFilter{
		Source: q.Get("source"),
		Limit:  parseLimit(r, 100, 500),
	}
	if v := q.Get("since"); v != "" {
		ms, err := strconv.ParseInt(v, 10, 64)
		if err != nil || ms < 0 {
			writeJSONError(w, http.StatusBadRequest, "audit.time_range_invalid")
			return
		}
		filter.Since = &ms
	}
	if v := q.Get("until"); v != "" {
		ms, err := strconv.ParseInt(v, 10, 64)
		if err != nil || ms < 0 {
			writeJSONError(w, http.StatusBadRequest, "audit.time_range_invalid")
			return
		}
		filter.Until = &ms
	}
	if filter.Source != "" && !validAuditSource(filter.Source) {
		writeJSONError(w, http.StatusBadRequest, "audit.source_invalid")
		return
	}
	rows, err := MultiSourceAuditQuery(h.Store, filter)
	if err != nil {
		h.Logger.Error("adm3.multi_source_query_failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"sources": AuditSources,
		"rows":    rows,
	})
}

// validAuditSource reports whether s is one of the 4 enum (反字面散布).
func validAuditSource(s string) bool {
	for _, v := range AuditSources {
		if v == s {
			return true
		}
	}
	return false
}

// MultiSourceAuditQuery executes the UNION ALL across 4 source tables.
// audit_events (server) — actor_id/action/target_user_id/metadata/created_at
// audit_events with action like 'plugin.%' (plugin) — same table, kind 区分
// channel_events / global_events (agent) — DL-2 #615 双流 (channel_id/kind/payload/created_at)
// host_bridge — placeholder until HB-1 audit table lands (留 HB-1 后续,
//               v1 0 行返回, 反空 UNION 跑空查反 SQL syntax err).
//
// 设计 ② 单一来源 — 单 helper, 反多处散布.
func MultiSourceAuditQuery(s *store.Store, f MultiSourceAuditFilter) ([]MultiSourceAuditRow, error) {
	rows := make([]MultiSourceAuditRow, 0, f.Limit)
	include := func(src string) bool {
		return f.Source == "" || f.Source == src
	}

	if include(AuditSourceServer) || include(AuditSourcePlugin) {
		// audit_events covers both server (admin actions) + plugin (BPP-8
		// lifecycle stamped via action kind 'plugin.*'). We split by
		// action prefix at projection time.
		serverRows, err := queryAuditEvents(s, f)
		if err != nil {
			return nil, err
		}
		for _, r := range serverRows {
			src := AuditSourceServer
			// BPP-8 #532 plugin lifecycle actions share audit_events table;
			// distinguish by action prefix `plugin_*` (DB CHECK enum).
			if len(r.Action) >= 7 && r.Action[:7] == "plugin_" {
				src = AuditSourcePlugin
			}
			if !include(src) {
				continue
			}
			r.Source = src
			rows = append(rows, r)
		}
	}

	if include(AuditSourceAgent) {
		// DL-2 #615 channel_events + global_events (agent-emitted kinds).
		agentRows, err := queryAgentEvents(s, f)
		if err != nil {
			return nil, err
		}
		rows = append(rows, agentRows...)
	}

	// host_bridge: HB-1 audit table 未落 v1 (留 HB-1 后续, 反向约束:
	// MultiSourceAuditQuery 不假设表存在). 当前 0 行, 占位反 4 源缺漏.
	_ = include(AuditSourceHostBridge)

	// Trim to LIMIT after merge (per-source LIMIT could miss recent rows
	// from a sparse source). v1 简单兜底, 阈值哨触发后调.
	sortByTSDesc(rows)
	if len(rows) > f.Limit {
		rows = rows[:f.Limit]
	}
	return rows, nil
}

func queryAuditEvents(s *store.Store, f MultiSourceAuditFilter) ([]MultiSourceAuditRow, error) {
	q := `SELECT actor_id, action, target_user_id, metadata, created_at FROM audit_events`
	args := []any{}
	where := ""
	addClause := func(clause string, a ...any) {
		if where == "" {
			where = " WHERE " + clause
		} else {
			where += " AND " + clause
		}
		args = append(args, a...)
	}
	if f.Since != nil {
		addClause("created_at >= ?", *f.Since)
	}
	if f.Until != nil {
		addClause("created_at <= ?", *f.Until)
	}
	q += where + ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, f.Limit)

	sqlRows, err := s.DB().Raw(q, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()
	out := []MultiSourceAuditRow{}
	for sqlRows.Next() {
		var actorID sql.NullString
		var action sql.NullString
		var target sql.NullString
		var meta sql.NullString
		var ts sql.NullInt64
		if err := sqlRows.Scan(&actorID, &action, &target, &meta, &ts); err != nil {
			return nil, err
		}
		actor := actorID.String
		if target.String != "" {
			actor = actorID.String + "→" + target.String
		}
		out = append(out, MultiSourceAuditRow{
			TS:      ts.Int64,
			Actor:   actor,
			Action:  action.String,
			Payload: meta.String,
		})
	}
	return out, sqlRows.Err()
}

func queryAgentEvents(s *store.Store, f MultiSourceAuditFilter) ([]MultiSourceAuditRow, error) {
	// UNION ALL channel_events + global_events. Both share lex_id/kind/
	// payload/created_at; channel_events adds channel_id which we project
	// into actor.
	q := `SELECT 'channel:' || channel_id AS actor, kind, payload, created_at FROM channel_events
	      UNION ALL
	      SELECT 'global' AS actor, kind, payload, created_at FROM global_events`
	q = `SELECT actor, kind, payload, created_at FROM (` + q + `)`
	args := []any{}
	where := ""
	if f.Since != nil {
		where = " WHERE created_at >= ?"
		args = append(args, *f.Since)
	}
	if f.Until != nil {
		if where == "" {
			where = " WHERE created_at <= ?"
		} else {
			where += " AND created_at <= ?"
		}
		args = append(args, *f.Until)
	}
	q += where + ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, f.Limit)

	sqlRows, err := s.DB().Raw(q, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()
	out := []MultiSourceAuditRow{}
	for sqlRows.Next() {
		var actor, kind, payload sql.NullString
		var ts sql.NullInt64
		if err := sqlRows.Scan(&actor, &kind, &payload, &ts); err != nil {
			return nil, err
		}
		out = append(out, MultiSourceAuditRow{
			Source:  AuditSourceAgent,
			TS:      ts.Int64,
			Actor:   actor.String,
			Action:  kind.String,
			Payload: payload.String,
		})
	}
	return out, sqlRows.Err()
}

// sortByTSDesc sorts in-place by TS descending (newest first).
func sortByTSDesc(rows []MultiSourceAuditRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j-1].TS < rows[j].TS; j-- {
			rows[j-1], rows[j] = rows[j], rows[j-1]
		}
	}
}

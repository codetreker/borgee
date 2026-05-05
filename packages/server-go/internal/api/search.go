// Package api — search.go: CV-6 server handler for artifact full-text
// search via SQLite FTS5 (Phase 5+, #cv-6).
//
// Blueprint: docs/blueprint/current/canvas-vision.md §1.4 (artifact 集合, "首屏快读
// 不是浏览器内全量解码"). Spec brief:
// docs/implementation/modules/cv-6-spec.md (战马C v0, d2fe1f0).
//
// 立场反查 (3 立场 + 6 边界):
//   - ① 复用 SQLite FTS5 (不另起 elasticsearch / opensearch / typesense /
//     meilisearch / sonic / bleve search service); contentless virtual
//     table 跟 artifacts 单源 SSOT.
//   - ② search owner-only — channel-scoped (channel.created_by gate);
//     非 member → 403 + search.channel_not_member; cross-org → 403 (走
//     AP-3 #521 cross-org gate 自动经 HasCapability 单源).
//   - ⑥ archived_at IS NOT NULL 不出现 (CV-1 archive 既有不变量).
//   - ⑨ snippet() server-side 高亮 `<mark>` 字面 (跟 client 既有 markdown
//     sanitize path 兼容, 不另起 client 高亮 lib).
//
// Endpoint:
//
//	GET /api/v1/artifacts/search?q=<query>&channel_id=<id>&limit=<n>
//
// 反约束 (cv-6-spec.md §3 反约束 grep):
//   - 不另起 search 表 (FTS5 contentless 跟 artifacts 单源).
//   - 不引入 elasticsearch / opensearch / typesense / meilisearch / sonic
//     / bleve / blevesearch (反向 grep go.mod count==0 by 7 keyword).
//   - 不暴露其他 owner artifact (search.cross_owner / search.all_artifacts
//     反向 grep count==0).
//   - 错码字面单源 (跟 AP-1/AP-2/AP-3/CV-2 v2/CV-3 v2 const 同模式).
package api

import (
	"net/http"
	"strconv"
	"strings"

	"borgee-server/internal/auth"
)

// SearchErrCode constants — byte-identical 跟 cv-6-spec.md §0 立场 ④
// + content-lock §1 + cv-6-content-lock.md §4 同源 (改 = 改三处: const +
// client toast map + content-lock).
const (
	SearchErrCodeNotOwner         = "search.not_owner"
	SearchErrCodeChannelNotMember = "search.channel_not_member"
	SearchErrCodeQueryEmpty       = "search.query_empty"
	SearchErrCodeQueryTooLong     = "search.query_too_long"
	SearchErrCodeCrossOrgDenied   = "search.cross_org_denied"
)

// SearchQueryMaxLen — 256 字符上限 (跟 content-lock §2 maxlength byte-
// identical, 反 DoS query 拒先于 FTS5 查询).
const SearchQueryMaxLen = 256

// SearchDefaultLimit / SearchMaxLimit — 50 默认 / 200 上限.
const (
	SearchDefaultLimit = 50
	SearchMaxLimit     = 200
)

type searchResult struct {
	ArtifactID    string `json:"artifact_id"`
	Title         string `json:"title"`
	Snippet       string `json:"snippet"`
	Kind          string `json:"kind"`
	ChannelID     string `json:"channel_id"`
	CurrentVersion int64 `json:"current_version"`
}

// handleArtifactSearch implements GET /api/v1/artifacts/search.
//
// 反约束守 (立场 ①②③④⑤⑥):
//   - admin (no auth user) → 401 (admin god-mode 不入业务路径).
//   - q empty → 400 + search.query_empty.
//   - q > 256 chars → 400 + search.query_too_long.
//   - channel_id provided + non-member → 403 + search.channel_not_member.
//   - cross-org user (走 AP-3 HasCapability 自动 enforce) → 403 (AP-3
//     立场 ① 同源, search 路径自动经).
//   - archived_at IS NOT NULL 不出现 (立场 ⑥).
//   - server-side snippet `<mark>...</mark>` 字面 byte-identical (跟
//     content-lock §3 ResultList row 同精神).
func (h *ArtifactHandler) handleArtifactSearch(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSONError(w, http.StatusBadRequest, SearchErrCodeQueryEmpty+": query is required")
		return
	}
	if len(q) > SearchQueryMaxLen {
		writeJSONError(w, http.StatusBadRequest, SearchErrCodeQueryTooLong+": query exceeds 256 chars")
		return
	}

	channelID := strings.TrimSpace(r.URL.Query().Get("channel_id"))
	limit := SearchDefaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			if n > SearchMaxLimit {
				n = SearchMaxLimit
			}
			limit = n
		}
	}

	// 立场 ② channel-scoped — channel_id 必填 (v0 不开 cross-channel global
	// search, 留 v2+; cv-6-spec.md §4 不在范围). 反约束: 不暴露其他 owner
	// artifact.
	if channelID == "" {
		writeJSONError(w, http.StatusBadRequest, SearchErrCodeQueryEmpty+": channel_id is required (v0)")
		return
	}

	// 立场 ② channel membership gate (跟 CV-1.2 既有 path).
	if !h.canAccessChannel(channelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, SearchErrCodeChannelNotMember+": not a channel member")
		return
	}

	// 立场 ⑤ AP-3 cross-org gate 走 HasCapability 单源 (auto-enforce 当 AP-3
	// merged; 在此先走 channel ACL gate 保护 — HasCapability 失败统一映射
	// search.cross_org_denied 错码字面 byte-identical 跟 content-lock §1).
	if !auth.HasCapability(r.Context(), h.Store, auth.ReadArtifact, auth.ChannelScopeStr(channelID)) {
		writeJSONError(w, http.StatusForbidden, SearchErrCodeCrossOrgDenied+": cross-org or capability denied")
		return
	}

	// FTS5 query: snippet() args 5 byte-identical (跟 content-lock §1 +
	// stance 立场 ⑧): col=1 (body), prefix='<mark>', suffix='</mark>',
	// ellipsis='...', tokens=32.
	type row struct {
		ID             string  `gorm:"column:id"`
		Title          string  `gorm:"column:title"`
		Snippet        string  `gorm:"column:snippet"`
		Type           string  `gorm:"column:type"`
		ChannelID      string  `gorm:"column:channel_id"`
		CurrentVersion int64   `gorm:"column:current_version"`
		ArchivedAt     *int64  `gorm:"column:archived_at"`
	}
	var rows []row
	// JOIN artifacts via rowid (FTS5 contentless content_rowid).
	// WHERE channel_id = ? AND archived_at IS NULL — 立场 ②⑥.
	if err := h.Store.DB().Raw(`
		SELECT a.id, a.title,
		       snippet(artifacts_fts, 1, '<mark>', '</mark>', '...', 32) AS snippet,
		       a.type, a.channel_id, a.current_version, a.archived_at
		FROM artifacts_fts
		JOIN artifacts a ON a.rowid = artifacts_fts.rowid
		WHERE artifacts_fts MATCH ?
		  AND a.channel_id = ?
		  AND a.archived_at IS NULL
		ORDER BY rank
		LIMIT ?`,
		q, channelID, limit,
	).Scan(&rows).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, "search query failed")
		return
	}

	results := make([]searchResult, 0, len(rows))
	for _, r := range rows {
		results = append(results, searchResult{
			ArtifactID:     r.ID,
			Title:          r.Title,
			Snippet:        r.Snippet,
			Kind:           r.Type,
			ChannelID:      r.ChannelID,
			CurrentVersion: r.CurrentVersion,
		})
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"query":   q,
		"results": results,
		"total":   len(results),
	})
}

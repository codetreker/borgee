// Package api — artifact_comments.go: CV-5 server API for artifact-level
// comment threads (canvas-vision §0 L24 字面 "Linear issue + comment, 不是
// Miro 白板").
//
// Blueprint锚: docs/blueprint/current/canvas-vision.md L24 + DM-2.2 #372 mention
// router 同精神 (comment / message 同表同语义不裂) + RT-3 #488 cursor 共序锚
// + thinking subject 5-pattern 反约束链 (RT-3 / BPP-2.2 / AL-1b / CV-5 第 4
// 处). Spec brief: docs/implementation/modules/cv-5-spec.md (战马E v0
// 857170d, 3 条原则 + 3 拆段).
//
// Endpoints (cv-5-spec.md §1 字面):
//
//	POST /api/v1/artifacts/{artifactId}/comments  create comment (channel-scoped)
//	GET  /api/v1/artifacts/{artifactId}/comments  list comments
//
// 设计反查 (cv-5-spec.md §0):
//
//   - ① comment 走 messages 表单源 — 不另起 artifact_comments 表; comment
//     row 落 messages 表 + channel_id 走虚拟 `artifact:<artifact_id>` namespace
//     channel (跟 DM-2 #312 dm: namespace 同模式 — 字面 prefix 在 channel.name,
//     channel.id 仍是 UUID).
//   - ② frame `artifact_comment_added` 走 RT-3 #488 hub.cursors 共序 +
//     BodyPreview 80 rune cap (跟 DM-2.2 MentionPushedFrame 同精神).
//   - ③ agent comment 必带 thinking subject — server reject 400
//     `comment.thinking_subject_required` 当 sender_role==agent + body
//     字面 5-pattern 任一. 5-pattern 改 = 改 4 处 (RT-3 / BPP-2.2 / AL-1b /
//     CV-5 byte-identical 同步).
//
// admin (god-mode) cookie 不入此 rail (跟 ADM-0 §1.3 红线一致).
package api

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"


	"borgee-server/internal/idgen"
	"gorm.io/gorm"

	"borgee-server/internal/store"
)

// CV-5 设计 ② frame type discriminator (源 ws.FrameTypeArtifactCommentAdded).
const FrameTypeArtifactCommentAdded = "artifact_comment_added"

// CV-5 设计 ① channel name namespace prefix (跟 DM-2 dm: 同模式).
// grep 检查 `channel.*name.*"artifact:"` ≥ 1 hit 在此处.
const ArtifactCommentChannelNamePrefix = "artifact:"

// CV-5 设计 ② BodyPreview cap (跟 DM-2.2 MentionPushed 80 rune 同 cap, 隐私 §13).
const ArtifactCommentBodyPreviewMaxRunes = 80

// CV-5 设计 ③ error code constants (跟 DM-2.2 mention.target_not_in_channel 同模式).
const (
	ArtifactCommentErrThinkingSubjectRequired = "comment.thinking_subject_required"
	ArtifactCommentErrTargetNotFound          = "comment.target_artifact_not_found"
	ArtifactCommentErrCrossChannelReject      = "comment.cross_channel_reject"
)

// thinking 5-pattern reverse-grep 第 4 处链 (RT-3 / BPP-2.2 / AL-1b / CV-5
// byte-identical). server-side body validator: agent sender body 字面命中
// 任一 → reject. 5 patterns:
//  1. body 末 "thinking$" — body trimmed ends with literal "thinking"
//  2. defaultSubject literal — sentinel marker (placeholder leak)
//  3. fallbackSubject literal — sentinel marker (placeholder leak)
//  4. "AI is thinking" — well-known fallback string
//  5. subject="" 空字符串 — empty body / whitespace-only body
//
// 反约束: 改 5-pattern 字面 = 同步改 4 处 (RT-3 + BPP-2.2 + AL-1b + CV-5).
var thinkingSubjectSentinels = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bthinking\s*$`),
	regexp.MustCompile(`defaultSubject`),
	regexp.MustCompile(`fallbackSubject`),
	regexp.MustCompile(`AI is thinking`),
}

// validateAgentCommentSubject — CV-5 设计 ③: returns true (== violation) when
// agent body fails the 5-pattern guard. Pattern 5 (subject="") is the
// empty-body case handled separately so the caller can return a clearer message.
func violatesThinkingSubject(body string) bool {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return true // pattern 5: subject="" 空字符串
	}
	for _, re := range thinkingSubjectSentinels {
		if re.MatchString(trimmed) {
			return true
		}
	}
	return false
}

// ArtifactCommentPusher is the seam between api and ws.Hub for the
// `artifact_comment_added` frame (mirrors AnchorCommentPusher pattern,
// CV-2.2 anchors.go). Cursor goes through hub.cursors.NextCursor —
// shared sequence with RT-1.1 / RT-3 / DM-2.2 / BPP-2 / BPP-3.1.
type ArtifactCommentPusher interface {
	PushArtifactCommentAdded(
		commentID string,
		artifactID string,
		channelID string,
		senderID string,
		senderRole string,
		bodyPreview string,
		createdAt int64,
	) (cursor int64, sent bool)
}

// ArtifactCommentsHandler exposes the CV-5.1 HTTP surface.
type ArtifactCommentsHandler struct {
	Store  *store.Store
	Logger *slog.Logger
	Pusher ArtifactCommentPusher
	Now    func() time.Time
	NewID  func() string
}

func (h *ArtifactCommentsHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *ArtifactCommentsHandler) newID() string {
	if h.NewID != nil {
		return h.NewID()
	}
	return idgen.NewID()
}

func (h *ArtifactCommentsHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }
	mux.Handle("POST /api/v1/artifacts/{artifactId}/comments", wrap(h.handleCreateComment))
	mux.Handle("GET /api/v1/artifacts/{artifactId}/comments", wrap(h.handleListComments))
}

// loadArtifactRow — minimal artifact lookup (subset of artifacts.go::loadArtifact).
type artifactCommentArtifactRow struct {
	ID        string
	ChannelID string `gorm:"column:channel_id"`
}

func (h *ArtifactCommentsHandler) loadArtifact(id string) (*artifactCommentArtifactRow, error) {
	var rows []artifactCommentArtifactRow
	err := h.Store.DB().Raw(`SELECT id, channel_id FROM artifacts WHERE id = ?`, id).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &rows[0], nil
}

// ensureArtifactChannel — CV-5 设计 ①: get-or-create the virtual `artifact:`
// namespace channel (跟 DM-2 dm: 同模式: 真 channel row, name 走 namespace
// prefix, id 仍是 UUID). On first call:
//   - INSERT channels row name="artifact:<artifactId>" type="artifact" visibility="private" created_by=hostChannel.CreatedBy
//   - Copy host channel members (artifact channel 自动 ACL = host members).
//
// Subsequent calls return the existing row.
func (h *ArtifactCommentsHandler) ensureArtifactChannel(artifactID, hostChannelID string) (*store.Channel, error) {
	wantName := ArtifactCommentChannelNamePrefix + artifactID
	var existing store.Channel
	err := h.Store.DB().Where("name = ? AND type = ? AND deleted_at IS NULL", wantName, "artifact").First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	host, err := h.Store.GetChannelByID(hostChannelID)
	if err != nil {
		return nil, err
	}
	now := h.now().UnixMilli()
	ch := &store.Channel{
		ID:         h.newID(),
		Name:       wantName,
		Topic:      "",
		Visibility: "private",
		CreatedAt:  now,
		CreatedBy:  host.CreatedBy,
		Type:       "artifact",
		Position:   "0|aaaaaa",
		OrgID:      host.OrgID,
	}
	if err := h.Store.DB().Create(ch).Error; err != nil {
		return nil, err
	}
	// Copy host channel members so artifact comment ACL byte-identical to
	// host channel ACL (设计 ④ ACL 继承).
	members, err := h.Store.ListChannelMembers(hostChannelID)
	if err == nil {
		for _, m := range members {
			_ = h.Store.AddChannelMember(&store.ChannelMember{
				ChannelID:   ch.ID,
				UserID:      m.UserID,
				JoinedAt:    now,
				Silent:      m.Silent,
				OrgIDAtJoin: m.OrgIDAtJoin,
			})
		}
	}
	return ch, nil
}

// senderRoleFor — 'agent' or 'human' (mirrors ArtifactHandler.committerKindForUser).
func (h *ArtifactCommentsHandler) senderRoleFor(u *store.User) string {
	if u != nil && u.Role == "agent" {
		return "agent"
	}
	return "human"
}

// truncateBodyPreview — 80-rune UTF-8 safe cap (mirrors ws.TruncateBodyPreview).
func truncateArtifactCommentBodyPreview(body string) string {
	runes := []rune(body)
	if len(runes) <= ArtifactCommentBodyPreviewMaxRunes {
		return body
	}
	return string(runes[:ArtifactCommentBodyPreviewMaxRunes])
}

// ----- POST /api/v1/artifacts/{artifactId}/comments -----

type createArtifactCommentRequest struct {
	Body    string `json:"body"`
	AgentID string `json:"agent_id,omitempty"` // optional — when human user posts on behalf of an agent
}

type artifactCommentResponse struct {
	ID         string `json:"id"`
	ArtifactID string `json:"artifact_id"`
	ChannelID  string `json:"channel_id"`
	SenderID   string `json:"sender_id"`
	SenderRole string `json:"sender_role"`
	Body       string `json:"body"`
	CreatedAt  int64  `json:"created_at"`
	Cursor     int64  `json:"cursor,omitempty"`
}

func (h *ArtifactCommentsHandler) handleCreateComment(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	artifactID := r.PathValue("artifactId")
	if artifactID == "" {
		writeJSONErrorCode(w, http.StatusBadRequest, ArtifactCommentErrTargetNotFound, "artifactId required")
		return
	}
	art, err := h.loadArtifact(artifactID)
	if err != nil {
		writeJSONErrorCode(w, http.StatusNotFound, ArtifactCommentErrTargetNotFound, "artifact not found")
		return
	}
	// 设计 ④ cross-channel reject — caller must be member of the host channel.
	if !h.Store.CanAccessChannel(art.ChannelID, user.ID) {
		writeJSONErrorCode(w, http.StatusForbidden, ArtifactCommentErrCrossChannelReject, "not a member of artifact's channel")
		return
	}

	var req createArtifactCommentRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	senderRole := h.senderRoleFor(user)
	// 设计 ③ thinking subject 反约束 (5-pattern 第 4 处链).
	if senderRole == "agent" && violatesThinkingSubject(req.Body) {
		writeJSONErrorCode(w, http.StatusBadRequest, ArtifactCommentErrThinkingSubjectRequired,
			"agent comment must carry a concrete subject (thinking-only body rejected)")
		return
	}
	if strings.TrimSpace(req.Body) == "" {
		writeJSONError(w, http.StatusBadRequest, "body is required")
		return
	}

	// 设计 ① ensure virtual artifact: namespace channel.
	ch, err := h.ensureArtifactChannel(art.ID, art.ChannelID)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("ensureArtifactChannel failed", "err", err, "artifact_id", art.ID)
		}
		writeJSONError(w, http.StatusInternalServerError, "create artifact channel failed")
		return
	}

	now := h.now().UnixMilli()
	msg := &store.Message{
		ID:          h.newID(),
		ChannelID:   ch.ID,
		SenderID:    user.ID,
		Content:     req.Body,
		ContentType: "artifact_comment",
		CreatedAt:   now,
		OrgID:       user.OrgID,
	}
	if err := h.Store.CreateMessage(msg); err != nil {
		if h.Logger != nil {
			h.Logger.Error("CreateMessage failed", "err", err, "artifact_id", art.ID)
		}
		writeJSONError(w, http.StatusInternalServerError, "create comment failed")
		return
	}

	preview := truncateArtifactCommentBodyPreview(req.Body)
	var cursor int64
	if h.Pusher != nil {
		cursor, _ = h.Pusher.PushArtifactCommentAdded(
			msg.ID, art.ID, ch.ID, user.ID, senderRole, preview, now,
		)
	}

	writeJSONResponse(w, http.StatusCreated, artifactCommentResponse{
		ID:         msg.ID,
		ArtifactID: art.ID,
		ChannelID:  ch.ID,
		SenderID:   user.ID,
		SenderRole: senderRole,
		Body:       req.Body,
		CreatedAt:  now,
		Cursor:     cursor,
	})
}

// ----- GET /api/v1/artifacts/{artifactId}/comments -----

func (h *ArtifactCommentsHandler) handleListComments(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	artifactID := r.PathValue("artifactId")
	art, err := h.loadArtifact(artifactID)
	if err != nil {
		writeJSONErrorCode(w, http.StatusNotFound, ArtifactCommentErrTargetNotFound, "artifact not found")
		return
	}
	if !h.Store.CanAccessChannel(art.ChannelID, user.ID) {
		writeJSONErrorCode(w, http.StatusForbidden, ArtifactCommentErrCrossChannelReject, "not a member of artifact's channel")
		return
	}
	// Look up the artifact: namespace channel; if it doesn't exist yet, return empty list.
	wantName := ArtifactCommentChannelNamePrefix + art.ID
	var ch store.Channel
	err = h.Store.DB().Where("name = ? AND type = ? AND deleted_at IS NULL", wantName, "artifact").First(&ch).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeJSONResponse(w, http.StatusOK, map[string]any{"comments": []any{}})
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "lookup artifact channel failed")
		return
	}
	var msgs []store.Message
	if err := h.Store.DB().Where("channel_id = ? AND deleted_at IS NULL", ch.ID).
		Order("created_at ASC").Find(&msgs).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, "list comments failed")
		return
	}
	out := make([]artifactCommentResponse, 0, len(msgs))
	for _, m := range msgs {
		// Resolve sender role at read time (cheap — sender_id is FK to users).
		role := "human"
		if u, err := h.Store.GetUserByID(m.SenderID); err == nil {
			role = h.senderRoleFor(u)
		}
		out = append(out, artifactCommentResponse{
			ID:         m.ID,
			ArtifactID: art.ID,
			ChannelID:  ch.ID,
			SenderID:   m.SenderID,
			SenderRole: role,
			Body:       m.Content,
			CreatedAt:  m.CreatedAt,
		})
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"comments": out})
}

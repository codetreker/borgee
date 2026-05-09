// Package api — mention_dispatch.go: DM-2.2 server-side parser + fanout
// orchestrator for `@<target_user_id>` mentions.
//
// Blueprint锚: docs/blueprint/current/concept-model.md §4 (agent 代表自己 —
// mention 只 ping target, 不抄送 owner) + §4.1 (离线 fallback owner
// system DM + 节流 5min/(agent,channel) + 不转发原 body) + §13 隐私.
// Spec brief: docs/implementation/modules/dm-2-spec.md §0 设计 ①②③ +
// §1 拆段 DM-2.2 (#312, merged 7de76f9). Acceptance template:
// docs/qa/acceptance-templates/dm-2.md §1.1-§2.4 (#293, merged).
// Content lock: docs/qa/dm-2-content-lock.md §1 ③ (#314, merged) —
// system DM body byte-identical.
// Schema: #361 message_mentions (merged 2d2ac4e).
//
// Three responsibilities locked at this seam (单 PR):
//   1. ParseMentionTargets: regex `@([0-9a-f-]{36})` 抓 token, 跟 §1 ①
//      字面对齐 (UUID v4 lowercase hex). 不复用 store.parseMentionIDs
//      ('<@id>' 旧 token, AP-1 历史路径 — 设计 ⑥ 同语义但 grammar 不同,
//      混用会让反查 grep 路径覆盖不全).
//   2. PersistMentions: 写 message_mentions(message_id, target_user_id);
//      UNIQUE 由 schema 兜 dedup, 设计 ⑥ user / agent 同表同语义.
//      Cross-channel reject 在 dispatcher 入口前置 (api 层 400) 而非这里.
//   3. Dispatch: per-target — IsOnline true → PushMentionPushed (target-only
//      WS, 反约束: 不抄送 owner); IsOnline false → enqueueOwnerSystemDM
//      with 5min/(agent,channel) throttle + body 文案锁 (#314 §1 ③).
//
// 反约束 (spec §0 + §3 + acceptance §2.4 + 野马 #314 ③):
//   - enqueueOwnerSystemDM payload MUST NOT contain raw message body —
//     文案 byte-identical `{agent_name} 当前离线，#{channel} 中有人 @ 了它，
//     你可能需要处理` 仅 `{agent_name}` / `{channel}` 占位; sniff DM
//     body grep 不含 mention 原 body 字符串 (隐私 §13 红线).
//   - body_preview 在 ws frame 路径走 ws.TruncateBodyPreview(80 runes),
//     此 dispatcher 直接传完整 body, frame 层做 cap (避免在 dispatcher
//     重复 trim 漂移).
//   - 节流 in-memory map[(agent_id,channel_id)]lastSentMs — 进程内 5min
//     窗口; 跨进程节流留 Phase 5+ (acceptance §2.3 字面 in-memory clock
//     fixture, dm-2-spec §1 DM-2.2 拆段 "clock fixture, 跟 G2.3 节流模式同").
//   - 不读 anonymous 路径: target.OwnerID 为 nil (即 target 不是 agent
//     而是真人) → 离线时不发 fallback DM (人离线 mention 没有 owner DM
//     需要 ping; 蓝图 §4.1 仅 agent 离线场景).
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"borgee-server/internal/presence"
	"borgee-server/internal/store"
	"borgee-server/internal/ws"


	"borgee-server/internal/idgen"
	"gorm.io/gorm"
)

// MentionTokenRegex matches `@<id>` tokens — either UUID v4 (RFC 4122
// 8-4-4-4-12 lowercase hex) for legacy rows or ULID (Crockford base32, 26
// upper-alnum chars) for ULID-MIGRATION post-#WIRE-1 wave. Word boundary
// kept by `@` literal prefix; downstream resolver tolerates either.
//
// 设计 ⑥ user / agent 同语义: regex 不区分 role, downstream 走
// users.role / users.owner_id 决定 fallback. ULID-MIGRATION forward-compat:
// 既有 UUID + 新 ULID 共存 (db column TEXT, 不限长度).
var MentionTokenRegex = regexp.MustCompile(`@([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|[0-9A-HJKMNP-TV-Z]{26})`)

// OfflineOwnerDMTemplate is the byte-identical fallback DM body locked
// by content-lock #314 §1 ③ + acceptance §2.2. {agent_name} / {channel}
// are the ONLY substitution slots; nothing else may be parameterized
// (隐私 §13: raw message body NEVER 出现在 system DM).
//
// grep anchor: "当前离线，#" + "中有人 @ 了它，你可能需要处理".
const OfflineOwnerDMTemplate = "%s 当前离线，#%s 中有人 @ 了它，你可能需要处理"

// OfflineOwnerDMThrottleWindow caps owner system DM emission to 1 per
// (agent, channel) per window — acceptance §2.3 + spec §1 DM-2.2 5min.
const OfflineOwnerDMThrottleWindow = 5 * time.Minute

// MentionFrameBroadcaster is the WS push surface DM-2.2 needs from the
// hub. Subset of ws.Hub — declared as an interface so api tests can
// inject a fake without spinning up a real cursor allocator + clients.
type MentionFrameBroadcaster interface {
	PushMentionPushed(messageID, channelID, senderID, mentionTargetID, bodyPreview string, createdAt int64) (int64, bool)
}

// MentionPushNotifier is the DL-4.6 cross-device fan-out seam. Mention
// dispatch invokes this for EACH target (online + offline) so users
// receive Web Push on devices where the SPA tab is not focused (browser
// SW handles visibility-based dedup).
//
// Best-effort: implementation MUST NOT propagate errors (跟 ws push 同
// 模式). Implemented by *push.Gateway in production; tests inject a
// recording fake to assert call invocation.
//
// 反约束 (DL-4 spec §0 设计 ②): push fire-and-forget — Notify 返回
// attempts count 仅 observability, 不 error 语义.
type MentionPushNotifier interface {
	NotifyMention(targetUserID, senderID, channelName, bodyPreview string, createdAt int64) int
}

// MentionDispatcher fans out mentions parsed from a fresh message:
//   - Validates targets exist + are members of the channel (caller-side
//     cross-channel reject is done before this — see Validate).
//   - Persists message_mentions rows (#361 schema).
//   - Pushes mention_pushed WS frame to online targets.
//   - Enqueues throttled owner system DM for offline agents.
//
// Concurrency: throttle map guarded by mu; map writes happen on the
// fanout path which is per-message (not hot enough to warrant sharding).
type MentionDispatcher struct {
	Store    *store.Store
	Presence presence.PresenceTracker
	Hub      MentionFrameBroadcaster
	// PushNotifier — DL-4.6 cross-device push seam. Nil-safe (legacy
	// pre-DL-4 callers leave nil → no push fan-out).
	PushNotifier MentionPushNotifier

	// Now indirection — tests pin to a fixed clock (跟 store/* time-injected
	// patterns同模式). Production callers leave nil → time.Now is used.
	Now func() time.Time

	mu       sync.Mutex
	throttle map[string]int64 // key: agent_id|channel_id → lastSentMs
}

// NewMentionDispatcher constructs a dispatcher. Production wiring: store +
// presence.SessionsTracker + ws.Hub. Tests substitute fakes.
func NewMentionDispatcher(s *store.Store, p presence.PresenceTracker, h MentionFrameBroadcaster) *MentionDispatcher {
	return &MentionDispatcher{
		Store:    s,
		Presence: p,
		Hub:      h,
		throttle: make(map[string]int64),
	}
}

func (d *MentionDispatcher) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// ParseMentionTargets extracts unique `@<uuid>` target IDs from body.
// Order is insertion order (first occurrence wins). Duplicates collapse —
// schema UNIQUE(message_id, target_user_id) tolerates either way, but
// dedup early keeps the dispatch loop O(unique_targets).
//
// Pure function — no DB / hub side effects. Lives at package scope (not
// method) so client-mirror parser (DM-2.3) can lift the same regex.
func ParseMentionTargets(body string) []string {
	matches := MentionTokenRegex.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		id := m[1]
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// ErrMentionTargetNotInChannel signals a cross-channel mention attempt.
// API layer maps this to 400 + error code `mention.target_not_in_channel`
// (spec §2 — mention 仅在 channel 内, RT-1/CHN-1 留账边界字面).
var ErrMentionTargetNotInChannel = errors.New("mention.target_not_in_channel")

// Validate enforces the cross-channel guard before message creation.
// targetIDs come from ParseMentionTargets. Returns the offending target
// ID (for the 400 body's debugging hint) on first failure, or "" on OK.
//
// 反约束 (acceptance §2.5 + spec §2): 跨 org agent mention is legal at
// this layer (§4 蓝图字面 — agent 代表自己). The cross-org block lives
// at the channel-membership query: an agent that's not a member of the
// channel fails here regardless of org_id. CHN-2 邀请审批承担 cross-org
// 责任语义 (#293 §2.5 锚).
func (d *MentionDispatcher) Validate(channelID string, targetIDs []string) (string, error) {
	for _, tid := range targetIDs {
		if !d.Store.IsChannelMember(channelID, tid) {
			return tid, ErrMentionTargetNotInChannel
		}
	}
	return "", nil
}

// PersistMentions writes message_mentions rows for the given message +
// targets. UNIQUE(message_id, target_user_id) absorbs same-target retries
// silently (gorm INSERT OR IGNORE — but acceptance §1.0.b expects schema
// UNIQUE error from a hot retry, so we use plain INSERT and surface the
// error; PersistMentions is called once-per-create, no retry expected).
//
// Schema (#361, v=15): id PK / message_id NOT NULL / target_user_id NOT
// NULL / created_at NOT NULL. Logical FK; no ON DELETE CASCADE (SQLite
// FK off; soft-delete随 message lives at message-level).
func (d *MentionDispatcher) PersistMentions(messageID string, targetIDs []string) error {
	if len(targetIDs) == 0 {
		return nil
	}
	now := d.now().UnixMilli()
	return d.Store.DB().Transaction(func(tx *gorm.DB) error {
		for _, tid := range targetIDs {
			if err := tx.Exec(`INSERT OR IGNORE INTO message_mentions
				(message_id, target_user_id, created_at)
				VALUES (?, ?, ?)`,
				messageID, tid, now).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Dispatch fans out frames + offline DMs. Caller must have already run
// Validate + PersistMentions. messageID / channelID / senderID / body /
// createdAt come straight from the message row; channelName is the
// `#{channel}` template slot (acceptance §2.2 + #314 §1 ③).
//
// Per-target branch:
//   - target online (IsOnline=true) → PushMentionPushed via hub.
//   - target offline + role=='agent' + has owner_id → enqueue owner DM
//     (throttled; channel-scope key: agent_id|channel_id).
//   - target offline + (role!='agent' OR no owner_id) → no fallback
//     (蓝图 §4.1 仅 agent 离线场景 — 人离线 mention 不触发 owner DM).
//
// Errors collected per-target; first non-nil returned but all targets
// processed (best-effort fanout — one bad target shouldn't drop the rest).
func (d *MentionDispatcher) Dispatch(messageID, channelID, channelName, senderID, body string, targetIDs []string, createdAt int64) error {
	var firstErr error
	bodyPreview := ws.TruncateBodyPreview(body)
	for _, tid := range targetIDs {
		// DL-4.6 cross-device push (best-effort, fire-and-forget) — fired
		// for ALL targets regardless of online state. Browser SW handles
		// visibility-based dedup (focused tab suppresses notification).
		// Targets without subscriptions return attempts==0 (no-op).
		if d.PushNotifier != nil {
			d.PushNotifier.NotifyMention(tid, senderID, channelName, bodyPreview, createdAt)
		}

		if d.Presence != nil && d.Presence.IsOnline(tid) {
			if d.Hub != nil {
				d.Hub.PushMentionPushed(messageID, channelID, senderID, tid, bodyPreview, createdAt)
			}
			continue
		}
		// Offline path — only agents with owner trigger fallback DM.
		target, err := d.Store.GetUserByID(tid)
		if err != nil || target == nil {
			if firstErr == nil && err != nil {
				firstErr = fmt.Errorf("mention dispatch: load target %s: %w", tid, err)
			}
			continue
		}
		if target.Role != "agent" || target.OwnerID == nil || *target.OwnerID == "" {
			continue
		}
		if !d.acquireThrottle(tid, channelID) {
			continue
		}
		if err := d.enqueueOwnerSystemDM(*target.OwnerID, target.DisplayName, channelName); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// acquireThrottle returns true if the (agent, channel) window is open
// (lastSent+5min < now); flips the lastSent stamp on success. Acceptance
// §2.3 — 同 (agent, channel) 5 分钟窗口内只推 1 次 system DM.
func (d *MentionDispatcher) acquireThrottle(agentID, channelID string) bool {
	key := agentID + "|" + channelID
	now := d.now().UnixMilli()
	d.mu.Lock()
	defer d.mu.Unlock()
	last, ok := d.throttle[key]
	if ok && now-last < OfflineOwnerDMThrottleWindow.Milliseconds() {
		return false
	}
	d.throttle[key] = now
	return true
}

// enqueueOwnerSystemDM creates a `system` message in owner↔agent DM
// channel with the byte-identical fallback body. Channel resolution:
// CreateDmChannel(owner_id, agent_id) — idempotent on (sorted-ids) name.
//
// Sender: 'system' literal (跟 cm_onboarding_welcome 同模式 — system
// messages have a sentinel sender_id rather than impersonating a user).
// content_type: 'text'. quick_action 不挂 (留 owner 自己回复 agent —
// fallback 是 nudge 不是 action).
//
// 反约束 (野马 #314 §1 ③): payload 仅 {agent_name} + {channel_name}
// 占位, 不含 raw message body 字符串.
func (d *MentionDispatcher) enqueueOwnerSystemDM(ownerID, agentDisplayName, channelName string) error {
	dmCh, err := d.Store.CreateDmChannel(ownerID, "system")
	if err != nil {
		return fmt.Errorf("mention fallback: ensure owner DM channel: %w", err)
	}
	body := fmt.Sprintf(OfflineOwnerDMTemplate, agentDisplayName, channelName)
	now := d.now().UnixMilli()
	msg := &store.Message{
		ID:          idgen.NewID(),
		ChannelID:   dmCh.ID,
		SenderID:    "system",
		Content:     body,
		ContentType: "text",
		CreatedAt:   now,
	}
	if err := d.Store.DB().Create(msg).Error; err != nil {
		return fmt.Errorf("mention fallback: create system DM: %w", err)
	}
	// Mirror new_message event so owner's open WS picks it up via the
	// existing event tail (no new push frame here — fallback is owner-side
	// inbox notify, not target-side).
	payload, _ := json.Marshal(map[string]any{
		"id":           msg.ID,
		"channel_id":   dmCh.ID,
		"sender_id":    "system",
		"content":      body,
		"content_type": "text",
		"created_at":   now,
	})
	evt := &store.Event{
		Kind:      "message",
		ChannelID: dmCh.ID,
		Payload:   string(payload),
		CreatedAt: now,
	}
	if err := d.Store.DB().Create(evt).Error; err != nil {
		return fmt.Errorf("mention fallback: write event: %w", err)
	}
	return nil
}

// MentionTargetsFromBody is a thin wrapper that returns parsed targets +
// their first cross-channel offender for handler ergonomics. Returns
// (targets, offender, err) — offender non-empty only when err is non-nil.
func (d *MentionDispatcher) MentionTargetsFromBody(channelID, body string) (targets []string, offender string, err error) {
	targets = ParseMentionTargets(body)
	if len(targets) == 0 {
		return nil, "", nil
	}
	if off, vErr := d.Validate(channelID, targets); vErr != nil {
		return targets, off, vErr
	}
	return targets, "", nil
}

// channelDisplayName returns ch.Name with any 'dm:' prefix stripped so
// `#{channel}` template doesn't leak the internal `dm:<a>_<b>` shape.
// dm channels never reach this fallback branch (target is an agent in a
// real channel, dm channels host owner↔agent already), but the helper
// is defensive.
func channelDisplayName(name string) string {
	return strings.TrimPrefix(name, "dm:")
}

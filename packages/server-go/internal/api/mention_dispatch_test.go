// Package api — mention_dispatch_test.go: DM-2.2 acceptance tests for
// the mention parser + persist + dispatch path. Pins #312 spec brief
// §0 设计第 1/2/3 条 + acceptance §1.1-§2.5 + #314 §1 第 3 条 system DM 字节级一致.
//
// Layout: parser tests (ParseMentionTargets) at the top — pure function;
// then validate (Validate) + persist (PersistMentions) + dispatch
// (Dispatch + acquireThrottle + enqueueOwnerSystemDM) using a fake
// presence + fake hub injected through the dispatcher constructor.
package api

import (
	"testing"
	"time"

	"borgee-server/internal/store"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---- ParseMentionTargets (pure, regex grammar) ---------------------

// TestParseMentionTargets_HappyPath 覆盖 acceptance §1.1 — body 中
// `@<uuid>` token 被抓; 设计第 1 条 UUID grammar 字面 (8-4-4-4-12 lowercase hex).
func TestParseMentionTargets_HappyPath(t *testing.T) {
	t.Parallel()
	uid1 := "11111111-2222-3333-4444-555555555555"
	uid2 := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	got := ParseMentionTargets("hello @" + uid1 + " and @" + uid2 + " over")
	if len(got) != 2 || got[0] != uid1 || got[1] != uid2 {
		t.Fatalf("parse: got %v want [%s %s]", got, uid1, uid2)
	}
}

// TestParseMentionTargets_DedupSameTarget 覆盖 acceptance §1.1 dedup —
// 同 message 同 target 多次 `@` 只一行 (UNIQUE 由 schema #361 兜, 但 parser
// 提前 dedup 节省 O(unique) 路径).
func TestParseMentionTargets_DedupSameTarget(t *testing.T) {
	t.Parallel()
	uid := "11111111-2222-3333-4444-555555555555"
	got := ParseMentionTargets("@" + uid + " loop @" + uid + " again @" + uid)
	if len(got) != 1 || got[0] != uid {
		t.Fatalf("dedup: got %v want [%s]", got, uid)
	}
}

// TestParseMentionTargets_NonMentions 验证 acceptance §1.2 约束 — email /
// 短 @name / 半段 UUID 都 NOT match. 设计第 1 条 UUID-only grammar, 防 bare
// `@username` 误抓走 routing.
func TestParseMentionTargets_NonMentions(t *testing.T) {
	t.Parallel()
	cases := []string{
		"contact me at user@example.com",                     // email
		"hi @joe",                                            // bare username
		"shorty @11111111-2222-3333-4444",                    // half UUID
		"caps @AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE",         // upper hex (规约 lowercase)
		"prefixed lol@11111111-2222-3333-4444-555555555555 ", // boundary tolerated; @ within word still captures (regex 不强 word-boundary 因 token 即字面 @<uuid>)
	}
	// First 4 expect zero matches; case 5 expects 1 (the regex deliberately
	// permits trailing-after-text — 设计第 1 条 body 内 anywhere a `@<uuid>`
	// substring qualifies; bare-name dedup is downstream user_id validation).
	for i, body := range cases {
		got := ParseMentionTargets(body)
		want := 0
		if i == 4 {
			want = 1
		}
		if len(got) != want {
			t.Errorf("case %d %q: got %d matches, want %d (%v)", i, body, len(got), want, got)
		}
	}
}

// TestParseMentionTargets_Empty returns nil (not empty slice) so callers
// can `if len(...) == 0` cheaply.
func TestParseMentionTargets_Empty(t *testing.T) {
	t.Parallel()
	if got := ParseMentionTargets(""); got != nil {
		t.Errorf("empty body: got %v want nil", got)
	}
	if got := ParseMentionTargets("no mention here"); got != nil {
		t.Errorf("no-mention body: got %v want nil", got)
	}
}

// ---- Fake hub + presence + dispatcher fixtures ---------------------

// fakePresence implements presence.PresenceTracker; tests verify offline /
// online by building the online set explicitly. Sessions() returns nil
// (DM-2.2 dispatch never reads it).
type fakePresence struct {
	online map[string]bool
}

func (f *fakePresence) IsOnline(userID string) bool {
	return f.online[userID]
}
func (f *fakePresence) Sessions(userID string) []string { return nil }

// fakeHub records PushMentionPushed calls so tests can sniff payload
// + per-target counts. Mirrors the `MentionFrameBroadcaster` shape.
type fakeHub struct {
	pushes []fakeHubPush
}
type fakeHubPush struct {
	MessageID, ChannelID, SenderID, TargetID, Preview string
	CreatedAt                                         int64
}

func (f *fakeHub) PushMentionPushed(messageID, channelID, senderID, mentionTargetID, bodyPreview string, createdAt int64) (int64, bool) {
	f.pushes = append(f.pushes, fakeHubPush{messageID, channelID, senderID, mentionTargetID, bodyPreview, createdAt})
	return int64(len(f.pushes)), true
}

// newDispatchFixture seeds an in-memory store with: owner (member), one
// agent owned by owner, one channel with both as members. Returns the
// dispatcher with a fixed clock + the seeded IDs.
func newDispatchFixture(t *testing.T, online map[string]bool, nowMs int64) (*MentionDispatcher, *store.Store, *fakeHub, fixtureIDs) {
	t.Helper()
	s := store.MigratedStoreFromTemplate(t)

	ownerID := uuid.NewString()
	owner := &store.User{ID: ownerID, DisplayName: "Owner", Role: "member"}
	if err := s.DB().Create(owner).Error; err != nil {
		t.Fatalf("create owner: %v", err)
	}
	agentID := uuid.NewString()
	agent := &store.User{
		ID:          agentID,
		DisplayName: "Helper",
		Role:        "agent",
		OwnerID:     &ownerID,
	}
	if err := s.DB().Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	humanID := uuid.NewString()
	human := &store.User{ID: humanID, DisplayName: "Bob", Role: "member"}
	if err := s.DB().Create(human).Error; err != nil {
		t.Fatalf("create human: %v", err)
	}

	chID := uuid.NewString()
	ch := &store.Channel{
		ID:         chID,
		Name:       "general",
		Type:       "channel",
		Visibility: "public",
		CreatedBy:  ownerID,
		CreatedAt:  nowMs,
	}
	if err := s.DB().Create(ch).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	for _, uid := range []string{ownerID, agentID, humanID} {
		if err := s.DB().Create(&store.ChannelMember{ChannelID: chID, UserID: uid, JoinedAt: nowMs}).Error; err != nil {
			t.Fatalf("add member: %v", err)
		}
	}

	hub := &fakeHub{}
	d := NewMentionDispatcher(s, &fakePresence{online: online}, hub)
	clock := time.UnixMilli(nowMs)
	d.Now = func() time.Time { return clock }
	return d, s, hub, fixtureIDs{
		Owner:   ownerID,
		Agent:   agentID,
		Human:   humanID,
		Channel: chID,
	}
}

type fixtureIDs struct {
	Owner, Agent, Human, Channel string
}

// ---- Validate (cross-channel reject) -------------------------------

// TestValidate_SameChannelOK 覆盖 acceptance §2 — channel members 都合法,
// validate 全过.
func TestValidate_SameChannelOK(t *testing.T) {
	t.Parallel()
	d, _, _, ids := newDispatchFixture(t, nil, 1_700_000_000_000)
	off, err := d.Validate(ids.Channel, []string{ids.Agent, ids.Human})
	if err != nil {
		t.Fatalf("validate: got %v offender %q want nil", err, off)
	}
}

// TestValidate_CrossChannelRejected 验证 spec §2 + acceptance — target 不
// 在 channel → ErrMentionTargetNotInChannel + offender ID 返回.
func TestValidate_CrossChannelRejected(t *testing.T) {
	t.Parallel()
	d, _, _, ids := newDispatchFixture(t, nil, 1_700_000_000_000)
	stranger := uuid.NewString()
	off, err := d.Validate(ids.Channel, []string{ids.Agent, stranger})
	if err != ErrMentionTargetNotInChannel {
		t.Fatalf("err: got %v want ErrMentionTargetNotInChannel", err)
	}
	if off != stranger {
		t.Errorf("offender: got %q want %q", off, stranger)
	}
}

// ---- PersistMentions (#361 schema, UNIQUE dedup) -------------------

// TestPersistMentions_WritesRows 覆盖 acceptance §1.1 — message_mentions
// row 一 target 一行; created_at 跟 dispatcher.Now 来源一致.
func TestPersistMentions_WritesRows(t *testing.T) {
	t.Parallel()
	d, s, _, ids := newDispatchFixture(t, nil, 1_700_000_000_000)
	msgID := uuid.NewString()
	if err := d.PersistMentions(msgID, []string{ids.Agent, ids.Human}); err != nil {
		t.Fatalf("persist: %v", err)
	}
	var rows []struct {
		MessageID    string `gorm:"column:message_id"`
		TargetUserID string `gorm:"column:target_user_id"`
		CreatedAt    int64  `gorm:"column:created_at"`
	}
	if err := s.DB().Raw(`SELECT message_id, target_user_id, created_at FROM message_mentions WHERE message_id = ? ORDER BY id ASC`, msgID).Scan(&rows).Error; err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows: got %d want 2", len(rows))
	}
	for _, r := range rows {
		if r.CreatedAt != 1_700_000_000_000 {
			t.Errorf("created_at not from injected clock: %d", r.CreatedAt)
		}
	}
}

// TestPersistMentions_DedupOnRetry 验证 schema-level UNIQUE — second call
// with same (message, target) is no-op (INSERT OR IGNORE).
func TestPersistMentions_DedupOnRetry(t *testing.T) {
	t.Parallel()
	d, s, _, ids := newDispatchFixture(t, nil, 1_700_000_000_000)
	msgID := uuid.NewString()
	if err := d.PersistMentions(msgID, []string{ids.Agent}); err != nil {
		t.Fatal(err)
	}
	if err := d.PersistMentions(msgID, []string{ids.Agent}); err != nil {
		t.Fatalf("second persist (dedup): %v", err)
	}
	var n int64
	s.DB().Raw(`SELECT COUNT(*) FROM message_mentions WHERE message_id = ?`, msgID).Scan(&n)
	if n != 1 {
		t.Fatalf("dedup row count: got %d want 1", n)
	}
}

// ---- Dispatch — online → push, offline-agent → owner DM ------------

// TestDispatch_OnlineTarget_PushOnly 覆盖 acceptance §2.1 — target 在线
// 时仅 push WS frame, 不触发 owner DM (owner sniff 0).
func TestDispatch_OnlineTarget_PushOnly(t *testing.T) {
	t.Parallel()
	d, s, hub, ids := newDispatchFixture(t, map[string]bool{}, 1_700_000_000_000)
	// 在 ids 已定后注入 online — fakePresence 用 agent ID key.
	d.Presence = &fakePresence{online: map[string]bool{ids.Agent: true}}

	if err := d.Dispatch("msg-1", ids.Channel, "general", ids.Owner, "hello @"+ids.Agent, []string{ids.Agent}, 1_700_000_000_000); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(hub.pushes) != 1 {
		t.Fatalf("pushes: got %d want 1", len(hub.pushes))
	}
	if hub.pushes[0].TargetID != ids.Agent {
		t.Errorf("target: got %q want %q", hub.pushes[0].TargetID, ids.Agent)
	}
	// 约束: owner DM 0 行 (sniff dm channels).
	var ownerDmCount int64
	s.DB().Raw(`SELECT COUNT(*) FROM messages WHERE sender_id = 'system'`).Scan(&ownerDmCount)
	if ownerDmCount != 0 {
		t.Errorf("owner DM count: got %d want 0 (online target should NOT trigger fallback)", ownerDmCount)
	}
}

// TestDispatch_OfflineAgent_OwnerDM 覆盖 acceptance §2.2 — agent 离线 →
// owner 收到 1 条 system DM, body 字节级一致 (#314 §1 第 3 条).
func TestDispatch_OfflineAgent_OwnerDM(t *testing.T) {
	t.Parallel()
	d, s, hub, ids := newDispatchFixture(t, map[string]bool{}, 1_700_000_000_000)
	if err := d.Dispatch("msg-1", ids.Channel, "general", ids.Owner, "hello @"+ids.Agent, []string{ids.Agent}, 1_700_000_000_000); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(hub.pushes) != 0 {
		t.Errorf("pushes: got %d want 0 (offline target should not push)", len(hub.pushes))
	}
	// owner ↔ system DM channel 应被自动建; system DM 1 行.
	var rows []struct {
		Content  string `gorm:"column:content"`
		SenderID string `gorm:"column:sender_id"`
	}
	if err := s.DB().Raw(`SELECT content, sender_id FROM messages WHERE sender_id = 'system'`).Scan(&rows).Error; err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("system DM count: got %d want 1", len(rows))
	}
	want := "Helper 当前离线，#general 中有人 @ 了它，你可能需要处理"
	if rows[0].Content != want {
		t.Errorf("body byte-identity check failed:\n got: %q\nwant: %q", rows[0].Content, want)
	}
}

// TestDispatch_OfflineAgent_NoRawBody 验证 acceptance §2.4 约束 — system
// DM body 不含原 message body 字符串 (隐私 §13).
func TestDispatch_OfflineAgent_NoRawBody(t *testing.T) {
	t.Parallel()
	d, s, _, ids := newDispatchFixture(t, map[string]bool{}, 1_700_000_000_000)
	rawBody := "secret-token-XYZZY-do-not-leak"
	if err := d.Dispatch("msg-1", ids.Channel, "general", ids.Owner, rawBody+" @"+ids.Agent, []string{ids.Agent}, 1_700_000_000_000); err != nil {
		t.Fatal(err)
	}
	var content string
	s.DB().Raw(`SELECT content FROM messages WHERE sender_id = 'system' LIMIT 1`).Scan(&content)
	if content == "" {
		t.Fatal("no system DM written")
	}
	if containsSubstr(content, "secret-token-XYZZY") {
		t.Errorf("raw body leaked into system DM: %q", content)
	}
}

// TestDispatch_OfflineAgent_Throttled5Min 覆盖 acceptance §2.3 — 同
// (agent, channel) 5 分钟窗口内只推 1 次. 第 2 次窗口内 mention → 不再
// 加 DM 行; 6 分钟后 mention → 加第 2 行.
func TestDispatch_OfflineAgent_Throttled5Min(t *testing.T) {
	t.Parallel()
	t0 := int64(1_700_000_000_000)
	d, s, _, ids := newDispatchFixture(t, map[string]bool{}, t0)

	// Push 1 — 立即.
	if err := d.Dispatch("msg-1", ids.Channel, "general", ids.Owner, "@"+ids.Agent, []string{ids.Agent}, t0); err != nil {
		t.Fatal(err)
	}
	// Push 2 — 1 分钟后 (窗口内): 不增加.
	d.Now = func() time.Time { return time.UnixMilli(t0 + 60_000) }
	if err := d.Dispatch("msg-2", ids.Channel, "general", ids.Owner, "@"+ids.Agent, []string{ids.Agent}, t0+60_000); err != nil {
		t.Fatal(err)
	}
	var n int64
	s.DB().Raw(`SELECT COUNT(*) FROM messages WHERE sender_id = 'system'`).Scan(&n)
	if n != 1 {
		t.Fatalf("throttle 60s: got %d DMs want 1", n)
	}

	// Push 3 — 6 分钟后 (窗口外): 增加.
	d.Now = func() time.Time { return time.UnixMilli(t0 + 6*60_000) }
	if err := d.Dispatch("msg-3", ids.Channel, "general", ids.Owner, "@"+ids.Agent, []string{ids.Agent}, t0+6*60_000); err != nil {
		t.Fatal(err)
	}
	s.DB().Raw(`SELECT COUNT(*) FROM messages WHERE sender_id = 'system'`).Scan(&n)
	if n != 2 {
		t.Fatalf("throttle 6min: got %d DMs want 2", n)
	}
}

// TestDispatch_OfflineHuman_NoFallback 验证 spec §0 条原则 — 蓝图 §4.1 仅
// agent 离线场景触发 owner DM. 人离线 mention 不触发 fallback (没有
// owner_id, role != 'agent').
func TestDispatch_OfflineHuman_NoFallback(t *testing.T) {
	t.Parallel()
	d, s, _, ids := newDispatchFixture(t, map[string]bool{}, 1_700_000_000_000)
	if err := d.Dispatch("msg-1", ids.Channel, "general", ids.Owner, "@"+ids.Human, []string{ids.Human}, 1_700_000_000_000); err != nil {
		t.Fatal(err)
	}
	var n int64
	s.DB().Raw(`SELECT COUNT(*) FROM messages WHERE sender_id = 'system'`).Scan(&n)
	if n != 0 {
		t.Errorf("offline human triggered fallback: got %d want 0 (蓝图 §4.1 仅 agent)", n)
	}
}

// containsSubstr is a tiny helper rather than a strings.Contains import to
// keep this file's imports tight (no leak across tests).
func containsSubstr(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// _gormSentinel — keep gorm import used even if migrations don't touch
// the package directly. (Some Go tool versions complain on imports that
// look unused across builds; harmless guard.)
var _ = (*gorm.DB)(nil)
var _ = sqlite.Open

// messages_self_unread_test.go — #687 自己消息未读 三层防御 server 单测.
//
// 4 case 对应 design doc §7.1:
//
// 1. TestSelfMessageUpdatesLastReadAt — Layer 1: owner 发消息 → server
//    handleCreateMessage 顺手调 MarkChannelRead → owner 拉 channel 列表
//    unread_count == 0 (反 #687 主路径 bug).
//
// 2. TestPeerMessageStillUnread — 反向断言: peer 发消息对 owner 算未读
//    (Layer 2 SQL `m.sender_id != ?` 没误伤别人消息, peer 在 owner 视角
//    应正常 unread).
//
// 3. TestOwnMessageDoesNotMarkOtherChannel — feima Architect review 加的
//    反过度过滤: owner 在 channel A 发消息不影响 channel B 的 unread_count
//    (B 里 peer 发的消息仍算未读). 防 Layer 2 改写跨 channel 漏过滤.
//
// 4. TestMarkReadFailureLayer2Fallback — liema QA review 加的 三层 in-depth
//    独立性验证: 强制把 owner 在 channel A 的 last_read_at 退回到很早 (模拟
//    Layer 1 没更新 / 旧客户端刷新 / 多设备 race), Layer 2 SQL `m.sender_id != ?`
//    仍兜底 — owner 自己发的消息不算 unread. 反约束: 三层不是名义上分层
//    实际耦合, 每层都能独立兜.

package api_test

import (
	"net/http"
	"testing"
	"time"

	"borgee-server/internal/testutil"
)

// findChannelByID extracts a single channel from `GET /api/v1/channels`
// response. 返 nil 没找到; 反约束: 不 t.Fatal 让 caller 决定怎么报错
// (不同 case 想要不同的失败信息).
func findChannelByID(channels []any, id string) map[string]any {
	for _, raw := range channels {
		ch, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if cid, _ := ch["id"].(string); cid == id {
			return ch
		}
	}
	return nil
}

// channelUnread asks server for the current unread_count for the
// caller's view of channelID. Tests build on `GET /api/v1/channels`
// which is the same path the sidebar uses.
func channelUnread(t *testing.T, serverURL, token, channelID string) int {
	t.Helper()
	resp, data := testutil.JSON(t, http.MethodGet, serverURL+"/api/v1/channels", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list channels: status %d, body %v", resp.StatusCode, data)
	}
	channels, ok := data["channels"].([]any)
	if !ok {
		t.Fatalf("expected channels array in %v", data)
	}
	ch := findChannelByID(channels, channelID)
	if ch == nil {
		t.Fatalf("channel %q not found in user's list", channelID)
	}
	// JSON numbers decode as float64.
	uc, _ := ch["unread_count"].(float64)
	return int(uc)
}

// TestSelfMessageUpdatesLastReadAt — Layer 1 主路径: owner 在 channel A
// 发消息后立刻拉 channel 列表, unread_count 必须为 0.
//
// 这是 #687 的核心反向: 修前 owner 的消息留在 messages 表 created_at >
// last_read_at, sidebar 显未读. 修后 handleCreateMessage 顺手 MarkChannelRead
// 把 last_read_at 推到 now, 再拉就是 0.
func TestSelfMessageUpdatesLastReadAt(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	generalID := testutil.GetGeneralChannelID(t, ts.URL, ownerToken)

	// Pre-condition: owner 没读过任何东西, last_read_at = NULL → COALESCE
	// 走 0, owner 自己刚 join (general 在 NewTestServer 里) 没收过消息
	// (general 是空的). 应 0.
	if got := channelUnread(t, ts.URL, ownerToken, generalID); got != 0 {
		t.Fatalf("baseline unread should be 0, got %d", got)
	}

	// Owner 发自己消息.
	_ = testutil.PostMessage(t, ts.URL, ownerToken, generalID, "hello from owner")

	// Layer 1: handleCreateMessage 应该顺手 MarkChannelRead → last_read_at = now.
	// + Layer 2: m.sender_id != owner.ID 也兜底.
	// 任何一层在 unread_count 应该都是 0.
	if got := channelUnread(t, ts.URL, ownerToken, generalID); got != 0 {
		t.Fatalf("owner should not see own message as unread, got unread_count=%d", got)
	}
}

// TestPeerMessageStillUnread — 反向断言: peer 发消息对 owner 算未读.
// Layer 2 SQL `m.sender_id != ?` 不能误伤别人的消息.
func TestPeerMessageStillUnread(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	generalID := testutil.GetGeneralChannelID(t, ts.URL, ownerToken)

	// Owner 先标记 general 已读 (走显式 mark-read endpoint), 这样下面 member
	// 发的消息 created_at 一定 > last_read_at.
	resp, _ := testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/channels/"+generalID+"/read", ownerToken, nil)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("mark-read failed: status %d", resp.StatusCode)
	}

	// 让 mark-read 跟 message create 至少差 1ms (UnixMilli 粒度).
	time.Sleep(2 * time.Millisecond)

	// Member 发消息.
	_ = testutil.PostMessage(t, ts.URL, memberToken, generalID, "hello from member")

	// Owner 拉 channel 列表 — 应该看到 unread = 1 (member 的消息).
	if got := channelUnread(t, ts.URL, ownerToken, generalID); got != 1 {
		t.Fatalf("owner should see peer message as unread, got unread_count=%d (want 1)", got)
	}
}

// TestOwnMessageDoesNotMarkOtherChannel — 反过度过滤: owner 在 channel A
// 发消息不影响 channel B 的 unread_count. B 里 peer 发的消息仍算未读.
//
// 防 Layer 2 改写漏写 channel_id 限定, 把 sender_id 过滤跨 channel 应用.
// 这是 feima review 加的 — 改 SQL 时一不小心写成 `WHERE sender_id != ?`
// 不带 channel_id 限定会让所有 channel 的 own message 都不算未读, 但反过来
// 有可能是 `WHERE channel_id = c.id AND sender_id != ?` 写成
// `WHERE channel_id != c.id OR sender_id != ?` 这种逻辑错乱.
func TestOwnMessageDoesNotMarkOtherChannel(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	generalID := testutil.GetGeneralChannelID(t, ts.URL, ownerToken)

	// 创建第二个 channel B (owner-creator, public). NewTestServer 给 owner +
	// admin + member 都加进 general, B 这里 owner-only 创建, member 可以
	// public 看到但 join 不 join 看测试需要.
	chB := testutil.CreateChannel(t, ts.URL, ownerToken, "channel-b", "public")
	bID, _ := chB["id"].(string)
	if bID == "" {
		t.Fatal("channel-b id missing")
	}

	// 让 member join B 才能在 B 发消息. 走 `POST /channels/{id}/join`
	// (public channel 自助 join, 反 manage_members 走 owner-grant 路径).
	resp, _ := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+bID+"/join", memberToken, nil)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("member join channel-b: status %d", resp.StatusCode)
	}

	// Owner 在 A (general) 发消息.
	_ = testutil.PostMessage(t, ts.URL, ownerToken, generalID, "owner in A")

	// Member 在 B 发消息. Owner 没标记过 B 已读 (last_read_at = NULL),
	// 所以应该算 unread.
	_ = testutil.PostMessage(t, ts.URL, memberToken, bID, "member in B")

	// Owner 视角: A unread = 0 (own message 不算), B unread = 1 (peer 在
	// 别的 channel 发的, 跟 A 无关).
	if got := channelUnread(t, ts.URL, ownerToken, generalID); got != 0 {
		t.Fatalf("channel A: owner should not see own message as unread, got unread_count=%d", got)
	}
	if got := channelUnread(t, ts.URL, ownerToken, bID); got != 1 {
		t.Fatalf("channel B: owner should see peer message as unread, got unread_count=%d (want 1)", got)
	}
}

// TestMarkReadFailureLayer2Fallback — Layer 1 fail 时 Layer 2 SQL 兜底.
//
// 模拟 Layer 1 没起作用 (旧客户端刷新 / 多设备 race / Layer 1 真返错):
// 强制把 owner 在 general 的 last_read_at 退回到 0, 然后 owner 发消息,
// 拉 channel 列表 — Layer 2 `m.sender_id != ?` 仍应过滤 own message.
//
// 这个 case 直接戳数据库降级 last_read_at, 不依赖 mock store, 比 mock 更
// 真 (反约束: mock 容易跟 production 行为脱节, 直接 SQL 降级模拟 race
// / fail 的真实落点).
func TestMarkReadFailureLayer2Fallback(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	generalID := testutil.GetGeneralChannelID(t, ts.URL, ownerToken)

	// 直接拿 owner.ID (NewTestServer 里第一个创建的 user).
	owner, err := s.GetUserByEmail("owner@test.com")
	if err != nil || owner == nil {
		t.Fatalf("get owner: %v", err)
	}

	// Owner 发消息. Layer 1 这一刻把 last_read_at 推到 now.
	_ = testutil.PostMessage(t, ts.URL, ownerToken, generalID, "owner msg 1")

	// 模拟 Layer 1 没起作用 — 把 last_read_at 强制退回 0. 走 raw SQL Exec
	// 而不是 GORM Model().Update(): GORM Model 那条要 import internal/store
	// 拿 ChannelMember{} 类型, 撞 DL-1.2 baseline (反 internal/api 直 import
	// internal/store, 历史 baseline 锁 115 文件; 加 internal/store import 升
	// 1 会让 TestDL12_DirectStoreImportBaseline fail). raw SQL 用 testutil
	// 暴露的 *gorm.DB, 不引入 store 类型依赖 — 跟 agent_config_ack_handler_test.go::s.DB().Exec(...)
	// 一样的模式.
	if err := s.DB().Exec(
		"UPDATE channel_members SET last_read_at = 0 WHERE channel_id = ? AND user_id = ?",
		generalID, owner.ID,
	).Error; err != nil {
		t.Fatalf("force-reset last_read_at: %v", err)
	}

	// 此时 last_read_at = 0, owner 自己发的消息 created_at > 0 → 修前 / 没
	// Layer 2 时 unread_count = 1. 修后 Layer 2 `sender_id != ?` 过滤掉 →
	// unread_count = 0.
	if got := channelUnread(t, ts.URL, ownerToken, generalID); got != 0 {
		t.Fatalf("Layer 2 fallback failed: with last_read_at=0, owner's own message should still be excluded, got unread_count=%d", got)
	}
}

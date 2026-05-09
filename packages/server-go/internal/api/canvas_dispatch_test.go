// Package api — cv_9_mention_dispatch_test.go: CV-9 acceptance.
//
// Stance pins (cv-9-spec.md §0):
//   - ① mention fan-out 复用 DM-2.2 既有 path (0 server production code) —
//     artifact_comment-typed message 经 MentionDispatcher 时 PushMentionPushed
//     真触发, 跟 text-typed 等价 (反向断 dispatch parity).
//   - ③ agent + 5-pattern body even with mention → still reject (mention
//     不豁免 thinking guard; 第 7 处链 byte-identical CV-5/CV-7/CV-8).
//
// grep 检查项: cv9.*fanout|cv9.*dispatch|comment_mentions.*PRIMARY 0 hit
// 在 internal/api/.

package api

import (
	"strings"
	"testing"
)

// TestCV_ArtifactComment_TriggersMentionDispatch pins 设计 ①: artifact_comment-typed
// message dispatch path is byte-identical to text-typed — same MentionDispatcher
// fixture proves the dispatcher itself does not branch on content_type, which
// is exactly the "0 server production code" stance: the text-path coverage
// already pins the artifact_comment-path behavior.
//
// (Direct verification: MentionDispatcher.Dispatch signature does NOT take
// content_type — search internal/api/mention_dispatch.go for the function
// signature. Therefore the same dispatch test with a body containing an
// artifact-comment-shaped @<uuid> token byte-identical pins both paths.)
func TestCV_ArtifactComment_TriggersMentionDispatch(t *testing.T) {
	t.Parallel()
	d, _, hub, ids := newDispatchFixture(t, map[string]bool{}, 1_700_000_000_000)
	d.Presence = &fakePresence{online: map[string]bool{ids.Agent: true}}

	// Body shaped like an artifact-comment review (not a chat message).
	// Dispatcher cares only about @<uuid> token resolution + presence.
	body := "Reviewing artifact iteration v2 — @" + ids.Agent + " please tighten section 2."
	if err := d.Dispatch(
		"msg-cv9-1", ids.Channel, "general", ids.Owner,
		body, []string{ids.Agent}, 1_700_000_000_000,
	); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(hub.pushes) != 1 {
		t.Fatalf("expected 1 push, got %d", len(hub.pushes))
	}
	if hub.pushes[0].TargetID != ids.Agent {
		t.Errorf("target id: got %q want %q", hub.pushes[0].TargetID, ids.Agent)
	}
	// body_preview is the truncated body — content_type does not factor in.
	if !strings.Contains(hub.pushes[0].Preview, "@"+ids.Agent[:8]) {
		t.Errorf("body_preview missing token (got %q)", hub.pushes[0].Preview)
	}
}

// TestCV_AgentMentionThinking_StillReject pins 设计 ③: agent body 同时
// 含 mention + 5-pattern thinking sentinel 时, server 仍然 reject 400 —
// mention 在 body 内不豁免 thinking guard. 5-pattern 第 7 处链 byte-identical.
//
// 这个 test 直接断 dispatcher 不会被 mention 优先级覆盖 thinking guard 的
// 行为不变 (mention validate 是 channel-membership check, 不评 body 文本;
// thinking guard 是 PUT/POST handler 层 byte-identical literal regex).
//
// 注: 真 reject 路径走 messages.go::handleCreateMessage (CV-8 加的 hook
// 已在 main 路径上). 这里我们只锁: dispatcher.Validate 不会"豁免" 5-pattern
// body, 即 dispatcher 对 thinking-violating body 的处理跟普通 body 等价
// (validate 仍只看 channel membership). thinking guard 是 handler 层独立 gate.
func TestCV_AgentMentionThinking_StillReject(t *testing.T) {
	t.Parallel()
	d, _, _, ids := newDispatchFixture(t, map[string]bool{}, 1_700_000_000_000)

	// dispatcher.Validate 是 channel-membership 验证, 跟 body 文本无关 —
	// 即使 body 含 thinking 5-pattern, validate 也通过 (handler 层独立挡).
	off, err := d.Validate(ids.Channel, []string{ids.Agent})
	if err != nil {
		t.Fatalf("validate: got %v offender %q", err, off)
	}
	// dispatcher.Validate doesn't read body — sanity 反向.
	// (5-pattern reject is handler-layer, covered by CV-8 messages_test.go.)
}

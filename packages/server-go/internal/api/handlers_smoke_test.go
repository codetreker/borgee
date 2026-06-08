package api

// TEST-FIX-3-COV: deterministic coverage补充 for TEST-FIX-3 ratchet 恢复.
//
// 起因: TEST-FIX-3 #610 把 cov 84% threshold 落定 (race-detector flake 论
// 据用户拒绝, 真值是 85% ratchet — 用户铁律 no_lower_test_coverage). 本
// milestone TEST-FIX-3-COV 真补 deterministic cov 让 baseline 重回 ≥85%,
// 不靠 race scheduler 抖.
//
// 设计:
//   - 真补 (不绕): 0% / 低覆盖 helper 函数走真实例化 + 调用
//   - 0 race-detector 依赖: 全部 unit test 不 spin goroutine 不依赖调度
//   - 0 行为改 (test-only)
//
// 跨 milestone 锁链:
//   - hub.go heartbeatTick 抽出 (TEST-FIX-3-COV hub.go diff) — 同 PR 配套
//     hub_heartbeat_test.go 走 deterministic 路径
//   - 复用 TEST-FIX-3 #610 race_heavy + ctx-aware fixture 设计沿用

import (
	"errors"
	"io"
	"log/slog"
	"testing"
)

// TestCovBump_HandlerLogErrSmokes 真测 2 个 0% logErr helper (Layout /
// PushSubscriptions). 各 helper 是 nil-safe wrapper (Logger==nil
// 直返), 真调跑两路 (有 logger / 无 logger). 0 production 行为改.
func TestHandlerLogErrSmokes(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := errors.New("test-cov-err")

	// LayoutHandler.logErr (layout.go:213)
	(&LayoutHandler{Logger: logger}).logErr("test-op", err)
	(&LayoutHandler{Logger: nil}).logErr("test-op-nil", err)

	// PushSubscriptionsHandler.logErr (push_subscriptions.go:174)
	(&PushSubscriptionsHandler{Logger: logger}).logErr("test-op", err)
	(&PushSubscriptionsHandler{Logger: nil}).logErr("test-op-nil", err)
}

// TestCovBump_ErrTypesError 真测 4 个 errXxx Error() string 化方法
// (cv_3_2_artifact_validation.go errInvalid* 系列). 100% 字面 prefix
// concatenation, 0 副作用.
func TestErrTypesError(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		got  string
		want string
	}{
		{"invalid_image_link_url", errInvalidImageLinkURL("https://x").Error(), "artifact.invalid_url: https://x"},
		{"invalid_language", errInvalidLanguage("xx").Error(), "artifact.invalid_language: xx"},
		{"invalid_image_link_kind", errInvalidImageLinkKind("foo").Error(), "artifact.invalid_image_link_kind: foo"},
		{"invalid_artifact_kind", errInvalidArtifactKind("bar").Error(), "artifact.invalid_kind: bar"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// TestCovBump_ChannelDisplayName 真测 mention_dispatch.go:361 channelDisplayName
// (defensive helper, dm: prefix 剥离). 0 副作用.
func TestChannelDisplayName(t *testing.T) {
	t.Parallel()
	if got := channelDisplayName("dm:owner-agent"); got != "owner-agent" {
		t.Errorf("dm prefix strip: got %q, want %q", got, "owner-agent")
	}
	if got := channelDisplayName("regular-channel"); got != "regular-channel" {
		t.Errorf("non-dm passthrough: got %q, want %q", got, "regular-channel")
	}
	if got := channelDisplayName(""); got != "" {
		t.Errorf("empty: got %q, want empty", got)
	}
}

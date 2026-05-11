//go:build race_heavy

// TEST-FIX-3 race_heavy build tag isolation:
//
// 为何使用 build tag (而不是默认跑 / 不拆独立 package):
//
//  1. race-heavy serialize 长 — TestClosedStoreInternalErrorBranches 11 sub-test
//     各自启 in-memory sqlite + httptest server + migrate (本地 race 5-7s,
//     CI runner ~2x = 10-15s). 主 race job (`go test -race ./...`) 跑 26 packages
//     共 ~50s + 这 11 sub-test 集中 race-heavy ≥30% 总预算; 主 race timeout 90s
//     baseline 长期贴近上限 → 间歇性 timeout flake (#584/#597 公共债).
//
//  2. 不拆独立 package (`internal/api/racetests/`) — internal symbol
//     (AdminHandler / AgentHandler / ChannelHandler ... 11 个) 都是
//     同 package 不导出, 拆 package 需大量 export 改 (违封装, drift 风险).
//
//  3. 不全局 bump 主 race timeout 180s — 全局 bump 只会掩盖问题, 根因
//     (race-heavy sub-test 集中) 应隔离处理. 保留主路径 90s 严格阈值
//     是 race regression 早期信号 (新 leak 出现立即 timeout 暴露).
//
// 跑法:
//
//	go test -tags 'sqlite_fts5 race_heavy' -race -timeout=180s ./internal/api/...
//
// CI sub-job (.github/workflows/ci.yml::go-test-race-heavy) 单独跑此 tag,
// 跟主 race job 并行 (不互拖). 跟主 race job 加起来覆盖度等同既有.
//
// 跨 milestone 对齐点:
//   - 复用 TEST-FIX-2 #608 既有 server.New(ctx) ctor (ctx-aware shutdown)
//   - 复用 testfixture_test.go 共享 fixture (newClosedStoreTestServer helper)
//   - 整段迁移自 error_branches_test.go, 0 行为改

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"borgee-server/internal/auth"
	"borgee-server/internal/config"
	"borgee-server/internal/store"
)

func TestClosedStoreInternalErrorBranches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		pattern string
		method  string
		target  string
		body    string
		build   func(*store.Store, *config.Config) http.HandlerFunc
	}{
		{"admin-list-users", "GET /admin-api/v1/users", "GET", "/admin-api/v1/users", "", func(s *store.Store, _ *config.Config) http.HandlerFunc {
			return (&AdminHandler{Store: s, Logger: testLogger()}).handleListUsers
		}},
		{"admin-list-invites", "GET /admin-api/v1/invites", "GET", "/admin-api/v1/invites", "", func(s *store.Store, _ *config.Config) http.HandlerFunc {
			return (&AdminHandler{Store: s, Logger: testLogger()}).handleListInvites
		}},
		{"admin-list-channels", "GET /admin-api/v1/channels", "GET", "/admin-api/v1/channels", "", func(s *store.Store, _ *config.Config) http.HandlerFunc {
			return (&AdminHandler{Store: s, Logger: testLogger()}).handleListChannels
		}},
		{"agent-list", "GET /api/v1/agents", "GET", "/api/v1/agents", "", func(s *store.Store, _ *config.Config) http.HandlerFunc {
			return (&AgentHandler{Store: s, Logger: testLogger()}).handleListAgents
		}},
		{"channel-list", "GET /api/v1/channels", "GET", "/api/v1/channels", "", func(s *store.Store, cfg *config.Config) http.HandlerFunc {
			return (&ChannelHandler{Store: s, Config: cfg, Logger: testLogger()}).handleListChannels
		}},
		{"group-list", "GET /api/v1/channel-groups", "GET", "/api/v1/channel-groups", "", func(s *store.Store, cfg *config.Config) http.HandlerFunc {
			return (&ChannelHandler{Store: s, Config: cfg, Logger: testLogger()}).handleListGroups
		}},
		{"dm-list", "GET /api/v1/dm", "GET", "/api/v1/dm", "", func(s *store.Store, cfg *config.Config) http.HandlerFunc {
			return (&DmHandler{Store: s, Config: cfg, Logger: testLogger()}).handleListDms
		}},
		{"remote-list", "GET /api/v1/remote/nodes", "GET", "/api/v1/remote/nodes", "", func(s *store.Store, _ *config.Config) http.HandlerFunc {
			return (&RemoteHandler{Store: s, Logger: testLogger()}).handleListNodes
		}},
		{"user-online", "GET /api/v1/online", "GET", "/api/v1/online", "", func(s *store.Store, _ *config.Config) http.HandlerFunc {
			return (&UserHandler{Store: s, Logger: testLogger()}).handleOnlineUsers
		}},
		{"workspace-all", "GET /api/v1/workspaces", "GET", "/api/v1/workspaces", "", func(s *store.Store, cfg *config.Config) http.HandlerFunc {
			return (&WorkspaceHandler{Store: s, Config: cfg, Logger: testLogger()}).handleListAllWorkspaces
		}},
		{"workspace-mkdir", "POST /api/v1/channels/{channelId}/workspace/mkdir", "POST", "/api/v1/channels/ch/workspace/mkdir", `{"name":"dir"}`, func(s *store.Store, cfg *config.Config) http.HandlerFunc {
			return (&WorkspaceHandler{Store: s, Config: cfg, Logger: testLogger()}).handleMkdir
		}},
	}

	for _, tc := range tests {
		tc := tc // capture loop var for parallel sub-test
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel() // TEST-FIX-2: 11 sub-test 各启独立 in-memory store + httptest, 互不依赖, 用 parallel 把 race 总耗时从 ~30s 串行降到 ~3s 并发 (CI runner 慢需要双重加速). 跟 TEST-FIX-1 #596 同一思路, 与 ctx-aware leak fix 互补.
			// TEST-FIX-3 设计 ②: 使用 SSOT helper newTestServerWithClosedStore
			// (testfixture_test.go) 替代 inline boilerplate; helper 内置 ctx-aware
			// 双保险 (t.Context() + WithCancel + t.Cleanup), 反 #608 leak 复发.
			_, ts, s, cfg := newTestServerWithClosedStore(t)
			token := loginAs(t, ts.URL, "owner@test.com", "password123")
			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}
			handler := tc.build(s, cfg)
			rec := exerciseAuthedHandler(t, s, cfg, token, tc.pattern, tc.method, tc.target, body, func(w http.ResponseWriter, r *http.Request) {
				_ = s.Close()
				handler(w, r)
			})
			// ADM-0.3: workspace-mkdir's membership pre-check now runs before
			// the store call (no admin short-circuit), so a non-member request
			// against an unknown channel exits with 403 before triggering the
			// closed-store 500 path. Other handlers still hit the closed store
			// directly and return 500.
			if tc.name == "workspace-mkdir" {
				if rec.Code != http.StatusForbidden {
					t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
				}
				return
			}
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}

	ts, s, cfg := setupFullTestServer(t)
	token := loginAs(t, ts.URL, "owner@test.com", "password123")
	mux := http.NewServeMux()
	(&CommandHandler{Store: s, Logger: testLogger(), Hub: commandSourceStub{}}).RegisterRoutes(mux, auth.AuthMiddleware(s, cfg))
	req := httptest.NewRequest("GET", "/api/v1/commands", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("commands expected 200, got %d", rec.Code)
	}
}

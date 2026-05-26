package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"borgee-server/internal/admin"
	"borgee-server/internal/agent"
	"borgee-server/internal/api"
	"borgee-server/internal/auth"
	"borgee-server/internal/bpp"
	"borgee-server/internal/config"
	"borgee-server/internal/datalayer"
	"borgee-server/internal/devartifacts"
	"borgee-server/internal/presence"
	"borgee-server/internal/push"
	"borgee-server/internal/store"
	"borgee-server/internal/testutil/clock"
	"borgee-server/internal/ws"
)

type Server struct {
	cfg    *config.Config
	logger *slog.Logger
	store  *store.Store
	// dl is the DL-1 canonical bundle (Storage / Presence / EventBus / 3 Repository).
	// Wired once in New(); handlers receive it via DI to avoid direct store
	// dependency. v3+ swaps underlying implementations in datalayer.NewDataLayer only.
	dl           *datalayer.DataLayer
	mux          *http.ServeMux
	hub          *ws.Hub
	agentTracker *agent.Tracker
	startTime    time.Time
	// ctx is the lifetime-bound context for goroutines spawned in New() +
	// Handler() (rateLimiter cleanup loop). Tests pass t.Context() so the
	// goroutines exit on test teardown, preventing leak into next sub-test
	// (TEST-FIX-2: was nil ctx → unbounded ticker leak → DB-closed write
	// → panic: test timed out after 2m0s).
	ctx context.Context
	// clk is the time source for JWT mint (PERF-JWT-CLOCK). Defaults to Real
	// in New(); tests inject *clock.Fake via SetClock to advance JWT iat
	// without sleeping. nil-safe via getClock().
	clk clock.Clock
	// authHandler is held so SetClock can wire injected clock post-construction.
	authHandler *api.AuthHandler
}

// SetClock injects a clock for JWT mint. Tests use *clock.Fake to advance
// JWT iat (1s granularity) without time.Sleep. Production never calls this —
// New() leaves clk nil and AuthHandler falls back to time.Now() (matching the
// pre-refactor path).
func (s *Server) SetClock(c clock.Clock) {
	s.clk = c
	// Re-wire AuthHandler.Clock — handler was constructed in SetupRoutes
	// with Clock=nil; we patch the field here so subsequent JWT mints use
	// the injected clock. Routes are only mounted once, AuthHandler is the
	// owner of /api/v1/auth/login so this is the single mutation point.
	if s.authHandler != nil {
		s.authHandler.Clock = c
	}
}

func New(ctx context.Context, cfg *config.Config, logger *slog.Logger, s *store.Store) *Server {
	hub := ws.NewHub(s, logger, cfg)

	// AL-3.2: wire the presence write end so /ws lifecycle hooks can
	// fan in TrackOnline / TrackOffline. NewSessionsTracker only errors
	// on a nil DB handle, which is a boot-time programming error.
	var presenceTracker presence.PresenceTracker
	if pw, err := presence.NewSessionsTracker(s.DB()); err == nil {
		hub.SetPresenceWriter(pw)
		presenceTracker = pw
	} else {
		logger.Error("presence tracker init failed (continuing without presence_sessions writes)", "err", err)
	}

	// DL-1.2: wire the canonical 4-interface bundle (Storage / Presence / EventBus
	// / 3 Repository). v1 preserves existing store + presence behavior.
	dl := datalayer.NewDataLayer(s, presenceTracker, logger)

	srv := &Server{
		cfg:          cfg,
		logger:       logger,
		store:        s,
		dl:           dl,
		mux:          http.NewServeMux(),
		hub:          hub,
		agentTracker: agent.NewTracker(),
		startTime:    time.Now(),
		ctx:          ctx,
	}
	srv.SetupRoutes()

	hub.SetHandler(srv.Handler())

	// BPP-3 plugin-upstream BPP frame dispatcher boundary. Wires the
	// AL-2b agent_config_ack frame ingress (deferred from #481 to BPP-3
	// per plugin.go default-case routing). Construction order:
	//   1. concrete api-side AgentConfigAckHandler + OwnerResolver
	//   2. typed bpp.AckDispatcher (validates Status/Reason/cross-owner)
	//   3. AckFrameAdapter wraps typed dispatcher into FrameDispatcher
	//   4. PluginFrameDispatcher registers (FrameTypeBPPAgentConfigAck → adapter)
	//   5. hub.SetPluginFrameRouter — plugin.go read loop default case
	//      now routes any non-RPC envelope frame here.
	ackHandler := &api.AgentConfigAckHandlerImpl{Store: s, Logger: logger}
	ownerResolver := &api.AgentOwnerResolver{Store: s}
	ackDispatcher := bpp.NewAckDispatcher(ackHandler, ownerResolver)
	pfd := bpp.NewPluginFrameDispatcher(logger)
	pfd.Register(bpp.FrameTypeBPPAgentConfigAck, bpp.NewAckFrameAdapter(ackDispatcher))

	// BPP-5 reconnect handshake — plugin upstream signals reconnect
	// with last_known_cursor; handler resolves cursor via RT-1.3
	// ResolveResume (incremental mode) + clears agent error (AL-1
	// 5-state error → online valid edge, agent.Tracker.Clear is the central reset).
	// Reuses the BPP-3 PluginFrameDispatcher boundary.
	reconnectHandler := bpp.NewReconnectHandler(s,
		&channelScopeAdapter{store: s},
		ownerResolver,
		srv.agentTracker,
		logger)
	pfd.Register(bpp.FrameTypeBPPReconnectHandshake, reconnectHandler)

	// BPP-6 cold-start handshake — plugin upstream signals process restart
	// (state is lost, no cursor; distinct from BPP-5 reconnect). Handler uses
	// AL-1 #492 AppendAgentStateTransition any→online + agent.Tracker.Clear;
	// reason reuses `runtime_crashed` from the existing AL-1a reason set.
	// Reuses the BPP-3 PluginFrameDispatcher boundary.
	coldStartHandler := bpp.NewColdStartHandler(s, ownerResolver, srv.agentTracker, logger)
	pfd.Register(bpp.FrameTypeBPPColdStartHandshake, coldStartHandler)

	// RT-3 task lifecycle → AgentTaskStateChanged fanout. Plugin upstream
	// task_started / task_finished frames produce AgentTaskStateChangedFrame
	// (busy/idle) via Hub.PushAgentTaskStateChanged. Multi-device fanout uses
	// BroadcastToChannel (P1MultiDeviceWebSocket #197 pattern). ValidateTask*
	// enforces non-empty thinking subject, the 3-value outcome enum, and AL-1a reasons.
	taskLifecycleHandler := bpp.NewTaskLifecycleHandler(
		&hubAgentTaskPusherAdapter{hub: hub}, logger)
	// WIRE-1 wire-3 — DL-4 push gateway fanout (mobile background) via
	// AgentTaskNotifier. Member lookup uses store.ListChannelMembers; notifier
	// uses push.NewAgentTaskNotifier around pushGW. nil-safe: when pushGW is noop,
	// notifier nil skips fanout.
	pushGWForTask, errPush := push.NewGateway(s, logger)
	if errPush != nil {
		pushGWForTask = push.NewNoopGateway(logger)
	}
	agentTaskNotifier := push.NewAgentTaskNotifier(pushGWForTask)
	taskLifecycleHandler.SetPushFanout(
		&channelMemberFetcherAdapter{store: s},
		agentTaskNotifier,
	)
	pfd.Register(bpp.FrameTypeBPPTaskStarted, taskLifecycleHandler.StartedAdapter())
	pfd.Register(bpp.FrameTypeBPPTaskFinished, taskLifecycleHandler.FinishedAdapter())

	hub.SetPluginFrameRouter(&pluginFrameRouterAdapter{pfd: pfd})

	// BPP-4.1 heartbeat watchdog: 30s plugin liveness threshold, flips
	// stale agents to error/network_unreachable via agent.Tracker
	// (AL-1a reason set). Boot-only wire-up; nil-safe in
	// tests via separate (NewTestServer doesn't invoke this path).
	watchdog := bpp.NewHeartbeatWatchdog(&hubLivenessAdapter{hub}, srv.agentTracker, logger)

	if !cfg.DisableBackgroundWorkers {
		go hub.StartHeartbeat(ctx)
		go watchdog.Run(ctx)
	}

	return srv
}

func (s *Server) Hub() *ws.Hub {
	return s.hub
}

func (s *Server) SetupRoutes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)

	authHandler := &api.AuthHandler{
		Store:  s.store,
		Config: s.cfg,
		Logger: s.logger,
		Clock:  s.clk, // nil → handler.now() falls back to time.Now() (production path)
	}
	s.authHandler = authHandler
	authHandler.RegisterRoutes(s.mux)

	authMw := auth.AuthMiddleware(s.store, s.cfg)
	s.mux.Handle("GET /api/v1/users/me", authMw(http.HandlerFunc(authHandler.HandleGetMe)))

	broadcaster := &hubBroadcastAdapter{s.hub}

	// Messages
	// DM-2.2 (#312): wire the mention dispatcher. PresenceTracker is the
	// AL-3 read side (#310 SessionsTracker); ws.Hub satisfies the
	// MentionFrameBroadcaster interface via PushMentionPushed (#NNN
	// mention_pushed_frame.go). Nil presence => skip dispatch (legacy
	// boot path, smoke survives without DM-2 fanout).
	//
	// DL-4.3 push gateway — server→browser fan-out via VAPID. Init early
	// so MentionDispatcher can pick up MentionPushNotifier seam. Falls
	// back to noop when VAPID env unset (跟 admin Bootstrap 区分: push
	// 是体验补丁不阻 server 启动).
	pushGW, err := push.NewGateway(s.store, s.logger)
	if err != nil {
		s.logger.Info("push.NewGateway: VAPID env unset, falling back to noop", "err", err)
		pushGW = push.NewNoopGateway(s.logger)
	}
	mentionPushNotifier := push.NewMentionNotifier(pushGW)

	var mentionDispatcher *api.MentionDispatcher
	if pt, err := presence.NewSessionsTracker(s.store.DB()); err == nil {
		mentionDispatcher = api.NewMentionDispatcher(s.store, pt, s.hub)
		// DL-4.6 cross-device fan-out — mention also fires push (best-
		// effort, browser SW handles visibility-based dedup).
		mentionDispatcher.PushNotifier = mentionPushNotifier
	} else {
		s.logger.Warn("mention dispatcher init skipped — presence tracker unavailable", "err", err)
	}

	msgHandler := &api.MessageHandler{
		Store:    s.store,
		Logger:   s.logger,
		Hub:      broadcaster,
		Mentions: mentionDispatcher,
	}
	sendPerm := auth.RequirePermission(s.store, "message.send", func(r *http.Request) string {
		return "channel:" + r.PathValue("channelId")
	})
	// AP-0-bis: agent default capability set 锁 [message.send, message.read].
	// Legacy agents are backfilled by migration v=8 (ap_0_bis_message_read).
	readPerm := auth.RequirePermission(s.store, "message.read", func(r *http.Request) string {
		return "channel:" + r.PathValue("channelId")
	})
	msgHandler.RegisterRoutes(s.mux, authMw, sendPerm, readPerm)

	// DM-4.1 — agent message edit 多端同步. PATCH /api/v1/channels/{channelId}/messages/{messageId}
	// 走 RT-3 既有 fan-out (events INSERT message_edited + Hub.BroadcastEventToChannel
	// 多端覆盖). DM-only 路径校验 (channel.Type != "dm" → 403). owner-only ACL.
	dm4EditHandler := &api.MessageEditHandler{Store: s.store, Hub: broadcaster, Logger: s.logger}
	dm4EditHandler.RegisterRoutes(s.mux, authMw)

	// Users
	userHandler := &api.UserHandler{
		Store:     s.store,
		DataLayer: s.dl,
		Logger:    s.logger,
	}
	userHandler.RegisterRoutes(s.mux, authMw)

	// CHN-3.2 user_channel_layout — personal preferences (users read and write
	// their own rows; admin API allowlist excludes user_channel_layout, ADM-0 §1.3).
	layoutHandler := &api.LayoutHandler{Store: s.store, Logger: s.logger}
	layoutHandler.RegisterRoutes(s.mux, authMw)

	// BPP-3.2.2 owner DM one-click grant — POST /api/v1/me/grants.
	// Owner-only ACL; capability must be one of the AP-1 constants and scope
	// must match the v1 scope levels. action ∈ {grant, reject, snooze};
	// reject/snooze v1 only writes audit data.
	meGrantsHandler := &api.MeGrantsHandler{Store: s.store, Logger: s.logger}
	meGrantsHandler.RegisterRoutes(s.mux, authMw)

	// AL-2a.2 agent_configs — canonical REST endpoints (owner-only,
	// fail-closed runtime-field reject, acceptance #264 §4.1.a-d). After PATCH,
	// fanout uses hub.PushAgentConfigUpdate best-effort; offline plugins miss the
	// frame and fetch the latest config after reconnect.
	agentConfigHandler := &api.AgentConfigHandler{
		Store:  s.store,
		Logger: s.logger,
		Pusher: s.hub, // AL-2b §2.1 server→plugin BPP fanout seam
	}
	agentConfigHandler.RegisterRoutes(s.mux, authMw)

	// HB-3.1 host_grants canonical REST endpoints. Owner-only ACL; admin API
	// does not handle user-owned grant writes (ADM-0 §1.3). Audit log fields stay
	// aligned with HB-1 / HB-2 / BPP-4.
	hostGrantsHandler := &api.HostGrantsHandler{
		Store:  s.store,
		Logger: s.logger,
	}
	hostGrantsHandler.RegisterRoutes(s.mux, authMw)

	// Helper enrollment/status foundation. User management routes use authMw;
	// claim/status/uninstall use the distinct Helper credential rail.
	helperEnrollmentHandler := &api.HelperEnrollmentHandler{Repo: s.dl.HelperEnrollmentRepo, JobRepo: s.dl.HelperJobRepo, Logger: s.logger, PublicHelperOrigin: s.cfg.PublicHelperOrigin}
	helperEnrollmentHandler.RegisterRoutes(s.mux, authMw)
	// PR-4 amend (#1033) — load BORGEE_MANIFEST_SIGNING_KEY here (earlier
	// than the HB-1 plugin manifest handler below) so the helper-jobs
	// handler can hold a HelperManifestProvider on the same key. The
	// plugin manifest handler also reuses hb1SigningKey at its
	// registration point further down — single read, two consumers, one
	// trust root on the daemon side (BORGEE_MANIFEST_SIGNING_PUBKEY).
	hb1SigningKey, hb1KeyErr := api.LoadSigningKey(s.logger)
	if hb1KeyErr != nil && s.logger != nil {
		s.logger.Error("hb1.signing_key_invalid",
			"env", api.EnvManifestSigningKey,
			"error", hb1KeyErr.Error(),
			"effect", "manifest entries served unsigned; install-butler will reject in production")
	}
	helperJobsHandler := &api.HelperJobsHandler{
		Repo: s.dl.HelperJobRepo,
		// #1049: list endpoint verifies enrollment ownership BEFORE
		// returning rows so cross-owner returns 404 rather than 200+empty.
		EnrollmentRepo: s.dl.HelperEnrollmentRepo,
		// PR-4 amend (#1033): server emits the signed canonical helper-
		// policy manifest body in every leased-job payload so the daemon
		// can run jobpolicy.Evaluate against an actual signed manifest
		// (binding's PathIDs/ServiceIDs are resolved against the manifest's
		// authoritative declarations).
		ManifestProvider: api.NewHelperManifestProvider(hb1SigningKey),
		Logger:           s.logger,
	}
	// PR-4 final amend: wire the WS push adapter so handleEnqueue can
	// push immediately to a connected daemon (sub-second job latency
	// instead of REST poll's ~5s retry-after window). The adapter
	// closes over *ws.Hub; api package stays import-clean of internal/ws.
	helperJobsHandler.SetPushAdapter(&helperJobsPushAdapter{hub: s.hub})
	// PR-4 final amend: on every daemon WS reconnect, drain any jobs
	// that queued while it was disconnected. The hook fires once per
	// RegisterHelper from internal/ws.HandleHelper.
	s.hub.SetHelperConnectHook(func(enrollmentID string) {
		helperJobsHandler.PushQueuedToHelper(context.Background(), enrollmentID)
	})
	helperJobsHandler.RegisterRoutes(s.mux, authMw)

	// AL-1.4 agent state log — owner-only GET /api/v1/agents/:id/state-log
	// (蓝图 §2.3 "故障可解释" — owner 看 agent state 历史轨迹查病因).
	al14Handler := &api.AgentStateLogHandler{Store: s.store, Logger: s.logger}
	al14Handler.RegisterRoutes(s.mux, authMw)

	// AL-5 agent error recovery — owner-only POST /api/v1/agents/:id/recover
	// (蓝图 §2.3 5-state error → online recovery; 复用 AL-1 #492 single-gate
	// helper, 不裂状态机).
	al5Handler := &api.AgentRecoverHandler{Store: s.store, DataLayer: s.dl, Logger: s.logger}
	al5Handler.RegisterRoutes(s.mux, authMw)

	// BPP-8.2 plugin lifecycle audit list — owner-only GET
	// /api/v1/agents/{agentId}/lifecycle (复用 admin_actions audit forward-only,
	// 跟 ADM-2.1 + AP-2 + BPP-4 跨四 milestone audit 同精神 锁链第 5 处;
	// no admin override route is mounted; ADM-0 §1.3 prohibits that path).
	bpp8Handler := &api.PluginListHandler{Store: s.store, Logger: s.logger}
	bpp8Handler.RegisterRoutes(s.mux, authMw)
	// HB-3 v2 heartbeat decay list — owner-only GET
	// /api/v1/agents/{agentId}/heartbeat-decay (decay 状态从 agent_runtimes.
	// last_heartbeat_at 反向 derive, 0 schema 改; AL-1a 锁链第 14 处 复用
	// reasons.NetworkUnreachable; no admin override route is mounted per ADM-0 §1.3).
	hb3v2Handler := &api.HostDecayListHandler{Store: s.store, Logger: s.logger}
	hb3v2Handler.RegisterRoutes(s.mux, authMw)

	// DL-4 web push subscriptions — POST/DELETE /api/v1/push/subscribe.
	// 蓝图 client-shape.md L22 (Mobile PWA + Web Push VAPID).
	pushSubsHandler := &api.PushSubscriptionsHandler{
		Store:  s.store,
		Logger: s.logger,
	}
	pushSubsHandler.RegisterRoutes(s.mux, authMw)

	// DL-4.4 PWA Web App Manifest — GET /api/v1/pwa/manifest (公开 endpoint,
	// 浏览器 install prompt 在 login 前 fetch). 蓝图 client-shape.md L42
	// (manifest + install prompt + Web Push + standalone).
	// Endpoint naming note: distinct from HB-1 #491 GET /api/v1/plugin-manifest
	// (binary plugin manifest, dual signatures required). Different endpoint,
	// different security model; keep names distinct during endpoint review.
	pwaManifestHandler := &api.PWAManifestHandler{}
	pwaManifestHandler.RegisterRoutes(s.mux)

	// HB-1 install-butler server-side `GET /api/v1/plugin-manifest` (v0 [A]
	// scope). Bearer api-key auth is enforced by authMw; no admin override route.
	// manifest data 走 LoadManifestEntries 三档 fallback (env JSON > env file
	// > 内置默认), 立场 ② 0 schema 不变.
	// 私钥 BORGEE_MANIFEST_SIGNING_KEY (base64 ed25519 seed, 32 字节解码后)
	// 启动时一次性读取 (above, at helperJobsHandler init — same key for
	// both manifest endpoints); 缺省 fall-soft (per-entry Signature 留空 +
	// 顶层 signature 走 test placeholder), 日志 warn 一行. 生产由监控抓
	// warn 强制配齐. 详 docs/current/host-bridge/manifest-signing.md.
	hb1ManifestHandler := &api.PluginManifestHandler{Logger: s.logger, SigningKey: hb1SigningKey}
	hb1ManifestHandler.RegisterRoutes(s.mux, authMw)

	// #1050 blocker #6 dev-stack artifact server. Only mounted when
	// BORGEE_DEV_ARTIFACTS_DIR is set (production leaves it unset and
	// these handlers are never registered). Serves the openclaw sentinel
	// shipped in scripts/dev-stack/artifacts/ so a local docker-compose
	// dev-stack can complete an end-to-end install_from_manifest job
	// without reaching the unprovisioned production CDN. Signing key
	// reuses hb1SigningKey so install-butler's ed25519 trust check
	// passes byte-identical with prod.
	if devArtifactsDir := strings.TrimSpace(os.Getenv("BORGEE_DEV_ARTIFACTS_DIR")); devArtifactsDir != "" {
		reg, err := devartifacts.LoadFromDir(devArtifactsDir, s.logger)
		if err != nil {
			s.logger.Error("devartifacts load failed", "err", err)
		} else if len(reg.Entries()) > 0 {
			devHandler := &devartifacts.Handler{
				Registry:        reg,
				SigningKey:      hb1SigningKey,
				ManifestURLBase: strings.TrimSpace(os.Getenv("BORGEE_DEV_MANIFEST_ORIGIN_BASE")),
				Logger:          s.logger,
			}
			devHandler.RegisterRoutes(s.mux)
			s.logger.Info("devartifacts.mounted",
				"dir", devArtifactsDir,
				"entries", len(reg.Entries()))
		}
	}

	// (DL-4.3 push gateway init moved earlier — line ~85 — to feed
	// MentionDispatcher.PushNotifier.)

	// Channels
	// AL-3.3 agent member presence fold — handleListMembers attaches the
	// live runtime state on `role=='agent'` rows so the client's
	// MemberPresence has a REST fallback when WS presence cache is cold.
	// Mirrors DmHandler.withPeerState (api/dm.go) — same adapter, same
	// resolver. Constructed inline here (deps are s.hub + s.agentTracker
	// — both ready well before this boot point).
	channelStateAdapter := &agentRuntimeAdapter{hub: s.hub, tracker: s.agentTracker}
	channelHandler := &api.ChannelHandler{Store: s.store, Config: s.cfg, Logger: s.logger, Hub: broadcaster, State: channelStateAdapter}
	channelHandler.RegisterRoutes(s.mux, authMw)
	// CHN-5 archived channels — owner-only user-rail GET + admin-rail readonly
	// GET (no PATCH/PUT/DELETE on admin path; admin god-mode ADM-0 §1.3 红线).
	channelHandler.RegisterCHN5Routes(s.mux, authMw)
	// CHN-6 channel pin/unpin — owner-only user-rail POST/DELETE; 0 schema
	// 改 (复用 CHN-3.1 user_channel_layout, position < 0 = pinned). admin
	// god-mode 不挂 (ADM-0 §1.3 红线 — pin 是 per-user preference).
	channelHandler.RegisterCHN6Routes(s.mux, authMw)
	// CHN-7 channel mute/unmute — owner-only user-rail POST/DELETE; 0 schema
	// 改 (复用 CHN-3.1 user_channel_layout, collapsed bitmap bit 1 = mute).
	// No admin override route is mounted (ADM-0 §1.3; mute is a per-user preference).
	channelHandler.RegisterCHN7Routes(s.mux, authMw)
	// CHN-15 channel readonly toggle — owner-only user-rail PUT/DELETE; 0
	// schema 改 (复用 user_channel_layout.collapsed bitmap bit 4 走
	// channel.created_by canonical row). No admin override route is mounted (ADM-0 §1.3).
	channelHandler.RegisterCHN15Routes(s.mux, authMw)
	// CHN-10 channel description (owner-only PUT /channels/:id/description,
	// 0 schema 改 复用 channels.topic 既有列, 500 字符上限 matches
	// 跟 GORM size:500 + client DESCRIPTION_MAX_LENGTH 同源 — 双向锁守门).
	// No admin override route is mounted (ADM-0 §1.3); 既有 PUT /topic member-level
	// path unchanged (CHN-2 #406 既有 path 不破).
	chn10DescHandler := &api.ChannelDescriptionHandler{Store: s.store, Logger: s.logger}
	chn10DescHandler.RegisterUserRoutes(s.mux, authMw)
	// CHN-14 channel description edit history audit — owner-only user-rail GET.
	// schema v=44 ALTER channels ADD COLUMN description_edit_history TEXT NULL
	// (跟 DM-7.1 #558 messages.edit_history 同模式; 跨八 milestone ALTER ADD
	// nullable). UpdateChannelDescription canonical wrapper SELECT old + JSON append
	// + UPDATE; CHN-10 #561 既有 PUT path 调用此包装 unchanged (length
	// cap 500 + owner-only ACL 不变). admin readonly GET 在 adminMw 段挂.
	chn14HistoryHandler := &api.ChannelDescriptionHistoryHandler{Store: s.store, Logger: s.logger}
	chn14HistoryHandler.RegisterUserRoutes(s.mux, authMw)
	// CHN-8 channel notification preferences — owner-only PUT three states
	// (`all`/`mention`/`none`). 0 schema 改 (复用 user_channel_layout.collapsed
	// bits 2-3 kept separate from CHN-3 bit 0 + CHN-7 bit 1). No admin override
	// route is mounted (ADM-0 §1.3; pref is a per-user preference).
	channelHandler.RegisterCHN8Routes(s.mux, authMw)

	// RT-4 channel presence indicator — member-only GET /channels/:id/presence;
	// 0 schema (复用 AL-3.1 #277 presence_sessions). 0 新 WS frame (presence
	// push 留 v3). No admin override route is mounted (ADM-0 §1.3). 既有 RT-2 typing
	// path unchanged.
	rt4Tracker, _ := presence.NewSessionsTracker(s.store.DB())
	rt4PresenceHandler := &api.RealtimePresenceHandler{
		Store:   s.store,
		Tracker: rt4Tracker,
		Logger:  s.logger,
	}
	rt4PresenceHandler.RegisterUserRoutes(s.mux, authMw)

	agentStateAdapter := &agentRuntimeAdapter{hub: s.hub, tracker: s.agentTracker}

	// DMs
	dmHandler := &api.DmHandler{Store: s.store, Config: s.cfg, Logger: s.logger, State: agentStateAdapter}
	dmHandler.RegisterRoutes(s.mux, authMw)

	// Admin (ADM-0.2: cookie拆 + RequirePermission去admin短路 + god-mode 元数据-only)
	// Bootstrap is fail-loud (panics on missing env). Tests inject env or use
	// testutil. The legacy api.AdminAuthHandler / api.AdminAuthMiddleware paths
	// are retired in this PR — admin auth is exclusively the borgee_admin_session
	// cookie backed by admin_sessions rows.
	if err := admin.Bootstrap(s.store.DB()); err != nil {
		s.logger.Error("admin bootstrap failed", "error", err)
	}
	adminAuthHandler := &admin.Handler{DB: s.store.DB(), Logger: s.logger, IsDevelopment: s.cfg.IsDevelopment()}
	adminAuthHandler.RegisterRoutes(s.mux)
	adminMw := admin.RequireAdmin(s.store.DB(), nil)
	adminHandler := &api.AdminHandler{Store: s.store, Logger: s.logger}
	adminHandler.RegisterRoutes(s.mux, adminMw)
	// CHN-5 admin-rail readonly archived channels GET (无 PATCH/PUT/DELETE
	// — admin god-mode ADM-0 §1.3 红线 — admin 只观察 audit, 不直接改).
	channelHandler.RegisterCHN5AdminRoutes(s.mux, adminMw)
	// CHN-14 description edit history admin readonly GET — admin god-mode
	// 不挂 PATCH/DELETE (ADM-0 §1.3 红线 — admin 看 audit 不直接改).
	chn14HistoryHandler.RegisterAdminRoutes(s.mux, adminMw)
	// AL-4.2 admin god-mode metadata read for agent_runtimes (acceptance
	// §2.6 — read-only white-list, last_error_reason omitted; ADM-0 §1.3
	// rail isolation + 立场 ⑦ same source).
	adminRuntimeHandler := &api.AdminRuntimeHandler{Store: s.store, Logger: s.logger}
	adminRuntimeHandler.RegisterRoutes(s.mux, adminMw)
	// ADM-2.2 admin-rail audit-log (/admin-api/v1/audit-log). User-rail
	// (/api/v1/me/admin-actions + /api/v1/me/impersonation-grant CRUD) was
	// removed in #975 with the user-facing privacy UI; admin SPA consumes
	// /admin-api/v1/audit-log directly.
	adm2Handler := &api.AdminEndpointsHandler{Store: s.store, Logger: s.logger}
	adm2Handler.RegisterAdminRoutes(s.mux, adminMw)
	// ADM-3 multi-source audit 合并查询 (admin-rail only, ADM-0 §1.3 红线).
	// 4 source UNION ALL: server (audit_events) / plugin (audit_events kind=plugin.*) /
	// host_bridge (HB-1 placeholder) / agent (DL-2 channel_events + global_events).
	(&api.AdminAuditMultiSourceHandler{Store: s.store, Logger: s.logger}).RegisterAdminRoutes(s.mux, adminMw)
	// DM-7 message edit history — sender-only user-rail GET + admin readonly
	// admin-rail GET (admin route is readonly; no PATCH/DELETE per ADM-0 §1.3).
	dm7EditHistoryHandler := &api.MessageEditHistoryHandler{Store: s.store, Logger: s.logger}
	dm7EditHistoryHandler.RegisterUserRoutes(s.mux, authMw)
	dm7EditHistoryHandler.RegisterAdminRoutes(s.mux, adminMw)
	// CV-15 artifact comment edit history — 0 schema 改 (复用 messages.edit_history
	// DM-7.1 v=34 既有列), GET endpoint scoped to content_type='artifact_comment'
	// (避免跟 DM-7 既有 /messages/{id}/edit-history 混淆). user-rail sender-only +
	// admin readonly admin-rail (no admin PATCH/DELETE/PUT routes, ADM-0 §1.3).
	cv15CommentEditHistoryHandler := &api.CanvasCommentEditHistoryHandler{Store: s.store, Logger: s.logger}
	cv15CommentEditHistoryHandler.RegisterUserRoutes(s.mux, authMw)
	cv15CommentEditHistoryHandler.RegisterAdminRoutes(s.mux, adminMw)
	// AL-7.2 admin-rail audit retention override (admin-model.md §3 retention
	// + ADM-0 §1.3 红线 admin 操作必走 audit row). admin-rail only — 反向 grep
	// `audit_retention_override` 在 user-rail handler 0 hit.
	al7RetentionHandler := &api.AgentRetentionOverrideHandler{Store: s.store, Logger: s.logger}
	al7RetentionHandler.RegisterAdminRoutes(s.mux, adminMw)
	// HB-6 heartbeat lag percentile monitor — admin-rail readonly GET
	// /admin-api/v1/heartbeat-lag (synchronous 30s rolling-window aggregate
	// from agent_runtimes.last_heartbeat_at, 0 schema 改). admin readonly
	// 不挂 PATCH/POST/DELETE (ADM-0 §1.3 红线). Reuses BPP-4 watchdog
	// 30s threshold via the shared WindowSeconds const.
	hb6LagHandler := &api.HostLagHandler{Store: s.store, Logger: s.logger}
	hb6LagHandler.RegisterAdminRoutes(s.mux, adminMw)
	// AL-7.2 retention sweeper goroutine (1h ticker, ctx-aware shutdown). Same
	// pattern as AP-2 ExpiresSweeper #525. Forward-only soft-archive via
	// admin_actions.archived_at column (立场 ① 不真删 / 不裂表).
	if !s.cfg.DisableBackgroundWorkers {
		(&auth.RetentionSweeper{Store: s.store, Logger: s.logger}).Start(s.ctx)
	}
	// HB-5.2 heartbeat retention sweeper + admin override (复用 AL-7 既有
	// audit retention override action; metadata target='heartbeat' 字面区分,
	// 立场 ② 不挂 admin_actions CHECK 第 13 项 enum).
	hb5HeartbeatRetentionHandler := &api.HostRetentionOverrideHandler{Store: s.store, Logger: s.logger}
	hb5HeartbeatRetentionHandler.RegisterAdminRoutes(s.mux, adminMw)
	if !s.cfg.DisableBackgroundWorkers {
		(&auth.HeartbeatRetentionSweeper{Store: s.store, Logger: s.logger}).Start(s.ctx)
	}
	// DL-2 events retention sweeper (1h ticker, ctx-aware shutdown). Reaps
	// expired rows from channel_events + global_events per per-kind retention
	// (must-persist 4 类永久 / channel/message 30d / agent_task/artifact 60d /
	// 默认 90d). 跟 AL-7 / HB-5 retention sweeper 同精神承袭.
	if !s.cfg.DisableBackgroundWorkers {
		datalayer.NewEventsRetentionSweeper(s.store.DB(), s.logger, time.Hour).Start(s.ctx)
	}
	// DL-3 §1 阈值哨 — 4 metric (db_size_mb / wal_pending_pages /
	// write_lock_wait_ms / events_row_count) WARN/CRITICAL slog 输出, ctx-aware
	// shutdown (蓝图 data-layer.md §5). 反 admin god-mode endpoint
	// (ADM-0 §1.3 红线): 仅 slog stdout, 不挂 /admin-api/threshold.
	if !s.cfg.DisableBackgroundWorkers {
		datalayer.NewThresholdMonitor(s.store.DB(), s.logger, time.Hour).Start(s.ctx)
	}
	// WIRE-1 §1 wire-2 — DL-3 cold archive offloader 真启 (跟 ThresholdMonitor
	// 同精神 ctx-aware Start). 1h ticker, threshold/cutoffAge/archiveDir 走
	// default (1M rows / 30d / ./data). audit "events.archive_offload" 走
	// dl.EventBus (DL-2 cold consumer 必落 kind).
	if !s.cfg.DisableBackgroundWorkers {
		datalayer.NewEventsArchiveOffloader(s.store.DB(), s.dl.EventBus, s.logger, "", 0, 0, time.Hour).Start(s.ctx)
	}
	// Note: AdminHandler.RegisterAppRoutes (the legacy /api/v1/admin/* user-rail
	// god-mode mount) is intentionally NOT wired — review checklist §ADM-0.2 §1
	// 反向断言 2.B (user cookie 调 admin endpoints 必须 401).

	// Agents
	agentHandler := &api.AgentHandler{Store: s.store, DataLayer: s.dl, Logger: s.logger, Hub: &hubPluginAdapter{s.hub}, State: agentStateAdapter}
	agentHandler.RegisterRoutes(s.mux, authMw)

	// AL-4.2 runtime registry user-rail (acceptance §2.1-§2.5 + §2.7) —
	// owner-only via the same inline OwnerID check used by agents.go handleDeleteAgent /
	// handleRotateAPIKey. Admin traffic does not enter this rail (admin path
	// 只 read 元数据 via AdminRuntimeHandler above).
	runtimeHandler := &api.RuntimeHandler{Store: s.store, Logger: s.logger}
	runtimeHandler.RegisterRoutes(s.mux, authMw)

	// Agent invitations (CM-4.1 + RT-0 #40 push wiring)
	agentInvitationHandler := &api.AgentInvitationHandler{Store: s.store, Logger: s.logger, Hub: s.hub}
	agentInvitationHandler.RegisterRoutes(s.mux, authMw)

	// Reactions
	reactionHandler := &api.ReactionHandler{Store: s.store, Logger: s.logger, Hub: broadcaster}
	reactionHandler.RegisterRoutes(s.mux, authMw)

	// DM-11 cross-DM message search (DM-only scope, channel-member ACL)
	dm11SearchHandler := &api.MessageSearchHandler{Store: s.store}
	dm11SearchHandler.RegisterRoutes(s.mux, authMw)

	// DM-10 message pin/unpin (DM-only scope)
	dm10PinHandler := &api.MessagePinHandler{Store: s.store, Logger: s.logger}
	dm10PinHandler.RegisterRoutes(s.mux, authMw)

	// Commands
	commandHandler := &api.CommandHandler{Store: s.store, DataLayer: s.dl, Logger: s.logger, Hub: &hubCommandAdapter{s.hub}}
	commandHandler.RegisterRoutes(s.mux, authMw)

	// Upload
	uploadHandler := &api.UploadHandler{Config: s.cfg, Logger: s.logger}
	uploadHandler.RegisterRoutes(s.mux, authMw)

	// Workspace
	workspaceHandler := &api.WorkspaceHandler{Store: s.store, Config: s.cfg, Logger: s.logger}
	workspaceHandler.RegisterRoutes(s.mux, authMw)

	// Remote
	remoteHandler := &api.RemoteHandler{Store: s.store, DataLayer: s.dl, Logger: s.logger, Hub: &hubRemoteAdapter{s.hub}}
	remoteHandler.RegisterRoutes(s.mux, authMw)

	// Poll/SSE
	pollHandler := &api.PollHandler{Store: s.store, Logger: s.logger, Hub: s.hub, Config: s.cfg}
	pollHandler.RegisterRoutes(s.mux, authMw)

	// CV-1.2 artifacts (canvas-vision §0; channel-scoped artifact CRUD +
	// commit + rollback + WS push). Pusher routes to ws.Hub which owns
	// the RT-1.1 ArtifactUpdated frame envelope (#290 contract).
	// IterationPusher (CV-4.2 commit path) routes the
	// running→completed transition push when commit carries
	// `?iteration_id=` query.
	artifactHandler := &api.ArtifactHandler{
		Store:           s.store,
		Logger:          s.logger,
		Hub:             broadcaster,
		Pusher:          &hubArtifactAdapter{s.hub},
		IterationPusher: &hubIterationAdapter{s.hub},
	}
	artifactHandler.RegisterRoutes(s.mux, authMw)

	// CV-2.2 anchor comments (canvas-vision §1.6; per-version anchor threads
	// + WS push). Pusher routes to ws.Hub which owns the AnchorCommentAdded
	// frame envelope (10 fields, matching spec v2 literals).
	anchorHandler := &api.AnchorHandler{
		Store:  s.store,
		Logger: s.logger,
		Hub:    broadcaster,
		Pusher: &hubAnchorAdapter{s.hub},
	}
	anchorHandler.RegisterRoutes(s.mux, authMw)

	// CV-5 artifact comments (canvas-vision §0 L24 字面 "Linear issue +
	// comment"). Comment row falls into messages table + virtual
	// `artifact:<id>` namespace channel (follows the DM-2 dm: namespace pattern — messages remains
	// the canonical table). Pusher routes to ws.Hub which owns the
	// ArtifactCommentAdded frame envelope (9 字段, RT-3 cursor 共序).
	artifactCommentsHandler := &api.ArtifactCommentsHandler{
		Store:  s.store,
		Logger: s.logger,
		Pusher: &hubArtifactCommentAdapter{s.hub},
	}
	artifactCommentsHandler.RegisterRoutes(s.mux, authMw)

	// CV-4.2 iterations (canvas-vision §1.4 + §1.5; owner-only iterate
	// orchestration + state machine + WS push). Pusher routes to ws.Hub
	// which owns the IterationStateChanged frame envelope (9 字段
	// matching spec #365 literals).
	iterationHandler := &api.IterationHandler{
		Store:  s.store,
		Logger: s.logger,
		Pusher: &hubIterationAdapter{s.hub},
	}
	iterationHandler.RegisterRoutes(s.mux, authMw)

	// WebSocket endpoints
	s.mux.HandleFunc("/ws", ws.HandleClient(s.hub))
	s.mux.HandleFunc("/ws/plugin", ws.HandlePlugin(s.hub))
	s.mux.HandleFunc("/ws/remote", ws.HandleRemote(s.hub))
	// PR-2 #1038 host-bridge helper WS transport. Authenticates via the
	// same Bearer credential + X-Helper-Device-Id pair the REST helper
	// rail uses (UpdateLastSeen does the digest + device check + last-
	// seen-at update under one DB write). The processor adapter routes
	// daemon-sent ack/result frames through the same store mutations
	// helper_jobs.go's REST handlers call.
	helperAuthAdapter := &helperEnrollmentAuthAdapter{repo: s.dl.HelperEnrollmentRepo}
	s.mux.HandleFunc("/ws/helper/{enrollmentId}", ws.HandleHelper(s.hub, helperAuthAdapter, helperJobsHandler))

	s.mux.HandleFunc("/api/v1/", respondNotImplemented)

	s.mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(s.cfg.UploadDir))))

	s.mux.HandleFunc("/", s.handleStatic)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"timestamp":  time.Now().UnixMilli(),
		"uptime":     time.Since(s.startTime).Seconds(),
		"ws_clients": s.hub.ClientCount(),
	})
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/admin-api" || strings.HasPrefix(r.URL.Path, "/admin-api/") || strings.HasPrefix(r.URL.Path, "/ws") {
		JSONError(w, http.StatusNotFound, "Not found")
		return
	}

	if r.URL.Path == "/admin" || strings.HasPrefix(r.URL.Path, "/admin/") {
		adminPath := filepath.Join(s.cfg.ClientDist, "admin.html")
		if _, err := os.Stat(adminPath); err == nil {
			http.ServeFile(w, r, adminPath)
			return
		}
	}

	filePath := filepath.Join(s.cfg.ClientDist, filepath.Clean(r.URL.Path))

	if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
		http.ServeFile(w, r, filePath)
		return
	}

	// SPA fallback: serve index.html for routes without file extensions
	if filepath.Ext(r.URL.Path) == "" {
		indexPath := filepath.Join(s.cfg.ClientDist, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)
			return
		}
	}

	http.NotFound(w, r)
}

func (s *Server) Handler() http.Handler {
	rl := newRateLimiter(s.ctx, s.cfg)

	var handler http.Handler = s.mux
	handler = rateLimitMiddleware(rl, s.store, s.cfg, handler)
	handler = securityHeadersMiddleware(handler)
	handler = corsMiddleware(s.cfg.IsDevelopment(), s.cfg.CORSOrigin, handler)
	handler = loggerMiddleware(s.logger, handler)
	handler = requestIDMiddleware(handler)
	handler = recoverMiddleware(s.logger, handler)

	return handler
}

type hubCommandAdapter struct {
	hub *ws.Hub
}

func (a *hubCommandAdapter) GetAllCommands() []api.AgentCommandGroup {
	all := a.hub.CommandStore().GetAll()
	result := make([]api.AgentCommandGroup, len(all))
	for i, g := range all {
		cmds := make([]api.AgentCmdDef, len(g.Commands))
		for j, c := range g.Commands {
			cmds[j] = api.AgentCmdDef{
				Name:        c.Name,
				Description: c.Description,
				Usage:       c.Usage,
			}
		}
		result[i] = api.AgentCommandGroup{
			AgentID:   g.AgentID,
			AgentName: g.AgentName,
			Commands:  cmds,
		}
	}
	return result
}

type hubRemoteAdapter struct {
	hub *ws.Hub
}

func (a *hubRemoteAdapter) IsNodeOnline(nodeID string) bool {
	return a.hub.GetRemote(nodeID) != nil
}

func (a *hubRemoteAdapter) ProxyRequest(nodeID string, action string, params map[string]string) (json.RawMessage, error) {
	rc := a.hub.GetRemote(nodeID)
	if rc == nil {
		return nil, fmt.Errorf("node offline")
	}
	return rc.SendRequest(map[string]any{
		"action": action,
		"params": params,
	})
}

type hubBroadcastAdapter struct {
	hub *ws.Hub
}

func (a *hubBroadcastAdapter) BroadcastEventToChannel(channelID string, eventType string, payload any) {
	a.hub.BroadcastEventToChannel(channelID, eventType, payload)
}

func (a *hubBroadcastAdapter) BroadcastEventToAll(eventType string, payload any) {
	a.hub.BroadcastEventToAll(eventType, payload)
}

func (a *hubBroadcastAdapter) BroadcastEventToUser(userID string, eventType string, payload any) {
	a.hub.BroadcastToUser(userID, map[string]any{
		"type": eventType,
		"data": payload,
	})
	a.hub.SignalNewEvents()
}

func (a *hubBroadcastAdapter) SignalNewEvents() {
	a.hub.SignalNewEvents()
}

// hubArtifactAdapter exposes ws.Hub.PushArtifactUpdated through the
// api.ArtifactPusher interface so internal/api does not import internal/ws
// (mirrors the AgentInvitationPusher / hubPluginAdapter pattern).
type hubArtifactAdapter struct {
	hub *ws.Hub
}

func (a *hubArtifactAdapter) PushArtifactUpdated(artifactID string, version int64, channelID string, updatedAt int64, kind string) (cursor int64, sent bool) {
	return a.hub.PushArtifactUpdated(artifactID, version, channelID, updatedAt, kind)
}

// hubAnchorAdapter exposes ws.Hub.PushAnchorCommentAdded through the
// api.AnchorCommentPusher interface so internal/api stays free of the
// internal/ws import (mirrors hubArtifactAdapter pattern).
type hubAnchorAdapter struct {
	hub *ws.Hub
}

func (a *hubAnchorAdapter) PushAnchorCommentAdded(
	anchorID string,
	commentID int64,
	artifactID string,
	artifactVersionID int64,
	channelID string,
	authorID string,
	authorKind string,
	createdAt int64,
) (cursor int64, sent bool) {
	return a.hub.PushAnchorCommentAdded(anchorID, commentID, artifactID, artifactVersionID, channelID, authorID, authorKind, createdAt)
}

// hubArtifactCommentAdapter exposes ws.Hub.PushArtifactCommentAdded through
// the api.ArtifactCommentPusher interface (CV-5, mirrors hubAnchorAdapter).
type hubArtifactCommentAdapter struct {
	hub *ws.Hub
}

func (a *hubArtifactCommentAdapter) PushArtifactCommentAdded(
	commentID string,
	artifactID string,
	channelID string,
	senderID string,
	senderRole string,
	bodyPreview string,
	createdAt int64,
) (cursor int64, sent bool) {
	return a.hub.PushArtifactCommentAdded(commentID, artifactID, channelID, senderID, senderRole, bodyPreview, createdAt)
}

// hubIterationAdapter exposes ws.Hub.PushIterationStateChanged through the
// api.IterationStatePusher interface so internal/api stays free of the
// internal/ws import (mirrors hubAnchorAdapter pattern). CV-4.2 commit path —
// same hub instance routes commit → completed push.
type hubIterationAdapter struct {
	hub *ws.Hub
}

func (a *hubIterationAdapter) PushIterationStateChanged(
	iterationID string,
	artifactID string,
	channelID string,
	state string,
	errorReason string,
	createdArtifactVersionID int64,
	completedAt int64,
) (cursor int64, sent bool) {
	return a.hub.PushIterationStateChanged(iterationID, artifactID, channelID, state, errorReason, createdArtifactVersionID, completedAt)
}

type hubPluginAdapter struct {
	hub *ws.Hub
}

// helperEnrollmentAuthAdapter — PR-2 #1038 host-bridge WS adapter. The
// hub's HandleHelper takes a narrow `HelperEnrollmentAuthenticator`
// interface (not the full repo) so unit tests can substitute a fake;
// in production, we route the call through the SQLite repo's
// UpdateLastSeen — same call the prior REST POST /status used to
// validate credential + device id and bump last_seen_at.
type helperEnrollmentAuthAdapter struct {
	repo datalayer.HelperEnrollmentRepository
}

func (a *helperEnrollmentAuthAdapter) UpdateLastSeen(ctx context.Context, id, credential, helperDeviceID string, now time.Time) (*datalayer.HelperEnrollment, error) {
	return a.repo.UpdateLastSeen(ctx, id, credential, helperDeviceID, now)
}

// helperJobsPushAdapter — PR-4 final amend host-bridge WS push
// adapter. Closes over *ws.Hub so the api.HelperJobsHandler can push
// frames without importing internal/ws (api → ws would cycle on
// ws.HelperJobProcessor). The hub exposes session lookup + send via
// concrete methods; this adapter forwards both.
type helperJobsPushAdapter struct {
	hub *ws.Hub
}

func (a *helperJobsPushAdapter) GetHelperSessionPlatform(enrollmentID string) (string, string, string, bool) {
	if a == nil || a.hub == nil {
		return "", "", "", false
	}
	sess := a.hub.GetHelper(enrollmentID)
	if sess == nil {
		return "", "", "", false
	}
	return string(sess.Platform()), sess.DeviceID(), sess.Credential(), true
}

func (a *helperJobsPushAdapter) SendJobFrameToHelper(enrollmentID string, jobJSON json.RawMessage) bool {
	if a == nil || a.hub == nil {
		return false
	}
	return a.hub.SendJobToHelper(enrollmentID, jobJSON)
}

func (a *hubPluginAdapter) ProxyPluginRequest(agentID string, method string, path string, body []byte) (int, []byte, error) {
	pc := a.hub.GetPlugin(agentID)
	if pc == nil {
		return 0, nil, fmt.Errorf("agent not connected")
	}
	resp, err := pc.SendRequest(method, path, body)
	if err != nil {
		return 0, nil, err
	}
	return resp.Status, resp.Body, nil
}

// agentRuntimeAdapter — AL-1a (#R3) wiring. Folds hub plugin presence
// (online vs offline) with the in-memory error tracker. ResolveAgentState
// is the single read path; SetAgentError is the runtime fault sidedoor
// the api package calls when ProxyPluginRequest reports a classified error.
type agentRuntimeAdapter struct {
	hub     *ws.Hub
	tracker *agent.Tracker
}

func (a *agentRuntimeAdapter) ResolveAgentState(agentID string) agent.Snapshot {
	return a.tracker.Resolve(agentID, a.hub.GetPlugin(agentID) != nil)
}

func (a *agentRuntimeAdapter) SetAgentError(agentID, reason string) {
	a.tracker.SetError(agentID, reason)
}

// pluginFrameRouterAdapter wires *bpp.PluginFrameDispatcher into the
// ws.PluginFrameRouter interface (跟 hubArtifactAdapter / hubAnchorAdapter
// 同模式 — internal/ws 不 import internal/bpp; bpp.PluginSessionContext
// matches ws.PluginSessionContext's single OwnerUserID field).
type pluginFrameRouterAdapter struct {
	pfd *bpp.PluginFrameDispatcher
}

func (a *pluginFrameRouterAdapter) Route(raw []byte, sess ws.PluginSessionContext) (bool, error) {
	return a.pfd.Route(raw, bpp.PluginSessionContext{OwnerUserID: sess.OwnerUserID})
}

// hubLivenessAdapter wires *ws.Hub.SnapshotPluginLastSeen into the
// bpp.PluginLivenessSource interface (跟 pluginFrameRouterAdapter 同模式
// — internal/ws 不 import internal/bpp; 接口名差异 SnapshotPluginLastSeen
// vs SnapshotLastSeen 用 adapter 桥).
type hubLivenessAdapter struct {
	hub *ws.Hub
}

func (a *hubLivenessAdapter) SnapshotLastSeen() map[string]time.Time {
	return a.hub.SnapshotPluginLastSeen()
}

// channelScopeAdapter wires *store.Store.GetUserChannelIDs into the
// bpp.ChannelScopeResolver interface (跟 hubLivenessAdapter 同模式) —
// signature 差异: store 返 []string 无 error, interface 返 ([]string, error)
// 跟 RT-1.3 acceptance §2.5 同 scope.
type channelScopeAdapter struct {
	store *store.Store
}

func (a *channelScopeAdapter) ChannelIDsForOwner(ownerUserID string) ([]string, error) {
	return a.store.GetUserChannelIDs(ownerUserID), nil
}

// hubAgentTaskPusherAdapter wires *ws.Hub.PushAgentTaskStateChanged into
// the bpp.AgentTaskPusher interface (跟 hubLivenessAdapter / channelScopeAdapter
// 同模式) — RT-3 server派生 hook 通过此 adapter 跨 bpp ↛ ws 包边界 (bpp
// 不 import ws, internal/server 是唯一胶水点).
type hubAgentTaskPusherAdapter struct {
	hub *ws.Hub
}

func (a *hubAgentTaskPusherAdapter) PushAgentTaskStateChanged(
	agentID, channelID, state, subject, reason string, changedAt int64,
) (int64, bool) {
	return a.hub.PushAgentTaskStateChanged(agentID, channelID, state, subject, reason, changedAt)
}

// channelMemberFetcherAdapter wires *store.Store.ListChannelMembers into
// the bpp.ChannelMemberFetcher interface (WIRE-1 wire-3 — RT-3 push fanout
// 跨 bpp ↛ store 包边界). Returns user_ids slice for the given channel.
type channelMemberFetcherAdapter struct {
	store *store.Store
}

func (a *channelMemberFetcherAdapter) ListChannelMemberUserIDs(channelID string) ([]string, error) {
	members, err := a.store.ListChannelMembers(channelID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(members))
	for _, m := range members {
		out = append(out, m.UserID)
	}
	return out, nil
}

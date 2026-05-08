# Acceptance Template — DL-4: Web Push gateway + PWA install 三件套 (must-fix 收口)

> 蓝图: `client-shape.md` L22 (Mobile PWA + Web Push VAPID) + L37 ("没推送 = AI 团队像后台脚本不像同事") + L42 ("manifest + install prompt + Web Push + standalone") + L46 (实现路径锚)
> Implementation: `docs/implementation/modules/dl-4-spec.md` (战马E v0+v1, ≤200 行)
> 配套: AL-2b #481 (ack dispatcher seam 同源); RT-3 #488 (AgentTaskNotifier 同 frame); HB-1 #491 (plugin-manifest endpoint 命名拆死锚 zhanma-a drift audit)
> Owner: 战马E 实施 / 烈马 验收

## 验收清单

### §1 Schema (DL-4.1 — `web_push_subscriptions` 表 v=24)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 `web_push_subscriptions` 8 列 byte-identical: `{id PK, user_id, endpoint UNIQUE, p256dh_key, auth_key, user_agent, created_at, last_used_at NULL}` + idx_user_id (fan-out 热路径) | unit (PRAGMA columns + UNIQUE) | 烈马 | `internal/migrations/dl_4_1_web_push_subscriptions.go` v=24 + `dl_4_1_web_push_subscriptions_test.go::TestDL41_CreatesWebPushSubscriptionsTable` (7 NOT NULL + 1 nullable) + `TestDL41_EndpointUNIQUE` (重复 INSERT reject) + `TestDL41_HasUserIDIndex` |
| 1.2 反约束 — 12 forbidden 列名 0 hit (vapid_secret/vapid_private/api_key/token/session_token/device_id/device_kind/device_type/org_id/cursor/enabled/paused/muted) | unit (NoDomainBleed 列名扫) | 烈马 | `TestDL41_NoDomainBleed` 12 列名扫 |
| 1.3 idempotent migration + v=24 sequencing (顺延 ADM-2.2 v=23 #484) | unit (双 Run() 不报错 + Version=24) | 烈马 | `TestDL41_Idempotent` + `TestDL41_VersionIs24` |

### §2 Server REST (DL-4.2 — POST/DELETE `/api/v1/push/subscribe`)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 POST UPSERT (同 endpoint 重注册原地刷 p256dh/auth, ON CONFLICT(endpoint) DO UPDATE) | E2E (真 4901 fixture round-trip) | 战马E | `internal/api/push_subscriptions.go::handleSubscribe` + `dl_4_2_push_subscriptions_test.go::TestDL42_SubscribeRoundTrip` + `TestDL42_UpsertSameEndpoint` (重注册 row 数 ==1, p256dh refreshed) |
| 2.2 DELETE idempotent (重复退订仍 204) + 缺 endpoint 400 | unit + E2E | 战马E | `TestDL42_UnsubscribeIdempotent` + `TestDL42_UnsubscribeRequiresEndpoint` |
| 2.3 cross-user reject — POST 409 + DELETE 403 (REG-INV-002 fail-closed) | unit (双 token cross-test) | 战马E / 烈马 | `TestDL42_CrossUserReject` |
| 2.4 401 unauthorized + 400 invalid payload (4 field 字面缺一即 reject) | unit | 战马E | `TestDL42_UnauthorizedNoToken` + `TestDL42_InvalidPayload` (4 sub-case) |

### §3 Push gateway (DL-4.3 — VAPID + 410 GC)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 VAPID env-driven (BORGEE_VAPID_PUBLIC_KEY/_PRIVATE_KEY/_SUBJECT 缺即 error, dev 回退 noop) | unit (env mat) | 战马E | `internal/push/gateway.go::NewGateway` + `gateway_test.go::TestDL43_NewGateway_RequiresEnv` + `TestDL43_NewGateway_AllEnvSet` + `TestDL43_NoopGateway` (dev fallback) |
| 3.2 Send fan-out (user 全 subscription 派 + attempts count 返) — fire-and-forget 不 propagate error | unit | 战马E | `TestDL43_Send_ZeroSubscriptions` (0 attempts 不 panic) |
| 3.3 410 Gone → DELETE row (单源退订, 蓝图 L22 字面) | integration (httptest 假 410 endpoint) | 战马E | `TestDL43_Send_410GoneDeletesRow` (假 410 server + 单源 GC 真验证) |
| 3.4 反约束 — secret 不入 row/log/body (跟 spec §0 立场 ① 同源) | unit (TestDL41_NoDomainBleed) | 烈马 | `TestDL41_NoDomainBleed` 12 列名 + `TestDL44_PWAManifest_NoSecretsLeak` 8 substring |

### §4 PWA Web App Manifest (DL-4.4 — `/api/v1/pwa/manifest`)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 GET endpoint 公开 (无 authMw, install prompt 在 login 前 fetch) + Content-Type `application/manifest+json` (W3C 标准 MIME) | E2E + unit | 战马E | `internal/api/pwa_manifest.go` + `pwa_manifest_test.go::TestDL44_PWAManifest_PublicEndpoint` + `TestDL44_PWAManifest_ContentType` |
| 4.2 W3C 字段集 (name/short_name/start_url/display=standalone/theme_color/background_color/scope/icons[3]) + 192/512 W3C 基线 | unit (字段断言) | 战马E | `TestDL44_PWAManifest_RequiredFields` (display=standalone 蓝图 L22 + 192/512 基线) |
| 4.3 反约束 — 命名拆死锚 (DL-4 endpoint 字面**不**含 `plugin-manifest`, HB-1 #491 独占字面); manifest body 不漏 secret | unit (实测 server 不响应 HB-1 字面) | 战马E / 烈马 | `TestDL44_PWAManifest_NameNotPluginManifest` (实测 /api/v1/plugin-manifest 返非 2xx) + `TestDL44_PWAManifest_NoSecretsLeak` (8 substring scan) |

### §5 Client subscribe (DL-4.5 — service worker + PushManager)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 5.1 sw.js push handler — 收 push event 渲染 mention/agent_task 通知 (payload byte-identical 跟 internal/push/mention_notifier.go shape) | unit (sw.js text scan) | 战马E | `packages/client/public/sw.js` push event listener + showNotification + notificationclick handler; e2e text-scan lock |
| 5.2 pushSubscribe.ts helper — registerServiceWorker / subscribeToPush / unsubscribeFromPush / getCurrentSubscriptionState | vitest unit | 战马E | `packages/client/src/lib/pushSubscribe.ts` + `__tests__/pushSubscribe.test.ts` (6 test: isPushSupported / state / urlBase64ToUint8Array W3C 编码 4 sub-case) |
| 5.3 e2e — manifest fetch + sw.js 加载 + push handler text-scan + 命名拆死 e2e (HB-1 字面 server 不响应) | playwright | 战马E | `packages/e2e/tests/dl-4-pwa-subscribe.spec.ts` (3 case: PWA manifest W3C / 命名拆死 / sw.js push handler 真路径) |

### §6 Fan-out hook (DL-4.6 — mention → push, AgentTaskNotifier seam)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 6.1 MentionDispatcher 派单 → fire push (online + offline 都派, browser SW 处理 visibility dedup) | integration (recordingGateway) | 战马E | `internal/api/mention_dispatch.go::Dispatch` 加 PushNotifier 调用 + `internal/push/mention_notifier_test.go::TestDL46_NotifyMention_PayloadShape` (5 字段 byte-identical) |
| 6.2 AgentTaskNotifier seam 留 RT-3.2 派生 hook 接 (待 BPP-2.2 plugin 上行落地) | unit (compile gate) | 战马E | `mention_notifier.go::AgentTaskNotifier` + `TestDL46_NotifyAgentTask_PayloadShape` (busy + idle 双路径) — RT-3 真接 deferred |
| 6.3 Notifiers nil-safe (Gateway==nil → return nil; nil receiver Notify* → 0 attempts 不 panic) | unit | 战马E | `TestDL46_NewMentionNotifier_NilSafe` + `TestDL46_NewAgentTaskNotifier_NilSafe` + `TestDL46_Notifiers_NilNotifier_NoOp` |

### §7 反向 grep 7 锚 (DL-4 实施 PR 必跑)

| 锚 | count |
|---|---|
| `web_push_subscriptions\b` in `packages/server-go/internal/` | ≥1 hit (schema + handler) |
| `PushManager.subscribe\|navigator.serviceWorker` in `packages/client/src/` | ≥1 hit (client subscribe 入口) |
| `VAPID\|vapid` in `packages/server-go/internal/` | ≥1 hit (env 读 + library 调) |
| `push.*device_id\|push.*device_kind` in `packages/server-go/internal/` | 0 hit (AL-3.1 multi-session 立场承袭) |
| `web_push_subscriptions.*cursor\|push.*hub\.cursors\.NextCursor` in `packages/server-go/` | 0 hit (push 不下沉 sequence) |
| `web_push_subscriptions.*enabled\|paused\|muted` in `packages/server-go/` | 0 hit (单源退订) |
| `admin.*push\.Gateway\|admin.*PushSubscribe` in `packages/server-go/internal/api/admin*.go` | 0 hit (ADM-0 §1.3 红线) |
| `manifest/plugins\|plugin-manifest` in `pwa_manifest.go` + `packages/client/src/` | 0 hit (HB-1 #491 拆死锚) |

## §8 deferred (Phase 后续)

- DL-4.6.b RT-3 server-derive hook 真接 AgentTaskNotifier (待 BPP-2.2 plugin 上行 task_started/finished 落地, RT-3.2 follow-up commit 同点接)
- DL-4.7 closure 后 follow-up: HB-1 #491 plugin-manifest 真签 + CS-3 Mobile PWA standalone display 真 install demo 截屏

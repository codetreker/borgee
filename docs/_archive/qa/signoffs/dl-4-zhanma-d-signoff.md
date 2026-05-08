# DL-4 Web Push gateway + PWA install 三件套 — 战马D (代签 zhanma-e idle) signoff

> **状态**: ✅ **SIGNED** (战马D acceptance 代签, 2026-04-29, post-#490 b2d1d25)
> **范围**: DL-4 milestone — Web Push gateway + PWA install 三件套 (web_push_subscriptions schema v=24 + REST POST/DELETE /api/v1/push/subscribe + VAPID gateway + 410 GC + PWA manifest endpoint + sw.js + pushSubscribe.ts helper + MentionDispatcher fan-out hook), 跟蓝图 client-shape.md L22+L37+L42+L46 byte-identical 真实施 (W3C App Manifest + Web Push VAPID + standalone PWA + 单源退订)
> **关联**: DL-4 #490 整 milestone 一 PR (跟 BPP-2 #485 + AL-1 #492 + AL-2a #480 同模式); REG-DL4-001..006 6🟢 + REG-DL4-007 1⏸️ follow-up; 7 server unit + 6 vitest + 3 playwright e2e PASS
> **方法**: 跟 #403 G3.3 + #449 G3.1+G3.2+G3.4 + AL-1 烈马代签机制承袭 — 真单测实施证据 + 立场反查 + acceptance template 闭锁 + 跨 milestone byte-identical 链承袭 + 战马D 代签 (zhanma-e idle 太多, DL-4 是 Phase 1 user-perceivable infra 已 merged 主线, 用户感知层 manifest/sw.js 已落 client, 此 signoff 替烈马走代签流, 跟 cm-4 / adm-0 / adm-2 deferred 同模式)

---

## 1. 验收清单 (战马D acceptance 视角 5 项, 跟 acceptance dl-4.md §1-§7 byte-identical)

| # | 验收项 | 立场锚 | 结果 | 实施证据 (PR/SHA + 测试名 byte-identical) |
|---|--------|--------|------|------|
| ① | web_push_subscriptions schema v=24 (id PK / user_id / endpoint UNIQUE / p256dh_key / auth_key / user_agent / created_at / last_used_at NULL) + idx_user_id (fan-out 热路径) + 反向 12 forbidden 列 (vapid_secret / vapid_private / api_key / token / session_token / device_id / device_kind / device_type / org_id / cursor / enabled / paused / muted) — secret 不入 row 立场 ① + 单源退订立场 (跟 AL-3.1 multi-session 不挂 device_id 同精神) | acceptance §1.1-§1.3 + 蓝图 L22 Web Push VAPID + 立场 ① secret 不下沉 schema | ✅ pass | `internal/migrations/dl_4_1_web_push_subscriptions_test.go` 4 PASS (CreatesWebPushSubscriptionsTable 7 NOT NULL + 1 nullable + EndpointUNIQUE 重复 reject + HasUserIDIndex + NoDomainBleed 12 列名扫 + Idempotent + VersionIs24) — REG-DL4-001 |
| ② | REST POST/DELETE /api/v1/push/subscribe — UPSERT (ON CONFLICT(endpoint) DO UPDATE p256dh/auth refresh) + DELETE idempotent (重复退订 204) + 缺字段 400 push.endpoint_invalid + cross-user 409+403 push.cross_user_reject (REG-INV-002 fail-closed) + 401 unauth | acceptance §2.1-§2.4 + 立场 ② 单源退订 + 立场 ③ ACL fail-closed | ✅ pass | `internal/api/dl_4_2_push_subscriptions_test.go` 7 PASS (SubscribeRoundTrip + UpsertSameEndpoint row 数==1 p256dh refreshed + CrossUserReject 双 token + InvalidPayload 4 sub-case + UnsubscribeIdempotent + UnsubscribeRequiresEndpoint + UnauthorizedNoToken) — REG-DL4-002 |
| ③ | VAPID gateway env-driven (BORGEE_VAPID_PUBLIC_KEY / _PRIVATE_KEY / _SUBJECT 缺即 error, dev fallback noop 不阻 server 启动) + Send fan-out (user 全 subscription 派 + attempts count 返 + fire-and-forget 不 propagate) + 410 Gone → DELETE row 单源 GC (蓝图 L22 字面承袭) | acceptance §3.1-§3.4 + 蓝图 L22 单源退订 + 立场 ③ env-driven secret 不入 body | ✅ pass | `internal/push/gateway_test.go` 6 PASS (NoopGateway dev fallback + NewGateway_RequiresEnv + NewGateway_AllEnvSet + Send_ZeroSubscriptions 0 attempts 不 panic + Send_410GoneDeletesRow httptest 假 410 server + 单源 GC 真验证 + Gateway_InterfaceShape compile gate) — REG-DL4-003 |
| ④ | PWA Web App Manifest GET /api/v1/pwa/manifest 公开 endpoint (无 authMw, install prompt 在 login 前 fetch) + Content-Type application/manifest+json (W3C 标准 MIME) + W3C 字段集 (name / short_name / start_url / display=standalone 蓝图 L22 字面 / theme_color / background_color / scope / icons[3] 含 192/512 W3C 基线) + 反向命名拆死锚 (endpoint 不含 plugin-manifest, HB-1 #491 独占字面) + NoSecretsLeak 8 substring scan | acceptance §4.1-§4.3 + 蓝图 L22 manifest standalone + zhanma-a drift audit DL-4 vs HB-1 拆死锚 | ✅ pass | `internal/api/pwa_manifest_test.go` 5 PASS (PublicEndpoint 200 + ContentType W3C MIME + RequiredFields 字段集 + display + 192/512 + NoSecretsLeak 8 substring + NameNotPluginManifest 实测 /api/v1/plugin-manifest 返非 2xx) — REG-DL4-004 |
| ⑤ | client subscribe 三件套 — sw.js push event handler + showNotification + notificationclick 跳 SPA 路由; pushSubscribe.ts helper 4 export (registerServiceWorker / subscribeToPush / unsubscribeFromPush / getCurrentSubscriptionState) + urlBase64ToUint8Array W3C VAPID 编码; e2e 真路径 manifest fetch + sw.js text-scan + 命名拆死 e2e 实测 + MentionDispatcher fan-out hook (online + offline 都派 SW dedup, AgentTaskNotifier seam 留 RT-3.2 deferred, nil-safe Gateway/Notifier nil → 0 attempts 不 panic) | acceptance §5.1-§5.3 + §6.1-§6.3 + 蓝图 L37 "AI 团队像同事" + 立场 ④ SW 处理 visibility dedup | ✅ pass | `packages/client/src/__tests__/pushSubscribe.test.ts` 6 vitest PASS + `packages/e2e/tests/dl-4-pwa-subscribe.spec.ts` 3 case PASS + `internal/push/mention_notifier_test.go` 5 PASS (NotifyMention_PayloadShape 5 字段 byte-identical + NotifyAgentTask_PayloadShape busy/idle 双路径 + NewMentionNotifier_NilSafe + NewAgentTaskNotifier_NilSafe + Notifiers_NilNotifier_NoOp) — REG-DL4-005 + REG-DL4-006 |

**总体**: 5/5 通过 (覆盖 acceptance §1-§7 全锚 + 7 反向 grep 锚) → ✅ **SIGNED**, DL-4 Web Push + PWA milestone 通过.

---

## 2. 反向断言 (核心立场守门 byte-identical)

DL-4 三处反向断言全 PASS:

- **secret 不入 schema/row/log/body**: web_push_subscriptions 12 forbidden 列 (vapid_secret / vapid_private / api_key / token / session_token 等) 反向 grep 0 hit + PWA manifest body 8 substring scan (TestDL44_PWAManifest_NoSecretsLeak) 0 hit; VAPID env-driven 不入 row, 跟 ADM-0 §1.3 红线 secret 不下沉同精神
- **单源退订 + 不下沉 sequence**: 410 Gone → DELETE row 是退订**唯一路径** (反向 grep `web_push_subscriptions.*enabled|paused|muted` 0 hit); 反向 grep `web_push_subscriptions.*cursor|push.*hub.cursors.NextCursor` 0 hit (push 不下沉 RT-1.3 cursor sequence, 跟 DM-3 #508 cursor 复用 RT-1.3 同精神 — 不分裂 sequence)
- **命名拆死锚 DL-4 vs HB-1 #491**: pwa_manifest.go + client 反向 grep `manifest/plugins|plugin-manifest` 0 hit + 实测 /api/v1/plugin-manifest 返非 2xx (TestDL44_PWAManifest_NameNotPluginManifest) — zhanma-a drift audit 锚源, DL-4 PWA W3C App Manifest vs HB-1 plugin-manifest 双源各自独立; ADM-0 §1.3 红线承袭 (反向 grep `admin.*push.Gateway|admin.*PushSubscribe` 在 admin*.go 0 hit)

---

## 3. 跨 milestone byte-identical 链验 (DL-4 是 Phase 1 PWA infra 锚, 多 milestone 锚承袭)

DL-4 兑现/承袭多源 byte-identical:

- **AL-3 #305 multi-session 立场承袭**: web_push_subscriptions 不挂 device_id / device_kind / device_type (反向 12 forbidden 列, REG-DL4-001) — 跟 AL-3.1 multi-session 一 user 多 session 同精神 (subscription 多 endpoint 是天然 fan-out, 不需 device 维度)
- **RT-1.3 #290 cursor 不分裂**: push 不下沉 sequence (反向 grep `push.*cursor` 0 hit) — 跟 DM-3 #508 cursor 复用 RT-1.3 + AL-2b #481 + CV-* + BPP-3.1 #494 共一根 sequence 同精神 (push 是 transport 层 fire-and-forget, browser SW 自处理 visibility dedup, 不入 sequence 序列)
- **RT-3 #488 AgentTaskNotifier seam 同 frame**: DL-4.6 NotifyAgentTask payload shape 5 字段 byte-identical 跟 RT-3 thinking subject 5-pattern + busy/idle 双路径; AgentTaskNotifier seam 留 RT-3.2 真接 (待 BPP-2.2 plugin 上行 task_started/finished 落地, deferred follow-up)
- **HB-1 #491 plugin-manifest 拆死锚**: DL-4.4 PWA manifest endpoint 字面**不**含 plugin-manifest, HB-1 #491 独占字面 (zhanma-a drift audit) — 双源各自独立, 跟 CV-2 v2 #cv-2 vs CV-1 拆死同模式 (一字一锚)
- **ADM-0 §1.3 admin god-mode 红线承袭**: push.Gateway 不挂 admin 路径 (反向 grep `admin.*push.Gateway|admin.*PushSubscribe` 0 hit) — 跟 AL-3 #303 ⑦ + AL-4 #379 v2 + AL-2b #471 §2.4 + ADM-2 #484 + BPP-2 #485 + AL-1 #492 + AL-5 #516 同模式
- **REG-INV-002 fail-closed**: cross-user POST 409 + DELETE 403 — 跟 AP-1 #493 capability check fail-closed + AL-2a #480 owner-only ACL + BPP-3.2 #498 owner DM grant 同精神 (任何 cross-user 操作默认 reject)
- **forward-only 跨 milestone 同精神**: 410 Gone → DELETE row 是退订单源 — 但 row 写入是 INSERT/UPSERT 单门 (跟 AL-1 agent_state_log + ADM-2.1 admin_actions + ADM-2.2 impersonation_grants 立场 ⑤ forward-only 同精神, push subscriptions 是 transient device-side state 单源 GC reset)

---

## 4. 留账 (DL-4 闭闸不阻, 留 follow-up — 跟蓝图 L22+L37+L42+L46 边界 cross-ref)

- ⏸️ **REG-DL4-007 §7 反向 grep 8 锚 CI 阶段执行** — 当前 acceptance §7 反向 grep 锚已落实施代码内 + dl-4 PR 必跑, 但未注入 CI lint 阶段 (跟 BPP-1 envelope lint #304 同模式 follow-up); 当 PERF-AST-LINT #506 astscan helper 合入 main 后改调 helper inline reverse grep (跟 CHN-4 wrapper REG-CHN4-004b 同模式)
- ⏸️ **REG-DL4-006.b RT-3.2 server-derive hook 真接 AgentTaskNotifier** — 待 BPP-2.2 plugin 上行 task_started/finished 真落地 (REG-AL1-006 dispatcher → audit append wire 同期), RT-3.2 follow-up commit 同点接; 当前 seam ready, compile gate test 已守 (TestDL46_NotifyAgentTask_PayloadShape 占位 busy/idle 双路径 byte-identical)
- ⏸️ **CS-3 Mobile PWA standalone display 真 install demo 截屏** — 蓝图 L42 字面 manifest standalone display 已落 endpoint, 真 install prompt + standalone shell 真 install 截屏野马验收 deferred (跟 G3.4 野马 5 张截屏 + G4.4 双 agent 截屏野马签同模式 deferred ⏸️)
- ⏸️ **HB-1 #491 plugin-manifest 真签后 cross-name 真活演练** — DL-4 PWA manifest vs HB-1 plugin-manifest 拆死锚已 lock, HB-1 真接管 host-bridge install daemon 真签后跑一次 e2e 双源活演练 (跟 chn-4 7 源 byte-identical 锁同模式 follow-up)

跟蓝图 L22+L37+L42+L46 边界 cross-ref:
- 蓝图 L22 "Mobile PWA + Web Push VAPID + manifest standalone" — 全落 (REG-DL4-001..005 byte-identical)
- 蓝图 L37 "没推送 = AI 团队像后台脚本不像同事" — MentionDispatcher fan-out + AgentTaskNotifier seam 落 (REG-DL4-006)
- 蓝图 L42 "manifest + install prompt + Web Push + standalone" — 全落 (REG-DL4-004 W3C + REG-DL4-005 sw.js)
- 蓝图 L46 "实现路径锚" — 战马E v0+v1 spec 200 行 byte-identical 真实施

---

## 5. 解封路径 (Phase 5 主线 + DL-4 是 Phase 1 PWA infra 主线)

- ✅ **G4.1 ADM-1**: 野马 ✅ #459
- ✅ **G4.2 ADM-2**: 烈马 ✅ #484
- ✅ **G4.3 BPP-2**: 烈马 ✅ G4 batch
- ✅ **G4.4 CM-5**: 烈马 ✅ G4 batch
- ✅ **G4.5 AL-2a + AL-2b + AL-4 联签**: 烈马 ✅ G4 batch
- ✅ **AL-1 状态四态 wrapper**: 烈马 ✅ #492 (AL-1 wrapper signoff post)
- ✅ **DL-4 Web Push + PWA 三件套**: 战马D 代签 ✅ 本 signoff (5/5 验收 + REG-DL4-001..006 6🟢 + 7 server unit + 6 vitest + 3 playwright e2e PASS + 反向断言全过 + 跨 milestone 拆死锚 + nil-safe + 单源退订)
- ⏸️ **REG-DL4-007 + REG-DL4-006.b + CS-3 + HB-1 cross-name 真活演练** 4⏸️ follow-up — CI lint 阶段反向 grep 8 锚 + RT-3.2 hook 真接 + Mobile PWA 真 install 野马截屏 + HB-1 真签后 cross-name e2e (跟 G4.audit 同期收口, 飞马职责)
- ⏸️ **G5.audit** Phase 5 代码债 audit (软 gate 飞马职责) — 含 DL-4 4⏸️ follow-up + AL-5 client UI v2 + DM-3 e2e 多端真验
- ⏸️ **Phase 5 closure announcement** (Phase 5 主线 ~88% + DL-4 ✅ + AL-5 #516 待合 + G5.audit 后链入飞马职责)

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 战马D (代签 zhanma-e idle) | v0 — DL-4 Web Push gateway + PWA install 三件套 milestone ✅ SIGNED post-#490 b2d1d25 (战马E 整 milestone 一 PR). 5/5 验收通过 (web_push_subscriptions schema v=24 + 12 forbidden 列反向 / REST POST/DELETE UPSERT + idempotent + cross-user 409/403 fail-closed / VAPID gateway env-driven + 410 GC 单源退订 / PWA W3C App Manifest endpoint + standalone + 192/512 + 命名拆死锚 / sw.js + pushSubscribe.ts + e2e + MentionDispatcher fan-out + AgentTaskNotifier seam nil-safe). 跟 G4.2 ADM-2 + AL-1 烈马代签 + G4.1-G4.5 烈马代签机制同模式 (DL-4 Phase 1 PWA infra 已 merged 主线, 用户感知层 manifest+sw.js 已落 client, 此 signoff 战马D 代签替 zhanma-e 走代签流). REG-DL4-001..006 6🟢 + 7 server unit + 6 vitest + 3 playwright PASS + 反向断言三处 (secret 不入 schema/row/log/body + 单源退订不下沉 sequence + 命名拆死锚 DL-4 vs HB-1 + ADM-0 §1.3 admin 红线) 全过. 跨 milestone byte-identical 链全锚 (AL-3 multi-session 不挂 device + RT-1.3 cursor 不分裂 + RT-3 AgentTaskNotifier seam + HB-1 拆死锚 + ADM-0 红线 + REG-INV-002 fail-closed + forward-only 同精神). 留账 4 项 ⏸️ deferred (REG-DL4-007 CI lint 反向 grep 8 锚 + REG-DL4-006.b RT-3.2 hook 真接 + CS-3 Mobile PWA 真 install 野马截屏 + HB-1 真签后 cross-name e2e), 跟蓝图 L22+L37+L42+L46 边界 cross-ref. 不动 §5 totals (留烈马/飞马 closure flip 时同期翻牌). |

# DL-4 spec brief — Web Push gateway + PWA installable manifest API (must-fix 收口)

> 战马E · Phase 4 · ≤200 行 spec · 蓝图 [`client-shape.md`](../../blueprint/current/client-shape.md) L22 ("Mobile PWA + Web Push (VAPID)") + L37 ("没推送 = AI 团队像后台脚本不像同事") + L46 (实现路径: manifest.json + push subscription endpoint + VAPID key + server-go push 通道接 [data-layer §3.4 global_events fan-out](../../blueprint/current/data-layer.md)).
>
> ⚠️ **拆死锚 — 跟 HB-1 #491 plugin-manifest 命名碰撞防御** (zhanma-a 抓出 drift): 本 spec 的 manifest endpoint 是 **PWA installable web app manifest** (浏览器 install prompt 用, HTTPS + bearer 无签). HB-1 的 `GET /api/v1/plugin-manifest` 是 **install-butler 消费的 binary plugin manifest** (双签必需, 蓝图 host-bridge §1.2 ① + §4.5 "未签 100% reject"). 两个 endpoint 两套安全模型, 本 PR endpoint 命名走 `/api/v1/pwa/manifest` (含 `pwa` 字面避混淆), 不用 `manifest/plugins` 字面.

## 0. 关键约束 (3 条立场, 蓝图字面)

1. **Web Push 走 VAPID 单源, server 持私钥, client 持 endpoint+p256dh+auth** (蓝图 L22 + L46 字面 "VAPID key 生成 + server-go 一个 push 通道"): subscription 表 `web_push_subscriptions` 仅存 client 上行的公开字段 (endpoint UNIQUE / p256dh_key / auth_key); VAPID 私钥在 server env (`BORGEE_VAPID_PRIVATE_KEY`), 不入表. **反约束**: 不挂 secret 列 (vapid_secret / vapid_private / token / api_key); 不挂 device 路由键 (device_id / device_kind, 跟 AL-3.1 multi-session last-wins 立场承袭, UA 是 audit hint 不是路由键).

2. **Push 是 fire-and-forget, 不走 hub.cursors sequence** (跟 RT-1/CV-2/DM-2/CV-4/AL-2b/RT-3 6 frame 共序拆死): server 收 mention/agent_task_state_changed 派生 → 查 `web_push_subscriptions WHERE user_id=?` → web-push 库加密 + 发 endpoint (best-effort, 410 Gone → 表删除). **反约束**: 不挂 `cursor` 列; 不入 hub.cursors NextCursor 路径; client 端按 ws frame cursor 不按 push timestamp 排序 (跟 RT-1 立场承袭).

3. **subscription 存在=订阅 / DELETE=退订, 不开 enable/paused 双源** (蓝图 L22 字面 "退订" 单源): POST `/api/v1/push/subscribe` UPSERT (同 endpoint 重注册 revive p256dh/auth); DELETE `/api/v1/push/subscribe?endpoint=...` 行删除. **反约束**: 不挂 `enabled` / `paused` / `muted` 列 (避双源); 不开 PATCH 路径 (退订就是删, 不挂 soft-delete).

## 1. 拆段实施 (单 PR 全闭, 跟 BPP-2 / AL-2b 一 milestone 一 PR 协议)

| 段 | 文件 | 范围 |
|---|---|---|
| DL-4.1 schema | `internal/migrations/dl_4_1_web_push_subscriptions.go` (新, v=24) | 8 列 (id PK / user_id NOT NULL / endpoint UNIQUE / p256dh_key / auth_key / user_agent / created_at / last_used_at NULL) + idx_user_id + 6 test (Creates / EndpointUNIQUE / NoDomainBleed / HasIndex / Idempotent / VersionIs24) |
| DL-4.2 server endpoints | `internal/api/push_subscriptions.go` (新) | POST `/api/v1/push/subscribe` UPSERT body{endpoint, p256dh, auth} + DELETE `/api/v1/push/subscribe?endpoint=...` 行删除 + auth 走 user cookie + owner-only ACL (跟 layout.go 同模式) |
| DL-4.3 push gateway | `internal/push/gateway.go` (新) + `internal/api/server.go` wire (改) | `Gateway.Send(userID, payload)` 查 user 全 subscription → web-push 库加密 → POST endpoint; 410 Gone → 表删除 last_used_at stamp; failure best-effort log warn 不阻 caller |
| DL-4.4 manifest API | `internal/api/pwa_manifest.go` (新) + `packages/client/public/manifest.webmanifest` (新) | GET `/api/v1/pwa/manifest` (PWA installable web app manifest — name/short_name/start_url/display/icons; 浏览器 install prompt 消费, **跟 HB-1 #491 plugin-manifest 拆死命名**); manifest.webmanifest 静态文件 (PWA install) |
| DL-4.5 client subscribe | `packages/client/src/push/subscribe.ts` (新) + main.tsx wire | navigator.serviceWorker + PushManager.subscribe(VAPID public key) → POST /api/v1/push/subscribe + 设置页 toggle (订阅/退订) |
| DL-4.6 fan-out hook | `internal/api/mention_dispatch.go` (改) + `internal/ws/agent_task_state_changed_frame.go` (改) | 每个 push frame 派发后 server-side check: recipient online via ws → ws push (现有路径); offline → push.Gateway.Send |
| DL-4.7 e2e + closure | `packages/e2e/tests/dl-4-push.spec.ts` (新) + REG-DL4-001..010 + acceptance + PROGRESS [x] | 5 cases: subscribe/unsubscribe round-trip + push payload encryption / fan-out online vs offline 路由 / 410 Gone 表 GC / cross-user reject (REG-INV-002) / DELETE single source |

## 2. 错误码 byte-identical (跟 BPP-2.2 / AL-2b 命名同模式)

- `push.endpoint_invalid` — 缺/坏 endpoint URL reject
- `push.subscription_not_found` — DELETE 时 endpoint 不在表
- `push.cross_user_reject` — DELETE/POST 跨 user reject (REG-INV-002 fail-closed)
- `push.vapid_misconfigured` — server env VAPID 私钥 缺 → POST 500 fail-loud (跟 admin bootstrap 同模式)

## 3. 反查 grep 锚 (Phase 4 验收 + DL-4 实施 PR 必跑)

```
git grep -nE 'web_push_subscriptions\b' packages/server-go/internal/   # ≥ 1 hit (schema + handler 字面)
git grep -nE 'PushManager.subscribe|navigator.serviceWorker' packages/client/src/   # ≥ 1 hit (client 订阅入口)
git grep -nE 'VAPID|vapid' packages/server-go/internal/                # ≥ 1 hit (env 读 + library 调用)
# 反约束 (6 条 0 hit)
git grep -nE 'push.*device_id|push.*device_kind' packages/server-go/internal/   # 0 hit (跟 al_3_1 multi-session 同源)
git grep -nE 'web_push_subscriptions.*cursor|push.*hub\.cursors\.NextCursor' packages/server-go/   # 0 hit (push 不下沉 sequence)
git grep -nE 'web_push_subscriptions.*enabled|web_push_subscriptions.*paused|web_push_subscriptions.*muted' packages/server-go/   # 0 hit (单源)
git grep -nE 'admin.*push\.Gateway|admin.*PushSubscribe' packages/server-go/internal/api/admin*.go   # 0 hit (ADM-0 §1.3 红线)
git grep -nE 'push.*api_key|push.*secret|push.*token' packages/server-go/internal/migrations/dl_4_1_*   # 0 hit (secret 在 env 不入表)
# DL-4 vs HB-1 endpoint 拆死锚 (zhanma-a drift audit) — DL-4 endpoint 字面不能含 'plugin-manifest'
git grep -nE 'manifest/plugins|plugin-manifest' packages/server-go/internal/api/pwa_manifest.go packages/client/src/   # 0 hit in DL-4 范围 (HB-1 独占该字面)
```

## 4. 不在本轮范围 (反约束 deferred)

- ❌ 浏览器原生通知 (`Notification.requestPermission`) — Web Push 用 service worker push event 渲染, 不直接调 Notification API (蓝图 L46 路径)
- ❌ 跨设备 device fingerprinting (蓝图 §1.4 隐私 + AL-3.1 立场承袭)
- ❌ payload encryption beyond web-push library (库默认 AES-GCM, 不另起加密层)
- ❌ admin god-mode 主动给特定用户 push (ADM-0 §1.3 红线 — admin 不入业务 fan-out 路径)

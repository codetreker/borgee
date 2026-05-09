# Web Push (DL-4) — implementation note

> DL-4 (#490) · Phase 4 · 蓝图 [`client-shape.md`](../../blueprint/current/client-shape.md) L22 (Mobile PWA + Web Push VAPID) + L37 ("没推送 = AI 团队像后台脚本不像同事") + L46 (实现路径).

## 1. 设计

`web_push_subscriptions` 表 (v=24) + REST POST/DELETE + push gateway (VAPID, SherClockHolmes/webpush-go v1.4.0) + mention/agent-task 派生 fan-out hook. Push 是 fire-and-forget, 不走 hub.cursors sequence (跟 RT-1/CV-2/DM-2/CV-4/AL-2b/RT-3 6 frame 共序拆死). 退订单源 = DELETE row (蓝图 L22 字面).

## 2. Schema (v=24, `internal/migrations/dl_4_1_web_push_subscriptions.go`)

| 列 | 备注 |
|---|---|
| `id` PK | UUID |
| `user_id` NOT NULL | 归属用户 (FK users.id 逻辑) |
| `endpoint` NOT NULL UNIQUE | browser-issued push endpoint URL — UNIQUE 防同设备多 row, ON CONFLICT DO UPDATE 重注册 revive p256dh/auth |
| `p256dh_key` NOT NULL | subscription public key (base64 url-safe), web-push 库加密必填 |
| `auth_key` NOT NULL | subscription auth secret (base64 url-safe) |
| `user_agent` NOT NULL DEFAULT '' | UA hint for admin diag, audit only — **不**是路由键 (跟 AL-3.1 multi-session 一致) |
| `created_at` NOT NULL | Unix ms |
| `last_used_at` NULL | NULL until first push; bump on success or 410 reap |

**反约束** (TestDL41_NoDomainBleed 12 列名 0 hit): vapid_secret / vapid_private / api_key / token / session_token / device_id / device_kind / device_type / org_id / cursor / enabled / paused / muted.

## 3. REST endpoints (`internal/api/push_subscriptions.go`)

| Method | Path | 行为 |
|---|---|---|
| `POST` | `/api/v1/push/subscribe` | UPSERT body `{endpoint, p256dh, auth, user_agent?}`; cross-user 409 `push.cross_user_reject`; 缺字段 400 `push.endpoint_invalid` |
| `DELETE` | `/api/v1/push/subscribe?endpoint=...` | idempotent (重复 DELETE 仍 204); cross-user 403; 缺 endpoint 400 |

Auth 走 `borgee_token` user cookie + REG-INV-002 fail-closed 跨 user 拦.

## 4. Push gateway (`internal/push/gateway.go`)

```go
type Gateway interface {
    Send(ctx, userID, payload) int  // 返 attempts count, observability only
}
```

- `NewGateway(store, logger)` 读 env `BORGEE_VAPID_PUBLIC_KEY` / `_PRIVATE_KEY` / `_SUBJECT` (mailto: 或 https URL); 缺即 error
- `NewNoopGateway(logger)` dev/test fallback (server 启动失败回退, 跟 admin Bootstrap 区分: push 不阻 server 启动)
- 410 Gone / 404 → `DELETE FROM web_push_subscriptions WHERE id=?` (单源退订, 蓝图 L22)
- 其他 4xx/5xx + transport error → log warn 不 propagate (best-effort, 跟 DM-2.2 #372 同模式)

## 5. Fan-out hook (`internal/push/mention_notifier.go`)

| Notifier | 触发 | Payload byte-identical |
|---|---|---|
| `MentionNotifier.NotifyMention` | DM-2.2 mention dispatch (`internal/api/mention_dispatch.go::Dispatch`) | `{kind:"mention", from, channel, body, ts}` |
| `AgentTaskNotifier.NotifyAgentTask` | RT-3 派生 hook (待 BPP-2.2 plugin 上行落地, RT-3.2 后续) | `{kind:"agent_task", agent_id, state, subject, reason, ts}` |

两 notifier 都 nil-safe (Gateway==nil → return nil; nil receiver Notify* → return 0). MentionDispatcher.PushNotifier 字段 nil-safe (legacy 调用方可不传).

## 6. ⚠️ 命名拆死 — DL-4 vs HB-1 #491

| | endpoint | 用途 | 安全模型 |
|---|---|---|---|
| HB-1 #491 | `GET /api/v1/plugin-manifest` | install-butler 消费 binary plugin manifest | **双签必需** (蓝图 host-bridge §1.2 ① + §4.5 "未签 100% reject") |
| DL-4 (本) | `GET /api/v1/pwa/manifest` | PWA installable web app manifest (浏览器 install prompt) | 公开 endpoint (HTTPS, 无 auth — install prompt 在 login 前 fetch) |

**反约束**: DL-4 endpoint 字面**不**含 `plugin-manifest` (HB-1 独占). grep 检查 `manifest/plugins|plugin-manifest` 在 `internal/api/pwa_manifest.go` + `packages/client/src/` count==0 (zhanma-a drift audit 来源). `TestDL44_PWAManifest_NameNotPluginManifest` 实测断言 DL-4 server 不响应 HB-1 路径 (404).

## 6a. PWA Web App Manifest (`internal/api/pwa_manifest.go`)

W3C App Manifest 标准 endpoint, 浏览器 install prompt 触发器.

| 字段 | 值 | 锚 |
|---|---|---|
| `name` / `short_name` | "Borgee" / "Borgee" | install prompt + 主屏 label |
| `start_url` | "/" | 桌面图标点击进 SPA 根 |
| `display` | "standalone" | 跟蓝图 L22 字面一致 |
| `theme_color` / `background_color` | "#16213e" / "#1a1a2e" | byte-identical 跟 packages/client/public/manifest.json 静态文件 |
| `scope` | "/" | navigation scope (全应用) |
| `icons` | 3 项 SVG (192x192 + 512x512 + favicon any maskable) | 引用 packages/client/public/icons/ + favicon.svg 现有资源 |

`Content-Type: application/manifest+json` (W3C 标准 MIME, 浏览器 install prompt 严格识别). `Cache-Control: public, max-age=3600` (静态内容).

**反约束** (TestDL44_PWAManifest_NoSecretsLeak 守门): manifest body 不含 `vapid_secret` / `vapid_private` / `private_key` / `api_key` / `secret` / `token` / `borgee_token` / `borgee_admin_session` 字面.

**5 test 全绿**: PublicEndpoint (无 auth) / ContentType (W3C MIME) / RequiredFields (W3C 字段集 + display=standalone + 192x192/512x512 基线) / NoSecretsLeak / NameNotPluginManifest (拆死实测).

## 7. 相关参考

- spec brief: [`docs/implementation/modules/dl-4-spec.md`](../../implementation/modules/dl-4-spec.md)
- 实施: `internal/migrations/dl_4_1_*` (6 schema test) + `internal/api/push_subscriptions.go` (7 endpoint test) + `internal/api/dl_4_2_push_subscriptions_test.go` + `internal/api/pwa_manifest.go` (5 PWA manifest test) + `internal/push/gateway.go` (6 gateway test 含 410 GC) + `internal/push/mention_notifier.go` (5 fan-out test)
- 留尾后续: DL-4.4 PWA manifest API + DL-4.5 client subscribe + DL-4.6.b RT-3 派生 hook + DL-4.7 e2e + closure

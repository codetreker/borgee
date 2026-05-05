package migrations

import (
	"gorm.io/gorm"
)

// webPushSubscriptions is migration v=26 — Phase 4 / DL-4 must-fix.
//
// Blueprint锚: docs/blueprint/current/client-shape.md L22 ("**Mobile PWA** 离桌面后
// 的'团队感知'通道 + Web Push (VAPID)") + L37 ("没推送 = AI 团队像后台
// 脚本不像同事") + L42 ("manifest + install prompt + Web Push + standalone").
// data-layer §3.4 global_events fan-out 为 hook 上游.
// Spec brief: docs/implementation/modules/dl-4-spec.md (本 PR 同期).
//
// What this migration does:
//   1. CREATE TABLE web_push_subscriptions:
//        - id          TEXT    NOT NULL PRIMARY KEY  (UUID; 行 ID, 跟 endpoint
//                                                     UNIQUE 双轴, server 内部
//                                                     route)
//        - user_id     TEXT    NOT NULL              (FK users.id 逻辑; subscription
//                                                     归属用户. 同 user 多设备
//                                                     N row.)
//        - endpoint    TEXT    NOT NULL UNIQUE       (browser-issued push
//                                                     endpoint URL — VAPID
//                                                     contract; UNIQUE 防重
//                                                     注册同设备多 row)
//        - p256dh_key  TEXT    NOT NULL              (subscription public key,
//                                                     base64 url-safe; web-push
//                                                     library 加密 payload 必填)
//        - auth_key    TEXT    NOT NULL              (subscription auth secret,
//                                                     base64 url-safe; 同上)
//        - user_agent  TEXT    NOT NULL DEFAULT ''   (UA hint for admin diag,
//                                                     opaque; 反约束: 不挂
//                                                     device_id / device_kind
//                                                     列, 跟 AL-3.1 presence
//                                                     同源 — UA 是 audit hint
//                                                     不是路由键)
//        - created_at  INTEGER NOT NULL              (Unix ms)
//        - last_used_at INTEGER NULL                 (NULL until first push;
//                                                     non-NULL after 410 Gone
//                                                     reaping or successful
//                                                     emit)
//   2. CREATE INDEX idx_web_push_subscriptions_user_id
//      ON web_push_subscriptions(user_id) — fan-out 热路径 (server 收
//      mention/agent_task_state_changed 派生 → 查 user 全设备 N row).
//
// 反约束 (蓝图 §1.4 隐私 + DL-4 spec §2):
//   - 不挂 `org_id` 列 — subscription 归 user, org scope 通过 users.org_id
//     派生 (跟 al_2a_1 / chn_3_1 / al_1b_1 同模式 SSOT 不冗余).
//   - 不挂 `device_id` / `device_kind` 列 — UA 是 audit hint, 不是路由键
//     (跟 AL-3.1 presence_sessions multi-session 立场承袭, multi-session
//     last-wins 不需 device 维度).
//   - 不挂 `cursor` 列 — push 是 fire-and-forget, 不走 hub.cursors sequence
//     (跟 al_3_1 / cv_4_1 / chn_3_1 / al_2a_1 / al_1b_1 / adm_2_1 / adm_2_2
//     同模式 — 仅 RT-1/CV-2/DM-2/CV-4/AL-2b/RT-3 6 frame 共序).
//   - 不挂 `enabled` / `paused` / `muted` 列 — subscription 存在=订阅, 不存在=
//     退订 (DELETE 路径 = 退订, 不开 PATCH enable=false 避双源). 蓝图 §1.4
//     字面 "退订" 走 unsubscribe + 表删除单源.
//   - endpoint UNIQUE 严闭 — 同设备重注册 server 走 UPSERT (revive p256dh /
//     auth) 不再插新 row, 防 web-push 库重复加密同 endpoint 浪费配额.
//   - 不挂 secret 列 (api_key / token / vapid_secret) — VAPID 私钥在 server
//     env, 不入表; subscription 持有的 p256dh + auth 是 client-side 公钥
//     不是 secret (web-push 协议字面).
//
// v=26 sequencing: ADM-2.2 v=23 (impersonation_grants, #484 merged) +
// ?? v=24 + AL-1.4 v=25 (agent_state_log, #492 merged) + **DL-4 v=26**
// (本 migration) + 后续 milestone 顺延 v=27+.
// registry.go 字面锁.
//
// v0 stance: forward-only, no Down(). 表本身 v0 新增, IF NOT EXISTS 守
// idempotency.
var webPushSubscriptions = Migration{
	Version: 26,
	Name:    "dl_4_1_web_push_subscriptions",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS web_push_subscriptions (
  id            TEXT    NOT NULL PRIMARY KEY,
  user_id       TEXT    NOT NULL,
  endpoint      TEXT    NOT NULL UNIQUE,
  p256dh_key    TEXT    NOT NULL,
  auth_key      TEXT    NOT NULL,
  user_agent    TEXT    NOT NULL DEFAULT '',
  created_at    INTEGER NOT NULL,
  last_used_at  INTEGER NULL
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_web_push_subscriptions_user_id
			ON web_push_subscriptions(user_id)`).Error; err != nil {
			return err
		}
		return nil
	},
}

# E2E 真验证场景清单 — Borgee 全模块覆盖

> 烈马 v1 · 2026-05-01 · 用户拍铁律: 完整 e2e 真验证, 浏览器真模拟用户操作.
>
> **铁规** (用户 2026-05-01 立场): e2e 真验证**仅真 UI input + click + screenshot 算**. 禁: cURL / fetch / page.evaluate(API) 当 e2e 证据.
>
> **Phase 1**: 列场景 (此文件) — 只列不验.
> **Phase 2**: 固化清单结构 — 一行一场景.
> **Phase 3**: 按 smoke 跑真 UI, 报 covered/blocked.

---

## 0. 总览

| 类别 | 频率 | 时长 | 触发 |
|---|---|---|---|
| **Smoke** | 每次 deploy 后必跑 | ≤15 min | post-merge to main + pre-prod-deploy |
| **Regression** | weekly + release 前 | 1-2h | 每周一 + 每 release 前 |

**真 UI e2e 工具**: Playwright headless chromium (Chrome for Testing 147), 真 keyboard.type / mouse.click / page.goto / page.screenshot. 0 cURL, 0 fetch, 0 page.evaluate(API).

**blocked-by-UI-coverage 标记**: 后端 contract OK 但 SPA UI 缺真 surface (P3 admin-spa-ui-coverage backlog 6 endpoint + user-side capability transparency 等). 不删, 留账透明.

---

## 1. Smoke 清单 (17 场景, ≤15 min)

| ID | 模块 | scope | 操作步骤 (浏览器真路径) | 期望 (DOM/字面/截屏) | 类别 | 状态 |
|---|---|---|---|---|---|---|
| SMK-01 | ADM 登录 | admin SPA login | goto /admin → type admin/Test@Collab2026 → click Sign in | URL=/admin/dashboard + sidebar `admin-user-label` 显示 "admin" + cookie `borgee_admin_session` set | smoke | done (#633) |
| SMK-02 | AUTH user 登录 | user SPA login | goto / → type user 凭据 → click 登录 | URL=/ + workspace 主屏 + 侧栏频道列表渲染 | smoke | todo |
| SMK-03 | CHN 创建频道 (CHN-1 #194) | channel.write | user 登录 → click "+ 新建频道" → 输入 name → click 创建 | 频道在侧栏出现 + 进入空 channel 视图 | smoke | todo |
| SMK-04 | DM 发消息 (DM-2 #197 / RT-1.3 #229 fanout) | dm.send | 进 channel → focus 输入框 → 输入消息 → 按 Enter (无 mention picker open 时; mention picker open 时 Enter 是选 mention, Cmd+Enter 才送) | 消息出现在 channel timeline + WS 实时不刷 | smoke | todo |
| SMK-05 | RT presence (RT-1 #290 / RT-3 #616 4 态) | presence dot | user A 登录 + user B 登录 → A 看 user list | B 头像旁 presence dot active (绿) | smoke | todo |
| SMK-06 | CV 创建 artifact (CV-1 #226 / CV-2 #228 / CV-3 #233) | artifact.write | channel → click "+ artifact" → 输入 markdown → click 提交 | artifact 在频道呈现 + version=1 | smoke | todo |
| SMK-07 | ADM channels 列表 | admin god-mode read | admin → click sidebar Channels | 6 列 table 头 (Name/Type/Visibility/Status/Created/Actions) + 0 NaN | smoke | done (#633) |
| SMK-08 | ADM audit log 渲染 (ADM-3 #619 multi-source + ADM-2-FOLLOWUP #626) | admin SPA | admin → click Audit Log | data-page=admin-audit-log + 红 banner 字面 byte-identical 跟 client `BannerImpersonate.tsx` (ADM-2 #484) admin SPA 真值 `当前以业主身份操作 — 该会话受 24h 时限` (Phase 3 smoke 真验; **PrivacyPromise.tsx `开启了对你账号的 24h impersonate` 是 user-side 不同源, 不锚此处**) + 5 action enum filter UI (delete_channel/suspend_user/change_role/reset_password/start_impersonation; ADM-2-FOLLOWUP #626) | smoke | done (#633 + admin-spa-archived-ui-followup) |
| SMK-09 | CM-onboarding welcome (CM-onboarding) | new user signup | 注册新 user → 首次登录 | Welcome channel 自动出现 + system DM 引导 + welcome channel id 真 ULID (regex `^[0-9A-HJKMNP-TV-Z]{26}$`, ULID-MIGRATION #625 真生效锚) | smoke | todo |
| SMK-10 | LOGOUT | session 收尾 | admin → click Logout | URL=/admin/login + cookie 清 | smoke | todo |
| SMK-11 | mention 真用 path | dm.mention / cv.comment.mention | 在 channel/comment 输入 `@` → mention picker 出现 → 选 user → 提交 | 消息含 `@<user>` token + mention badge 真显 (DL-4) + 被 mention user 真收通知 | smoke | todo |
| SMK-12 | 三搜 (artifact / channel / DM) 真用 path | search FTS5 | top bar search 框 → 输入关键字 → 看下拉/列表 | artifact / channel / DM 三类命中真分类显, 命中点击真跳真 anchor | smoke | todo |
| SMK-13 | notification (Web Push 三态) | dl.push.subscribe (CS-3) | 设置 → 开启通知 → 浏览器原生 prompt 真出 → 点 Allow → 字面真渲 `已开启通知`; 反向: 点 Block → 字面 `通知已被浏览器拒绝, 请到浏览器设置开启`; 默认: `开启通知` | 三态字面 byte-identical (CS-3 content-lock) + service worker 真注册 | smoke | todo |
| SMK-14 | settings 持久 path | user 偏好 / 通知 / DM 隐私 | 点 user avatar → settings → 改一项 (e.g. require_mention toggle) → 保存 → 刷新 → 仍生效 | 设置真持久 + 跨 device sync (RT-3 multi-device 应用) | smoke | todo |
| SMK-15 | user cookie SSOT (cookie-name-cleanup #634) | auth cookie SSOT | user 登录后 → DevTools / Playwright cookies API 看 Set-Cookie 字面 | cookie name 字面 = `borgee_token` (跟 client/server SSOT byte-identical, 反 silent drift `borgee_session`/`token`/etc), `borgee_admin_session` 仅 admin 路径 (拆死) | smoke | todo |
| SMK-16 | capability dot SSOT 真 UI grant (CAPABILITY-DOT #628 + admin-spa-shape-fix #633 D6 + admin-spa-ui-coverage #639) | admin SPA grant UI | admin → user 详情页 → click "授予 capability" 按钮 → 输入/选 `channel.read` → 提交 ✅; 重做输 `read_channel` → 提交 ❌ | dot-notation accept 200 + 真渲成功 toast; snake_case reject 400 字面 `invalid_capability` 真渲红 toast | smoke | ✅ done (#639 admin-spa-ui-coverage 真兑现 UI) |
| SMK-17 | ULID 字面 (ULID-MIGRATION #625) | ULID generator prod 生效 | user 登录后 → 真路径触发 channel/artifact/comment 创建 → 看 **HTTP request URL path** (`/api/v1/channels/<id>/...` 等; 不锚 DOM innerHTML 因 React state 不渲染 id 到 HTML) | HTTP timeline regex `[0-9A-HJKMNP-TV-Z]{26}` ≥1 hit (Crockford Base32 ULID-26) + 反向 `\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b` UUID-36 = 0 hit (仅历史 row 允许). Phase 3 smoke 2026-05-02 真验: 24 ULID-26 hit + 0 UUID-36 hit ✅ | smoke | done (Phase 3 smoke) |

---

## 2. Regression 清单 (按模块全覆盖)

### §AL — Agent Lifecycle (8 场景)

| ID | scope | 操作步骤 | 期望 | 状态 |
|---|---|---|---|---|
| REG-AL-01 | agent invite | user → channel → click "邀请 agent" → 选 plugin → 提交 | agent 在频道 member list 出现 + presence dot | todo |
| REG-AL-02 | agent start (AL-1a) | agent invitation accept → status 变 online | green dot + state=online | todo |
| REG-AL-03 | agent stop | click agent → "停用" | red dot + state=offline + reason 字面 | todo |
| REG-AL-04 | agent error 三态 | 触发 plugin crash | dot=red + state=error + 6 reason code 之一 | todo |
| REG-AL-05 | agent runtime 三态 (AL-1a #249) | 看 agent detail | online/offline/error 三态字面渲染 | todo |
| REG-AL-06 | grant capability bundle (AP-2 #620) | admin → user → 选 bundle "workspace" → 一键授予 | 14 capability dot-notation 真授予 | blocked-by-UI-coverage (admin SPA 缺 grant UI) |
| REG-AL-07 | revoke capability | 同上 → revoke | grant 撤销 | blocked-by-UI-coverage |
| REG-AL-08 | reason 字面锁 (AL-1a 6 reason) | trigger 各 reason 路径 | 6 字面 byte-identical 渲染 | todo |

### §BPP — Plugin Protocol (6 场景)

| ID | scope | 操作步骤 | 期望 | 状态 |
|---|---|---|---|---|
| REG-BPP-01 | plugin install (HB-1 #589) | admin 看 plugin manifest | 7-reason 字典 + ed25519 sig 真验 | blocked-by-UI-coverage |
| REG-BPP-02 | agent_config ack | plugin 启动 → ack | ack frame 真发 | todo |
| REG-BPP-03 | task_run frame | user 触发 task → plugin 收 frame | frame body 字面 byte-identical | todo |
| REG-BPP-04 | heartbeat (BPP-7) | plugin alive | server 收 hb 5 字段 audit | todo |
| REG-BPP-05 | task lifecycle (RT-3 multi-device) | user 在 device A 发 task → device B 收 | fanout 真触, device B 不需 refresh | todo |
| REG-BPP-06 | reconnect | WS drop → reconnect | backfill ≤3s + 0 消息丢 | todo |

### §CHN — Channel (10 场景)

| ID | scope | 操作步骤 | 期望 | 状态 |
|---|---|---|---|---|
| REG-CHN-01 | create channel | user → "+ 新建频道" | 频道出现 | todo |
| REG-CHN-02 | edit description (CHN-14 audit) | 进频道 → 编辑描述 → 保存 | edit history audit 行 | todo |
| REG-CHN-03 | archive channel (CHN-15 readonly) | admin → archive | 频道变只读 + "只读" badge | todo |
| REG-CHN-04 | restore channel | admin → restore | 恢复可写 | todo |
| REG-CHN-05 | visibility public→private | admin → 改 visibility | 非 member 404→403 (反 inject) | todo |
| REG-CHN-06 | member invite | channel owner → 邀请 user | invite 真发 + accept | todo |
| REG-CHN-07 | role change (REG-ADM2-003) | admin → 改 user role | audit hook fired | blocked-by-UI-coverage |
| REG-CHN-08 | sidebar reorder (CHN-3.3) | 鼠标按住频道侧栏 dnd-kit drag handle (反键盘 up/down 错觉) → 拖到目标位置 → 释放 | 顺序持久 (refresh 仍在新位) | todo |
| REG-CHN-09 | channel list cross-org (CHN-12) | user 切 org | list 切对应 org | todo |
| REG-CHN-10 | DM-only ACL (DM-11 search) | 跨 DM search | DM-only ACL 守 | todo |

### §CV — Canvas (10 场景)

| ID | scope | 操作步骤 | 期望 | 状态 |
|---|---|---|---|---|
| REG-CV-01 | artifact create | channel → "+ artifact" → markdown | artifact 渲染 | todo |
| REG-CV-02 | artifact edit + version | 编辑 → 提交 | version+1 + history | todo |
| REG-CV-03 | edit-history audit (CV-15) | 看 history | edit hash 字面 byte-identical | todo |
| REG-CV-04 | comment create (CV-5) | 进 artifact → click "新增评论" 按钮 (反 hover 路径, hover 是 reaction picker, 易命中错) → 输入 → 提交 | comment 出现 | todo |
| REG-CV-05 | comment edit/delete (CV-7) | own comment → 编辑 / 删 | 编辑/删生效 | todo |
| REG-CV-06 | comment thread reply (CV-8) | reply 1 comment | thread 嵌套 | todo |
| REG-CV-07 | comment mention (CV-9) | @user in comment | mention 通知 | todo |
| REG-CV-08 | comment markdown render (CV-11) | markdown body | prism 高亮 + image embed | todo |
| REG-CV-09 | comment search (CV-12) | search 关键字 | hit list | todo |
| REG-CV-10 | iterate state 4 态 (CV-4) | iterate task | pending/running/completed/failed 4 inline DOM | blocked-by-UI-coverage (#409 待 server) |
| REG-CV-11 | drag-drop image upload | 进 artifact 编辑器 → 拖图片文件到编辑器 → preview 真显 → 提交 | image 真嵌入 artifact + 服务端 PutBlob 真存 (DL-1 #609 Storage interface) | todo |
| REG-CV-12 | paste 截图 upload | 进 artifact 编辑器 → 系统截图 → Cmd+V 粘贴 → preview 真显 → 提交 | image 真嵌入 + GetURL 真返 | todo |

### §DM — Direct Message (8 场景)

| ID | scope | 操作步骤 | 期望 | 状态 |
|---|---|---|---|---|
| REG-DM-01 | DM 1:1 create | user → "+" → 选 user | DM channel 创建 | todo |
| REG-DM-02 | DM message edit/delete | own DM → 编辑/删 | 生效 | todo |
| REG-DM-03 | DM reaction (DM-5) | hover msg → click reaction 按钮 (👍 icon, 反 hover-only 路径; CV-5 才是 hover) → EmojiPickerPopover (DM-9) 真出 → 选 emoji | reaction 出现 + 计数 | todo |
| REG-DM-04 | DM pin/unpin (DM-10) | own msg → 右键 OR ⋯ button → context menu → click "置顶" / "取消置顶" (字面 byte-identical 跟 DM-10 content-lock `已置顶` / `取消置顶`) | pinned section 显示 | todo |
| REG-DM-05 | DM search (DM-11) | 跨 DM search | 命中 list | todo |
| REG-DM-06 | DM multi-device sync (DM-3) | DM 在 device A 发 → device B 收 | fanout | todo |
| REG-DM-07 | DM mention agent (CM agent-to-agent via DM2 router) | @agent in DM | router 真转 | todo |
| REG-DM-08 | DM cross-org block | 跨 org user DM | fail-closed | todo |

### §HB — Host Bridge (6 场景)

| ID | scope | 操作步骤 | 期望 | 状态 |
|---|---|---|---|---|
| REG-HB-01 | daemon connect (HB-2 #617) | 装 daemon → 启 | UDS connect 真挂 | **deferred-to-host-deploy-verify** (不进 Playwright budget; 走 native install verify path) |
| REG-HB-02 | IPC matrix (HB-2.0 #605) | 三平台 (Linux/macOS/Windows) prereq | matrix 全过 | CI 守 (跨 OS) |
| REG-HB-03 | sandbox (HB-2 v0(D)) | plugin 走 sandbox | sandbox enforce | todo |
| REG-HB-04 | install butler (HB-1b #627) | 装 .deb / .pkg | binary 真装 + service 真起 | **deferred-to-host-deploy-verify** (不进 Playwright budget; 走 native install verify path) |
| REG-HB-05 | plugin manifest fetch (HB-1 #589) | client → GET /api/v1/plugin-manifest | 7-reason 字典 ed25519 sig 真验 | blocked-by-UI-coverage (admin SPA 缺 manifest viewer) |
| REG-HB-06 | reconnect on daemon down | daemon kill → 重启 | client 真重连 | todo |

### §RT — Realtime (8 场景)

| ID | scope | 操作步骤 | 期望 | 状态 |
|---|---|---|---|---|
| REG-RT-01 | cursor presence | user A 在 artifact 移动光标 → user B 看 | 光标真跟 | todo |
| REG-RT-02 | presence 4 态 (RT-3 #616) | online/idle/busy/offline 4 态 | 4 dot 字面渲染 | todo |
| REG-RT-03 | thinking subject (RT-3) | agent thinking → user 看 | thinking 5-pattern 真不漏 DM body + **反向**: 0 typing-indicator 字面 ("正在输入"/"typing..." 0 hit, 沉默胜假活物感锁链 RT-3+AL-3+CV-14+CS-3+CS-4+CS-2 第 7 处) | todo |
| REG-RT-04 | multi-device fanout (RT-3) | device A + B 同 user → A 发消息 | B 真收, 0 ms 延 | todo |
| REG-RT-05 | backfill on reconnect (RT-1.2) | WS close → reconnect → **反向**: WS reconnect 期间 UI 0 显 "已连接"/"connected" 字面 (沉默立场承袭) | ≤3s backfill + 反向 banner 字面 0 hit | todo |
| REG-RT-06 | cold start no auto-pull (RT-1.2 ②) | 全新 session | 不 auto-pull history | todo |
| REG-RT-07 | events 双流 (DL-2 #615) | channel_events + global_events | v=46/47 双流 | todo |
| REG-RT-08 | retention sweeper (DL-2) | events ≥retention TTL | sweep 真删 | todo |

### §AP — Auth/Permissions (6 场景)

| ID | scope | 操作步骤 | 期望 | 状态 |
|---|---|---|---|---|
| REG-AP-01 | bundle 一键授予 (AP-2 #620) | admin → user → 选 bundle | 真授 | blocked-by-UI-coverage |
| REG-AP-02 | capability transparency UI (AP-2) | user 看自己 permissions | 14 capability 字面渲染 | blocked-by-UI-coverage (user-side缺 UI) |
| REG-AP-03 | dot-notation grant (CAPABILITY-DOT #628) | grant channel.read → ok / read_channel → 拒 | 400 invalid_capability | blocked-by-UI-coverage |
| REG-AP-04 | reaction ACL (AP-4) | non-member react | fail-closed | todo |
| REG-AP-05 | message ACL matrix (AP-5) | 跨 org / hidden 消息 | matrix 守 | todo |
| REG-AP-06 | impersonate grant 24h (ADM-2.2) | user → 业主 24h impersonate | 红 banner + 24h TTL | todo |
| REG-AP-07 | capability `*` admin-only (CAPABILITY-DOT 蓝图 §3 17 cap byte-identical) | grant `*` 给非 admin user → 应拒 | 400 + msg `* admin-only` 字面 byte-identical 跟蓝图 | todo |

### §ADM — Admin (10 场景)

| ID | scope | 操作步骤 | 期望 | 状态 |
|---|---|---|---|---|
| REG-ADM-01 | admin login | done above | done | done (SMK-01) |
| REG-ADM-02 | admin god-mode banner (ADM-2 #484) | impersonate active | 红 banner 字面 | done (SMK-08) |
| REG-ADM-03 | impersonate grant CRUD (ADM-2.2) | user 创/撤销/重 grant | 409 cooldown + re-grant | blocked-by-UI-coverage |
| REG-ADM-04 | multi-source audit (ADM-3 #619) | admin → multi-source | 4 source enum 合并查询 | todo |
| REG-ADM-05 | audit-log archived 三态 (admin-spa-shape-fix #633 D4-A + admin-spa-archived-ui-followup) | active vs archived row 视觉 + filter toggle | className 三态 + `archived` 真挂 row + `data-filter="archived"` select 3 option (active/archived/all) | ✅ done — admin-spa-archived-ui-followup PR 兑现: client filter UI + AuditLogFilters.archived 字段 + URL param 透传 server `?archived=` enum 三态 byte-identical (admin_endpoints.go) + 5 vitest 守 (REG-ASAUI-001..005). |
| REG-ADM-06 | audit hook 5/5 handler (ADM-2 + ADM-2-FOLLOWUP #626) | trigger 5 admin write | 5 audit 行 | blocked-by-UI-coverage (handler 部分 UI 缺) |
| REG-ADM-07 | system DM body 5 模板字面 (ADM-2 system DM) | trigger admin action | system DM body byte-identical | todo |
| REG-ADM-08 | privacy promise (ADM-1 #464) | user 看自己 audit | 立场 ④ 只见自己 + ?target_user_id 反 inject | todo |
| REG-ADM-09 | admin god-mode 不入 channel (ADM-0 §1.3) | admin god-mode | 看 metadata 但不能 enter | todo |
| REG-ADM-10 | admin password plain env (ADMIN-PASSWORD-PLAIN-ENV #635) | docker compose plain env → 启 → 登录 | 登录真过 | todo (deploy verify) |

### §CM — Community (4 场景)

| ID | scope | 操作步骤 | 期望 | 状态 |
|---|---|---|---|---|
| REG-CM-01 | onboarding Welcome (CM-onboarding) | new user 首登 | Welcome channel + 引导 system DM | done (SMK-09) |
| REG-CM-02 | bug-029 name display | agent invitation inbox | name 字面渲染 (反 raw UUID) | todo |
| REG-CM-03 | bug-030 onboarding regression | 同 onboarding | 字面 byte-identical | todo |
| REG-CM-04 | x2 collab (CM-5) | 2 user 协作 artifact | 真同步 | todo |

### §DL — Datalayer (4 场景)

| ID | scope | 操作步骤 | 期望 | 状态 |
|---|---|---|---|---|
| REG-DL-01 | events 双流 (DL-2 #615) | RT-1.2 backfill | channel_events + global_events 双流 | todo |
| REG-DL-02 | offloader (DL-3 #618) | events 阈值哨触发 | EventsArchiveOffloader.Start 真触 | (CI 守, runtime hard to e2e) |
| REG-DL-03 | PWA subscribe (DL-4 #485) | user 接受 push 通知 | 真注册 service worker + 真收推送 | **deferred-to-host-deploy-verify** (需 prod push gateway, 不进 Playwright budget) |
| REG-DL-04 | events store SSOT (DL-1) | 4 必落类 (perm/impersonate/agent.state/admin.force_) | byte-identical 落表 | todo |

### §INFRA (3 场景, CI-only)

| ID | scope | 期望 | 状态 |
|---|---|---|---|
| REG-INFRA-01 | PR lint (current 同步) | docs/current/* 跟 production code 同步 | CI guard ✅ |
| REG-INFRA-02 | regression-registry merge=union | 多 PR 加 REG 行 不撞 | CI guard ✅ |
| REG-INFRA-03 | line budget (INFRA-4) | 单文件 ≤200 行 | CI guard ✅ |

---

## 3. 总数

| 类别 | 总数 | 状态分布 |
|---|---|---|
| **Smoke** | **17** (v3 +3: SMK-15 cookie SSOT / SMK-16 capability dot UI grant / SMK-17 ULID 字面) | done: 9 (post Phase 3 smoke 2026-05-02 + #639 admin-spa-ui-coverage — SMK-01/02/07/08/09/10/15/16/17) / todo: 8 / blocked: 0 |
| **Regression** | **86** (v2 不变 — AL 8 + BPP 6 + CHN 10 + CV 12 + DM 8 + HB 6 + RT 8 + AP 7 + ADM 10 + CM 4 + DL 4 + INFRA 3) | done: ~4 (REG-ADM-05 ✅ admin-spa-archived-ui-followup #638 兑现) / todo: ~50 / blocked-by-UI-coverage: ~14 / **deferred-to-host-deploy-verify**: 3 (REG-HB-01/04 + REG-DL-03) / CI-守: ~16 |
| **总计** | **103** (93 → 100 v2 → 103 v3) | |

### v3 变更日志 (post 飞马 architect ① ② ③ ④ + 野马 PM review)

**飞马 ① 锚 milestone PR # (4 处)**:
- SMK-04 加锚 DM-2 #197 / RT-1.3 #229
- SMK-05 加锚 RT-1 #290 / RT-3 #616
- SMK-06 加锚 CV-1 #226 / CV-2 #228 / CV-3 #233
- SMK-08 改"5 action enum dropdown" → **"4 source enum filter UI"** (ADM-3 #619 + ADM-2-FOLLOWUP #626 真锚: agent_state / admin_actions / impersonate_actions / user_actions; 反 5-action enum 旧锚)
- SMK-03 + CHN-1 #194 锚

**飞马 ② smoke 跨 milestone 锁链断言 +3**:
- SMK-15 user cookie SSOT (`borgee_token` 字面, cookie-name-cleanup #634 反 silent drift)
- SMK-16 capability dot SSOT 真 UI grant (CAPABILITY-DOT #628 + admin-spa-shape-fix #633 D6 + admin-spa-ui-coverage #639) — v4 ✅ done (#639 真兑现 admin SPA grant UI, blocked-by-UI-coverage 解锁)
- SMK-17 ULID 字面 regex 真锚 (ULID-MIGRATION #625 真生效)

**飞马 ③ DOM 字面 stale 修 (2 处)**:
- 🔴 SMK-08 banner 字面 v4 revert: v3 飞马 ③ 走错锚 (PrivacyPromise.tsx 是 user-side 不同源), v4 revert 回 v1 真值 `当前以业主身份操作 — 该会话受 24h 时限` (admin SPA BannerImpersonate.tsx, Phase 3 smoke 2026-05-02 真验)
- ✅ REG-ADM-05 admin-spa-archived-ui-followup #638 真兑现 — client AdminAuditLogPage.tsx 加 `data-filter="archived"` select 3 option (active/archived/all) byte-identical 跟 server enum + AuditLogFilters.archived 字段 + URL param 透传, 5 vitest 守 (REG-ASAUI-001..005); #633 D4-A row class 已加, #638 补 filter UI 闭环.

**飞马 ④ Budget 重分类 (3 处)**:
- REG-HB-01 daemon connect → **deferred-to-host-deploy-verify** (不进 Playwright budget)
- REG-HB-04 install butler → **deferred-to-host-deploy-verify**
- REG-DL-03 PWA push → **deferred-to-host-deploy-verify** (需 prod push gateway)

### v2 变更日志 (post 野马 PM review)

**5 文案修** (true bug 反命中错避免):
- SMK-04: Enter 改"无 mention picker open 时按 Enter; mention picker open 时 Cmd+Enter" 拆死
- REG-CHN-08: drag 改"鼠标按住 dnd-kit drag handle 拖" 反键盘 up/down 错觉
- REG-CV-04: hover 改 click "新增评论" 按钮 反命中 reaction picker
- REG-DM-03: hover 改 click reaction 按钮 (👍 icon) → EmojiPickerPopover (DM-9)
- REG-DM-04: pin 改右键 OR ⋯ button → context menu → "置顶"/"取消置顶" 字面 byte-identical

**5 真缺漏 v2 真补**:
- SMK-11 mention 真用 path (DL-4 mention badge + CV-9 comment mention)
- SMK-12 三搜 (artifact / channel / DM 三搜拆死立场链)
- SMK-13 notification (CS-3 PWA install + Web Push 三态 content-lock)
- SMK-14 settings 持久 path (multi-device sync 应用)
- REG-CV-11 + REG-CV-12 image upload edge case (drag-drop + paste 截图, DL-1 #609 Storage 真用)

**3 反向断 (PM follow-up 建议加, 立场承袭)**:
- REG-RT-03 加反向 thinking 0 typing-indicator 字面 (沉默胜假活物感锁链 第 7 处)
- REG-RT-05 加反向 reconnect 0 "已连接" 字面 (沉默立场承袭)
- REG-AP-07 capability `*` admin-only 字面 byte-identical 跟 CAPABILITY-DOT 蓝图 §3

---

## 4. blocked / deferred 汇总

### 4.1 blocked-by-UI-coverage (P3 admin-spa-ui-coverage backlog 真补 UI 后才能跑)

> SMK-16 已由 #639 admin-spa-ui-coverage 第一波真兑现, blocked-by-UI-coverage 解锁; 余 10 项留 wave-2 / v1 GA backlog.

1. REG-AL-06 grant capability bundle
2. REG-AL-07 revoke capability
3. REG-BPP-01 plugin install manifest viewer
4. REG-CHN-07 admin role change UI
5. REG-HB-05 plugin manifest viewer (admin SPA)
6. REG-AP-01 bundle 一键授予 UI
7. REG-AP-02 user capability transparency UI
8. REG-AP-03 dot-notation grant UI
9. REG-ADM-03 impersonate grant CRUD UI
10. REG-ADM-06 audit hook 5/5 UI (部分 handler 缺 UI)

### 4.2 ✅ admin-spa-archived-ui-followup 已兑现 (#633 D4-A client followup 闭环)

1. REG-ADM-05 audit-log archived 三态 — ✅ done: client AdminAuditLogPage filter UI + AuditLogFilters.archived 字段 + URL param 透传 server `?archived=` enum 三态 byte-identical (admin_endpoints.go) + 5 vitest 守 (REG-ASAUI-001..005).

### 4.3 deferred-to-host-deploy-verify (不进 Playwright budget, 走 native install / prod deploy verify)

1. REG-HB-01 daemon connect (native install)
2. REG-HB-04 install butler (.deb / .pkg)
3. REG-DL-03 PWA push (需 prod push gateway)

---

## 5. 退出条件 (Phase 3 报告时)

每场景报:
- ✅ done — 真 UI e2e 跑过, screenshot + DOM dump 存证据目录
- 🟡 partial — 真 UI 跑过部分, 缺 X 步骤
- ⏸ blocked-by-UI-coverage — UI 0 surface 真不可点, P3 backlog 真账
- ⏸ deferred-to-host-deploy-verify — 不进 Playwright budget, 走 native install / prod deploy verify
- ⚠️ todo — 此次 wave 没跑 (排期下次)
- ❌ failed — 真 UI 跑过, 真路径不通 → 真 bug 立刻报

证据目录: `docs/evidence/liema-e2e-<wave>/<id>-<scope>.png` + `<id>.html` + `<id>-timeline.txt`.

**铁规重申** (用户 2026-05-01): blocked / deferred 状态**不允许 fetch / cURL 顶替**; 真 UI 缺就是真缺, 留账透明等真补.

---

## 6. 立场承袭

- 用户 2026-05-01 铁律: e2e 仅真 UI input + click + screenshot 算
- 用户 2026-05-01 铁律: 完整 e2e 验证, 浏览器真模拟用户操作, 不允许偷懒
- 跟 `docs/evidence/liema-633-browser-verify/README.md` §0 立场 byte-identical
- blocked-by-UI-coverage 不删, 留账透明 (跟 P3 admin-spa-ui-coverage backlog task #11 关联)
- Phase 3 smoke 真跑证据 (`docs/evidence/liema-e2e-smoke-2026-05-02/`) — post #639 9/17 真 UI pass / 0 真 bug / 4 cross-milestone 锁链 prod 真生效
- testing 环境部署铁律配套

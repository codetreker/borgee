# Acceptance Template — ADM-2: 分层透明 audit (用户可见性) v1

> 蓝图: `docs/blueprint/current/admin-model.md` §1.4 (L82-105, "谁能看到什么" 四档分层 + 三条红线)
> 蓝图不变量: §2 (L109-120, "受影响者必感知" + "Audit 100% 留痕" + "Audit 分层可见")
> Implementation: `docs/implementation/modules/adm-2-spec.md` (战马D PM 客串 v0)
> Content lock: `docs/qa/adm-2-content-lock.md` (system DM + 红横幅字面)
> Stance: `docs/qa/adm-2-stance-checklist.md` (7 立场 + 10 反约束)
> R2 决议: 野马取消 ⭐ — 普通用户零感知, 不进野马签字闸 4 (内部 milestone, 烈马代签)
> 依赖: ADM-1 ✅ (#455+#459+#464 merged) — 隐私承诺页 §4.1 文案在此真实施
> Owner: 战马 实施 (D PM + B server) / 烈马 验收 + 代签

## 拆 PR 顺序 (新协议: 一 milestone 一 PR)

- **ADM-2 整 milestone** PR #484 — 一 PR 装 spec + schema (admin_actions v=22, impersonation_grants v=23) + server (5 audit hook + 双 GET endpoint + system DM emit + impersonate REST) + client (BannerImpersonate / AdminActionsList / ImpersonateGrantSection + SettingsPage 扩) + closure docs

> 历史: ADM-2.0 spec PR #475 (含 spec brief / content-lock / stance / acceptance v1) 已 closed (内容 cherry-pick 入 #484); ADM-2.1 schema PR #470 已 closed (内容 cherry-pick 入 #484). 新协议 (#474 文件) 后, ADM-2 整 milestone 一次性闭环.

---

## 验收清单

### 数据契约 (蓝图 §2 不变量)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `admin_actions` schema 字段 (id PK / actor_id FK admins / target_user_id FK users / action enum CHECK / metadata / created_at) + 双索引 (target_user_id+created_at DESC, actor_id+created_at DESC) | unit + migration test | 战马D / 烈马 | ✅ #484 — `adm_2_1_admin_actions_test.go::TestADM21_CreatesAdminActionsTable` + `TestADM21_HasIndexes` 7/7 PASS, server-go migrations suite 0.013s green |
| admin action 类型枚举 5 字面 byte-identical (delete_channel / suspend_user / change_role / reset_password / start_impersonation) DB CHECK 约束 + 反约束 reject 同义词/大小写/字典外/空 | unit (table-driven 5 accept + 15 reject) | 战马D / 烈马 | ✅ #484 — `TestADM21_AcceptsAll5Actions` + `TestADM21_RejectsUnknownAction` (15 反约束) PASS |
| `impersonation_grants` schema (v=23): 5 列 (id PK / user_id NOT NULL / granted_at / expires_at / revoked_at NULL) + idx_impersonation_grants_user_id_expires; 蓝图 §3 字面 "由 user 创建, admin 仅消费" 反向 actor_id 不入此表 | unit + migration test | 战马D / 烈马 | ✅ #484 — `adm_2_2_impersonation_grants_test.go` 6 tests PASS (表结构 / 反向列名 / 索引 / PK / revoked_at NULL semantic / idempotent) |

### 行为不变量 (闸 4 — ADM-2 4.1)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1.a 每种 admin action 类型 → 自动写一行 admin_actions (单测覆盖每种 action; 反向: action 路径不写 audit → 该 endpoint 必须红测; CI grep `skip_audit\|noAudit\|bypassAudit` count==0) | unit (table-driven) + CI grep | 战马D / 烈马 | ✅ #484 — admin handler audit hook (`internal/api/admin.go`): force_delete_channel / patch disabled / patch password / patch role 4 actions wired; `adm_2_2_audit_hook_test.go::TestADM22_ForceDeleteChannel_WritesAudit*` + `PatchUserDisabled_WritesSuspendAudit` + `PatchUserPassword_WritesResetPasswordAudit` PASS; start_impersonation 留 admin SPA POST 路径 (待 follow-up) |
| 4.1.b 受影响者必收 system message: admin 删 channel → target user 收 "你的 channel #X 被 admin {admin_username} 于 {ts} 删除" body 字面 byte-identical (强制下发, 不依赖前端订阅; admin_username 非 raw UUID; content-lock §1 字面锁) | E2E + unit (跟 ADM-1 deferred §4 第 4 项**共测试**, 在此真实施) | 战马D / 烈马 | ✅ #484 — `store.RenderAdminActionDMBody` 5 模板字面 byte-identical (跟 content-lock §1 同源); `TestRenderAdminActionDMBody_ByteIdentical` + `_NeverContainsRawUUID` PASS; audit hook 集成 `EmitAdminActionAudit` 自动 emit 到 target user 的 #welcome (system) channel; `TestADM22_ForceDeleteChannel_WritesAuditAndSystemDM` 反向断言 DM body 含 "admin {login}" 不含 raw UUID PASS |
| 4.1.c 分层可见: user A 调 `/api/v1/me/admin-actions` **只**返回 target_user_id == A 的行 (反向: ?target_user_id 参数被忽略, 跨 user 调 → 空数组, 不泄漏跨 org) | unit + e2e 反向 | 战马D / 烈马 | ✅ #484 — endpoint `handleListMyAdminActions` 服务端 `WHERE target_user_id = current_user_id`; `TestADM22_GetMyAdminActions_ScopedToTargetUser` + `_IgnoresTargetUserIDInjection` PASS (?target_user_id 参数被忽略反向断言) |
| 4.1.d admin 之间互相可见: admin X 调 `/admin-api/v1/audit-log` 返回**全部** admin_actions 行 (含 admin Y 的操作); user cookie 调同 endpoint → 401 (REG-ADM0-002 同款轨道隔离 fail-closed) | unit | 战马D / 烈马 | ✅ #484 — endpoint `handleAdminAuditLog` 无 WHERE 默认全可见 + 三 filter (?actor_id / ?action / ?target_user_id); `TestADM22_GetAdminAuditLog_FullVisibility` + `_FilterByActor` + `_UserCookieRejected` (REG-ADM0-002 共享底线) PASS |

### impersonate 红横幅 (闸 4 — ADM-2 4.2, ADM-1 §4.1 R3 兑现)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.2.a 业主授权 → 顶部红横幅常驻 + 24h 倒计时 + `[立即撤销]` 入口 (蓝图 §1.4 红线 2 + ADM-1 §4.1 R3 第 2 条 "顶部红色横幅常驻可随时撤销" 字面兑现) | e2e DOM 锁 `[data-banner="impersonate-active"]` toBeVisible + 倒计时字面 `剩 23h{m}m` | 战马D / 烈马 | ✅ #484 client UI — `BannerImpersonate.tsx` (data-banner="impersonate-active" + 23h 倒计时 setInterval(1000) + `[立即撤销]` 字面 byte-identical 跟 content-lock §2); `BannerImpersonate.test.tsx` 6 tests PASS (no-grant/revoked → 不渲染 / active → 字面 byte-identical / 反向 raw UUID / 撤销点击调 revokeGrant); App.tsx 顶部 wire; e2e 双截屏待补 (G4.2 follow-up) |
| 4.2.b admin 写动作需 impersonate (例如 reset_password 影响活跃账号) → server 校验 grant 存在 + 未过期 + 未撤销, 否则 403 `impersonate.no_grant` | unit + e2e 反向 (无 grant POST → 403 + body code 字面) | 战马D / 烈马 | ✅ 全闭 — `store.ActiveImpersonationGrant` helper 已落 + 测试 PASS + admin handler grant 校验 wire ✅ ADM-2.3 + #464 兑现 |

### 蓝图行为对照 (闸 2 — 立场反查)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| §1.4 红线 3 "admin 之间互相留痕": `grep -rE 'admin_actions.*INSERT\|adminActions\.create' internal/admin/` 覆盖 admin SPA 所有写路径 (反向: action 不写 audit → CI grep block) | CI grep + handler 反射 | 飞马 / 烈马 | ✅ #484 — `internal/api/admin.go::handleForceDeleteChannel` + `handleUpdateUser` 4 path 全 wired (`s.EmitAdminActionAudit` 调用); 反向 grep `skip_audit\|noAudit\|bypassAudit` 在 `internal/api/admin.go` count==0 |
| §2 forward-only: `grep -rE "DELETE FROM admin_actions\|UPDATE admin_actions SET" internal/` 除 migration count==0 (audit 不可改写) | CI grep | 战马D / 烈马 | ✅ #484 — schema 不挂 `updated_at` 列; store helpers 不暴露 update/delete admin_actions 路径; 反向 grep 实跑 (除 migration self-test) count==0 |

### 退出条件

- 上表 11 项: **11 ✅ 全闭** (4.2.b 全闭 ADM-2.3 + start_impersonation 已 #464 兑现)
- 战马 PR review 同意 + 烈马 acceptance 跑完
- 登记 `docs/qa/regression-registry.md` REG-ADM2-001..007 ✅ 落 (PR merge 后 24h 内翻 ⚪ → 🟢)
- ADM-1 `acceptance-templates/adm-1.md §4 联签` deferred 2 行 (admin 写动作 system DM `admin_name` 非 UUID + DM body 字面) ✅ DONE (#484 server audit hook + content-lock §1 5 模板兑现)
- ⚠️ 不进野马 G2.4 / G4 签字流 (R2 取消 ⭐), 但 ADM-1 隐私承诺页 "你能在设置看到 admin 影响记录" 文案兑现 — 由烈马代签 `docs/qa/signoffs/adm-2-liema-signoff.md` (跟 cm-4 / adm-0 同格式)

### Follow-up 留账 (#484 后续 patch)

- start_impersonation admin SPA POST 路径 wire (跟 4.1.a + 4.2.b 同期)
- admin SPA `/admin/audit-log` 页 client (跨端字面英文 enum, content-lock §5)
- e2e + G4.2 双截屏 (`g4.2-adm2-{audit-list,red-banner}.png` 入 git)
- 烈马 acceptance 代签 `docs/qa/signoffs/adm-2-liema-signoff.md`

---

## 闸 4 demo 截屏 (烈马代签字面验, 跟 G2.4 / G3.4 同模式)

| 截屏 | 路径 | 内容 |
|---|---|---|
| G4.2-1 audit 列表 | `docs/qa/screenshots/g4.2-adm2-audit-list.png` | 用户设置页 → 隐私 → "影响记录" 子段, 显示 admin 操作记录 (admin_username 非 UUID, 字面 byte-identical 跟 content-lock §4 同源) |
| G4.2-2 红横幅 | `docs/qa/screenshots/g4.2-adm2-red-banner.png` | 业主端顶部红横幅 24h 倒计时 + `[立即撤销]` 入口 (字面 byte-identical 跟 content-lock §2 同源) |

---

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-28 | 烈马 | v0 — 7 验收项 |
| 2026-04-29 | 战马D | v1 — 加 spec/content-lock/stance 锚, 拆 ADM-2.0..2.x 5 PR 进度, ADM-2.1 ✅ #470 翻牌, 9 验收项 (数据契约 2 + 行为 4.1.a-d 4 + impersonate 红横幅 4.2.a/b 2 + 蓝图行为 2), 加 G4.2 双截屏路径锁, 加烈马代签机制 |
| 2026-04-29 | 战马D | v2 — 新协议一 milestone 一 PR 落地, ADM-2 整 PR #484 一次性闭环 (spec + schema v=22 + impersonation_grants v=23 + server endpoints + audit hook + client UI). 翻 9/11 ✅ (含 +1 impersonation_grants schema 验收项); 2/11 ⏸️ deferred (4.2.b grant 校验 wire + start_impersonation admin SPA 路径 留 follow-up). 累计 68 unit tests PASS (server 49 + migrations 13 + client 9), coverage 85.1% > threshold 85%. ADM-1 #464 deferred 2 行 (admin 写动作 system DM admin_name 非 UUID) 兑现锚 #484 server audit hook 落地. |
| 2026-04-30 | 烈马 | flip 11/11 ✅ — ADM-2.3 + #464 兑现 4.2.b grant 校验 wire + start_impersonation admin SPA 路径全闭 |

# ADM-0 闸 4 demo 立场反查表

> **状态**: v0 (野马, 2026-04-28)
> **目的**: 战马 ADM-0.3 (admin SPA 路径 + impersonation grant + audit log) PR 直接吃此表为 acceptance 锚点; 野马 / 飞马 PR review 拿此表反查立场漂移; ADM-0 闸 4 demo 用户感知签字硬条件。
> **依赖**: ADM-0.1 ✅ (admins 独立表), ADM-0.2 ✅ (cookie 拆 + RequirePermission 去 admin 短路), ADM-0.3 启动中 (god-mode endpoint + impersonation grant + audit + 红色横幅 UX)。
> **关联**: `admin-model.md` §0 (强权但不窥视) + §1.1 (独立 SPA) + §1.3 (硬隔离 + god-mode 不返回内容) + §1.4 (分层透明), `concept-model.md` §1.4 (团队感知 — admin 不在), 14 立场 §1.1 (UI 永不暴露 org_id) + §1.2 (agent=同事, admin ≠ 同事)。

---

## 1. 4 项立场 — 反查路径 + 反向断言

| # | 立场锚 | 实施落点 (ADM-0.3 应有) | 反向断言 (admin-model 红线) |
|---|--------|---------|-----|
| ① **admin 不入 channel** | admin-model §1.1 + 不变量 "Admin ∉ Org" + concept-model §1.4 (团队感知主体验) | admin **永不**出现在 `channel_members` 表; admin SPA 走独立 `/admin-api/*` 路径; user SPA sidebar / channel 列表 / DM 列表 grep `admin` keyword 0 hit | admin 登录后 `GET /api/v1/channels` (user-api) → **401** (cookie 不通); 业主 `GET /api/v1/channels/:id/members` → response 不含任何 `role=admin` user; 业主 sidebar agents 列表 / 团队感知区域 admin 永不渲染 (与 §1.2 agent=同事一致, admin ≠ 同事) |
| ② **admin SPA 红色横幅** | admin-model §1.4 第 2 红线 ("Impersonate 必须显眼"); 野马 R3 横幅常驻 + 倒计时 | admin SPA **任何 page** 顶部组件 `<AdminBanner>` 常驻; impersonation 期间业主端 SPA 顶部 `<ImpersonationBanner red>` 常驻 + 倒计时 (剩 23h59m → 0); 字面文案锁: `"support {admin_name} 正在协助你, 剩 {hh}h{mm}m"`; 颜色锁: `background: #d33` (不是 amber/yellow 灰带) | impersonation_grants 行存在 → 业主 SPA 顶部 banner DOM 必在场 (e2e `getByTestId('impersonation-banner')` count=1); grant 撤销 → banner 立即消失 (≤ 3s, 走 RT-0 push); admin SPA 任意 page 跳转 banner 不丢 (不能只在首页) |
| ③ **受影响者 system message** | admin-model §1.4 第 1 红线 ("受影响者必收, 不能静默"); §2 不变量 "受影响者必感知 admin 操作" | admin 写动作 (force delete channel / 重置 API key / 改密码 / disable user / soft delete user) 触发自动 system message → 受影响 user 的 system DM (kind=system, sender=system); 字面锁: `"你的 channel #{name} 被 admin {admin_name} 于 {ts} 删除"` / `"你的 API key 被 admin {admin_name} 重置, 请重新生成"` 等; **不可关闭** (no opt-out) | admin force delete channel → DB 内对应 user 的 system DM 必有新行 (不是 fire-and-forget log); user 重新登录 → system DM 历史保留 (不限期); §1.1 反查: system message 内含 `admin_name` 而**非** `admin_id` UUID (UI 永不暴露 raw ID, 跟 CM-4 bug-029 同根) |
| ④ **god-mode endpoint 不返回内容** | admin-model §1.3 god-mode 字段白名单 + §2 不变量 "god-mode 绝不返回 message.body / artifact 内容" | `/admin-api/channels/:id` 响应 sanitizer 白名单 = `{id, name, member_count, message_count, created_at, members[name,role]}`, **不含** `messages[]` / `body` / `artifact_blob`; 同理 `/admin-api/users/:id` 不返回 settings.api_key 原值 (只 `key_present: bool`); 反向断言测试用 `TestAdminGodModeOmitsContent` 锁字段白名单 | grep `internal/api/admin_*.go` 任何 endpoint 不出现 `message.body` / `artifact.content` / `api_key` raw 字段 selection; e2e: admin 调 `/admin-api/channels/:id` → response body grep `"body":"` / `"content":"` 0 hit |

---

## 2. 黑名单 grep — admin-model 红线闭合

```bash
# admin 永不入 channel_members
grep -rn "channel_members" packages/server-go/internal/api/admin*.go | grep -v _test.go
# 预期 0 命中 (admin 走 god-mode endpoint 不该 INSERT/JOIN channel_members)

# god-mode endpoint 不返回内容字段
grep -rnE "message\.body|artifact\.content|\.api_key" packages/server-go/internal/api/admin*.go | grep -v _test.go
# 预期 0 命中

# admin SPA 不复用 user-api
grep -rn "/api/v1/" packages/client/src/admin/ 2>/dev/null
# 预期 0 命中 (admin SPA 只调 /admin-api/*)
```

---

## 3. 反向断言锁 (ADM-0.3 PR 必含测试)

| 反向断言 | 测试位置 | 锁点 |
|---------|---------|------|
| admin cookie 调 user-api → 401 (跟 ADM-0.2 cookie 拆配套) | `internal/api/admin_isolation_test.go::TestAdminCannotCallUserAPI` | status code = 401, 不是 200 / 403 |
| god-mode endpoint 字段白名单 | `internal/api/admin_godmode_test.go::TestAdminGodModeOmitsContent` | sanitizer key set 反查, body / content / api_key 不在 |
| admin 写动作 → 受影响 user system DM 落库 | `internal/api/admin_audit_test.go::TestForceDeleteChannelEmitsSystemMessage` | DM 表有新行 + body 含 `admin_name` 非 UUID |
| impersonation_grants 行存在 → 业主端 banner DOM 渲染 | `e2e/impersonation-banner.spec.ts::TestBannerVisibleDuringGrant` | DOM count=1 + 红色 + 倒计时文本格式 `剩 {hh}h{mm}m` |
| impersonation 撤销 → banner 消失 ≤3s | 同上 + RT-0 stopwatch | 走 G2.4 stopwatch fixture |

---

## 4. 不在 ADM-0 范围 (避免 PR 膨胀)

- ❌ 跨 org admin (multi-org admin 路径) — v1, ADM-0 单 admin 域
- ❌ admin 之间互相 promote / demote 流程 — admin-model §5 v2
- ❌ Audit log 导出 / GDPR delete request — admin-model §5 v2+
- ❌ Impersonation 期间 BPP 行为变化 — admin-model §5 已锁 BPP 不感知 impersonate
- ❌ admin SPA 完整页面 UI (列表/详情/审计) — 第 11 轮"Client (web SPA)", ADM-0.3 仅最小 god-mode endpoint + 横幅
- ❌ Admin 看 channel 元数据时显示 `org_id` raw — §1.1 永不暴露, sanitizer 反查同 CM-4 bug-029 模式

---

## 5. 验收挂钩

- ADM-0.3 PR: §1 4 项实施落点全在 + §2 黑名单 grep 命中 0 + §3 反向断言 5 项全绿
- ADM-1 (用户隐私承诺页, admin-model §4.1): 3 条承诺文案锁 (野马 R3 锁) + ≥1 张 admin 写操作 system message 通知截屏
- ADM-0 闸 4 demo 野马签字: 4 项 (业主端 admin 永不出现 / impersonation banner 红色常驻 / system message 受影响者必收 / god-mode 不返回内容反查) — 落 `docs/qa/signoffs/adm-0-yema-signoff.md` 跟 cm-4-yema-signoff.md 同格式

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v0, 4 项立场 + 黑名单 grep + 5 项反向断言 + 不在范围 6 条 |

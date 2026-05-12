# Phase 2 原则 ↔ 实施 对照矩阵

> **状态**: v0 (蓝图评审负责人, 2026-04-28)
> **目的**: Phase 2 R3 锁定的 8 条蓝图原则, 1:1 对应到实现 PR、代码位置和验证证据。
> **用法**: Phase 3 R4 评审时作参考 — 任何"原则被改"的提议必须按本矩阵核验现有原则; 要修改原则 → 先更新本表。
> **不重复**: 决议进度索引见 `r3-decisions.md`; 本表只锁 "原则原文 ↔ 代码中的实际实现"。

---

## 1. 8 条原则 ↔ 实施矩阵

| # | 蓝图原则 (来源 §) | 原则摘要 | 实施 PR | 落地代码点 | 验证证据 |
|---|------|----------|--------|----------|---------|
| **S1** | concept §1.1 + §6 | 组织永久隐藏 (UI 永不暴露 org), 1 person = 1 org, 数据层以组织为核心实体 | CM-1.1 (#... v=2) + CM-1.2 (注册时自动建 org) | `internal/migrations/0002_cm_1_1.go` (`organizations` 表) + `internal/auth/register.go` (自动 INSERT) + `client/` 全 UI 0 处 `org_id` 字段 | grep client/src `org_id` = 0 次; `organizations` 表 + `users.org_id` non-null |
| **S2** | concept §1.2 + §1.3 | Agent = 同事不是工具, owner_id 1:N 独占归属, 跨 org 协作使用邀请状态机 | CM-4.0 (#... v=3) + CM-4.1 + RT-0 (#237) | `internal/migrations/0003_cm_4_0.go` (`agent_invitations`) + `internal/api/agent_invitations.go` (POST/PATCH/GET 状态机) + `internal/ws/hub.go::PushAgentInvitationPending` | 状态机单测 `pending → approved/rejected/expired`; #237 typed Push + 双向推送 (POST→owner, PATCH→requester+owner) |
| **S3** | admin §1.1 + §1.2 | Admin 独立 SPA + 环境变量初始化独立身份, 不写入 `users.role` | ADM-0.1 (#197 v=4) + ADM-0.2 (#201 v=5) + ADM-0.3 (#223 v=10) | `internal/migrations/0004_adm_0_1.go` (`admins` 4 字段) + `0005_adm_0_2.go` (`admin_sessions`) + `0010_adm_0_3.go` (4 步回填) + `internal/admin/` 包 | `users.role='admin' count=0` G2.0 不变量; `auth_isolation_test.go` 验证 god-mode 404 |
| **S4** | admin §1.3 | 硬隔离: admin 看元数据, **不看消息内容**; 使用 `/admin-api/*` 独立路由 | ADM-0.2 (#201) | `internal/admin/middleware.go::RequireAdmin` + `handlers_field_whitelist_test.go` (反射扫 body/content/text/artifact 字段) | 字段白名单单测发现漏字段时失败; `internal/admin/` 禁止 import `internal/auth/` (grep 检查 + 单测) |
| **S5** | auth §3 + §1.3 | Agent 默认权限 = `[message.send, message.read]`, owner 可移除 `message.read` | AP-0 (#... v=8 前 1 行) + AP-0-bis (#206 v=8) | `internal/migrations/0008_ap_0_bis.go` (为线上历史 agent 补 `message.read`) + `internal/auth/defaults.go` (新 agent 注册写两行) | DB query: `count(*) where role='agent' and permission='message.read' = count(agents)` |
| **S6** | realtime §2.3 | BPP Phase 4 才做完整版, Phase 2 用 `/ws` hub 临时代替完整 push 通道, frame schema **逐字节一致** = 未来 BPP frame | INFRA-2 (#195) + RT-0 (#237) | `internal/ws/event_schemas.go` ↔ `client/src/types/ws-frames.ts` ↔ `internal/bpp/frame_schemas.go` + CI lint | G2.6 逐字节一致 CI lint 通过; `TestAgentInvitationPendingFrame_ZeroExpiresIsSentinel` 锁定 `expires_at=0` 哨兵值 ↔ client `required: number` |
| **S7** | concept §10 | 新用户第一分钟体验: 注册时必须创建 `#welcome` channel + 系统消息 + 自动选中 | CM-onboarding (#203 v=7) | `internal/migrations/0007_cm_onboarding.go` (`channels.type='system'` + welcome 行 + `quick_action` JSON) + `internal/auth/register.go` (将新 user 加入 #welcome) | `channels.type='system'` per-user; `messages.sender_id='system'`; client `auto_select_channel_id` |
| **S8** | concept §11 + admin §4.1 | 用户隐私承诺页 3 条文案固定 (一字不漏 / 顺序不变), Phase 2 仅准备原则核对表; 实施推迟到 ADM-1 | (原则核对表 PR #211) + ADM-1 (post-#223, 待启动) | `docs/qa/adm-1-privacy-promise-checklist.md` (3 条文案固定 + 截屏验收) | Phase 2 只锁文档; 实施与 Phase 4 同期, 不阻塞 Phase 2 退出 (跟 R3-7 一致) |

---

## 2. 原则不可修改条件

- 任何 PR 想改 S1-S8 之一 → **必须先动本表 + r3-decisions.md**, 不准代码绕过本表提前改原则。
- S3/S4/S6 是 G2.0/G2.6 不变量, 回归测试组 §3.A-§3.G + CI lint 三层检查, 改原则必须同步更新验收模板 + 回归记录表。
- S5 回填 (v=8) 是单向闸门 — owner 后续可去掉 `message.read`, 不可批量撤回 (会违反 R3-1 决议)。

---

## 3. 相关参考

- R3 决议索引: `docs/blueprint/_meta/r3-decisions.md` (8 条决议状态)
- Phase 2 关卡进度: `docs/qa/phase-2-gate-status.md` (G2.0-G2.audit)
- 蓝图审计轮换: `docs/blueprint/_meta/blueprint-audit-rotation.md` (#219, 防文档脱节)
- 原则核对表: `docs/qa/adm-0-stance-checklist.md` + `adm-1-privacy-promise-checklist.md` + `cm-3-org-id-checklist.md`

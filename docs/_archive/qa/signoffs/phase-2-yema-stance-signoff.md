# Phase 2 立场签字 — 野马 (PM 视角)

> **签字**: 野马 (PM) · 2026-04-28
> **范围**: Phase 2 已 merged milestone 业主感知保证立场反查; 不含 RT-0 (in-flight) 与 ADM-1 实施 (Phase 4)。
> **前置**: ADM-0 整段 ✅ + CM-onboarding ✅ + AP-0-bis ✅ + CM-3 ✅ + bug-029/030 修 ✅
> **配套**: PR #225 Phase 2 退出公告草稿 + PR #232 G2.4 解封路径 + #216 R3 看板

---

## 1. 5 个 milestone 立场签字表

| Milestone | 业主感知锚 | 立场过 | demo 截屏 | 签 |
|-----------|----------|--------|---------|-----|
| **ADM-0 整段** (0.1+0.2+0.3) | admin 不入 channel (§1.1) + 红横幅 #d33 (§1.4) + god-mode 字段白名单 (§1.3 + §2 不变量) + admin cookie 不通 user-api (ADM-0.2) | ✅ 立场过 (黑名单 grep + 反向断言锁 in #205) | 🟡 #6 截屏 spec.skip 占位 (#230, 等 ADM-1 + admin e2e fixture) | 🟡 立场签 / demo 等 ADM-1 |
| **CM-onboarding** | Welcome 第一眼非空屏 + system message "欢迎来到 Borgee" + [创建 agent] 字面锁 + §11 反约束 (旧空态文案 0 命中) | ✅ 立场过 (drift test `WelcomeMessageBody` doc-as-truth) | ✅ #1 + #5 截屏 (#213 已签 partial 2/5 → 也是这条 4/4) | ✅ SIGNED |
| **AP-0-bis** (agent 默认 `message.read`) | agent 进入 channel 直接读消息不需要再问 (符合"agent=同事"立场 §1.2); owner 可随时收回 read 让 agent 不偷看历史 | ✅ 立场过 (R3-1 决议 + #206 backfill 单测) | ❌ 无 demo (业主感知层面**不可见**, 是隐性体验, 不需要截屏) | ✅ SIGNED (默认行为不需 demo) |
| **CM-3** (cross-org 403) | 业主看不到别 org 内容 (跨 org GET → 403, EXPLAIN 走 `idx_*_org_id`) | ✅ 立场过 (#208 反向断言 + G1.4 audit 全绿 in g1-exit-gate.md) | ❌ 无 demo (反向断言层面**不可见**, 跨 org 体验是"看不到", 截屏没意义) | ✅ SIGNED (反向断言充分, 不需 demo) |
| **bug-029 / bug-030 修** | inbox 邀请文案渲染 agent name (`助手`) 而非 raw UUID; system message 含 admin/agent name 非 ID | ✅ 立场过 (sanitizer + seed lock + #196 已签 + #198 lint 修补) | ✅ #196 SIGNED 闸 4 demo + 跟 G2.4 #3 关联 | ✅ SIGNED |

---

## 2. 整体立场判断

**Phase 2 立场签字 = ✅ SIGNED (有限)**, 4/5 milestone 立场全过 + ADM-0 demo 待 ADM-1 落地补 6/6 截屏。

**业主感知保证一句话**:
> 注册即看到 #welcome 频道 + 欢迎消息 + [创建 agent] 按钮; 你创建的 agent 默认能读你的频道历史 (你可收回); admin 永远不入你的 channel/DM/团队列表; 跨 org 看不到别人的内容; 邀请显示 agent 名字不显示 raw ID。

跟 PR #225 Phase 2 退出公告 §2 业主感知 5 条 1:1 对应 (除 ⑤ 邀请 ≤ 3s 等 RT-0)。

---

## 3. 留账 (不阻 Phase 2 立场签, 跟 Phase 4 同期补)

- 🟡 **G2.4 截屏 2/6 → 4/6** 等 RT-0 server merged + 烈马 spec 第一批 (#3 + #4 解锁)
- 🟡 **G2.4 截屏 5/6** 等 ADM-1 实施 merged (#6 解锁 → AdminBanner 字面锁 demo)
- 🟡 **G2.4 截屏 6/6** 等 AL-1b (Phase 4 BPP busy/idle) merged (#2 解锁 → sidebar 团队感知 demo)
- 🟡 **业主感知 ⑤ 邀请 ≤ 3s** 等 RT-0 server merged (Phase 2 退出 gate 联签条件)

---

## 4. 验收挂钩

- 这条签字 + PR #225 公告 + RT-0 merged → team-lead 做 Phase 2 milestone announcement
- 后续 Phase 4 (ADM-1 / AL-1b) merged 时野马补签 G2.4 4/6 → 5/6 → 6/6, 不动本签字 (本签 = Phase 2 已 merged 立场, 已落地不变)
- 跟 `g1-exit-gate.md` 烈马签字同模式 (5 个 milestone 表 + 整体一句话 + 留账 + 验收挂钩)

---

## 5. 签字

| Role | 名字 | 签字 | 日期 |
|------|------|------|------|
| PM | 野马 | ✅ Phase 2 立场签字 (有限): 4/5 milestone 立场全过 + ADM-0 demo 待 ADM-1 补 6/6; 业主感知 4 条已就位, ⑤ 邀请 ≤ 3s 等 RT-0 | 2026-04-28 |

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v1 — Phase 2 立场签字, 5 milestone + 业主感知一句话 + 留账 4 项 |

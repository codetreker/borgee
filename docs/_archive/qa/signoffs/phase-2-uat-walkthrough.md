# Phase 2 milestone UAT walkthrough — 业主视角实测剧本

> **状态**: v0 (野马, 2026-04-28) — 业主感知 5 step walkthrough, 文档级 (不跑实测, 引用已 merged PR + e2e spec 路径作证据)
> **配套**: PR #233 立场签字 + PR #225 公告 + PR #232 解封路径; 等 RT-0 server (#237) merged 后 Step 4 ≤3s 锚就位
> **目的**: 业主 / stakeholder / R4 review 直接照剧本对屏验收, 每步带 PR # + e2e spec + 预期文案

---

## 1. 5 step walkthrough

### Step 1: 注册 → 第一眼 #welcome 非空屏

| 字段 | 内容 |
|------|------|
| 用户预期文案 | 看到 channel "#welcome" 选中, 消息列表第一条系统消息含 **"欢迎来到 Borgee"** + 一个蓝色按钮 **`创建 agent`**; **不**看到 "👈 选择一个频道开始聊天" |
| 落地 PR | #203 (CM-onboarding) + #213 (G2.4 #1 截屏 spec) |
| e2e 证据 | `packages/e2e/tests/cm-onboarding.spec.ts:30` + `g2.4-demo-screenshots.spec.ts:#1` |
| 反约束 | README §核心 11 旧空态文案 0 命中 |

### Step 2: 点 [创建 agent] → AgentManager 弹出

| 字段 | 内容 |
|------|------|
| 用户预期文案 | 点击系统消息上的 `创建 agent` 按钮 → AgentManager 模态打开, 顶部含 "Agent" 标题; 不打开"加好友 / 加频道"流程 |
| 落地 PR | #203 (quick action chain) + #213 (G2.4 #5 局部截屏) |
| e2e 证据 | `cm-onboarding.spec.ts:102-105` + `g2.4-demo-screenshots.spec.ts:#5` |
| 反约束 | 字面锁 `创建 agent` 一字不差 (drift test `WelcomeQuickActionJSON`) |

### Step 3: 邀请 agent → inbox 渲染 agent 名字非 raw UUID

| 字段 | 内容 |
|------|------|
| 用户预期文案 | owner 邀请 agent "助手" 加入 channel "#design" → owner 端 inbox 显示 **"邀请你的 agent **助手** 加入 channel **#design**"**; raw UUID 仅在 hover title 中 (不在文本主体) |
| 落地 PR | #196 (CM-4.2 inbox UI) + #198 (bug-029 sanitizer + raw UUID 反向断言修) |
| e2e 证据 | `g2.4-demo-screenshots.spec.ts:#3` (`.skip` 占位, 等 RT-0 + agent fixture) |
| 反约束 | 14 立场 §1.1 — UI 永不暴露 org_id / raw UUID; sanitizer 反查 grep `[0-9a-f]{8}-[0-9a-f]{4}` 0 命中 |

### Step 4: agent 加入后, 邀请通知 ≤ 3s 推送到 inbox

| 字段 | 内容 |
|------|------|
| 用户预期文案 | agent 接受邀请 → owner 端 sidebar 🔔 角标 ≤ 3 秒内更新 (不是 60s 轮询); inbox 新行加载, 状态从 `pending` → `accepted` |
| 落地 PR | **#237 RT-0 server (in-flight)** + #218 client realtime 接入 (待 RT-0 merged) |
| e2e 证据 | `g2.4-demo-screenshots.spec.ts:#4` (`.skip` 等 RT-0) + 烈马 INFRA-2 stopwatch fixture (`packages/client/e2e/fixtures/stopwatch.ts`) |
| 反约束 | R3-4 锁 — `/ws` push frame schema = 未来 BPP frame byte-identical (CI lint 强制) |

### Step 5: 跨 org 试访问别人内容 → 403

| 字段 | 内容 |
|------|------|
| 用户预期文案 | user A (orgA) 拿到 user B (orgB) 的 messageId / channelId / fileId 直访 → 服务端返 **403 Forbidden** (不是 200, 不是 500); 业主端 SPA UI 永不出现 raw `org_id` 字面 |
| 落地 PR | #208 (CM-3 资源归属 org_id 直查) + #197/#201 (ADM-0.1+0.2) |
| e2e 证据 | `internal/api/messages_test.go::TestCrossOrgRead403` + `channels_test.go::TestCrossOrgChannel403` + `files_test.go::TestCrossOrgFile403` |
| 反约束 | G1.4 audit (`g1-audit.md` §2-3) — 黑名单 grep 0 命中 + EXPLAIN 走 `idx_*_org_id` |

---

## 2. UAT 结果汇总

| Step | 当前状态 | 阻塞条件 |
|------|---------|---------|
| 1 注册 → #welcome | ✅ 已就位 (#203 + #213) | — |
| 2 [创建 agent] CTA | ✅ 已就位 | — |
| 3 inbox name 渲染 | 🟡 立场过 / e2e 截屏 等 RT-0 | RT-0 + agent fixture |
| 4 ≤ 3s 推送 | 🟡 等 RT-0 server merged | #237 |
| 5 跨 org 403 | ✅ 已就位 (#208) | — |

**Phase 2 退出 gate UAT 闭合**: 3/5 ✅ + 2/5 🟡 等 RT-0 → RT-0 merged 后 5/5 ✅ → 联动 PR #233 立场签字 + PR #225 公告 → milestone announcement。

---

## 3. 验收挂钩

- 业主 / stakeholder 直接照本 doc 5 step 对屏验收
- R4 review 时 anchor: 每步 PR # + spec 路径都有, 不需要 demo 录像
- RT-0 merged 后野马补一行 v1: Step 3+4 翻 ✅, 不动其余 step

---

## 4. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v0 walkthrough — 5 step 业主视角实测剧本 |

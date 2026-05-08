# Phase 2 Exit Gate — 闸位 + 验证矩阵

> 飞马 · 2026-04-28 · 同 G1.audit 模板
> 源: `execution-plan.md` §"Phase 2 退出 gate" (R3 锁); 全过 ⇔ ADM-0.3 + RT-0 + G2.4 5/5 截屏

## G2.0 — ADM-0 cookie 串扰反向断言 (R3 加)

- 立场源: blueprint admin-model §1.3 + §2 不变量
- 验证: `internal/admin/cookie_isolation_test.go` 4.1.a/b/c 三轴 + post-migration `users WHERE role='admin'` count=0 (4.1.d)
- 状态: 🟡 partial — ADM-0.2 #201 已落 4.1.a/b/c; 4.1.d 等 ADM-0.3 v=10 backfill (task #63 战马A)
- merged PR: #197 (0.1) + #201 (0.2); ⏳ ADM-0.3

## G2.1 — 邀请审批 E2E (Playwright + ws push)

- 立场源: blueprint §1.2 agent 同事感 / CM-4.1+4.2
- 验证: `tests/e2e/agent-invite.spec.ts` (Playwright) — A 邀请 → B inbox ws push → accept → agent 自动加 channel
- 状态: ⏳ 待战马跑 — INFRA-2 scaffold #195 已落; CM-4.1/4.2 #198 已落; e2e spec 未挂 (烈马待派 task)
- merged PR: #195 / #198 (代码就位, e2e 未跑)

## G2.2 — 离线 fallback E2E

- 立场源: blueprint §1.2 离线兜底 / B.1
- 验证: Playwright e2e — A @ B-bot (offline) → B 5 秒内收 system message
- 状态: ⏳ 待战马; presence stub (G2.5) + RT-0 /ws (task #40) 双就位后才能跑
- merged PR: 无

## G2.3 — 节流不变量 (B.1)

- 立场源: blueprint §1.2 节流策略 v0
- 验证: 单测 `internal/notify/throttle_test.go` — 5 分钟内多次 @ → 系统仅发 1 条
- 状态: ⏳ 待烈马挂单测 (代码可能在 CM-onboarding #203 之后追加)
- merged PR: 无

## G2.4 — 用户感知签字 (B.2, 标志性 ⭐)

- 立场源: blueprint 核心 §13 + execution-plan §闸 4
- 验证: 野马主观签字 + 5 张截屏 (邀请通知 / 接受后成员列表 / 离线通知 / 节流第 6 次无通知 / 左栏团队感知 + Welcome 非空屏) + stopwatch ≤ 3s (R3 硬条件) + 口播 "agent↔agent Phase 4"
- 状态: ⏳ partial 2/5 — 野马 #199 spec + #210 G1 全签 已落; 截屏 #1 #5 ready, #2 等 AL-1b 后置, #3/#4 等 e2e 跑
- merged PR: #199 (spec) / #210 (G1 全签); ⏳ demo 跑

## G2.5 — presence 接口契约

- 立场源: blueprint §1.2 presence v0 stub
- 验证: `internal/presence/contract.go` 路径锁; `IsOnline + Sessions` 接口 grep 命中 1; BPP frame 建连触发点固化
- 状态: ⏳ 待飞马 + 战马 — 路径未建, RT-0 task #40 启动时一并落
- merged PR: 无

## G2.6 — /ws → BPP schema 等同性 (R3 加)

- 立场源: blueprint §1.2 BPP 复用 + execution-plan G2.6
- 验证: CI lint `bpp/frame_schemas.go` ↔ `ws/event_schemas.go` byte-identical 或 type alias, 分歧 fail
- 状态: ⏳ 待飞马 — RT-0 task #40 落地时一并加 lint
- merged PR: 无

## G2.audit — v0 代码债登记

- 立场源: execution-plan §闸 5
- 验证: audit 行覆盖 agent_invitations / presence map / 节流 / admin 拆表 (R3) / /ws schema lock (R3) / AP-0-bis (R3)
- 状态: ✅ 部分登记 — #212 audit 已起表; AP-0-bis #206 + admin 三段 全挂; presence/节流/ws 待 RT-0 后补
- merged PR: #212

## 通过判据

Phase 2 全过 ⇔ G2.0 完整 (ADM-0.3 落) + G2.1-G2.4 全 ✅ (野马签 + 截屏 5/5) + G2.5/G2.6 (RT-0 落) + G2.audit 6 项齐。

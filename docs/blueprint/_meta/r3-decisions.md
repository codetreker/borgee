# R3 决议索引 — 4 人评审 原则冲突落地索引

> **状态**: v0 (产品评审负责人, 2026-04-28)
> **目的**: PR #188 R3 评审决议固化分散在 4 篇蓝图 + 1 篇 conflicts 文档, 本文档单页索引表汇总, Phase 2 退出关卡时一目了然 ✅/待办。
> **来源**: PR #188 (be39d37) + b4ab99c + 47a4e55 + b29-vs-blueprint.md (实施评审方 6 条 P0/P1) + 产品评审方 R3 后续 §4.1 文案固定 + 验收评审方 R3 验收缺字。
> **更新规则**: ADM-0.3 / RT-0 / ADM-1 / AL-1b 落地后, 对应行状态改 ✅; 不动决议本身 (决议已敲定)。

---

## 1. 8 条 R3 决议索引表

| ID | 来源 § | 原则摘要 | 实施 milestone | 状态 |
|----|--------|---------|---------------|--------|
| **R3-1** | auth-permissions §3 + §1.3 | agent 默认权限 = `[message.send, message.read]` (按 B29 路线, owner 可去掉 read 让 agent 不偷看历史) | AP-0-bis (#41) | ✅ 已合并 |
| **R3-2** | concept-model §6 + admin-model §3.1 | `users.role` 收掉 `'admin'` enum; admin = B 环境变量初始化独立身份, 不在 users 表 (B29 完整路线) | ADM-0.1 / 0.2 / 0.3 (#43) | 🟡 0.1 ✅ + 0.2 ✅ + 0.3 在做 |
| **R3-3** | admin-model §1.3 (派生 R3-2) | admin **不能创 agent** (走独立 SPA, user-api `POST /agents` 自然不通) | ADM-0.2 cookie 拆 + 0.3 god-mode | ✅ (0.2 已落 RequirePermission 去 admin 短路) |
| **R3-4** | realtime §2.3 | BPP Phase 4 才做完整版, Phase 2 用 `/ws` hub 顶替 push (server→client frame schema **必须** = 未来 BPP frame, CI lint 强制逐字节一致) | RT-0 (#40) | ⏳ 待办 (验收负责人负责, 等 INFRA-2 后) |
| **R3-5** | agent-lifecycle §2.3 | busy/idle 推迟出 Phase 2, 跟 BPP 同期 (Phase 4); Phase 2 只承诺 online/offline + error 三态 | AL-1a (Phase 2) / AL-1b (Phase 4 BPP) | 🟡 AL-1a ✅ / AL-1b 待办 |
| **R3-6** | concept-model §10 | 注册时必须产出 #welcome channel + 系统消息 + 自动选中 (新用户第一分钟体验, README §核心 11 配套) | CM-onboarding (#42) + onboarding-journey.md | ✅ 已合并 (PR #203) |
| **R3-7** | admin-model §4.1 (产品评审方 R3 后续) | 用户隐私承诺页 3 条文案固定 (一字不漏 / 顺序不变), ADM-1 截屏验收硬标准 | ADM-1 (post-ADM-0.3) | ⏳ 待办 (核对表 PR #211 ✅ 已落, 实施未启动) |
| **R3-8** | realtime §2.3 + Playwright 前置 (验收评审方 R3) | CI lint 强制 `bpp/` ↔ `ws/` schema 逐字节一致; Playwright 必须前置到 CM-4.3a 之前 | INFRA-2 (#39) | ✅ 已合并 (PR #195) |

---

## 2. 决议依赖图 (Phase 2 退出关卡视角)

```
R3-2 (admin 拆表) ──┬─→ R3-3 (admin 不创 agent) ✅ 派生
                    └─→ R3-7 (隐私承诺页) ⏳ 等 0.3
R3-1 (message.read) ✅
R3-4 (/ws push) ⏳ ── 阻塞 G2.4 #3/#4 截屏 + AL-1b busy
R3-5 (busy 推迟出) ── AL-1b 阻塞 G2.4 #2 截屏 + Phase 4 入口
R3-6 (onboarding) ✅
R3-8 (CI lint + Playwright) ✅
```

**Phase 2 退出关卡条件**: R3-2 (0.3) + R3-4 + R3-7 三项 ✅ → 6/8 ✅ → 2/8 仍待办 (R3-5 AL-1b + R3-7 ADM-1 实施) 跟 Phase 4 同期, **不阻塞 Phase 2 退出**。

---

## 3. 参考资料 (核对资料 / 避免重复记录)

- 完整 6 条原则冲突分析: `docs/_archive/conflicts/b29-vs-blueprint.md` (实施评审方 R3 5-栏对照)
- 产品评审方 §4.1 文案固定原文: `docs/blueprint/current/admin-model.md` §4.1 + `docs/qa/adm-1-privacy-promise-checklist.md`
- 验收评审方 INFRA-2 验收缺字: `docs/qa/infra-2-acceptance.md`
- 蓝图评审方 P0 god-mode 元数据 vs 内容隔离: `docs/blueprint/current/admin-model.md` §1.3 + §2 + `docs/qa/adm-0-stance-checklist.md` §1 ④

---

## 4. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 产品评审负责人 | v0, 8 条 R3 决议索引 + 依赖图 + 参考资料; ADM-0.3 / RT-0 / ADM-1 / AL-1b 落地后逐行 ✅ |

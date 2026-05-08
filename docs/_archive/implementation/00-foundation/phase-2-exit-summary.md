# Phase 2 退出公告 — 业主感知 5 条 + 隐私承诺重申

> **草稿状态**: v0 (野马, 2026-04-28) — 待 ADM-0.3 + RT-0 双 merged 后由 team-lead 发布
> **受众**: 业主 / 投资人 / 早期用户 (不是 dev — 文案优先, 内部 ID / PR 编号尽量不出现)
> **文案锁**: 全文野马拍板, 战马 / 飞马 review 仅可指错字, 立场字面不动 (与 admin-model §4.1 + 14 立场 §1.1/§1.2/§1.4 同模式)

---

## 1. Phase 2 立场总结 — 4 条

Phase 2 是 Borgee 从"能跑通"到"用户敢用"的一段。这一段我们做完 4 件事:

1. **admin / user 完全身份分离** — admin 不再是"权限大的用户", admin 是平台运维, 走独立后台, 不在你的协作圈里。 (蓝图锚: `admin-model.md` §1.1 + §1.3)
2. **agent 默认权限可控** — 你创建的 agent 默认能 `发消息` + `读频道历史`, 你可以随时收回 `读` 让它不偷看历史。 (蓝图锚: `auth-permissions.md` §3 + R3-1)
3. **注册即上线** — 注册成功 30 秒内, 你看到一个 #welcome 频道 + 一条欢迎消息 + 一个 [创建 agent] 按钮; 不会落到空白页, 不会让你"先去找个频道"。 (蓝图锚: `concept-model.md` §10 + onboarding §3)
4. **实时邀请 ≤ 3 秒** — 你给 agent 发的入群邀请, 通过实时通道 (≤ 3 秒) 推到对方, 不靠 60 秒轮询凑数。 (蓝图锚: `realtime.md` §2.3 + R3-4)

---

## 2. 业主感知 5 条 — 你打开 Borgee 会看到什么

这 5 条是 Phase 2 退出 gate 的"用户感知签字"硬条件:

| # | 你看到什么 | 为什么这是承诺, 不是细节 |
|---|----------|------------------------|
| ① | **admin 不出现在你的频道列表 / DM 列表 / 团队感知区** | admin 不是同事 — `团队感知 §1.4` 主体验里没有 admin 这个角色。grep 反查: 业主端 SPA 任何位置 "admin" keyword 0 命中 |
| ② | **admin 协助你时, 顶部红色横幅常驻 (`#d33`) + 倒计时** — 文案: "support {admin_name} 正在协助你, 剩 {hh}h{mm}m" | 不能静默观察 — 受影响者必感知 (`admin-model §1.4` 第 2 红线) |
| ③ | **注册后第一眼就是 #welcome 频道 + 欢迎消息 + [创建 agent] 按钮**, 永远不会出现 "👈 选择一个频道开始聊天" 这种空白引导 | "沉默胜于假 loading, 但空屏不可接受" (`README §核心 11`) |
| ④ | **快速操作按钮上的字面是 `创建 agent` (一字不差)** — 点了打开 AgentManager, 不打开任何"加好友 / 加频道"流程 | 字面锁防漂移 — 你的肌肉记忆不该被换字 |
| ⑤ | **邀请 agent 入群, 对方 inbox 几秒内就看到 (≤ 3s)**; 邀请文案显示 agent 名字 (`助手`), 不显示原始 ID (UUID) | 永不暴露内部 ID (`14 立场 §1.1`) + 实时不靠轮询 (`R3-4`) |

---

## 3. 隐私承诺重申 — 3 条 (一字不差, 顺序不变)

> 来自 `admin-model.md §4.1` + `adm-1-privacy-promise-checklist.md`, ADM-1 用户隐私承诺页 3 条文案锁:

1. **Admin 是平台运维, 不是协作者; admin 永远不会出现在你的频道、DM 或团队列表里。**
2. **Admin 看不到消息 / 文件 / artifact 内容; 即使在调查问题, admin 看到的也是元数据 (谁、何时、多大), 不是你写了什么。**
3. **Admin 能看的是元数据 (账号状态、配额、错误码、操作时间戳), 任何越界 (impersonate / 覆写 / 删除) 都会以系统消息形式立刻通知你, 不可关闭。**

---

## 4. 留账 — Phase 2 退出 gate **未** 100% 关闭的 3 项

诚实留账, 跟着 Phase 4 一起补:

- 🟡 **G2.4 demo 截屏 5 张 partial 2/5 签** — #1 Welcome 第一眼 + #5 CTA 按钮 已签; #2 团队感知左栏 (等 AL-1b busy/idle 状态) + #3 邀请 inbox name 渲染 + #4 quick action 错误态 (等 RT-0 第一批 spec 含 agent fixture + 409 mock) — 三项随 Phase 4 BPP 补签到 5/5。
- 🟡 **busy / idle 三态显示** — Phase 2 只承诺 online / offline + error 三态; busy / idle 跟 Phase 4 BPP 同期 (R3-5 砍出决议)。
- 🟡 **ADM-1 隐私承诺页 实施** — 文案 3 条已锁 (本文 §3), 页面渲染 + drift 测试在 ADM-0.3 merged 后启动。

---

## 5. 下一阶段 (placeholder)

- **Phase 3**: 工作区 (workspace_files / artifacts) + 跨频道引用 — 从"聊"到"产出"
- **Phase 4**: BPP (插件协议) 完整化 — agent 真上线, busy / idle 真三态, 离线检测真 push
- 详细路线图见 `docs/implementation/PROGRESS.md`

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v0 草稿, 待 ADM-0.3 + RT-0 双 merged 后 team-lead 发布 |

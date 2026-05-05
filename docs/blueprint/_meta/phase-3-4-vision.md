# Phase 3 / Phase 4 立场 vision (草稿)

> **状态**: v0 草稿 (野马, 2026-04-28) — R4 review anchor, 不是详细计划; Phase 3 启动时再细化为 milestone 级 execution-plan
> **配套**: `r3-decisions.md` (#216, R3-4 / R3-5 / R3-7 三条 Phase 4 同期) + `../../implementation/00-foundation/phase-2-exit-summary.md` (#225) §5 placeholder 实化 + `g2.4-unblock-path.md` (#232) §2 里程碑表延伸
> **目的**: Phase 2 退出 gate 联签时给业主 / stakeholder 看 Phase 3+4 主线方向, 不承诺细节字面

---

## 1. Phase 3 主线 — "agent 真上线"

**主旨**: BPP 协议骨架落地 + agent runtime 接管, 业主创建的 agent 不再是数据库行 + 假状态, 而是真进程跑插件协议。

| 主题 | 蓝图锚 | 业主感知预期 |
|------|-------|-------------|
| BPP 协议骨架 | `plugin-protocol.md` + `host-bridge.md` §1.3 ("装时轻, 用时问") | agent 创建后真"上线" — 不是数据库标 online 假装 |
| agent runtime 接管 | `agent-lifecycle.md` §2.1 ("默认路径一键 onboarding") | 业主不需要装 SDK, host-bridge 自动跑插件 |
| frame schema = `/ws` push (R3-4 锁 byte-identical) | `realtime.md` §2.3 + R3-4 决议 | 邀请通知 / system message 实时, ≤ 3s (Phase 2 ⑤ 业主感知达成) |
| 离线检测 system DM | `concept-model.md` §4.1 | agent 进程死 → 业主收 system DM "你的 agent {name} 离线了" |

---

## 2. Phase 4 主线 — "三态完整 + 隐私页 + 配置热更新 + 退役"

**主旨**: agent 状态机闭环 + admin/user 边界 UI 落地 + 运维感强化。

| 主题 | 蓝图锚 | R3 看板 | 业主感知预期 |
|------|-------|---------|-------------|
| **AL-1b** busy/idle 三态 (R3-5 砍出 Phase 2 决议) | `agent-lifecycle.md` §2.3 | R3-5 | sidebar 看到 agent "正在熟悉环境" / "空闲" / busy 字面, 不再"online/offline" 二态糊弄 |
| **ADM-1** 用户隐私承诺页实施 (R3-7 锁) | `admin-model.md §4.1` 3 条文案 + `adm-1-implementation-spec.md` (#228) | R3-7 | 设置页"隐私"tab 顶部 3 条承诺 + 8 行 ✅/❌ 表格 (gray/红/amber 三色锁) |
| **AL-2** agent 配置 SSOT + 热更新 | `agent-lifecycle.md` §2.4 (Phase 4 加节) | (新增, R4 锁) | 业主改 agent 配置 (subject / 权限 / 工具) → ≤ 5s 生效, 不需重启 |
| **AL-3** presence 完整版 (含跨 org / 多 device) | `realtime.md` §3 (Phase 4 加节) | (新增, R4 锁) | 业主多设备登录 → presence 一致, 不闪 (与 Phase 3 离线检测配套) |
| **AL-4** agent 退役 (delete + 数据保留) | `agent-lifecycle.md` §2.5 (Phase 4 加节) | (新增, R4 锁) | 业主删 agent → 历史消息保留 (system kind + tombstone), 不留 raw UUID 引用 |

---

## 3. 业主感知预期 5 条 (Phase 2 公告外延)

跟 PR #225 Phase 2 公告 §2 5 条配套, Phase 3+4 加 5 条:

| # | 你看到什么 | 哪个 milestone |
|---|----------|---------------|
| ⑥ agent 创建后真上线 (host-bridge 启进程, 不是数据库 fake) | Phase 3 BPP |
| ⑦ agent 状态从二态升三态: busy/idle/error 字面准确 | Phase 4 AL-1b |
| ⑧ 设置页 "隐私" tab 顶部 3 条承诺 + 8 行 ✅/❌ 表格 (字面 1:1 锁) | Phase 4 ADM-1 |
| ⑨ agent 离线时收 system DM 通知 (不靠你刷新发现) | Phase 3 离线检测 |
| ⑩ 改 agent 配置 ≤ 5s 生效 (热更新, 不重启) | Phase 4 AL-2 |

---

## 4. 留账映射 — Phase 2 留 4 项 → Phase 3/4 落地

| Phase 2 留账 | 触发 milestone | 解锁后果 |
|-------------|---------------|---------|
| 业主感知 ⑤ 邀请 ≤ 3s | Phase 3 BPP / 早期 RT-0 server | PR #225 §2 ⑤ 锁 + G2.4 #3+#4 解 → 4/6 截屏 → Phase 2 退出 gate 联签条件达成 |
| G2.4 #2 sidebar 团队感知 | Phase 4 AL-1b | G2.4 5/6 → 6/6 全签 |
| G2.4 #6 ADM-0 立场 demo | Phase 4 ADM-1 | G2.4 → 6/6 全签 + ADM-1 闸 4 demo signoff |
| busy/idle 三态 | Phase 4 AL-1b | sidebar 业主感知 ⑦ 达成 |

---

## 5. 不在本 vision 范围

- ❌ multi-org admin / 跨 org 协作 — v1+
- ❌ artifact / workspace 完整 (Phase 3 后期议题, 不在 vision 主线)
- ❌ 国际化 / en 翻译 — v1
- ❌ canvas vision 落地路径 — `canvas-vision.md` 单独节, R5 起

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v0 草稿, Phase 3 BPP / Phase 4 AL-1b+ADM-1+AL-2+AL-3+AL-4 主线 + 业主感知 5 条外延 + 留账映射 |

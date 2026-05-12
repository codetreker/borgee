# Phase 3 / Phase 4 方向草稿

> **状态**: v0 草稿 (产品评审负责人, 2026-04-28) — R4 评审时作参考, 不是详细计划; Phase 3 启动时再细化为 milestone 级 execution-plan
> **配套**: `r3-decisions.md` (#216, R3-4 / R3-5 / R3-7 三条 Phase 4 同期) + `../../implementation/00-foundation/phase-2-exit-summary.md` (#225) §5 占位段落落实 + `g2.4-unblock-path.md` (#232) §2 里程碑表延伸
> **目的**: Phase 2 退出关卡联签时给业主 / 利益相关方看 Phase 3+4 主线方向, 不承诺具体细节字面

---

## 1. Phase 3 主线 — "agent 真上线"

**主旨**: BPP 协议骨架落地 + agent runtime 接管, 业主创建的 agent 不再是数据库行 + 假状态, 而是真实进程运行插件协议。

| 主题 | 蓝图出处 | 业主感知预期 |
|------|-------|-------------|
| BPP 协议骨架 | `plugin-protocol.md` + `host-bridge.md` §1.3 ("装时轻, 用时问") | agent 创建后真"上线" — 不是数据库标 online 假装 |
| agent runtime 接管 | `agent-lifecycle.md` §2.1 ("默认路径一键 onboarding") | 业主不需要装 SDK, host-bridge 自动运行插件 |
| frame schema = `/ws` push (R3-4 要求逐字节一致) | `realtime.md` §2.3 + R3-4 决议 | 邀请通知 / 系统消息实时, ≤ 3s (Phase 2 ⑤ 业主感知达成) |
| 离线检测 系统私信 | `concept-model.md` §4.1 | agent 进程死 → 业主收系统私信 "你的 agent {name} 离线了" |

---

## 2. Phase 4 主线 — "三态完整 + 隐私页 + 配置热更新 + 退役"

**主旨**: agent 状态机闭环 + admin/user 边界 UI 落地 + 运维体验强化。

| 主题 | 蓝图出处 | R3 决议 ID | 业主感知预期 |
|------|-------|---------|-------------|
| **AL-1b** busy/idle 三态 (R3-5 推迟出 Phase 2 决议) | `agent-lifecycle.md` §2.3 | R3-5 | 侧边栏看到 agent "正在熟悉环境" / "空闲" / busy 字面, 不再用 "online/offline" 二态糊弄 |
| **ADM-1** 用户隐私承诺页实施 (R3-7 决议) | `admin-model.md §4.1` 3 条文案 + `adm-1-implementation-spec.md` (#228) | R3-7 | 设置页"隐私"tab 顶部 3 条承诺 + 8 行 ✅/❌ 表格 (灰/红/橙 三色固定) |
| **AL-2** agent 配置单一真值来源 + 热更新 | `agent-lifecycle.md` §2.4 (Phase 4 加节) | (新增, R4 决议) | 业主改 agent 配置 (角色 / 权限 / 工具) → ≤ 5s 生效, 不需重启 |
| **AL-3** presence 完整版 (含跨 org / 多设备) | `realtime.md` §3 (Phase 4 加节) | (新增, R4 决议) | 业主多设备登录 → presence 一致, 不闪 (与 Phase 3 离线检测配套) |
| **AL-4** agent 退役 (删除 + 数据保留) | `agent-lifecycle.md` §2.5 (Phase 4 加节) | (新增, R4 决议) | 业主删 agent → 历史消息保留 (系统类型 + 墓碑标记), 不留裸 UUID 引用 |

---

## 3. 业主感知预期 5 条 (Phase 2 公告延伸)

跟 PR #225 Phase 2 公告 §2 5 条配套, Phase 3+4 增加 5 条:

| # | 你看到什么 | 哪个 milestone |
|---|----------|---------------|
| ⑥ agent 创建后真上线 (host-bridge 启进程, 不是数据库 假装) | Phase 3 BPP |
| ⑦ agent 状态从二态升三态: busy/idle/error 字面准确 | Phase 4 AL-1b |
| ⑧ 设置页 "隐私" tab 顶部 3 条承诺 + 8 行 ✅/❌ 表格 (字面 1:1 一致) | Phase 4 ADM-1 |
| ⑨ agent 离线时收系统私信通知 (不靠你刷新发现) | Phase 3 离线检测 |
| ⑩ 改 agent 配置 ≤ 5s 生效 (热更新, 不重启) | Phase 4 AL-2 |

---

## 4. 遗留项映射 — Phase 2 留 4 项 → Phase 3/4 落地

| Phase 2 遗留项 | 触发 milestone | 解锁后果 |
|-------------|---------------|---------|
| 业主感知 ⑤ 邀请 ≤ 3s | Phase 3 BPP / 早期 RT-0 server | PR #225 §2 ⑤ 锁定 + G2.4 #3+#4 解 → 4/6 截屏 → Phase 2 退出关卡联签条件达成 |
| G2.4 #2 侧边栏团队感知 | Phase 4 AL-1b | G2.4 5/6 → 6/6 全签 |
| G2.4 #6 ADM-0 原则 demo | Phase 4 ADM-1 | G2.4 → 6/6 全签 + ADM-1 关卡 4 demo 签字 |
| busy/idle 三态 | Phase 4 AL-1b | 侧边栏业主感知 ⑦ 达成 |

---

## 5. 不在本草稿范围

- ❌ 多 org admin / 跨 org 协作 — v1+
- ❌ artifact / workspace 完整 (Phase 3 后期议题, 不在主线)
- ❌ 国际化 / 英文翻译 — v1
- ❌ canvas 方向落地路径 — `canvas-vision.md` 单独章节, R5 起

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 产品评审负责人 | v0 草稿, Phase 3 BPP / Phase 4 AL-1b+ADM-1+AL-2+AL-3+AL-4 主线 + 业主感知 5 条延伸 + 遗留项映射 |

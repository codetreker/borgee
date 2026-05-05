# Phase 3 启动 trigger 条件

> **状态**: v0 (野马, 2026-04-28) — Phase 2 退出 gate 关闭后 Phase 3 启动条件清单
> **配套**: PR #234 vision (Phase 3 主线 BPP 骨架) + PR #233 立场签字 + PR #225 公告

---

## 1. Phase 2 工件齐 — 4 项硬条件

| # | 工件 | 状态 | PR / Doc |
|---|------|------|---------|
| ① | **G2.audit 全过** (Phase 2 跨 milestone codedebt audit, 同 g1-audit.md 模式) | ⏳ 烈马 owner, RT-0 merged 后跑 | (TBD) |
| ② | **野马 stance signoff** (Phase 2 立场 4/5 SIGNED + ADM-0 demo 待补) | ✅ | PR #233 |
| ③ | **Phase 2 公告 merged** (业主感知 5 条 + 隐私承诺 3 条文案锁) | ⏳ hold, 等 RT-0 + 飞马 #226/#227 联合 | PR #225 |
| ④ | **G2.4 ≥ 4/6 截屏 ✅** (#1+#3+#4+#5 联签条件) | 🟡 当前 2/6, 等 RT-0 → 4/6 | PR #213 + #232 |

**Phase 3 启动闭合**: ① + ② + ③ + ④ 全 ✅, 等同 RT-0 server merged 后 cascade 解锁 (RT-0 → ④ 4/6 → ① audit → ③ 公告 merged → Phase 3 启动)。

---

## 2. Phase 3 第一周派活预备

| 派活 | 主旨 | 蓝图锚 | owner |
|------|------|-------|-------|
| **BPP-1** 协议骨架 (frame schema + handshake + ping) | 与 R3-4 `/ws` push frame byte-identical, host-bridge 启进程时 BPP 替换 stub | `plugin-protocol.md` + `realtime.md` §2.3 + R3-4 | 战马A |
| **BPP-1 review checklist** (飞马预备) | frame schema 不变量 + handshake 状态机 + 重连退避策略反查 | `plugin-protocol.md` + r3-decisions.md R3-4 | 飞马 |
| **BPP-1 acceptance 模板** (烈马预备) | regression-registry.md 加 PHASE3-BPP1-001..N 槽位 + `internal/bpp/` package 接受 testdata | `host-bridge.md` §1.3 ("装时轻, 用时问") | 烈马 |
| **BPP-1 业主感知反查** (野马) | 业主感知 ⑥ "agent 创建后真上线" 立场反查表 v0 (锚 PR #234 §1) | `phase-3-4-vision.md` §1 + §3 ⑥ | 野马 |

---

## 3. 不在本 doc 范围

- ❌ Phase 3 完整 milestone 列表 — 待 G2.audit 后细化为 execution-plan
- ❌ Phase 4 启动条件 — 单独 doc (Phase 3 收尾时落)

---

## 4. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v0, 4 项 trigger 条件 + 第一周 4 派活预备 |

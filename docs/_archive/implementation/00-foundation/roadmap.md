# Roadmap — 11 模块缩略图

> **5 秒看完版本**: 从 [`../../current/`](../../current/) 到 [`../../blueprint/`](../../blueprint/) 的模块一览。
> **完整执行流程**: 见 [`execution-plan.md`](execution-plan.md) (Phase 边界 + 退出 gate + 4 道闸门)。
> **写法规范**: 见 [`how-to-write-milestone.md`](how-to-write-milestone.md)。
>
> 阶段: ⚡ v0 (无外部用户, 破坏式升级) → 🛡️ v1 (第一个用户上线, 严格模式)。切换 checklist 见 [`../README.md`](../README.md)。

## 模块依赖序

按"前置先做"排序的模块列表。**Phase 内的具体执行顺序见 execution-plan**, 不在本表锁死。

| # | 模块 | 蓝图 | 实施 | 所属 Phase | 状态 | Milestones |
|---|------|------|------|-----------|------|------------|
| 0 | 基建: schema_migrations 框架 | [data-layer §3.2](../../blueprint/current/data-layer.md) | [→](execution-plan.md#phase-0--基建闭环) | Phase 0 | TODO | INFRA-1 |
| 1 | concept-model | [→](../../blueprint/current/concept-model.md) | [→](../modules/concept-model.md) | Phase 1 + Phase 2 | TODO | CM-1 / CM-3 / CM-4 |
| 2 | channel-model | [→](../../blueprint/current/channel-model.md) | [→](../modules/channel-model.md) | Phase 3 | TODO | CHN-1 ~ CHN-4 |
| 3 | canvas-vision | [→](../../blueprint/current/canvas-vision.md) | [→](../modules/canvas-vision.md) | Phase 3 | TODO | CV-1 ~ CV-4 |
| 4 | agent-lifecycle | [→](../../blueprint/current/agent-lifecycle.md) | [→](../modules/agent-lifecycle.md) | Phase 4 | TODO | AL-1 ~ AL-4 |
| 5 | plugin-protocol (BPP) | [→](../../blueprint/current/plugin-protocol.md) | [→](../modules/plugin-protocol.md) | Phase 4 | TODO | BPP-1 ~ BPP-4 |
| 6 | host-bridge (Borgee Helper) | [→](../../blueprint/current/host-bridge.md) | [→](../modules/host-bridge.md) | Phase 4 | TODO | HB-1 ~ HB-4 |
| 7 | realtime | [→](../../blueprint/current/realtime.md) | [→](../modules/realtime.md) | Phase 4 | TODO | RT-1 ~ RT-3 |
| 8 | auth-permissions | [→](../../blueprint/current/auth-permissions.md) | [→](../modules/auth-permissions.md) | Phase 1+ (AP-0) / Phase 4 | TODO | AP-0 ~ AP-4 |
| 9 | admin-model | [→](../../blueprint/current/admin-model.md) | [→](../modules/admin-model.md) | Phase 4 | TODO | ADM-1 ~ ADM-3 |
| 10 | data-layer (剩余) | [→](../../blueprint/current/data-layer.md) | [→](../modules/data-layer.md) | Phase 4 | TODO | DL-1 ~ DL-3 |
| 11 | client-shape | [→](../../blueprint/current/client-shape.md) | [→](../modules/client-shape.md) | Phase 4 | TODO | CS-1 ~ CS-3 |

## 首波 demo 路径 (v0)

> v0 阶段优先级: **价值闭环驱动**, 不是模块依赖。让团队 (建军/飞马/野马) 尽早看到产品形态。

```
Phase 0  INFRA-1                        → 基建就绪, 可以开始落业务
Phase 1  CM-1                           → 数据层 org 概念落地
                CM-1.4 admin 调试页 (visibility checkpoint, 不是 acceptance)
                                        → 团队第一次"看到 org 概念"
         CM-3                           → 资源归属 org_id 直查
Phase 2  CM-4 ⭐ agent 同事感首秀        → 邀请审批 + 离线 fallback + 节流
                                        → 第一次产品 demo + 野马签字 + 关键截屏
Phase 3  channel-model M-1              → channel 协作场骨架
         canvas-vision M-1              → workspace + artifact 第一刀
                                        → 第二次产品 demo
```

每个 ⭐ 标志性 milestone 关闭前**必须**: 野马跑 demo + 签字 + 留 3-5 张关键截屏 (闸 4)。

## 当前进度

```
✅ 蓝图 (blueprint/)         11 篇全部完成
✅ 现状 (current/)           7 篇 audit 完成
🔄 实施 (implementation/)    源头文件 + 模板 + 11 模块大纲
⏳ 代码 (PRs)                未开始
```

## 进度可见性

| 谁 | 看什么 | 频率 |
|----|--------|------|
| 建军 | Phase 退出 gate 燃尽 + 阶段切换 checklist | 周 |
| 飞马 | PR 队列 + acceptance 四选一 + grep 锚点 (闸 2) | 日 |
| 野马 | 标志性 milestone demo 签字 + 截屏 (闸 4) | 触发时 |

> v0 阶段不公开 changelog (内部使用); v1 阶段开始把 changelog 公开。

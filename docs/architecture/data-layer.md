# Implementation · Data Layer (剩余)

> 蓝图: [`../../blueprint/current/data-layer.md`](../../blueprint/current/data-layer.md)
> 现状: 大部分总账已落 (随各模块进表), 剩 events 双流 / 阈值哨 / 分布式 ready 接口抽象
> 阶段: ⚡ v0 + 部分 v1 准备
> 所属 Phase: Phase 4 (DL-2/DL-3 部分作 v0→v1 切换 checklist 准备)

> 注: INFRA-1 (schema_migrations 框架) **属于 Phase 0**, 不在本模块拆出, 见 [`execution-plan.md`](../00-foundation/execution-plan.md) Phase 0。

## 1. 现状 → 目标 概览

**现状**: SQLite 一张表全, 缺 events 双流, 缺阈值哨, 接口未抽象 (硬编码 sqlite)。
**目标**: blueprint §3 + §4 + §5 — events 双流 (live + replay), 阈值哨 (容量监控), 分布式 ready 三层 (A 必修 / B 可换 / C 必重写)。

## 2. Milestones

### DL-1: 接口抽象 (A 必修 5 条)

- **目标**: blueprint §4.A — ID 协议 lock-in, 不留坑给 v1 分布式。
- **Owner**: 飞马 (主, 接口设计) / 战马 / 野马 / 烈马
- **范围**: ULID 全表; opaque cursor; type ID string 抽象; events 全 frame 化; 所有时间戳 UTC int64
- **预估**: ⚡ v0 1 周
- **Acceptance**: 数据契约 (interface 文件) + 蓝图行为对照 §4.A

### DL-2: events 双流 + 90 天 retention

- **目标**: blueprint §3.1 — events 双流 (live frame + replay table), retention 90 天。
- **Owner**: 飞马 / 战马 / 野马 / 烈马
- **范围**: `events_live` (in-process hub) + `events_replay` (SQL); 90 天 cron 清理; global events 必落清单 (蓝图 §3.4)
- **预估**: ⚡ v0 1-2 周
- **Acceptance**: 行为不变量 (replay 一致 live, 集成测试) + 数据契约 (必落清单)

### DL-3: 阈值哨 + 监控

- **目标**: blueprint §5 — 阈值哨, 凭指标切不凭感觉切。
- **Owner**: 飞马 / 战马 / 野马 / 烈马
- **范围**: 表行数 / 查询延迟 / 连接数 P95 监控; 阈值阈触发 alert (内部 admin SPA 显示)
- **依赖**: ADM-1
- **预估**: ⚡ v0 1 周

### DL-4: server-side services (plugin manifest API + Web Push gateway) ⚠️ must-fix 收口

> 飞马 review 新发现: HB-1 / CS-3 都建在不存在的后端上 — server 端 plugin manifest 与 Web Push gateway 没归任何模块, 必须独立 milestone 收口, 否则两个 client 模块各自假设。

- **目标**: 收口 server 端 plugin registry + Web Push gateway, 给 HB-1 / CS-3 提供必要的后端服务。
- **Owner**: 飞马 (主, 服务边界设计) / 战马 / 野马 / 烈马
- **范围**:
  - **plugin manifest API**: `GET /api/v1/plugins/manifest/<id>` 返回 (url / sha256 / signature / capabilities); 服务端单一注册表 (v0 写死 config 文件即可, registry v1+ 才需要)
  - **Web Push gateway**: 服务端 VAPID 签名 + push subscription 入表 (`push_subscriptions(user_id, endpoint, keys, created_at)`) + 通知发送 worker
- **不在范围**: plugin 自动更新 ❌; 多源 registry ❌; push notification 内容渲染 (CS-3 负责); FCM/APNs 直连 ❌ (Web Push 标准协议即可)
- **依赖**: 无 (可跟 BPP-1 并行)
- **预估**: ⚡ v0 1 周
- **Acceptance**:
  - 数据契约: manifest API schema + push_subscriptions 表
  - 行为不变量 4.1: 签名校验失败的 manifest → daemon 拒绝安装 (单测)
  - E2E (与 CS-3 配合): 用户在 Mobile PWA 订阅 → 服务端 push → 用户收到

## 3. 不在 data-layer 范围

- backup / restore (v0→v1 切换 checklist 单独做)
- 分布式实现 (B 可换 / C 必重写) → v1+ 业务推动时再做

## 4. Blueprint 反查表

| Milestone | §X.Y | 立场一句话 |
|-----------|------|-----------|
| DL-1 | data-layer §4.A | A 必修 5 条接口锁定 |
| DL-2 | data-layer §3.1 + §3.4 | events 双流 + 必落清单 |
| DL-3 | data-layer §5 | 凭指标切, 不凭感觉切 |
| DL-4 | (实施补缺口) | server 端 plugin manifest + Web Push gateway, 给 HB-1/CS-3 提供后端 |

# Borgee Implementation — 从 current 到 blueprint

> 这一目录是**实施层** —— 把 [`../current/`](../current/) 的代码现状一步一步推到 [`../blueprint/`](../blueprint/) 的目标态。
> 形式: milestone 列表 + per-milestone PR + acceptance spec。
>
> **进度打勾**: [`PROGRESS.md`](PROGRESS.md) (所有 milestone 状态一览)
> **路径可见性入口**: [`00-foundation/roadmap.md`](00-foundation/roadmap.md) (5 秒看完)
> **写 milestone 文档的规范**: [`00-foundation/how-to-write-milestone.md`](00-foundation/how-to-write-milestone.md)
> **第一个模块样板**: [`modules/concept-model.md`](modules/concept-model.md)

---

## ⚠️ 阶段策略 (核心约束)

Borgee 当前**无外部用户**。这给了实施巨大的简化空间——但需要明确**何时切换**到严格模式。

### v0 阶段:无外部用户 (现在 → 第一个非内部用户上线)

**核心方针:破坏式升级,删库重建,不做兼容期**

| 维度 | v0 策略 |
|------|---------|
| 数据迁移 | ❌ 不做 backfill 脚本 |
| 协议演进 | ❌ 不做协议版本协商,直接换 |
| 客户端兼容 | ❌ 不做老客户端兼容,直接发新版 |
| Schema 改动 | ✅ 每次改 schema 都允许"删库重建" |
| ULID 改 | ✅ 直接全表 ULID,不留 INT |
| Cursor 形态 | ✅ 直接换 opaque string,不留 INT cursor |
| Events 拆双流 | ✅ 直接改,旧 events 表扔 |
| BPP 协议 | ✅ 直接换协议,plugin 同步发版 |
| 回滚 | ❌ 不写回滚脚本,出问题删库重来 |

**唯一硬规则**: 每个 milestone 之后 main 分支能跑 + 有 acceptance spec 验证。

**为什么允许这么激进**:
- 没用户 = 数据无价值 = 删了无代价
- 不做兼容期 = 开发速度 ×3 = 提早跑通 blueprint
- 把"建一个能用的东西"放在"建一个能演进的东西"前面

### v1 阶段:第一个外部用户上线后

**Trigger**: 第一个非建军/飞马/野马的用户被邀请进入 prod 环境的那一刻。

**切到严格模式** (野马原始版本的增量边界):

| 维度 | v1 策略 |
|------|---------|
| 数据迁移 | ✅ 必须 forward-only + backfill 脚本 |
| 协议演进 | ✅ Cursor: protocol_version header 兼容期 |
| 客户端兼容 | ✅ Public API 永远兼容期;internal (BPP/Helper) 可同步升级 |
| ULID | ⚠️ 永久混用 (旧表 INT, 新表 ULID), 用 `type ID string` 抽象 |
| Events | ✅ 表拆增量 (旧 events 留 view 兼容) |
| BPP | ✅ 内部灰度大改造, 分批 plugin 重连, 禁止全站集体掉线 |
| 回滚 | ✅ 备份 + 不可逆 forward-only, 出问题靠 backup restore |
| 终端用户感知 | ✅ 永远不出现"全站停服公告",零强制升级 |

**底线**: 第一个外部用户上线之后 **永远不删库**, **永远不破坏 public 协议**。

### 切换 checklist (v0 → v1)

第一个外部用户上线**前**必须完成的事:

- [ ] schema_migrations 框架已建立 (forward-only)
- [ ] backup / restore 流程已演练
- [ ] Public API 版本协商机制 (`protocol_version` header) 已就位
- [ ] BPP 灰度发版机制已就位 (plugin 端先发, server 端后)
- [ ] 监控 + 阈值哨已就位 (data-layer §5)
- [ ] **v0 代码债 audit 表已结清** (见下)

未到 checklist 完成 → 可继续 v0 激进模式;
完成后 → 邀请第一个用户,**同步切换到 v1 模式**,所有人遵守。

### v0 代码债 audit (每 milestone 关闭时必更)

> 每个 v0 阶段做的"破坏式选择"都登记在这, v1 切换前逐条结清。
> **每完成一个 v0 milestone, 必须更新一行**, 否则 Phase gate 不算通过 (见 [`execution-plan.md`](00-foundation/execution-plan.md))。

| 改动 | v0 做法 | v1 切回要补的事 | 关联 Phase / Milestone | 状态 |
|------|---------|----------------|-----------------------|------|
| organizations 表 | v0 删库重建 (无现网数据); CM-1.1 走 forward-only `schema_migrations` v=2 直接 CREATE TABLE | v1 切回不需"删库" — 已经在 forward-only 引擎内, 维持 v=2 即可; 真要重命名/拆分时新写 v=N 迁移 + backfill, 不允许再删库 | Phase 1 / CM-1.1 | DONE |
| users.org_id NOT NULL DEFAULT '' | CM-1.1 ALTER TABLE 加列默认空串占位; CM-1.2 在应用层 `CreateOrgForUser` 兜底 `org_id != ''` | (a) 收紧列约束: 加 `CHECK (org_id != '')` migration; (b) backfill 现网空串行 (扫 users WHERE org_id='' → 为每个用户建 personal org → UPDATE); (c) 收紧后 register 路径失败要回滚 user (见 CM-1.2 行 (b)) | Phase 1 / CM-1.1 + CM-1.2 | DONE |
| events 表 | 直接换 schema | 旧 events 留 view 兼容 | (待 events 模块时填) | — |
| BPP 协议 | 直接换 | plugin 灰度发版 + protocol_version 协商 | (待 BPP 模块时填) | — |
| ULID | 全表 ULID, 删 INT | 永久混用 + `type ID string` 抽象 (野马原始版本立场) | (待 ID 模块时填) | — |
| Cursor 形态 | opaque string 直换 | 兼容期 INT cursor 解析 | (待 cursor 模块时填) | — |
| schema_migrations 框架 | forward-only, 无 Down 反向脚本 | 不需补 Down — v1 切换走 backup/restore 策略 (checklist §"backup / restore 流程已演练") | Phase 0 / INFRA-1a | DONE |
| 默认权限 human=`(*,*)` | 注册写一行通配, UI bundle 未上 | AP-2 落地后改写注册路径: 写默认 bundle (Messaging/Workspace 等) 而非 `(*, *)`; 已存在用户走数据迁移脚本把 `(*, *)` 拆成对应 bundle 集合 (按业务历史用过的 capability 推断) | Phase 1 / AP-0 | TODO |
| `auth.RequirePermission` 通配匹配 | 短路 `(*, *)` 行为全权 | AP-2 拿掉 `(*, *)` 默认后, 此分支自然消亡; 同时回写 unit test (现在加是为了测 AP-0 acceptance) | Phase 1 / AP-0 | TODO |
| main 已知 flaky test (`internal/server`) | 容忍 (CI rerun 兜底) | 修两处 goroutine leak: rateLimiter.cleanup ticker 未 Stop + Hub.StartHeartbeat 5min chan receive 退出路径; 单测 race + leak detector 必须 0 报警 | Phase 0 audit / 来自 #170 CI 偶发 fail | TODO |
| 注册自动建 org (CM-1.2) | register/admin-create-user 各自调用 `CreateOrgForUser` 在 app 层兜底 `org_id != ''`; 列上仍是 `NOT NULL DEFAULT ''` (CM-1.1 时用空串占位) | (a) v1 切真实多 org 模型时把 column constraint 收紧成 CHECK (`org_id != ''`) 并迁移现网空串行; (b) 注册路径若失败要回滚 user (现在用顺序 CreateUser→CreateOrgForUser, 失败留 user 但无 org_id, 接受 v0 边角); (c) 拆"个人 org"概念时改成显式 join 表 | Phase 1 / CM-1.2 | TODO |
| Migrate() 内嵌 forward-only 引擎 | `store.Migrate()` 末尾跑 `migrations.Default(db).Run(0)`, cmd/migrate 的 engine.Run 与之重复但幂等 | createSchema 拆成 v0 baseline migration 之后, `store.Migrate()` 整体退役, 只剩引擎一条路径 | Phase 1 / CM-1.2 | TODO |
| agent_invitations 状态机 | `state` 列直接 TEXT enum + CHECK ('pending'/'approved'/'rejected'/'expired'); 状态机 helper (`store.AgentInvitation.Transition`) 只有 3 条边 (pending → 三终态), 终态无出边 | 若 v1 出现"重新邀请"或"再次发送"语义, 拆 lookup 表 (state_id INT FK → invitation_states) 并允许扩展状态; 当前 enum 直接落 string 在 v0 接受 | Phase 2 / CM-4.0 | TODO |
| agent_invitations PATCH approved 非事务化 | CM-4.1 `handlePatch` 先 `UpdateAgentInvitationState(approved)` 再 `AddChannelMember`, 两步**不在同事务**; AddChannelMember 失败只 `Logger.Error` 不回滚 invitation state — 持久化决定是 source of truth, 等 sweep reconcile 重做 join | (a) 整段 tx wrap (`db.Transaction(func(tx) { update + addMember })`) **或** (b) 留非事务化 + CM-4.3 sweep 拿 approved 但无 channel_member 的行重试加成员 (saga 补偿); v1 二选一并补 unit test 覆盖 "approved 写入但 join 失败" 路径 | Phase 2 / CM-4.1 | TODO |
| ... | ... | ... | ... | ... |

**填表规则**:
- "改动" — 一句话写清做了什么破坏式动作
- "v1 切回要补的事" — 越具体越好, 不要写"加迁移", 写"forward-only 0042_users_org_id.sql, 步骤: ..."
- "状态" — TODO / IN PROGRESS / DONE

---

## 五条实施规则 (v0 / v1 通用)

继承 11 轮讨论时飞马野马提出的 form:

1. **PR ≤ 3 天**, **Milestone ≤ 2 周** —— 控制反馈循环
2. **可验证四选一**: e2e 断言 / 蓝图行为对照 / 数据契约 / 行为不变量 —— 每 PR 至少一种; **标志性 milestone 强制 4.1+4.2 双挂**
3. **5 秒看完路径** —— [`execution-plan.md`](00-foundation/execution-plan.md) 是源头, [`roadmap.md`](00-foundation/roadmap.md) 是缩略图
4. **PR 描述强制**: `Blueprint: <模块> §X.Y` + `Touches:` + `Current 同步:` 三个区块齐 —— 让追溯无歧义
5. **Milestone 末必须可发版** —— 中间态用 feature flag 隐藏
6. **current 同步硬规则**: 每 PR 改 `internal/<module>/` 必须同步改 `docs/current/<module>/`, **CI lint 卡 merge** (无同步 = fail); 烈马在 PR review 阶段确认。
7. **跨模块 PR 协议**: PR `Touches` 含 ≥ 2 子系统时 (server+plugin+client) 必须先合 **接口契约 PR (≤ 300 行: schema + frame + interface)** 锁住, 再合实现 PR。一次 diff > 2000 行 reviewer 看不动 (战马 R2)。
8. **revert 优先**: v0 阶段 main 跑不起来 → 直接 `git revert` + push, 不写 schema 回滚。
9. **测试 + regression 防护硬规则** (闸 5):
   - 每 PR 合并前: 单元测试 (含分支文件 cyclomatic > 1 覆盖率 ≥ 80%, schema/migration PR 豁免, CI 强制) + 至少 1 条集成测试 (跨模块 happy path E2E) + seed 脚本 (`testdata/<milestone>/seed.sql`, v0 删库后一键复现)
   - 每 Phase gate 前: 全回归套件强制 **先 seed → 再 migration → 再 assert** (不 reuse 已建库, 烈马 R2); 已合并 milestone 的 4.1 acceptance 自动入册, 烈马维护清单
   - 不写测试 = 后续 milestone 改 schema 打穿前面 milestone, 发现不了
   - **UI 层 E2E** 缺自动化时, 以 4.2 关键截屏代替 (烈马 R2 nice-to-have)

---

## 团队分工 (4 角色)

| 代号 | 职责 | 主要输出 | 在闸门里 |
|------|------|---------|---------|
| **飞马** (team-lead + architect) | 架构决策 / PR review / 拆分 milestone / 守护立场 | 模块文档定版 / PR review approval / 闸 1+2 | 闸 1 模板自检 / 闸 2 grep 锚点 |
| **战马** (dev) | 实现 / 写代码 / 写单测 / 写迁移脚本 | PR 主体代码 + commit | — |
| **野马** (PM) | 产品立场把关 / 验收 demo / 写文档反查表立场一句话 | 标志性 milestone 签字 + 3-5 张关键截屏 | 闸 3 反查表立场 / 闸 4 签字+截屏 |
| **烈马** (QA) | acceptance spec 跑通 / E2E 自动化 / 行为不变量测试 | acceptance 跑通报告 + 测试代码 | 每 PR 的 acceptance 都由烈马验证 |

**分工原则**:
- **每个 milestone 必须挂 4 个 owner**: 飞马 (review) / 战马 (实现) / 野马 (立场) / 烈马 (验证)
- **每个 PR 必须挂 2 个 owner**: 战马 (作者) + 飞马 (review)。acceptance 由烈马跑过才允许 merge
- **标志性 milestone (⭐) 关闭前**: 野马必须签字 + 留 3-5 张关键截屏 (闸 4 强制, AI 团队不录视频)
- 任何角色看到立场漂移 / 偏离, 都有"打回"权 — 不是单点决策

详见各模块 milestone 表格的 "Owner" 列。

---

## 文档导航

| 文档 | 内容 |
|------|------|
| [`PROGRESS.md`](PROGRESS.md) | **进度打勾**: 所有 Phase / milestone / PR / gate 的状态 |
| [`00-foundation/execution-plan.md`](00-foundation/execution-plan.md) | **源头**: 5 Phase + 退出 gate + 4 道防偏离闸门 |
| [`00-foundation/roadmap.md`](00-foundation/roadmap.md) | execution-plan 缩略图 (按模块依赖排序) |
| [`00-foundation/how-to-write-milestone.md`](00-foundation/how-to-write-milestone.md) | milestone 模板 + acceptance 四选一 + 反查表规范 |
| [`modules/concept-model.md`](modules/concept-model.md) | concept-model 模块 (CM-1, CM-3, CM-4) |
| [`modules/<其它>.md`](modules/) | 各模块大纲, 与 [`../blueprint/`](../blueprint/) 一一对应 |

## 与 blueprint 的对应

```
blueprint/current/concept-model.md     ← 目标态: 应该是什么样
   │
   ▼
implementation/concept-model.md ← 实施: 一步一步怎么走到那
   │
   ▼
代码 PRs                         ← 每个 milestone 拆出来的 PR
```

每个 PR 描述里都强制带 `Blueprint: concept-model §X.Y` 锚点,让代码可以反查到产品立场。

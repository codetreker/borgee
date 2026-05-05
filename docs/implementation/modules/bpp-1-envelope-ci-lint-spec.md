# BPP-1 envelope CI lint — spec brief (烈马 acceptance 守门)

> 烈马 · 2026-04-28 · v0
> 蓝图: `docs/blueprint/plugin-protocol.md` §2.1 (控制面 6 帧) + §2.2 (数据面 3 帧)
> 占号 stub: PR **#274** (`docs(impl): BPP-1 envelope CI lint spec stub`)
> 配套 acceptance: `docs/qa/acceptance-templates/al-2a.md` (BPP frame 在 AL-2a 反向断言 count==0)
> 守门闸: G2.6 留账 (Phase 2 退出公告 §4) → BPP-1 PR 内含真 lint workflow 后升 ✅

## 1. 目标 — CI lint 必须保证什么

| # | 不变量 | 蓝图引 | 反向断言 (CI fail 条件) |
|---|---|---|---|
| ① | 9 帧 envelope 与 RT-0 #237 envelope **byte-identical** (type/op/ts/v/payload 5 字段, 序无关 schema 锁) | §2.1 + §2.2 + RT-0 注释锁 #237 | `internal/bpp/frame_schemas.go` 任一帧多/少字段 → CI fail |
| ② | 控制面 6 帧 (connect / agent_register / runtime_schema_advertise / agent_config_update / agent_disable / agent_enable / inbound_message) **方向锁 server→plugin** | §2.1 表 | direction tag 缺失或 `plugin→server` → CI fail |
| ③ | 数据面 3 帧 (heartbeat / 语义动作 / error_report) **方向锁 plugin→server** | §2.2 表 | direction tag 缺失或 `server→plugin` → CI fail |
| ④ | 帧名 grep 反向断言: 蓝图未列帧名 (e.g. `agent_promote`, `god_mode`) 出现在 frame_schemas.go → CI fail | §2 全表 | 白名单 9 帧之外的 OpName 常量 → fail |
| ⑤ | godoc 注释引 `BPP-1.*byte-identical.*RT-0` 字面 (count≥1, 与 #237 注释锁同模式) | RT-0 #237 godoc | `grep -c 'BPP-1.*byte-identical' internal/bpp/frame_schemas.go == 0` → fail |

## 2. CI workflow 路径锁

- **新增 job**: `.github/workflows/server.yml` 加 `bpp-envelope-lint` step
- **lint script**: `scripts/lint-bpp-envelope.sh` (≤30 行 bash, 调 `go test -run TestBPPEnvelope -v ./internal/bpp/...`)
- **reflect test**: `internal/bpp/frame_schemas_test.go` (新建) — 反射扫 9 帧 struct 字段, 跟蓝图 §2 表对账
- **schema 等价测**: `internal/bpp/schema_equivalence_test.go` (跟飞马 RT-1 #269 配套, 反射对比 BPP-1 envelope vs RT-0 #237 envelope 字段集合)

## 3. acceptance 验收 (8 项, 全可机器化)

| # | 验收项 | 实施方式 | Owner |
|---|---|---|---|
| 1 | `frame_schemas.go` 9 帧 OpName 常量声明齐 | unit (反射枚举) | 飞马 |
| 2 | 控制面 6 帧 direction == `Server→Plugin` | unit (struct tag) | 飞马 / 烈马 |
| 3 | 数据面 3 帧 direction == `Plugin→Server` | unit (struct tag) | 飞马 / 烈马 |
| 4 | 与 RT-0 envelope byte-identical 字段集合 | unit (反射对比 #237) | 飞马 |
| 5 | 帧名白名单反向 grep (任何非 9 帧 OpName 常量 → fail) | CI grep | 烈马 |
| 6 | godoc 注释引 `BPP-1.*byte-identical.*RT-0` count≥1 | CI grep | 飞马 |
| 7 | `.github/workflows/server.yml` 加 `bpp-envelope-lint` job, CI 跑过 | CI native | 飞马 |
| 8 | `acceptance-templates/al-2a.md` BPP frame 反向断言 (count==0) 跟 BPP-1 落地后翻 ✅ | drift | 烈马 |

## 4. 红线 (烈马 acceptance 守门)

- ❌ **不允许"先实现 BPP frame, lint 后补"** — lint job 必须跟 frame_schemas.go 同 PR 落, 否则 G2.6 留账行不能升 ✅
- ❌ **不允许字面跑过但 reflect test 缺**: grep `BPP-1.*byte-identical` 仅证 godoc 锁, **不证字段集合一致** — schema_equivalence_test.go 是硬条件
- ❌ **不允许跳过 §2 表外帧的反向断言**: 帧名白名单是闸 4 防漂移核心 (admin-model §1.1 立场同模式)

## 5. 后续动作 (BPP-1 真 PR 落地路线)

1. 飞马: `internal/bpp/frame_schemas.go` 9 帧 struct + OpName 常量 + godoc 注释锁
2. 飞马: `internal/bpp/frame_schemas_test.go` 反射枚举 + direction tag 断言
3. 飞马: `internal/bpp/schema_equivalence_test.go` 跟 RT-0 #237 envelope 字段集合对账
4. 飞马: `.github/workflows/server.yml` 加 `bpp-envelope-lint` job + `scripts/lint-bpp-envelope.sh`
5. 烈马: `acceptance-templates/al-2a.md` BPP frame 反向断言行翻 ✅, `regression-registry.md` 加 REG-BPP1-001..008 (8 acceptance → 8 reg 行)
6. 飞马 + 烈马联签 → G2.6 CI lint 留账行从 DEFERRED 升 SIGNED → Phase 2 关闭最后一项闭

## 6. 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-28 | 烈马 | v0 — 占号 #274 配套 spec brief, 8 acceptance + 4 红线 |

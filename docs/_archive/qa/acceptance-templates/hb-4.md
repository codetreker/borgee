# Acceptance Template — HB-4 ⭐ release gate + 信任五支柱可见

> 蓝图: `host-bridge.md` §1.5 (v1 release 硬指标 6 行) + §2 信任五支柱 (开源/签名/可审计日志/可吊销/限定能力)
> Spec: `docs/implementation/modules/hb-4-spec.md` (战马A v0 a02f1d6)
> Stance: `docs/qa/hb-4-stance-checklist.md` (战马A v0)
> 不需 content-lock — release gate docs + CI, 无 DOM 文案
> 拆 PR: **HB-4 整 milestone 一 PR** `feat/hb-4` 三段一次合
> Owner: 战马A (实施) / 飞马 review / 烈马 验收 (4.1) / 野马签字 (4.2 demo)

## 验收清单

### §1 HB-4.1 — release-gate.md doc ≥10 硬条件清单

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 release-gate.md ≥10 硬条件清单, 每项三元组 (蓝图 § 锚 + CI workflow path + assertion) | doc lint | 战马A / 烈马 | `docs/release/host-bridge-release-gate.md` ≥10 numbered rows; CI grep 列数 |
| 1.2 蓝图 §1.5 6 行硬指标 byte-identical 入清单 (启动 < 800ms / 崩溃率 < 0.1% / 签名 0% fail / audit schema lock / 撤销 < 100ms / 写类 IPC reject) | doc lint + table reflect | 战马A / 烈马 | `host-bridge-release-gate.md` §1 表 6 行 byte-identical 跟蓝图 §1.5 |
| 1.3 跨 milestone 反约束扩 4 项 (BPP-4 best-effort AST + BPP-5 reconnect AST + HB-3 grants AST + audit schema 跨五处) | doc + CI cross-ref | 战马A / 飞马 | §2 跨 milestone 锁链锚 + CI step path |

### §2 HB-4.2 — CI workflow release-gate.yml

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 release-gate.yml step 数 ≥ 10, 任一 fail → workflow red | CI yml + dry-run | 战马A / 烈马 | `.github/workflows/release-gate.yml` step count ≥10 + 每 step `run:` 含 fail path |
| 2.2 audit schema 跨四 milestone reflect lint (`actor/action/target/when/scope`) — HB-3/BPP-4 实施 ≥4 hit (HB-1/HB-2 Rust 待真实施补) | CI grep | 战马A / 飞马 / 烈马 | `release-gate.yml` step `audit-schema-cross-milestone` 跑 grep `"actor".*"action".*"target".*"when".*"scope"` 在 internal/{api,bpp}/ ≥4 hit |
| 2.3 AST scan 锁链跨三 milestone (BPP-4 + BPP-5 + HB-3 forbidden tokens) — release 前 0 hit verify | CI grep | 战马A / 烈马 | step `forbidden-tokens-cross-milestone` 跑三组 grep, 任一 hit → red |
| 2.4 数字单源锁 (BPP-4 30s heartbeat + HB-3 撤销 100ms 真测) | CI grep + go test | 战马A / 烈马 | step `numeric-singletons` 跑 grep `BPP_HEARTBEAT_TIMEOUT_SECONDS\s*=\s*30` ≥1 hit |
| 2.5 反约束 admin merge bypass / sign-off skip 0 hit | CI grep | 飞马 / 烈马 | step `no-bypass` 跑 `release.gate.*skip\|admin.*bypass\|--admin.*merge` count==0 |

### §3 HB-4.3 — closure (REG + PROGRESS + 野马签字 placeholder)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 REG-HB4-001..010 (≥10 行) 全 🟢 入 docs/qa/regression-registry.md | docs flip | 飞马 / 烈马 | `regression-registry.md` REG-HB4-* 10 行 |
| 3.2 PROGRESS HB-4 [x] 翻 + ⭐ 标志性 milestone 关闭 | docs flip | 飞马 | `PROGRESS.md` HB-4 行 `[x]` + 4.1 ✅ + 4.2 ⏸️ deferred 留账 |
| 3.3 野马 4.2 demo 签字 placeholder (3 张截屏锚, 实际截屏 release 前补) | doc placeholder | 野马 (主) | `docs/qa/signoffs/hb-4-yema-signoff.md` placeholder 字面要求 + 截屏路径锚 |

### §4 反向 grep / e2e 兜底 (跨 HB-4 反约束)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 立场 ① 不允许跳过 — `release.gate.*skip\|release.gate.*manual.*sign\|allow.*bypass` count==0 在 `.github/workflows/` + `docs/release/` | CI grep | 飞马 / 烈马 | release-gate.yml step `no-bypass` |
| 4.2 立场 ④ 4.1 vs 4.2 拆死 — `release.gate.*human.review\|release.gate.*demo.signoff` 在 release-gate.yml count==0 | CI grep | 飞马 / 烈马 | 同 step (4.2 截屏走野马 signoff doc, 不混入 yml) |
| 4.3 立场 ⑦ admin god-mode 不入 — `internal/api/admin*.go` 反向 grep `release.gate\|HB4` count==0 | CI grep | 飞马 / 烈马 | step `no-admin-godmode-release` |

## 边界 (跟其他 milestone 关系)

| Milestone | 关系 | 字面承袭 |
|---|---|---|
| HB-1 #491 spec | release gate 第 3 行 (签名 0% fail), audit schema 第 1 处 (Rust crate 真实施时补) | install-butler 双签校验 |
| HB-2 #491 spec | release gate 第 6 行 (写类 IPC 100% reject), audit schema 第 2 处 (Rust crate 真实施时补) | host-IPC 仅读 |
| HB-3 #504 | release gate 第 5 行 (撤销 < 100ms 真测), audit schema 第 4 处 (实施已就绪) | host_grants forward-only revoke |
| BPP-4 #499 | AST scan forbidden 第 1 处 + 30s heartbeat 数字单源 | dead_letter best-effort |
| BPP-5 #503 | AST scan forbidden 第 2 处 (reconnect) | reconnect handler best-effort |
| HB-4 (本) | **5 milestone 单源锁收口** — audit + AST + 数字 + admin 红线 跨 milestone 反 drift 真守门 | release gate 是 ⭐ 标志性 |

## 退出条件

- §1 doc (3) + §2 CI workflow (5) + §3 closure (3) + §4 反约束 (3) **全 🟢**
- audit schema 跨五 milestone byte-identical 不漂 (改 = 改五处单测锁)
- AST scan 锁链跨三 milestone 在 release-gate.yml 真守 (BPP-4 + BPP-5 + HB-3)
- ≥10 行硬条件清单 + CI workflow 跑全闭, 任一 step fail → workflow red → release block
- ⭐ 4.2 野马 demo 签字 ⏸️ deferred 留账 (release 前真补 3 张截屏)

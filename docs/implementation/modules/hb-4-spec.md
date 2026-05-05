# HB-4 spec brief — release gate ⭐ + 信任五支柱可见 (≤80 行)

> 战马A · Phase 5 host-bridge release gate · ≤80 行 · 蓝图 [`host-bridge.md`](../../blueprint/current/host-bridge.md) §1.5 (v1 release 硬指标 6 行) + §2 信任五支柱 (开源/签名/可审计日志/可吊销/限定能力). 模块锚 [`host-bridge.md`](host-bridge.md) §HB-4 ⭐ (4.1+4.2 双挂). 依赖 HB-1 #491 spec + HB-2 #491 spec + HB-3 #504 实施 (audit log schema 跨四 milestone 同源已就绪) + BPP-4 #499 + BPP-5 #503 (best-effort 立场承袭). 烈马 R2 基准锁: GitHub Actions ubuntu-latest 4vCPU 16GB, CI 数字为准.

## 0. 关键约束 (3 条立场, 蓝图 §1.5+§2 字面)

1. **release gate = 硬条件清单 ≥10 项, 不是软建议** — 任意一行不过, ⭐ milestone 不能关 (蓝图 §HB-4 字面 "任意一行不达标 → ⭐ milestone 不能关"). 反约束: 反向 grep `release.gate.*skip|release.gate.*manual.*sign` 0 hit (不允许人工 sign-off 跳过任一项, 烈马 R2 立场承袭).

2. **每项可机器验** (CI test / grep / lint, 不是人工 review) — 每项 = (蓝图 § 锚 + CI workflow path + assertion) 三元组 byte-identical. 反约束: 反向 grep `release.gate.*manual|release.gate.*human.review` 0 hit (4.2 demo 签字单独走野马截屏, 不混入 4.1 行为不变量数字化路径).

3. **audit schema 锁定 JSON shape 跨四 milestone byte-identical** — `actor / action / target / when / scope` 5 字段在 HB-1 install + HB-2 host-IPC + BPP-4 dead-letter + HB-3 grants 同源 (HB-3 #504 锁链第 4 处已落). 反约束: AST scan 跨四源 struct 字段名 byte-identical, 改一处 = 改五处单测锁 (HB-1+HB-2+BPP-4+HB-3+本 HB-4 schema 文件).

## 1. 拆段 (一 milestone 一 PR, 整段一次合 — 跟 BPP-2/3/4/5 + HB-3 协议同源)

| 段 | 文件 | 范围 |
|---|---|---|
| HB-4.1 release-gate doc | `docs/implementation/modules/hb-4-release-gate.md` (新, ≤200 行) — ≥10 硬条件清单, 每项三元组 (蓝图 § 锚 + CI path + assertion); §1 蓝图 §1.5 6 行 (启动 < 800ms / 崩溃率 < 0.1% / 签名 0% fail / audit schema lock / 撤销 < 100ms / 写类 IPC reject); §2 跨 milestone 反约束追加 (BPP-4 best-effort AST scan / BPP-5 reconnect AST / HB-3 host_grants AST + DOM↔DB 双向锁); §3 信任五支柱 UI 可见 (设置页 5 行状态 API 合约) + 4.2 野马签字流程 (3 张截屏锚) |
| HB-4.2 CI workflow | `.github/workflows/release-gate.yml` (新) — 跑 ≥10 硬条件 (go test 覆盖 5 行: 签名校验 / audit schema reflect / 撤销 < 100ms 真测 / 写类 IPC reject 反向枚举 / AST scan 跨三 milestone forbidden tokens); benchmark job 跑启动时间 < 800ms (烈马 R2 基准); reverse-grep job 跑跨四 milestone audit schema 单源锁; **任一 step fail → workflow 红 → release block** (跟 #500 cov 84% gate 同模式) |
| HB-4.3 closure | `internal/migrations/no_op` (无 schema 改) + `docs/qa/acceptance-templates/hb-4.md` 翻 ✅ + REG-HB4-001..N 🟢 (10+ 行 — 6 蓝图 §1.5 + 3 跨 milestone 锁链 + 1 五支柱 UI 合约) + PROGRESS [x] + `docs/qa/signoffs/hb-4-yema-signoff.md` (野马截屏 placeholder, 实际截屏 release 前补) |

## 2. 留账边界

- **野马 4.2 demo 签字截屏** (留 release 前真补) — 3 张: 五支柱状态页 / 情境授权弹窗 / 撤销后 daemon 立即拒绝行为; 本 PR 仅锁 placeholder + 字面要求, 不卡 PR
- **崩溃率 < 0.1% 内部 dogfood 1 周** (留 release 前真测) — CI 仅锁 placeholder check, 实际 dogfood 跑数据后回填; 不阻 PR 合 (跟 G3 evidence #442 同模式)
- **HB-1/HB-2 Rust crate 真实施** (留 HB-1 + HB-2 真接入 PR) — HB-4 release gate 当前对 server-go 部分 (HB-3 + audit schema + BPP-4/5 立场) 强制; Rust 部分等真实施 PR 加 CI step
- **HB-4.4 客户端 SPA 五支柱状态页** (留 v0 follow-up) — 蓝图 §2 信任五支柱 UI 可见, 本 PR 仅锁 spec; 实施跟 ADM-1 SettingsPage 同模式可后续

## 3. 反查 grep 锚 (Phase 5 收尾验收 + HB-4 实施 PR 必跑)

```
git grep -nE 'release.gate.*skip|release.gate.*manual.*sign' .   # 0 hit (无人工跳过, 立场 ①)
git grep -nE 'release.gate.*human.review' .                       # 0 hit (4.1 vs 4.2 拆死, 立场 ②)
# 跨四 milestone audit schema 单源锁 (反约束第 3, 锁链延伸)
git grep -nE '"actor".*"action".*"target".*"when".*"scope"' packages/server-go/internal/   # ≥4 hit (HB-3/BPP-4 + HB-1/HB-2 待实施)
# AST scan 跨三 milestone forbidden tokens (BPP-4/BPP-5/HB-3 锁链单源)
git grep -nE 'pendingAcks|retryQueue|deadLetterQueue|ackTimeout' packages/server-go/internal/bpp/   # 0 hit (BPP-4)
git grep -nE 'pendingReconnects|reconnectQueue|deadLetterReconnect' packages/server-go/internal/bpp/   # 0 hit (BPP-5)
git grep -nE 'pendingGrants|grantQueue|deadLetterGrants' packages/server-go/internal/api/   # 0 hit (HB-3)
# release gate 数字单源锁
git grep -nE 'BPP_HEARTBEAT_TIMEOUT_SECONDS\s*=\s*30' packages/server-go/internal/bpp/   # ≥1 hit (BPP-4 #499)
```

## 4. 不在本轮范围 (反约束 deferred)

- ❌ HB-1 install-butler Rust crate 真实施 (留 HB-1 真接入 PR, 跟 DL-4 manifest endpoint 落地后启)
- ❌ HB-2 host-bridge daemon Rust crate 真实施 (留 HB-2 真接入 PR, HB-1 落地后启)
- ❌ HB-4.4 客户端 SPA 五支柱状态页 (留 v0 follow-up, 跟 ADM-1 SettingsPage 同模式)
- ❌ 内部 dogfood 1 周崩溃率真测 (留 release 前真测, CI placeholder)
- ❌ 野马 4.2 demo 截屏 (留 release 前真补, spec 仅锚字面要求)
- ❌ admin god-mode 走 release gate (admin 不参与用户主权 milestone, ADM-0 §1.3 字面承袭)

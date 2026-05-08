# HB-4 立场反查清单 (战马A v0)

> 战马A · 2026-04-29 · 立场 review checklist (跟 HB-3 #504 + BPP-4/5 + HB-1/HB-2 #491 stance 同模式)
> **目的**: HB-4 三段实施 (HB-4.1 release-gate doc / 4.2 CI workflow / 4.3 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/hb-4-spec.md` (战马A v0 a02f1d6) + acceptance `docs/qa/acceptance-templates/hb-4.md` (战马A v0)
> **不需 content-lock** — release gate 是 docs + CI workflow, 无 DOM 文案 (跟 BPP-3/4/5 server-only 同模式).

## §0 立场总表 (3 立场 + 4 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | release gate = 硬条件清单 ≥10 项, 任意一行不过 ⭐ 不能关 — **不允许人工 sign-off 跳过任一项** | host-bridge.md §1.5 + 模块 §HB-4 字面 "任意一行不达标 → ⭐ milestone 不能关" + 烈马 R2 立场 | 反向 grep `release.gate.*skip\|release.gate.*manual.*sign\|allow.*bypass` 在 `.github/workflows/release-gate.yml` + `docs/release/` count==0 |
| ② | 每项可机器验 (CI test / grep / lint, 不是人工 review) — 三元组 (蓝图 § 锚 + CI workflow path + assertion) byte-identical | 烈马 R2 立场 + 蓝图 §1.5 "硬指标 必须数字化" 字面 | release-gate.yml step 数 ≥ 10; 每 step 含真 assertion (test 失败 / grep count != expected / benchmark > 阈值 → workflow red) |
| ③ | audit schema 5 字段跨五 milestone byte-identical (HB-1+HB-2+BPP-4+HB-3+HB-4) — drift 防御链至 release 收口 | host-bridge.md §2 信任五支柱第 3 条 (可审计日志) + HB-4 §1.5 release gate 第 4 行 "审计日志格式锁定 JSON schema" | release-gate.yml 跑 cross-source reflect lint + grep `"actor".*"action".*"target".*"when".*"scope"` ≥4 hit (HB-1/HB-2 待 Go binary 真实施补到 5; HB stack Go 重审拍板, 撤 Rust crate 路径) |
| ④ (边界) | 4.1 行为不变量 vs 4.2 demo 签字拆死 — 4.1 数字化 CI / 4.2 野马截屏 (3 张) 走独立路径, **不混入 release gate 自动门** | 模块 §HB-4 字面 "⭐ 4.1+4.2 双挂" + 烈马 R2 拆轨 | 反向 grep `release.gate.*human.review\|release.gate.*demo.signoff` 在 release-gate.yml count==0 |
| ⑤ (边界) | release gate 数字单源 — 跨 milestone 常量 (BPP-4 30s heartbeat / HB-3 撤销 100ms / 启动 800ms) 反向 grep ≥1 hit 单源单测锁 | BPP-4 #499 + HB-3 #504 + 蓝图 §1.5 字面 | release-gate.yml 跑 grep `BPP_HEARTBEAT_TIMEOUT_SECONDS\s*=\s*30` ≥1 hit (BPP-4 单源) + 启动 benchmark ≤ 800ms |
| ⑥ (边界) | AST scan 锁链跨三 milestone 单源 (BPP-4 forbidden + BPP-5 forbidden + HB-3 forbidden) — release 前再次 verify, 防 PR review 后 drift | BPP-4 #499 + BPP-5 #503 + HB-3 #504 锁链 | release-gate.yml 跑 BPP-4/5/HB-3 三组 forbidden tokens AST scan, 任一 hit → workflow red |
| ⑦ (边界) | admin god-mode 不入 release gate — admin 不参与用户主权 milestone (蓝图 §HB-4 是用户视角信任建设, admin 跟它无关) | admin-model.md ADM-0 §1.3 红线 + 蓝图 §HB-4 字面 "用户敢装" 用户视角 | 反向 grep `admin.*release.gate\|admin.*HB4` 在 `internal/api/admin*.go` count==0 |

## §1 立场 ① 硬条件清单 ≥10 项 不允许跳过 (HB-4.1+4.2 守)

**蓝图字面源**: `host-bridge.md` §1.5 6 行硬指标 + 模块 §HB-4 字面 "任意一行不达标 → ⭐ milestone 不能关" + 烈马 R2 立场.

**反约束清单**:

- [ ] release gate 清单 ≥10 项 (蓝图 §1.5 6 行 + 跨 milestone 反约束 4 项至少); 每项三元组 (蓝图 § 锚 + CI path + assertion)
- [ ] CI workflow `.github/workflows/release-gate.yml` 跑全部 ≥10 项, 任一 fail → workflow red → release block
- [ ] 反向 grep `release.gate.*skip\|release.gate.*manual.*sign\|allow.*bypass` 在 `.github/workflows/` + `docs/release/` count==0 (CI lint 守门, 防隐式跳过)
- [ ] 反向 grep `--admin.*merge\|admin.*bypass.*release` count==0 (跟 #486 cron skill 反 admin merge bypass 立场承袭)

## §2 立场 ② 每项可机器验 (HB-4.2 CI workflow 守)

**蓝图字面源**: 烈马 R2 立场 (硬指标必须数字化) + 蓝图 §1.5 "硬指标" 字面.

**反约束清单**:

- [ ] release-gate.yml step 数 ≥ 10
- [ ] 每 step 含 fail 路径: test 失败 / grep count != expected / benchmark > 阈值 → workflow exit non-zero
- [ ] 启动时间 benchmark 锁基准 GitHub Actions ubuntu-latest 4vCPU 16GB (烈马 R2)
- [ ] CI 跟既有 ci.yml 共 runs-on 标签, 反向断言 self-hosted runner (本地数字仅参考)

## §3 立场 ③ audit schema 第 5 处单源锁 (HB-4.1+4.2 守)

**蓝图字面源**: `host-bridge.md` §2 信任五支柱第 3 条 + HB-4 §1.5 release gate 第 4 行.

**反约束清单**:

- [ ] release-gate.yml 跑 reflect lint 跨五 milestone audit schema (HB-3 已就绪 4 处, HB-4 是 5th lock chain link)
- [ ] grep `"actor".*"action".*"target".*"when".*"scope"` 在 `internal/api/host_grants.go` + `internal/bpp/dead_letter.go` 各 ≥1 hit
- [ ] HB-1 / HB-2 Go binary 真实施 PR 加同 schema reflect 测 (留账, HB-4 doc 锁字面 contract; Go 走 `encoding/json` reflect 替代 Rust serde)

## §4 蓝图边界 ④⑤⑥⑦ — 4.1 vs 4.2 拆死 / 数字单源 / AST scan 锁链 / admin 不入

**反约束清单**:

- [ ] 4.2 demo 签字 (野马 3 张截屏) 走 `docs/qa/signoffs/hb-4-yema-signoff.md` 路径, 不入 release-gate.yml
- [ ] release gate 数字常量单源: BPP_HEARTBEAT_TIMEOUT_SECONDS=30 (BPP-4 #499) + 100ms 撤销 (HB-3 #504) + 800ms 启动 (蓝图) 各 ≥1 hit
- [ ] AST scan 锁链跨三 milestone: BPP-4 dead_letter forbidden + BPP-5 reconnect forbidden + HB-3 grants forbidden (release-gate.yml 跑三组 grep)
- [ ] admin god-mode 0 hit (`internal/api/admin*.go` 反向 grep `release.gate\|HB4`)

## §5 退出条件

- §1 (4) + §2 (4) + §3 (3) + §4 (4) 全 ✅
- release gate ≥10 项硬条件 + CI workflow 跑全闭
- audit schema 跨五 milestone byte-identical 不漂 (HB-1+HB-2+BPP-4+HB-3+HB-4 = 5 处单测锁)
- AST scan 锁链跨三 milestone 在 release-gate.yml 真守
- 反向 grep `skip|manual.*sign.?off|allow.*bypass` 0 hit

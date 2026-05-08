# Borgee Helper v1 release gate ⭐

> **Source-of-truth.** This doc enumerates the ≥10 hard release-block
> conditions for Borgee Helper (HB-1 install-butler + HB-2 host-bridge
> daemon + HB-3 host_grants + HB-4 信任五支柱). Each row is a
> three-tuple: blueprint anchor + CI workflow path + assertion.
>
> **关闭条件 (蓝图 §HB-4 字面)**: 任一行 fail → ⭐ milestone 不能关.
> **不允许人工 sign-off 跳过任一行** (烈马 R2 立场). 4.2 demo 签字走野马
> 截屏 [`docs/qa/signoffs/hb-4-yema-signoff.md`](../qa/signoffs/hb-4-yema-signoff.md)
> 独立路径, **不混入** 本 doc 的 4.1 行为不变量数字化清单.
>
> CI workflow: [`.github/workflows/release-gate.yml`](../../.github/workflows/release-gate.yml).
> Run command: `gh workflow run release-gate.yml --ref main`. Workflow
> red → release block.

## §1 蓝图 §1.5 v1 release 硬指标 6 行 (byte-identical)

| # | 指标 | 阈值 | 蓝图 § | CI step | Assertion |
|---|------|------|--------|---------|-----------|
| 1 | Helper 启动时间 (冷启动) | < 800 ms | host-bridge.md §1.5 | `release-gate.yml::startup-benchmark` | benchmark exit code 0 + duration ≤ 800ms; 基准锁 GitHub Actions ubuntu-latest 4vCPU 16GB (烈马 R2) |
| 2 | Helper 崩溃率 (内部 dogfood 1 周) | < 0.1% (1k 会话最多 1 次) | host-bridge.md §1.5 | `release-gate.yml::dogfood-crash-rate` | 留账 placeholder — release 前真测 (1 周 dogfood 数据回填 from `docs/evidence/hb-4-dogfood/crash-stats.json`); CI 当前仅锁 schema 文件存在 |
| 3 | 签名校验失败率 (合法 plugin) | 0% | host-bridge.md §1.5 | `release-gate.yml::signature-pass-rate` | 留账 — HB-1 install-butler Rust crate 真实施 PR 加 contract test, CI 当前锁 HB-1 spec §3.2 字面 manifest schema 不漂 (跨 PR drift anchor 守) |
| 4 | 审计日志格式 | 锁定 JSON schema (含 actor/action/target/when/scope) | host-bridge.md §1.5 + §2 | `release-gate.yml::audit-schema-cross-milestone` | grep `"actor".*"action".*"target".*"when".*"scope"` 在 `internal/api/host_grants.go` + `internal/bpp/dead_letter.go` 各 ≥1 hit (HB-3 #504 + BPP-4 #499 实施已就绪 = 第 4 处单测锁链); HB-1/HB-2 真实施时补到第 5 处 |
| 5 | 撤销 grant → daemon 立即拒绝 | < 100 ms | host-bridge.md §1.5 + §1.3 | `release-gate.yml::revoke-latency` | go test `TestHB3_DELETE_RevokeStampsRevokedAt` (HB-3 #504): DELETE → revoked_at NOT NULL + GET 不返已 revoked grant; v1 daemon 不缓存 (反向 grep `cachedGrants\|grantsCache` 0 hit) |
| 6 | 任何写类 IPC 调用 | 一律拒绝 (v1 仅读) | host-bridge.md §1.4 + §1.5 | `release-gate.yml::no-write-ipc` | HB-2 spec §4 反约束 第 1 + 第 7 锁 (反向 grep `exec\.Cmd\|process::Command\|sh -c` 0 hit + 写类 8 action 反向枚举 0 hit), HB-2 Rust crate 真实施时 CI 跑真测; 当前锁 spec §4 字面契约 |

## §2 跨 milestone 反约束追加 (HB-4 收口)

| # | 立场 | 蓝图 / spec 锚 | CI step | Assertion |
|---|------|----------------|---------|-----------|
| 7 | BPP-4 best-effort 立场承袭 — server 端不挂 dead-letter retry queue (AST scan forbidden tokens) | bpp-4-spec.md §0.3 + #499 dead_letter_test::TestBPP4_NoRetryQueueInBPPPackage | `release-gate.yml::ast-scan-bpp4` | go test `TestBPP4_NoRetryQueueInBPPPackage` 0 hit (`pendingAcks\|retryQueue\|deadLetterQueue\|ackTimeout`) |
| 8 | BPP-5 reconnect best-effort 立场 (AST scan 锁链延伸第 2 处) | bpp-5-spec.md §0.3 + #503 reconnect_handler_test::TestBPP5_NoReconnectQueueInBPPPackage | `release-gate.yml::ast-scan-bpp5` | go test 0 hit (`pendingReconnects\|reconnectQueue\|deadLetterReconnect`) |
| 9 | HB-3 grants best-effort 立场 (AST scan 锁链延伸第 3 处) | hb-3-spec.md §0 + #504 host_grants_test::TestHB3_NoGrantQueueInAPIPackage | `release-gate.yml::ast-scan-hb3` | go test 0 hit (`pendingGrants\|grantQueue\|deadLetterGrants`) |
| 10 | host vs runtime 字典分立 — host_grants 不复用 user_permissions schema | hb-3-spec.md §0.2 + stance §2 立场 ② | `release-gate.yml::dict-isolation` | go test `TestHB3_NoUserPermissionsJoin` 0 hit (AST scan user_permissions reference in host_grants*.go) |

## §3 反约束守门 (本 release gate 不允许跳过)

| # | 反约束 | CI step | Assertion |
|---|--------|---------|-----------|
| 11 | 不允许人工 sign-off 跳过 release gate (烈马 R2 立场 ①) | `release-gate.yml::no-bypass` | grep `release.gate.*skip\|release.gate.*manual.*sign\|allow.*bypass\|--admin.*merge.*release` 在 `.github/workflows/release-gate.yml` + `docs/release/` count==0 |
| 12 | 4.1 行为不变量 vs 4.2 demo 签字拆死 (立场 ④) | 同 step | grep `release.gate.*human.review\|release.gate.*demo.signoff` 在 release-gate.yml count==0 (4.2 截屏走野马 signoff doc, 不混入 yml) |
| 13 | admin god-mode 不入 release gate (立场 ⑦) | `release-gate.yml::no-admin-godmode-release` | grep `admin.*release.gate\|admin.*HB4` 在 `internal/api/admin*.go` count==0 |
| 14 | 数字常量单源锁 (立场 ⑤) — BPP_HEARTBEAT_TIMEOUT_SECONDS=30 (BPP-4 #499) | `release-gate.yml::numeric-singletons` | grep `BPP_HEARTBEAT_TIMEOUT_SECONDS\s*=\s*30` 在 `internal/bpp/heartbeat_watchdog.go` ≥1 hit; reverse-grep 高于 30 的 timeout 0 hit (`heartbeat.*timeout.*[5-9][0-9]+s`) |

## §4 信任五支柱 UI 可见 (蓝图 §2)

| # | 支柱 | 实现 | 留账 |
|---|------|------|------|
| 15 | 开源 | GitHub repo public + `LICENSE` 文件 | server-go 已 public; HB-1/HB-2 Rust crate 真实施时 LICENSE 跟既有同源 |
| 16 | 签名 | HB-1 install-butler 双签 manifest (蓝图 §1.2 ①) | HB-1 spec §3.2 字面锁; Rust crate 真实施时 GPG signing 服务接入 |
| 17 | 可审计日志 | HB-3 audit log + BPP-4 dead_letter audit (5 字段 byte-identical) | 已实施 HB-3 #504 + BPP-4 #499 |
| 18 | 可吊销 | HB-3 host_grants DELETE → revoked_at + daemon 不缓存 (< 100ms) | 已实施 HB-3 #504 |
| 19 | 限定能力 | HB-3 grant_type 4-enum (install/exec/filesystem/network) + ttl_kind 2-enum (one_shot/always) CHECK constraint | 已实施 HB-3 #504 (跟 user_permissions 字典分立) |

⭐ 4.2 demo 签字 (野马, release 前真补): 3 张截屏锚 in
[`docs/qa/signoffs/hb-4-yema-signoff.md`](../qa/signoffs/hb-4-yema-signoff.md):
- 五支柱状态页 (设置页 5 行)
- 情境授权弹窗 (HB-3 HostGrantsPanel 真跑)
- 撤销后行为 (DELETE → daemon 立即拒绝)

## §5 退出条件 (HB-4 ⭐ 关闭条件)

- §1 蓝图 §1.5 6 行硬指标 全 ✅ (CI 实测或留账 placeholder 锁字面)
- §2 跨 milestone 反约束 4 项 (AST scan + 字典分立) 全 0 hit
- §3 反约束守门 4 项 (no-bypass + 4.1/4.2 拆死 + no-admin-godmode + 数字单源) 全 0 hit
- §4 信任五支柱 UI 可见 5 项 (HB-3 实施 3 项 + HB-1/HB-2 待实施 2 项 留账)
- 4.2 demo 签字 ⏸️ deferred 留账 (release 前野马补 3 张截屏)

**任一行 fail → ⭐ HB-4 不能关 → release block.**

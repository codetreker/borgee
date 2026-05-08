# Acceptance Template — HB-2 host-bridge daemon (Go binary, v0(C) 已 merge / v0(D) landlock + sqlite consumer)

> Spec brief `hb-2-spec.md` v1 + `hb-2-v0c-spec.md` (v0(C) merged PR #606). Owner: 战马E 实施 / 飞马 review / 烈马 验收 + ⭐ 野马 G4.x 主签字 (5 截屏 demo).
>
> **HB-2 范围**: host-bridge daemon Go binary 真接 — v0(C) PR #606 已落 (UDS POSIX + ACL + audit + sandbox stub + 8-dict reason). 本 v1 batch 接 v0(D): 真 landlock LSM + sqlite consumer (DL-2 #615 events_archive 接) + plugin manifest 真 ed25519 验签 + Borgee Helper 5 支柱状态字面 (HB-4 release-gate 守门). 立场承袭 HB-2.0 #605 IPCEndpointDefault + HB-2 v0(C) #606 8-dict reason + DL-2 #615 events_archive consumer + HB-1 #589 install-butler 7-reason. **0 server-go diff (独立 module 反 server bloat)**.

## 验收清单

### §1 daemon Go 启动 + landlock syscall 真过

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 v0(C) PR #606 已落 — daemon binary `borgee-helper` UDS POSIX listener + signal SIGINT/SIGTERM clean shutdown | inspect | PR #606 d5c9dc9e merged ✅ (REG-HB2-001..006 v0(C) 全 🟢) |
| 1.2 v0(D) 真 landlock LSM — `internal/sandbox/sandbox_linux.go` 替 v0(C) no-op stub 调 `landlock.Restrict(Path("/run/borgee-helper", PathReadWrite))` (反 v0(C) Apply no-op stub) | unit + e2e | `sandbox_linux_test.go::TestHB2D_LandlockRestrictSyscall` (Linux 真 syscall 锚 + 反 darwin/windows tag 排除) PASS |
| 1.3 daemon 真启 e2e — `cmd/borgee-helper/main.go` 启 `/run/borgee-helper/borgee-helper.sock` (HB-2.0 #605 IPCEndpointDefault byte-identical) + signal clean shutdown deterministic | E2E | `e2e/hb-2-daemon-startup.spec.go` Linux runner 真启 + handshake 真测 PASS |

### §2 plugin manifest 拉取 + ed25519 验签 (HB-1 #589 endpoint 真调)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 daemon 调 HB-1 #589 GET `/api/v1/plugin-manifest` 真接 — `internal/manifest/fetcher.go::FetchManifest(ctx, baseURL)` HTTP client | unit | `fetcher_test.go::TestHB2D_FetchManifest_Happy` + `_NetworkErr_8DictReason` PASS |
| 2.2 真 ed25519 验签 — `internal/manifest/verifier.go::VerifyManifest(manifest, pubkey)` 走 `ed25519.Verify(pubkey, manifest_canonical_bytes, signature)` (反 v0(C) skip 验签) | unit | `verifier_test.go::TestHB2D_VerifyManifest_GoodSig` + `_BadSig_RejectByte-identical` (8-dict reason `manifest_signature_invalid` 字面 byte-identical 跟 HB-1 7-reason 同源) PASS |
| 2.3 反约束 — manifest 拉失败 → 8-dict reason 字面映射 (`manifest_not_found` / `manifest_signature_invalid` / `network_unreachable`) byte-identical 跟 HB-1 7-reason 同源不漂 | grep | reverse grep `"manifest_signature_invalid"\|"manifest_not_found"\|"network_unreachable"` 在 fetcher.go + verifier.go body 同源 ≥1 hit ✅ |

### §3 sqlite consumer 真接 DL-2 events_archive

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 daemon 真接 DL-2 #615 events_archive — `internal/grants/sqlite_consumer.go::SQLiteGrantsConsumer` 实现 v0(C) Consumer interface (反 v0(C) MemoryConsumer mock); 走 SQL `SELECT * FROM channel_events WHERE kind='perm.grant' ORDER BY lex_id DESC LIMIT 1` 拉 grants | unit | `sqlite_consumer_test.go::TestHB2D_SQLiteConsumerLookup` (in-memory SQLite + DL-2 events 表 verify roundtrip) PASS |
| 3.2 grants 不缓存 (撤销 < 100ms HB-4 release gate 第 5 行守) — 反向 grep `grantsCache\|cachedGrants` 在 sqlite_consumer.go body 0 hit (除 v0(C) 注释 anchor 立场承袭) | grep | 0 hit ✅ |
| 3.3 must-persist 4 类 (perm./impersonate./agent.state/admin.force_) 跟 DL-2 #615 mustPersistKinds SSOT byte-identical (跨 milestone const SSOT 锁) | grep | reverse grep `MustPersistKindPrefixes` 在 sqlite_consumer.go 走 import datalayer/must_persist_kinds.go 单源 ≥1 hit ✅ |

### §4 5 支柱状态字面 byte-identical (HB-4 release-gate)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 4.1 5 支柱状态 byte-identical 蓝图 host-bridge.md §2 (`installed_runtimes` / `permission_grants` / `manifest_pubkey` / `audit_log` / `sandbox_profile`) — daemon `Status()` 返 5 字段单源 | grep | `internal/status/status.go::Pillar5` const 5 字面 byte-identical (字面跨蓝图 ↔ daemon ↔ release-gate.yml grep 三层锁) |
| 4.2 HB-4 release-gate 第 5 行 (撤销 < 100ms) + 第 6 行 (写类 100% reject 12 字面) 真守 | unit | v0(C) `TestHB23_RevocationImmediate` + `TestHB24_WriteActions100PercentRejected` 既有 PASS ✅ + v0(D) 不破 |
| 4.3 audit log 5-field SSOT 跟 HB-1 #589 install-butler byte-identical (改 = 改两处单测锁) — daemon 写 audit_events 表 (DL-2 #615) 跟 HB-1 audit.go 同源 | grep + unit | `audit/audit.go::Event5Fields` (actor/action/target/when/scope) byte-identical 跟 HB-1 install-butler audit log 同源 + cross-source reflect lint ≥2 hit (HB-1 + HB-2 双源单测锁) |

### §5 closure 验收 (REG + cov gate + ⭐ 5 截屏 demo)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 5.1 既有全包 unit + e2e + vitest 全绿不破 + post-#614 haystack gate 三轨过 (Func=50/Pkg=70/Total=85) | full test + CI | `cd packages/borgee-helper && go test -timeout=60s -race ./...` 全 PASS + go-test-cov SUCCESS |
| 5.2 0 server-go diff (独立 Go module `packages/borgee-helper/` 反 server bloat) | git diff | `git diff main -- packages/server-go/` ==0 行 ✅ (跟 v0(C) 立场承袭) |
| 5.3 ⭐ 5 截屏 demo (yema G4.x) — daemon 真启 / handshake / landlock / manifest 验签 / SQLite consumer 各 1 PNG | yema sign | `docs/qa/screenshots/hb-2-v0d-*.png` × 5 + yema G4.x signoff 入 |
| 5.4 反平行 / 反 admin god-mode bypass — 反向 grep `admin\b` 在 packages/borgee-helper/ 除 _test 0 hit (ADM-0 §1.3 红线立场承袭) + 反 NATS/Redis dep | grep | reverse grep test PASS (跟 v0(C) 立场承袭) |

## REG-HB2-* (v0(C) #606 已 🟢 / v0(D) 待翻)

- REG-HB2-001..006 🟢 (v0(C) PR #606 merged) — UDS + ACL + audit + sandbox stub + 8-dict reason + 0 server-go diff

**v0(D) 新增** (待本 milestone PR 翻):
- REG-HB2-007 ⚪ landlock LSM 真 syscall + plugin manifest 真 ed25519 验签 + 8-dict reason 字面 byte-identical 跟 HB-1 7-reason 同源
- REG-HB2-008 ⚪ SQLite consumer 真接 DL-2 #615 events_archive + must-persist 4 类 SSOT 跨 milestone byte-identical + ⭐ 5 截屏 demo (yema G4.x signoff)

## 退出条件

- §1 (3) + §2 (3) + §3 (3) + §4 (3) + §5 (4) 全绿 — 一票否决
- daemon Go 真启 (binary + landlock syscall 真过 Linux runner)
- plugin manifest 真 ed25519 验签 (反 v0(C) skip)
- SQLite consumer 真接 DL-2 events_archive (反 v0(C) MemoryConsumer mock)
- 5 支柱状态字面 byte-identical (HB-4 release-gate 第 5/6 行真守)
- 8-dict reason byte-identical 跟 HB-1 7-reason 同源
- audit log 5-field SSOT byte-identical 跟 HB-1 install-butler 同源
- 0 server-go diff + 0 NATS/Redis dep + admin god-mode 不挂 (ADM-0 §1.3)
- post-#614 haystack gate 三轨过 + 全包 unit + race + e2e PASS
- ⭐ 5 截屏 demo + yema G4.x signoff
- 登记 REG-HB2-007..008 (v0(C) 001..006 已 🟢)

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-30 | 烈马 | v0 — HB-2 v0(C) acceptance (REG-HB2-001..006 全 🟢, PR #606 merged). |
| 2026-05-01 | 烈马 | v1 — 扩 5 段验收覆盖 v0(D): landlock LSM 真 syscall + plugin manifest 真 ed25519 验签 + SQLite consumer 真接 DL-2 #615 events_archive + 5 支柱状态 byte-identical + ⭐ 5 截屏 demo. REG-HB2-007..008 ⚪ 占号. 立场承袭 HB-2.0 #605 IPCEndpointDefault + HB-2 v0(C) #606 8-dict reason + HB-1 #589 install-butler 7-reason audit log + DL-2 #615 mustPersistKinds 4 类 SSOT + post-#614 haystack gate. |

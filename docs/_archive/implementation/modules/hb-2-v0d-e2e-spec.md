# HB-2 v0(D) e2e spec brief — daemon + IPC + sandbox smoke (≤80 行)

> 飞马 · 2026-05-01 · post-Phase 4+ closure follow-up (HB-2 v0(D) #617 acceptance e2e 漏件兑现)
> **关联**: HB-2 v0(D) #617 ✅ Go daemon (3 platform sandbox + IPC + SQLite consumer) · 蓝图 host-bridge.md · HB-1 #491 plugin manifest endpoint

> ⚠️ E2E + acceptance template milestone — **0 production code 改 / 0 schema / 0 endpoint** (HB-2 v0(D) 实施已 merged, 本 PR 仅补 e2e 真测 + acceptance template).
> daemon 真启 e2e 跨 3 平台 build tag (跟 HB-2.0 #605 IPC matrix 同模式承袭).

## 0. 关键约束 (3 条立场)

1. **HB-2 v0(D) #617 实施 byte-identical 不破** (post-merge follow-up): 0 production .go 改 (cmd/borgee-helper/* + internal/{ipc,grants,sandbox,acl,audit,reasons}/* 全 byte-identical), 仅加 (a) `docs/qa/acceptance-templates/hb-2-v0d.md` 新 (b) e2e ≥3 case 跨平台 (c) REG-HB2D-001..010 真翻 🟢 (跟 HB-2 v0(D) closure 真翻).

2. **e2e ≥3 case 跨平台 build tag (跟 HB-2.0 #605 IPC matrix 同模式)**:
   - **case-1 daemon 真启**: cd packages/borgee-helper && `go build -o /tmp/borgee-helper ./cmd/borgee-helper/` + start daemon + UDS/Named-Pipe 真启 + signal SIGTERM clean shutdown 验证
   - **case-2 IPC handshake**: dial UDS (linux/macos) / Named Pipe (windows) + JSON-line frame send + handshake 真过 + reject malformed handshake (字面 reasons.HandshakeFailed)
   - **case-3 sandbox apply per-platform**: build tag matrix (linux landlock LSM available kernel ≥5.13 + macOS sandbox-exec / Windows Job Object) + 真测 file IO action 受限 (read_file 受 landlock allowed dir / write_file outside dir reject)
   反约束: e2e 文件名 byte-identical `packages/borgee-helper/e2e/{daemon_startup,ipc_handshake,sandbox_apply}_test.go` (Go test build tag `//go:build integration` + per-platform tag).

3. **0 production change + post-#617 haystack gate 三轨守 + CI matrix per-OS** (跟 HB-2.0 #605 fail-fast: false 同精神): PR diff 仅 (a) acceptance template 新 (b) e2e 3 文件 (c) ci.yml `hb-2-v0d-e2e` matrix job (linux/macos/windows) 加 step. 反约束: 0 packages/borgee-helper/internal/ production 改 + 0 schema / 0 endpoint URL.

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **HB2DE.1 acceptance template** | `docs/qa/acceptance-templates/hb-2-v0d.md` 新 (~80 行 跟 RT-3 #616 acceptance 模板承袭, §1 行为不变量 + §2 数据契约 0 改 + §3 e2e 3 case + §4 closure REG-HB2D-001..010 真翻) | 战马 / 飞马 review |
| **HB2DE.2 e2e 3 case 跨平台** | `packages/borgee-helper/e2e/daemon_startup_test.go` (~80 行 build tag integration + linux/darwin/windows 各 1 case go build + start + SIGTERM); `e2e/ipc_handshake_test.go` (~80 行 UDS dial / Named Pipe dial + handshake 真过 + malformed reject); `e2e/sandbox_apply_test.go` (~80 行 build tag per-platform + landlock available skip linux ≥5.13 + sandbox-exec macOS smoke + Windows Job Object skip-or-skip-with-reason) + ci.yml hb-2-v0d-e2e matrix job 加 step | 战马 / 飞马 review |
| **HB2DE.3 closure** | REG-HB2D-001..010 真翻 🟢 (10 反约束 grep + 3 e2e PASS per platform + 0 production 改 + acceptance template 真补 + post-#617 haystack 三轨过 + 既有 test 全 PASS) + acceptance ⚪→🟢 + content-lock 不需 (e2e infra) + 4 件套 spec 第一件 | 战马 / 烈马 |

## 2. 反向 grep 锚 (6 反约束)

```bash
# 1) acceptance template 真补 (反漏件)
test -f docs/qa/acceptance-templates/hb-2-v0d.md  # exists
wc -l docs/qa/acceptance-templates/hb-2-v0d.md  # ≥60 行 (跟 RT-3 acceptance 模板量级)

# 2) e2e 3 case 真建
ls packages/borgee-helper/e2e/{daemon_startup,ipc_handshake,sandbox_apply}_test.go  # 3 文件存在

# 3) build tag integration 守门
grep -lE '//go:build integration' packages/borgee-helper/e2e/*_test.go  | wc -l  # ≥3 hit

# 4) 0 production 改 (post-#617 byte-identical)
git diff origin/main -- packages/borgee-helper/cmd/ packages/borgee-helper/internal/ | grep -cE '^\+|^-'  # ≤2 hit (允许 import 微调)

# 5) CI matrix 加 hb-2-v0d-e2e job
grep -nE 'hb-2-v0d-e2e|hb2d-e2e' .github/workflows/ci.yml  # ≥1 hit (job name)

# 6) post-#617 haystack gate + 既有 test
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # ALL ≥阈值
cd packages/borgee-helper && go test -tags 'sqlite_fts5 integration' -timeout=300s ./...  # ALL PASS (含 e2e per platform)
```

## 3. 不在范围 (留账)

- ❌ **HB-1b real installer binary** — 留 HB-1B-INSTALLER 独立 milestone (P1 半漏项)
- ❌ **HB-2 v1 升级** (cgroupsv2 / outbound proxy / plugin signing rotation) — 全留 v2+
- ❌ **真 sandbox profile 自定义 UI** — 留 v3+ (HostGrantsPanel.tsx 已盖 4 grant_type 弹窗)
- ❌ **iOS/Android sandbox 真接** — 不在 v1 scope (蓝图 §1.4 仅 desktop OS)

## 4. 跨 milestone byte-identical 锁

- HB-2 v0(D) #617 production code byte-identical 不破
- HB-2.0 #605 IPC matrix CI fail-fast: false 模式承袭
- RT-3 #616 acceptance template 模板承袭 (§1+§2+§3+§4 byte-identical 结构)
- HB-1 #491 plugin manifest endpoint + ed25519 signing 不破
- ADM-0 §1.3 admin god-mode 不挂红线 (e2e 反向断言 admin /admin-api/.*host_grants 0 hit)

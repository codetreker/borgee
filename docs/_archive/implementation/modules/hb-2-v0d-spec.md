# HB-2 v0(D) spec brief — host-bridge daemon 真启 (≤80 行)

> 飞马 · 2026-05-01 · 用户拍板 (NAMING-1 ✅ → DL-2/RT-3/HB-2 v0(D) 并行) · zhanma 主战 + 飞马 review
> **关联**: HB-2.0 #605 ✅ IPC matrix prereq · HB-2 v0(C) #606 ✅ Go daemon stub · HB-1 #491 ✅ install-butler · HB-3 #520 ✅ host_grants schema · HB-4 release-gate ≥10 硬条件 · DL-1 #609 EventBus + PresenceStore · DL-2 (并行 events_archive)
> **命名**: HB-2 v0(D) = host-bridge stack 第 4 步真启 (真 IO + landlock + SQLite consumer 真接)

> ⚠️ Server-side daemon milestone (Go, 跟 #599 HB stack Go redesign byte-identical) — `packages/borgee-helper` 独立 Go module 不污染 server-go. **0 server-go schema 改 / 0 server-go endpoint 改 / 0 host_grants schema 改 (HB-3 #520 byte-identical)**.
> v0(C) → v0(D) 升级路径: stub → 真 landlock + 真 sandbox-exec + 真 IO + SQLite consumer + Windows Named Pipe.

## 0. 关键约束 (3 条立场)

1. **HB-1/HB-3/HB-4 byte-identical 不破 + DL-1/DL-2 接入** (跨 6 milestone 锁链承袭): HB-1 #491 install-butler audit log schema 5-field SSOT 不动; HB-3 #520 host_grants schema (channel_id + agent_id + path + actions + granted_at + revoked_at) 不动; HB-4 release-gate ≥10 硬条件 byte-identical (撤销 <100ms / 写类 100% reject / cross-agent 0 hit / admin god-mode 不挂 / 等); HB-2 8-dict reason 字典分立 (跟 AL-1a 6-dict + HB-1 7-dict 三字典分立守); SQLite consumer 走 DL-1 #609 Repository interface 真接 host_grants 表; cold-stream events 走 DL-2 cold consumer 路径 (落 channel_events 持久 host action 审计). 反约束: 反向 grep host_grants schema column 字面跟 HB-3 #520 byte-identical + 5-field audit JSON 跟 HB-1 #491 byte-identical.

2. **真 sandbox 三平台 + 真 IO + 真 SQLite consumer + admin god-mode 永久不挂**:
   - **Linux landlock LSM** — 走 `github.com/landlock-lsm/go-landlock` (替 v0(C) sandbox_linux.go stub), kernel ≥5.13 fallback AppArmor 跟 HB-2 main spec §3.6 byte-identical; cgroups 不动 (反向 grep `cgroups` in sandbox_linux.go 0 hit, landlock 限 mmap/exec 路径足)
   - **macOS sandbox-exec profile** — 真 profile 文件生成 + `os/exec sandbox-exec -f profile.sb borgee-helper`, profile 限 file-read-data + file-write-data 仅授权路径
   - **Windows Named Pipe** — `\\.\pipe\borgee-helper` (替 v0(C) main_other.go stub), 走 `github.com/microsoft/go-winio` 跟 HB-2.0 #605 IPCEndpointDefault byte-identical
   - **真 IO 解锁** — `read_file` / `list_files` action 真走 `os.ReadFile` / `os.ReadDir` (v0(C) 仅 ACL 决策 stub, v0(D) 加真 IO + landlock 守门)
   - **真 SQLite consumer** — `internal/grants/sqlite_consumer.go` 替 v0(C) MemoryConsumer mock, 走 DL-1 Repository interface 查 host_grants 表 (撤销 <100ms 真守, 反 cache 漂)
   - **admin god-mode 永久不挂** — 反向 grep `admin.*host_grants|/admin-api/.*host_grants` 0 hit (ADM-0 §1.3 红线, HB-2 spec §1.3 byte-identical)
   - 反约束: 反向 grep `MemoryConsumer` 在 production 路径 (除 _test.go) 0 hit; sandbox stub `// v0(C) stub` 注释 0 hit (全替真实现)

3. **0 server-go 改 + 0 host_grants schema 改 + 0 user-facing API 改** (host-bridge 域 wrapper, 跟 INFRA-3/4 / REFACTOR-1/2 / NAMING-1 / DL-2 / RT-3 同源): PR diff 仅 (a) `packages/borgee-helper/internal/sandbox/{linux,darwin,windows}.go` 真实现替 stub (~300 行) (b) `packages/borgee-helper/internal/grants/sqlite_consumer.go` 新 (~100 行 走 DL-1 Repository interface) (c) Windows Named Pipe ipc 真启 (~80 行) (d) systemd unit + launchd unit + sandbox-exec profile 配置 (~100 行 install-butler 拉起) (e) cmd/borgee-helper/main_windows.go 替 main_other.go stub. 反约束: 0 server-go production .go 改 + 0 routes.go 改 + 0 schema column 改 + 0 migration v 号 + DL-1 EventBus/Repository signature byte-identical (跟 DL-2 / RT-3 同要求).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **HB2D.1 sandbox 三平台真启** | `packages/borgee-helper/internal/sandbox/sandbox_linux.go` 替 stub (~120 行 go-landlock + AppArmor fallback); `sandbox_darwin.go` 替 stub (~80 行 sandbox-exec profile + os/exec); `sandbox_windows.go` 新 (~60 行 Job Object + Restricted Token); 反约束 unit per 平台 build tag (~60 行) | 战马 / 飞马 review |
| **HB2D.2 真 IO + SQLite consumer + Windows IPC** | `packages/borgee-helper/internal/grants/sqlite_consumer.go` 新 (~100 行 DL-1 Repository interface 真接 host_grants); `internal/io/file_actions.go` 新 (~80 行 read_file/list_files 真走 os.ReadFile/ReadDir + landlock 守门); `internal/ipc/ipc_windows.go` 替 main_other.go stub (~80 行 go-winio Named Pipe); 4 thinking-style log audit 跟 DL-2 events_archive 接 (cold consumer hook) | 战马 / 飞马 review |
| **HB2D.3 install-butler 拉起 + closure** | `packages/borgee-helper/install/borgee-helper.service` (systemd, ~30 行); `borgee-helper.plist` (launchd, ~30 行); `sandbox-exec.profile` (macOS, ~40 行); REG-HB2D-001..010 (10 反向 grep + sandbox 真启 + SQLite consumer 真接 + 真 IO 真过 + Windows Named Pipe 真启 + admin 永久不挂 + HB-3/HB-4/HB-1 byte-identical 不破 + DL-1 interface 不破 + haystack 三轨过 + 既有 test 全 PASS) + acceptance + content-lock 不需 (server-only daemon) + 4 件套 spec 第一件 | 战马 / 烈马 |

## 2. 反向 grep 锚 (10 反约束)

```bash
# 1) HB-1 install-butler audit schema byte-identical 不破
grep -rcE 'audit_id|action|target|outcome|user_agent' packages/borgee-helper/internal/audit/  # ==5 hit (5-field SSOT 跟 HB-1 #491 byte-identical)

# 2) HB-3 host_grants schema byte-identical 不破 (sqlite_consumer.go 字段对账)
grep -rE 'channel_id|agent_id|path|actions|granted_at|revoked_at' packages/borgee-helper/internal/grants/sqlite_consumer.go  | wc -l  # ≥6 (HB-3 #520 6 字段)

# 3) HB-4 release-gate ≥10 硬条件不破
grep -nE 'release-gate.*hb-4|TestHB23_RevocationImmediate|TestHB24_WriteActions100PercentRejected' packages/borgee-helper/  | wc -l  # ≥2 (撤销 + 写类 reject 真测延伸 v0(D))

# 4) MemoryConsumer 不在 production (反 v0(C) stub 残留)
grep -rE 'MemoryConsumer' packages/borgee-helper/cmd/ packages/borgee-helper/internal/grants/sqlite_consumer.go  # 0 hit (production 路径全走 SQLiteConsumer)
grep -rE '// v0\(C\) stub|// stub' packages/borgee-helper/internal/sandbox/  # 0 hit (全替真实现)

# 5) cgroups 不在 sandbox_linux.go (landlock 替)
grep -rE 'cgroups|cgroupv2' packages/borgee-helper/internal/sandbox/sandbox_linux.go  # 0 hit

# 6) 真 IO 真接 (反 stub)
grep -rE 'os\.ReadFile|os\.ReadDir' packages/borgee-helper/internal/io/  # ≥2 hit (真 IO)

# 7) Windows Named Pipe 真启 (反 main_other.go stub)
grep -rE '\\\\\\.\\\\pipe\\\\borgee-helper|go-winio' packages/borgee-helper/internal/ipc/ipc_windows.go cmd/borgee-helper/main_windows.go  # ≥2 hit

# 8) admin god-mode 永久不挂 (ADM-0 §1.3 红线)
grep -rE 'admin.*host_grants|/admin-api/.*host_grants|admin.*borgee-helper' packages/borgee-helper/ packages/server-go/internal/  # 0 hit

# 9) 0 server-go 改 + DL-1 interface signature byte-identical
git diff origin/main -- packages/server-go/internal/datalayer/eventbus.go packages/server-go/internal/datalayer/repository.go | grep -cE '^-.*func.*(Publish|Subscribe|Get|List|Create|Update|Delete)\('  # 0 hit
git diff origin/main -- packages/server-go/internal/migrations/ | grep -cE '^\+\s*Version:'  # 0 hit (0 migration)

# 10) haystack gate 三轨 + 既有 test
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # ALL ≥阈值
cd packages/borgee-helper && go test -tags 'integration' -timeout=300s ./...  # ALL PASS (含 sandbox per-platform unit)
```

## 3. 不在范围 (留账)

- ❌ **HB-5 heartbeat retention** / HB-6 lag percentile — 已 merged, 跟 v0(D) 不撞
- ❌ **outbound network proxy** — 留 v1.5+ (蓝图 §3 明示 "v1 仅 file IO + DB query, 不引出站网络")
- ❌ **真 cgroupsv2 limits** (memory / cpu) — 留 v2 (landlock 已限 path, cgroups 是辅助)
- ❌ **plugin signing rotation** (HB-1 #491 已固定 ed25519 keyset) — 留 v3+
- ❌ **HB stack admin 视图** (e.g. /admin-api/host-bridge-status) — 永久不挂 ADM-0 §1.3
- ❌ **HB-2 macOS notarization / Windows code-sign** — 留 install-butler v2 (HB-2.5 安装侧)

## 4. 跨 milestone byte-identical 锁

- 复用 HB-1 #491 install-butler audit log schema 5-field SSOT (跨 6 milestone audit 锁链)
- 复用 HB-3 #520 host_grants schema 6 字段 (sqlite_consumer.go 字面对账)
- 复用 HB-4 release-gate ≥10 硬条件 (撤销 <100ms + 写类 100% reject + cross-agent + admin 不挂 等)
- 复用 HB-2 v0(C) #606 8-dict reason 字典 (跟 AL-1a 6-dict + HB-1 7-dict 三字典分立)
- 复用 DL-1 #609 EventBus + Repository interface (sqlite_consumer 走 Repository, signature byte-identical)
- 复用 DL-2 events_archive cold consumer (host action audit 落 channel_events)
- 复用 ADM-0 §1.3 admin god-mode 不挂红线 (永久不挂 host_grants 域)
- 0-server-go-改 wrapper 决策树**变体**: 独立 Go module borgee-helper, 0 server-go production 改

## 5. 派活 + 双签

派 **zhanma-e** (HB-2.0 #605 + HB-2 v0(C) #606 主战熟手, 续作减学习成本) + 飞马 review.

双签流程: spec brief → team-lead → 飞马自审 ✅ APPROVED → yema stance + content-lock + liema acceptance → zhanma 起实施 (HB2D.1+2+3 三段一 PR, **teamlead 唯一开 PR**).

## 6. 飞马 (架构师) 自审表态

✅ **APPROVED with 3 必修条件**:

🟡 必修-1: **HB-1/HB-3/HB-4 byte-identical 三锁** — 反约束 grep #1+#2+#3 真守, audit schema + host_grants schema + release-gate 硬条件全字面 byte-identical. zhanma PR body 必示三 grep 输出.

🟡 必修-2: **真 sandbox 三平台 + 真 SQLite consumer 替 stub** — 反约束 grep #4+#5+#6+#7 真守, MemoryConsumer / `// v0(C) stub` / cgroups 字面 0 hit; 真 sandbox 真测 (Linux landlock kernel ≥5.13 / macOS sandbox-exec / Windows Job Object).

🟡 必修-3: **DL-1 interface signature byte-identical 不破** (跟 DL-2 / RT-3 三 milestone 同要求) — `git diff -- internal/datalayer/` Publish/Subscribe/Get/List 行 0 hit.

担忧 (1 项, 中度):
- 🟡 真 sandbox 三平台测试需 CI matrix (跟 HB-2.0 #605 同模式) — 战马实施需 CI matrix sandbox unit per OS, fail-fast: false; Linux runner 真 landlock kernel 版本验证 (5.13+ available on ubuntu-22.04+).

留账接受度全 ✅: outbound proxy / cgroupsv2 / plugin signing rotation / admin 视图 / notarization+code-sign 全留账.

**ROI 拍**: HB-2 v0(D) ⭐⭐⭐ — host-bridge stack 真启 (4 步路径终点), 阻塞 BPP-2 plugin 真接 host (file IO + agent task host action) 解锁; 跟 DL-2 (storage 端) / RT-3 (fanout 端) 并行不撞.

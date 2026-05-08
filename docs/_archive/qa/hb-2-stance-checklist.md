# HB-2 stance checklist — host-bridge daemon v0(D) Go 重写 + landlock + 真 IO + sqlite consumer

> 7 立场 byte-identical 跟 hb-2-spec.md §0+§2 (飞马 v0 待 commit). **真有 prod code (v0(C) #606 Go 框架延伸 + landlock 真 sandbox + 真 IO + SQLite consumer 真接 HB-3 grants) + client UI 字面 (content-lock 5 支柱)** 但 0 server schema 改 (daemon 是 client side, 跟 hb-2-spec §1.1 内部双 daemon 拆分立场承袭). 跟 #599 HB stack Go + #605 HB-2.0 + #606 HB-2 v0(C) + HB-3 host_grants + HB-4 release-gate 4 步路径 + 5 支柱锁链承袭. content-lock 必备 (daemon UI / status 5 支柱字面真锁). **取代 v1 docs-only Rust era stance (Rust crate 路径已 DROPPED #599 后)**.

## 1. Go 重写 (非 Rust, HB-1 重审决策)
- [ ] **#599 HB stack Go 单栈决策真兑现** — 反向 grep `Cargo|cargo|.rs|crate|workspace` 在 packages/borgee-helper/ 0 hit (跟 #606 v0(C) Cargo workspace 注释残留 nit 一并清, 真守 0 Rust 痕迹)
- [ ] 独立 Go module `packages/borgee-helper/` 持独立 `go.mod` (跟 #606 hb-2-spec §5.5 拆死锚承袭, 反 server-go binary bloat)
- [ ] PM 必修 #1 Borgee Helper 命名拆死真兑现 (daemon=Go binary OS-level vs PWA UI=browser, 跨 #599+CS-1/2/3 字面承袭一致)

## 2. landlock 真 sandbox (Linux only v0, mac/win 留 v1)
- [ ] **Linux landlock LSM 真启** — `internal/sandbox/sandbox_linux.go` v0(D) 真接 go-landlock dep (反 #606 v0(C) no-op stub)
- [ ] **mac/win 留 v1** — `sandbox_darwin.go` + `sandbox_windows.go` 显式 stub + 注释明示 v1 真启 (sandbox-exec / Windows AppContainer)
- [ ] sandbox build tag 拆死锚 (跟 #606 三文件 stub 同模式) byte-identical 不破
- [ ] 反向 grep `exec\.Cmd|os\.Exec` 在 sandbox/ 0 hit (sandbox 不漏权)
- [ ] 反 stub 阶段假启 sandbox 给用户错觉 (跟 PM 必修立场承袭)

## 3. 真 IO (本地文件读, 跟 HB-1 manifest endpoint 对接)
- [ ] **read_file / list_files 真 IO** — 替 #606 v0(C) ACL 决策仅判定的 stub, 真启文件 read (反向断 readPaths 仅在 grants 白名单内)
- [ ] 跟 HB-1 #589 manifest endpoint 对接 — daemon 启动期 fetch manifest (ed25519 校验) byte-identical
- [ ] 反向 grep `os\.WriteFile|os\.Create|ioutil\.WriteFile` 在 helper 路径 0 hit (反写, host-bridge 永久 read-only)
- [ ] HB-1 7-dict reason 字面承袭一致 (反 daemon 引入新错码漂)

## 4. sqlite consumer 真接 (跟 DL-2 events_archive 单源)
- [ ] **HB-3 host_grants SQLite consumer 真接** — 替 #606 v0(C) `MemoryConsumer` mock, 真接 SQLite (Consumer interface byte-identical 不改 daemon 一行)
- [ ] **嵌入 SQLite read-only mode** (PM HB-3 prep 倾向 A 真兑现) — 单源 + 撤销 < 100ms (HB-4 第 5 行真兑现)
- [ ] **跟 DL-2 events_archive 单源协同** — daemon read-only consumer 不污染 cold archive 写路径
- [ ] 反向 grep `grantsCache|cachedGrants|memoryGrants` 0 hit (反缓存, 跟 #606 立场 ③ 承袭)
- [ ] HB-2 8-dict reason 字面 byte-identical (跨 hb-2-spec §3.3 + reasons.go + REG-HB2-001 三处对锁)

## 5. 复用 HB-3 host_grants ACL (不另起授权)
- [ ] **HB-3 grants 真兑现 ACL** — daemon 不另起授权 schema, 反 grants_v2 / per_device_grants 漂
- [ ] cross-agent ACL 闸不漏 (Lookup 按 (agent_id, scope) 双键, 跟 #606 立场承袭)
- [ ] 反向 grep `grants_v2|host_grants_v2|daemon_grants` 0 hit (单源 SSOT)
- [ ] 蓝图 §1.3 4 类授权 (装机 install/exec + 触发 filesystem/network) 字面承袭一致

## 6. 0 server schema 改 (daemon 是 client side)
- [ ] 反向 grep `migrations/hb_2_` 在 packages/server-go/ 0 hit (daemon 不动 server schema)
- [ ] `currentSchemaVersion` 不动 (反向断 0 行改)
- [ ] 反 ALTER server-go 既有 schema 漂入
- [ ] daemon 真 IO 仅本地, 反 server-side 持久化路径

## 7. admin god-mode 不挂 daemon (ADM-0 §1.3 红线)
- [ ] 反向 grep `admin.*helper|admin.*host-bridge|admin.*daemon` 在 packages/borgee-helper/ 0 hit
- [ ] 反向 grep `/admin-api.*helper` 0 hit
- [ ] daemon 走用户机本地 OS user/group, 反 admin override (anchor #360 owner-only ACL 锁链 22+ PRs 立场延伸)
- [ ] HB-2 spec §1.3 红线 byte-identical 承袭

## 反约束 — 真不在范围
- ❌ mac sandbox-exec / Windows AppContainer 真启 (留 v1)
- ❌ 网络出站 outbound proxy 真接 (留 v1)
- ❌ 文件 write 真启 (host-bridge 永久 read-only, ADM-0 §1.3 红线)
- ❌ 0 server schema 改 / 0 endpoint / 0 既有 ACL 改
- ❌ 加新 CI step (跟 DL-1/2 + REFACTOR-1/2 + INFRA-3 + TEST-FIX-* 同精神)
- ❌ admin god-mode 加挂 daemon (永久不挂)

## 跨 milestone byte-identical 锁链 (5 链)
- **#599 HB stack Go + #605 HB-2.0 + #606 HB-2 v0(C)** — HB stack 4 步路径第 4 步真闭 (Go binary + UDS + ACL + audit + sandbox stub → v0(D) 真 sandbox + 真 IO + SQLite consumer)
- **HB-1 #589 manifest endpoint** — daemon 启动 fetch manifest (ed25519 校验) byte-identical 承袭
- **HB-3 host_grants SQLite consumer** — Consumer interface byte-identical (#606 锚守不改 daemon 一行真兑现)
- **HB-4 release gate 5 支柱** — daemon 真兑现启动 < 800ms / 崩溃率 < 0.1% / 签名 0% fail / audit schema lock / 撤销 < 100ms / 写类 IPC 100% reject
- **anchor #360 owner-only ACL 锁链 22+ PRs** + REG-INV-002 fail-closed + ADM-0 §1.3 红线

## PM 拆死决策 (3 段)
- **Go 重写 vs Rust 拆死** — Go 选 (#599 4 角色拍板, 反 Rust crate 路径 DROPPED), 命名拆死跟 PWA UI 真兑现
- **landlock 真 sandbox vs no-op stub 拆死** — Linux 真启 (本 PR 选), mac/win 显式 stub + 注释明示 v1 (反假启给用户错觉)
- **嵌入 SQLite consumer vs 缓存模式拆死** — A 嵌入 read-only (PM HB-3 prep 倾向真兑现), 反 in-memory cache / TTL / write-through / daemon 副本 / HTTP loopback 6 模式漂

## 用户主权红线 (5 项)
- ✅ host-bridge 永久 read-only (反向 grep os.WriteFile 0 hit, 反写真守)
- ✅ 既有 ACL gate 字面 + 行为 byte-identical (anchor #360 + REG-INV-002 + HB-3 grants)
- ✅ landlock 真启 (反 stub 阶段假启给用户错觉)
- ✅ 0 server schema 改 / 0 endpoint shape / 0 既有 ACL 改 (daemon client side)
- ✅ admin god-mode 不挂 daemon (ADM-0 §1.3 红线 + HB-2 spec §1.3 承袭)

## PR 出来 5 核对疑点
1. 反向 grep `Cargo|cargo|.rs|crate|grants_v2|memoryGrants|os.WriteFile` 在 packages/borgee-helper/ count==0
2. landlock Linux 真启 + mac/win stub 注释明示 v1 (build tag 三文件锚)
3. HB-2 8-dict reason byte-identical 跨 hb-2-spec/reasons.go/REG 三处对锁
4. 5 支柱字面 byte-identical 跨 content-lock + HB-4 release-gate + UI 三处对锁
5. cov ≥85% (#613 gate) + 0 race-flake + admin god-mode grep 0 hit

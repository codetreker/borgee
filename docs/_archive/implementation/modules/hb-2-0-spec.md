# HB-2.0 prerequisite spec — CI matrix + IPC primitive smoke (≤30 行 production)

> 战马E · Phase 4+ host-bridge stack 第 2 步 · ≤80 行 spec · HB stack 4 步路径锚 (HB-2 v0(C) Go 实施前置). HB-2.0 是 **CI matrix + IPC primitive smoke 真 infra PR** (跟 INFRA-2 #195 Playwright scaffold 同模式). 不开 docs-only.

## 0. 立场 (3 项, ≤30 行 production)

1. **CI matrix 加 macOS + Windows runner** — `os: [ubuntu-latest, macos-latest, windows-latest]` 跑 IPC primitive smoke (build-tag-per-platform). 反 HB-2 v0(C) 实施时跨平台 drift 漏抓 (Linux-only CI 漏 macOS sandbox-exec / Windows Named Pipe 边界).

2. **IPC primitive 选择器单源** — `internal/helper/ipc.go` 暴露 `IPCPlatform` 三态 (`linux-uds`/`darwin-uds`/`windows-named-pipe`) + `IPCEndpointDefault` 函数. HB-2 v0(C) Go daemon 调本 helper 选 IPC 路径, 反双源 drift.

3. **3 IPC unit per platform** — `ipc_linux_test.go` + `ipc_darwin_test.go` + `ipc_windows_test.go` 各 build tag, 反向断 IPC label + endpoint default 字面 byte-identical (反 HB-2 v0(C) 实施时改 const 漏跨 OS unit).

## 1. 拆段实施 (单 PR 全闭)

| 段 | 文件 | 范围 |
|---|---|---|
| HB-2.0.1 helper | `packages/server-go/internal/helper/ipc.go` (新, ~30 行) | IPCPlatform 三态 const + IPCEndpointDefault switch (Linux UDS / macOS UDS / Windows Named Pipe) |
| HB-2.0.2 unit per platform | `internal/helper/ipc_linux_test.go` + `ipc_darwin_test.go` + `ipc_windows_test.go` (新, 各 build tag) | TestHB20_IPC_UDSConnect_Linux + UDSConnect_macOS + NamedPipeConnect_Windows 字面对比锁 |
| HB-2.0.3 CI matrix | `.github/workflows/ci.yml` 加 `hb20-ipc-prereq` job | strategy.matrix.os 三平台 + go test ./internal/helper/... |
| HB-2.0.4 closure | REG-HB20-001..003 + acceptance + PROGRESS [x] | 3 立场 byte-identical 锁 + 反向 grep |

## 2. 反向 grep 锚

```
git grep -nE 'IPCPlatform' packages/server-go/internal/ | grep -v helper/  # 0 hit (单源)
git grep -nE 'IPCEndpointDefault' packages/server-go/internal/ | grep -v helper/  # 0 hit (单源)
git grep -nE '/run/borgee-helper|borgee-helper.sock|\\\\.\\\\pipe\\\\borgee-helper' packages/server-go/internal/ | grep -v helper/ | grep -v _test  # 0 hit (字面单源)
```

## 3. 不在本轮范围 (HB-2 v0(C) 真启动留 follow-up)

- ❌ HB-2 v0(C) Go daemon binary (~400-500 行, packages/borgee-helper/cmd/)
- ❌ UDS server / Named Pipe server 真启
- ❌ grants_consumer trait + in-memory mock
- ❌ cross-agent ACL 真测
- ❌ systemd unit + launchd unit + sandbox-exec profile
- ❌ HB-3 host_grants schema (留 HB-3 spec brief)

## 4. 4 步路径锚 (HB stack)

1. ✅ HB stack Go spec patch #599 — 改 Rust → Go decision
2. **本 PR HB-2.0** — CI matrix + IPC primitive smoke prerequisite
3. ⏸️ HB-2 v0(C) — Go daemon ~400-500 行 (UDS + Named Pipe + grants_consumer mock + cross-agent ACL + sandbox config)
4. ⏸️ HB-3 — host_grants schema + grants 弹窗 + 真接 HB-2 grants_consumer

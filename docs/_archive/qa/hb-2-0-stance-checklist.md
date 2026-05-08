# HB-2.0 stance checklist — CI matrix + IPC primitive smoke prerequisite

> 3 立场 byte-identical 跟 spec §0 (≤80 行).

## 1. CI matrix 加 macOS + Windows runner (跟 INFRA-2 #195 同模式)

- [x] `os: [ubuntu-latest, macos-latest, windows-latest]` strategy.matrix
- [x] `fail-fast: false` (反 macOS 失败不阻 Linux/Windows 真出反向 audit)
- [x] runs-on: `${{ matrix.os }}` 跨 OS 真启 runner
- [x] go test build-tag-per-platform 自动选对 ipc_*_test.go 文件
- [x] 反 HB-2 v0(C) 实施时 Linux-only CI 漏 macOS sandbox-exec / Windows Named Pipe 边界

## 2. IPC primitive 选择器单源 (helper package)

- [x] `internal/helper/ipc.go` 暴露 `IPCPlatform` 三态 const + `IPCEndpointDefault` 函数
- [x] HB-2 v0(C) Go daemon 调本 helper 选 IPC 路径 (反双源 drift)
- [x] 反向 grep `IPCPlatform|IPCEndpointDefault` 在 internal/ 除 helper/ 0 hit (单源闸)
- [x] 反向 grep IPC endpoint 字面 (`/run/borgee-helper|borgee-helper.sock|\\\\.\\\\pipe\\\\borgee-helper`) 在 internal/ 除 helper/ + 除 _test 0 hit (字面单源)

## 3. 3 IPC unit per platform (build tag)

- [x] `ipc_linux_test.go` (build tag `linux`) — TestHB20_IPC_UDSConnect_Linux 字面对比 endpoint + label byte-identical
- [x] `ipc_darwin_test.go` (build tag `darwin`) — TestHB20_IPC_UDSConnect_macOS 字面对比
- [x] `ipc_windows_test.go` (build tag `windows`) — TestHB20_IPC_NamedPipeConnect_Windows 字面对比
- [x] CI matrix 跑各 OS 真验证 build tag 正确选择测试文件
- [x] 反 HB-2 v0(C) 实施时改 const 漏跨 OS unit (build-tag-per-platform 强制约束)

## 反约束

- ❌ HB-2 v0(C) Go daemon binary (留 follow-up PR ~400-500 行)
- ❌ UDS server / Named Pipe server 真启 (留 v0(C))
- ❌ grants_consumer trait + mock impl (留 v0(C))
- ❌ cross-agent ACL 真测 (留 v0(C))
- ❌ systemd unit + launchd unit + sandbox-exec profile (留 v0(C))
- ❌ HB-3 host_grants schema (留 HB-3 spec brief)
- ❌ admin god-mode IPC (永久不挂, ADM-0 §1.3 跟 HB-2 spec §1.3 红线)

## 跨 milestone byte-identical 锁链

- INFRA-2 #195 Playwright scaffold 同模式 (真 infra PR, 不是 docs-only)
- HB-1 #491 install-butler audit log schema (跨 OS 同精神)
- HB-2 spec §1.3 红线 — admin god-mode 不挂 host IPC
- HB-3 host_grants schema (HB-2 v0(C) read-only consumer 接入点)
- HB stack 4 步路径 — HB stack Go spec patch #599 → 本 PR HB-2.0 → HB-2 v0(C) → HB-3
- AL-1a 6-dict + HB-1 7-dict + HB-2 8-dict 三字典分立 (HB-2.0 仅 IPC primitive label, 不入 reason 字典)

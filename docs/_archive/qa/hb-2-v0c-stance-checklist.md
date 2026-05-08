# HB-2 v0(C) stance checklist — Go daemon binary 真启

> 4 立场 byte-identical 跟 hb-2-v0c-spec.md §0.

## 1. 0 server-go diff (独立 Go module)

- [x] `packages/borgee-helper/go.mod` separate module (反 server-go binary bloat)
- [x] 反向 grep `package server` 在 `packages/borgee-helper/` 0 hit (模块拆死)
- [x] 反向 grep `borgee-server/` import 在 `packages/borgee-helper/` 0 hit
- [x] HB-1 install-butler 跟 host-bridge 同 mono-repo 但 cmd/ 子拆 (cmd/borgee-helper + 未来 cmd/install-butler 各独立 main)

## 2. 8-dict reason 字典分立 (跟 HB-1 7-dict + AL-1a 6-dict 不混)

- [x] `internal/reasons/reasons.go` 8 const 字面 byte-identical 跟 hb-2-spec §3.3
- [x] All() 反向枚举锚单测 (TestHB2_Reason8DictByteIdentical)
- [x] TestHB2_NoSeventhDictBleed 反向断 HB-1 7-dict + AL-1a 6-dict 字面 0 hit
- [x] reasons.OK / PathOutsideGrants / GrantExpired / GrantNotFound / HostExceedsMaxBytes / EgressDomainNotWhitelisted / CrossAgentReject / IOFailed (8 项 byte-identical)

## 3. read-only Consumer 接口锚 (HB-3 接入点)

- [x] `internal/grants/grants.go` Consumer interface (Lookup + LookupRaw)
- [x] MemoryConsumer mock (Put/Delete/SetNowFn 仅测试用, HB-3 SQLite consumer 不暴露)
- [x] grants 不缓存 — 反向 grep `grantsCache|cachedGrants` 0 hit (反约束 #3)
- [x] TestHB23_RevocationImmediate 反向断撤销立即生效 (< 100ms HB-4 release gate 第 5 行守)
- [x] grant_expired vs grant_not_found 字面区分 (LookupRaw 返回 exists+expired 双 bool)

## 4. sandbox build tag 拆死 (v0(C) stub, v0(D) 真接)

- [x] `sandbox_linux.go` (//go:build linux) Platform="linux" Apply no-op
- [x] `sandbox_darwin.go` (//go:build darwin) Platform="darwin" Apply no-op
- [x] `sandbox_other.go` (//go:build !linux && !darwin) Platform="other" Apply no-op
- [x] TestHB26_PlatformLabelMatchesBuildTag 反向断 build tag 选对 Platform 字面单一
- [x] cmd/borgee-helper main_other.go (//go:build !linux && !darwin) 提示 v0(D) 接 (反 build 失败)

## 反约束

- ❌ Windows Named Pipe 真启 (留 v0(D))
- ❌ 真 landlock LSM (留 v0(D) — go-landlock dep)
- ❌ 真 sandbox-exec profile 生成 (留 v0(D))
- ❌ systemd / launchd unit 文件 (留 v0(D))
- ❌ HB-3 host_grants SQLite consumer 真实现 (留 HB-3)
- ❌ 文件 read_file / list_files 真 IO (留 v0(D), v0(C) 仅 ACL 决策 + audit)
- ❌ 网络出站 outbound proxy 真接 (留 v0(D))

## 跨 milestone byte-identical 锁链

- HB-2.0 #605 (本 PR ipc/grants/acl/audit 复用 IPCEndpointDefault default `/run/borgee-helper/borgee-helper.sock` 字面)
- HB-1 #491 install-butler audit log schema 5-field (HB-2 audit.Event 同 SSOT — actor/action/target/when/scope byte-identical)
- HB-2 spec §1.3 红线 — admin god-mode 不挂 host IPC (反向 grep `admin` 在 `packages/borgee-helper/` 0 hit)
- HB-3 host_grants schema (HB-2 grants.Consumer 是接入点; HB-3 落地后 SQLite consumer 实现同 interface 不改 daemon 一行)
- HB-4 release gate 第 5 行 (< 100ms 撤销) + 第 6 行 (写类 100% reject) — TestHB23_RevocationImmediate + TestHB24_WriteActions100PercentRejected 守
- HB stack 4 步路径 — #599 → HB-2.0 #605 → 本 PR → HB-3
- AL-1a 6-dict + HB-1 7-dict + HB-2 8-dict 三字典分立

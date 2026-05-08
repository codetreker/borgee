# HB-2 v0(C) host-bridge daemon — implementation spec brief (≤80 行)

> 战马E · Phase 4+ host-bridge stack 第 3 步 · HB stack 4 步路径锚 (HB-2.0
> #605 prereq 之后, HB-3 host_grants schema 之前). v0(C) = Go daemon
> binary 真启 (UDS POSIX + handshake + ACL gate + audit + sandbox stub),
> 不挂真 landlock/sandbox-exec 调用 (留 v0(D) 跟 HB-1 #589 Go binary 同
> 期接 go-landlock dep).
>
> hb-2-spec.md (#599 已合) 是真值; 本 doc 仅记 v0(C) 落地段.

## 0. 立场 (4 项)

1. **0 server-go diff** — 独立 Go module `packages/borgee-helper` (separate
   `go.mod`, hb-2-spec §5.5 拆死), 不动 server-go binary; 反向 grep
   `package server` 在 `packages/borgee-helper/` 0 hit.

2. **8-dict reason 字典分立** — `internal/reasons/reasons.go` (8 字面 byte-
   identical 跟 hb-2-spec §3.3); 反向断 HB-1 7-dict + AL-1a 6-dict 字面 0
   hit (3 字典分立反约束守).

3. **read-only consumer 接口锚** — `internal/grants/grants.go` Consumer
   interface + MemoryConsumer mock; HB-3 落地后 SQLite consumer 直接实现
   同 interface 不改 daemon 一行 (HB-3 spec brief 待起).

4. **sandbox build tag 拆死** — `internal/sandbox/sandbox_{linux,darwin,
   other}.go` v0(C) no-op stub (反向断 build tag 选对); 真 landlock /
   sandbox-exec 调用留 v0(D) (依赖 go-landlock dep, HB-1 #589 binary 后接).

## 1. 拆段实施 (单 PR 全闭)

| 段 | 文件 | 范围 |
|---|---|---|
| HB-2 v0(C).1 module | `packages/borgee-helper/go.mod` (独立) | separate go.mod 防 server-go binary bloat |
| HB-2 v0(C).2 reasons | `internal/reasons/reasons.go` (27 行) | 8-dict const + All() 反向枚举锚 |
| HB-2 v0(C).3 audit | `internal/audit/audit.go` (48 行) | 5-field SSOT JSON-line writer (HB-1 byte-identical) |
| HB-2 v0(C).4 grants | `internal/grants/grants.go` (94 行) | Consumer interface + MemoryConsumer mock |
| HB-2 v0(C).5 acl | `internal/acl/acl.go` (128 行) | path normalize + cross-agent + grants gate + 写类 100% reject |
| HB-2 v0(C).6 ipc | `internal/ipc/ipc.go` (151 行) | handshake + JSON-line protocol + multiplex + audit on reject |
| HB-2 v0(C).7 sandbox | `internal/sandbox/sandbox_{linux,darwin,other}.go` (61 行) | build-tag stub; Apply no-op (留 v0(D)) |
| HB-2 v0(C).8 main | `cmd/borgee-helper/main.go` (98 行) + `main_other.go` (16 行) | UDS listener + signal shutdown + log audit (POSIX); other = 提示 v0(D) 接 |
| HB-2 v0(C).9 closure | REG-HB2-001..006 + acceptance + PROGRESS [x] | 4 立场 byte-identical 锁 |

合计 production: ~623 行 (目标 ~400-500 微超, daemon main.go 占 98 行
合理 — UDS lifecycle + signal handler).

## 2. 反向 grep 锚

```
git grep -nE 'package server' packages/borgee-helper/  # 0 hit (模块拆死)
git grep -nE 'grantsCache|cachedGrants' packages/borgee-helper/internal/  # 0 hit (反约束 #3)
git grep -nE 'exec\.Cmd|exec\.Command' packages/borgee-helper/internal/  # 0 hit (反约束 #1 零写命令)
git grep -nE 'manifest_signature|network_unreachable' packages/borgee-helper/internal/reasons/  # 0 hit (字典分立)
```

## 3. 不在本轮范围 (留 v0(D) / HB-3)

- ❌ Windows Named Pipe 真启 (留 v0(D) — go-winio dep)
- ❌ 真 landlock LSM 调用 (留 v0(D) — go-landlock dep, HB-1 #589 binary 后接)
- ❌ 真 sandbox-exec profile 生成 (留 v0(D))
- ❌ systemd unit / launchd unit 文件 (留 v0(D) — install-butler 拉起路径)
- ❌ HB-3 host_grants SQLite consumer 真实现 (留 HB-3 spec brief)
- ❌ 文件 IO 真启 (read_file/list_files action handler — v0(C) 仅 ACL 决策, 真 IO 留 v0(D))
- ❌ 网络出站 outbound proxy 真接 (留 v0(D))

## 4. 4 步路径锚 (HB stack)

1. ✅ HB stack Go spec patch #599
2. ✅ HB-2.0 prerequisite #605 (CI matrix + IPC primitive smoke)
3. **本 PR HB-2 v0(C)** — Go daemon binary 真启 (UDS + handshake + ACL + audit + sandbox stub)
4. ⏸️ HB-3 — host_grants schema + 弹窗 + 真接 grants_consumer (本 PR Consumer interface 是接入点)

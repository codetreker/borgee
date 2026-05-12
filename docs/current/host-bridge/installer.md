# Installer

`borgee-installer` 是 helper 安装器，不是 helper daemon 本身。它负责拉取 manifest、验证 ed25519 signature、显示权限确认、执行平台部署命令；它不创建 server admin route，也不直接写 `host_grants`。

```text
borgee-installer-linux / borgee-installer-darwin
  -> require manifest URL + pubkey + local artifact
  -> optional Bearer token
  -> Fetch manifest
  -> Verify ed25519 signature
  -> Confirm permission prompt
  -> deploy Linux .deb/systemd or macOS .pkg/launchd
```

## 负责什么

Linux installer 负责 `.deb` + systemd 部署。CLI 必填 `--manifest-url`、`--pubkey-base64`、`--deb`，可选 `--bearer-token`、`--dry-run`；部署步骤是 `sudo apt install`、`systemctl daemon-reload`、enable/start `borgee-helper.service`。证据：`packages/borgee-installer/cmd/borgee-installer-linux/main.go`、`packages/borgee-installer/internal/deploy/deploy.go`。

macOS installer 负责 `.pkg` + launchd 部署。CLI 必填 `--manifest-url`、`--pubkey-base64`、`--pkg`，可选 `--bearer-token`、`--dry-run`；部署步骤是 `sudo /usr/sbin/installer` 和 `launchctl load /Library/LaunchDaemons/cloud.borgee.host-bridge.plist`。证据：`packages/borgee-installer/cmd/borgee-installer-darwin/main.go`、`packages/borgee-installer/internal/deploy/deploy.go`。

manifest client 负责 HTTP GET、8 MiB body limit、decode envelope、ed25519 detached signature verify。签名覆盖 canonical JSON `{entries, signed_at}`；失败原因字典包括 `manifest_signature_invalid`、`manifest_fetch_failed` 等 7 个值。证据：`packages/borgee-installer/internal/manifest/fetcher.go`。

dialog 负责权限确认文本并等待用户输入 `y/yes`。当前 grant type 列表是 `read/write/exec/network`，提示文本逐项解释读、写、执行、网络能力。证据：`packages/borgee-installer/internal/dialog/dialog.go`。

## 不负责什么

installer 不负责运行时授权决策。安装完成后，实际请求仍由 helper 的 ACL、SQLite grant lookup 和 sandbox 决定。证据：`packages/borgee-helper/internal/acl/acl.go`、`packages/borgee-helper/internal/grants/sqlite_consumer.go`、`packages/borgee-helper/internal/sandbox/sandbox_linux.go`。

installer 不负责 admin API。代码注释和命令入口都表明它使用用户 sudo 和本地 artifact，不添加 installer admin API path。证据：`packages/borgee-installer/cmd/borgee-installer-linux/main.go`、`packages/borgee-installer/cmd/borgee-installer-darwin/main.go`。

installer 不负责 remote-agent。它部署的是 `borgee-helper` host-bridge artifact；remote-agent 是 Node CLI，通过 `/ws/remote` 连接 server。证据：`packages/remote-agent/src/index.ts`、`packages/borgee-installer/internal/deploy/deploy.go`。

## 和其他模块的接口

| 模块 | 接口 | 说明 | 证据 |
| --- | --- | --- | --- |
| server manifest endpoint | HTTP GET with optional Bearer | installer fetches manifest envelope | `packages/borgee-installer/internal/manifest/fetcher.go` |
| server auth | Bearer token optional | request sets `Authorization: Bearer <token>` if provided | `packages/borgee-installer/internal/manifest/fetcher.go` |
| helper service | platform package manager | installer deploys service unit, then system service owns daemon lifecycle | `packages/borgee-installer/internal/deploy/deploy.go` |
| user/operator | stdin/stdout confirmation | install continues only after `Confirm` returns true | `packages/borgee-installer/internal/dialog/dialog.go` |

## Current server manifest shape

server 的 `/api/v1/plugin-manifest` 当前返回 `manifest_version`、`issued_at`、`expires_at`、`signature`、`plugins`，并通过 `authMw` 挂在 user rail；该 endpoint 没有 admin path。证据：`packages/server-go/internal/api/host_manifest.go`、`packages/server-go/internal/server/server.go`。

installer manifest client 当前期望 envelope 字段是 `entries`、`signed_at`、`signature`，并对 `{entries, signed_at}` 做 canonical verification。证据：`packages/borgee-installer/internal/manifest/fetcher.go`。

## Known risk / unknown

- installer 期望的 manifest shape 与 server 当前 `/api/v1/plugin-manifest` shape 不一致：installer 要 `entries/signed_at`，server 返回 `plugins/issued_at/expires_at/manifest_version`。证据：`packages/borgee-installer/internal/manifest/fetcher.go`、`packages/server-go/internal/api/host_manifest.go`。
- installer dialog 的 grant type 是 `read/write/exec/network`，server `host_grants` schema 和 REST whitelist 是 `install/exec/filesystem/network`。证据：`packages/borgee-installer/internal/dialog/dialog.go`、`packages/server-go/internal/migrations/host_grants.go`、`packages/server-go/internal/api/host_grants.go`。
- server manifest handler `SigningKey` nil 时会生成测试占位 signature；生产私钥注入路径未在已核对 server wiring 中看到。证据：`packages/server-go/internal/api/host_manifest.go`、`packages/server-go/internal/server/server.go`。

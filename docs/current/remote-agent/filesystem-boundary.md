# Remote Agent Filesystem Boundary

remote-agent 的文件边界是进程内 allowlist，不是 OS sandbox。用户在启动 CLI 时通过 `--dirs` 明确暴露目录；之后所有 `ls/read/stat` 都在这些目录下做 `path.resolve` 前缀检查。

```text
--dirs=/a,/b
  -> allowedDirs ["/a", "/b"]
  -> request path P
  -> path.resolve(P)
  -> resolved == allowed dir OR startsWith(allowed dir + path.sep)
  -> fs.readdirSync / fs.statSync / fs.readFileSync
```

## 负责什么

CLI 负责收集本地目录边界。`borgee-remote-agent` 要求 `--dirs <dirs>`，按逗号拆分、trim、过滤空值；如果结果为空则退出。证据：`packages/remote-agent/src/index.ts`。

文件边界负责路径 allowlist。`isPathAllowed` 对请求路径和每个 allowed dir 都执行 `path.resolve`，只接受等于 allowed dir 或位于 allowed dir 子路径下的请求。证据：`packages/remote-agent/src/fs-ops.ts`。

文件操作负责只读返回。`ls` 使用 `fs.readdirSync` 和 `fs.statSync` 返回 name、directory flag、size、mtime；`readFile` 拒绝目录、限制最大 2 MiB、按 UTF-8 读取并返回 MIME；`stat` 返回 size、mtime、isDirectory。证据：`packages/remote-agent/src/fs-ops.ts`。

## 不负责什么

remote-agent 不负责绝对路径强制。代码对目标路径执行 `path.resolve`，相对路径会相对 agent 进程工作目录解析；是否落在 allowed dir 内由 resolved prefix 判断。证据：`packages/remote-agent/src/fs-ops.ts`。

remote-agent 不负责 symlink 展开后的真实路径边界。当前检查使用 `path.resolve`，没有 `fs.realpathSync`；如果 allowed dir 内存在 symlink，最终打开行为由 Node/OS 文件系统处理，不由 remote-agent 额外校验。证据：`packages/remote-agent/src/fs-ops.ts`。

remote-agent 不负责 host grant、ACL audit、Landlock 或 sandbox-exec。那些是 helper daemon 的 host-bridge 边界；remote-agent 的文件读取没有 host grant SQLite lookup，也没有 JSONL audit writer。证据：`packages/borgee-helper/internal/acl/acl.go`、`packages/borgee-helper/internal/audit/audit.go`、`packages/borgee-helper/internal/sandbox/sandbox_linux.go`、`packages/remote-agent/src/fs-ops.ts`。

## 和其他模块的接口

| 模块 | remote-agent filesystem 如何交互 | 证据 |
| --- | --- | --- |
| server REST remote API | REST handler 只把 `path` query 转成 proxy params；不在 server 端做本地路径校验 | `packages/server-go/internal/api/remote.go` |
| server WebSocket hub | hub 只转发 request/response；不解释文件系统结果 | `packages/server-go/internal/ws/remote.go` |
| remote-agent protocol | `handleRequest` 根据 action 调用 `ls/readFile/stat` | `packages/remote-agent/src/agent.ts` |
| host-bridge/helper | 无共享授权机制；helper 用 `host_grants` 和 UDS IPC，remote-agent 不使用 | `packages/borgee-helper/internal/ipc/ipc.go`, `packages/borgee-helper/internal/grants/sqlite_consumer.go` |

## 错误和限制

| 条件 | remote-agent error | server HTTP 映射 | 证据 |
| --- | --- | --- | --- |
| path outside `--dirs` | `path_not_allowed` | 403 | `packages/remote-agent/src/fs-ops.ts`, `packages/server-go/internal/api/remote.go` |
| read missing file | `file_not_found` | 404 | same |
| read over 2 MiB | `file_too_large` | 413 | same |
| read directory | `is_directory` | 502 | same |
| `ls/stat` missing path | `path_not_found` | 502 by current mapper | same |
| proxy timeout | server `context.DeadlineExceeded` | 504 | `packages/server-go/internal/ws/remote.go`, `packages/server-go/internal/api/remote.go` |

## Known risk / unknown

- `MAX_FILE_SIZE` 是 2 MiB，remote-agent 使用 UTF-8 读取；二进制文件即使 MIME 命中 image 类型，也会按 UTF-8 content 返回。证据：`packages/remote-agent/src/fs-ops.ts`。
- 目录 listing 没有数量上限；大目录会同步读取并 stat 每个 entry。证据：`packages/remote-agent/src/fs-ops.ts`。
- server 目前把 action `read/ls` 发在 `data.params.path`，而文件边界函数期望 agent 收到直接 path；实际联通性取决于该 wire mismatch 是否被修正。证据：`packages/server-go/internal/server/server.go`、`packages/remote-agent/src/agent.ts`。

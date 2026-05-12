# Borgee Current — 代码现状(as-is audit)

> 这一目录是**直接读源码**梳理出来的现状文档 —— Borgee 当前代码**实际**长什么样。
> 与 blueprint 的关系: 现状是起点, blueprint 是终点。

## 这是什么 / 不是什么

| 是 | 不是 |
|----|------|
| 代码实现的说明 | 产品设计或 should-be 规范 |
| audit 时间点的快照 | 当下代码的镜像(代码会继续变化) |
| 给"想理解代码的人"看的 | 给"想理解产品的人"看的 |

> **如果想知道"应该是什么样"** → 见 [`../blueprint/`](../blueprint/)

---

## 文档导航

| 文档 | 内容 |
|------|------|
| [`overview.md`](overview.md) | 系统全景: 进程拓扑 + 跨进程消息流 + 写路径扇出 |
| [`agents.md`](agents.md) | Agent 在代码中的身份模型 + API key + 接入路径 |
| [`server/`](server/README.md) | server-go: 路由、auth、realtime |
| [`server/data-model.md`](server/data-model.md) | SQLite 表结构、迁移策略、事件日志、LexoRank |
| [`client/`](client/README.md) | React SPA: 状态模型、WS 协议、slash command |
| [`client/ui/`](client/ui/README.md) | 用户端线框图: 登录 / 主界面 / 消息 / DM / 命令 等 |
| [`admin/`](admin/README.md) | 管理面: 双前缀路由、独立 cookie、admin SPA |
| [`admin/ui/`](admin/ui/README.md) | Admin 管理后台线框图 |
| [`plugin/`](plugin/README.md) | OpenClaw plugin: 传输自适应、多账号、消息分发 |
| [`remote-agent/`](remote-agent/README.md) | remote-agent daemon: 协议、沙箱、与 server 的对偶 |
| [`remote-agent/ui/`](remote-agent/ui/README.md) | Remote Explorer 线框图 |

## 仓库结构

```
packages/
├── server-go/          # Go 后端: HTTP API + WS + SSE + SQLite
├── client/             # React 18 SPA(用户端 + admin 端, 双 Vite 入口)
├── remote-agent/       # Node daemon, 暴露受限的本地文件系统
└── plugins/openclaw/   # OpenClaw Channel Plugin
```

## 进程拓扑(运行时)

```
┌────────────────┐  WS / REST   ┌───────────────────────────────┐
│  Browser SPA   │◀────────────▶│                               │
│  (client)      │              │       server-go               │
└────────────────┘              │  ┌──────────┬──────────────┐  │
                                │  │ HTTP API │ /ws (clients)│  │
┌─────────────────┐  WS         │  │          │ /ws/plugin   │  │
│ OpenClaw plugin │◀────────────│  │  SQLite  │ /ws/remote   │  │
│  (per agent)    │  SSE / poll │  │  events  │ SSE /stream  │  │
└─────────────────┘             │  └──────────┴──────────────┘  │
                                └───────────────────────────────┘
┌─────────────────┐  WS /ws/remote
│  remote-agent   │◀──── token ──────┘
│  (filesystem)   │
└─────────────────┘
```

详细写路径扇出与 cursor/事件日志机制见 [`overview.md`](overview.md)。

## 维护说明

- 现状文档**会过时**: 代码改动后, 这里的描述需要更新
- 更新触发: 重大重构 / 新模块加入 / 原假设被颠覆
- Non-goals: 不靠 CI 自动同步, 不固定到 line:column 这类代码细节, 保持"读得动"优先于"机器可读"

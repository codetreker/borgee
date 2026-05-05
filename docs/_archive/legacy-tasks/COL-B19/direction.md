# COL-B19: Remote Explorer — 方向文档

日期：2026-04-22 | 状态：Direction (讨论确认)

## 1. 概述

Remote Explorer 让用户在 Collab 浏览器界面中浏览远程机器上的文件，无需 SSH。用户场景：建军在手机上想看 Agent 正在改什么代码、日志输出了什么。

## 2. 核心设计决策

| 决策项 | 结论 | 决策人 |
|--------|------|--------|
| 读写 | **只读** v1 | 建军 |
| 可见性 | **仅 owner** 可见（API 校验） | 建军 |
| 多机器 | 一个 user 可绑多台机器（"我的阿里云"、"我的 oc-apps"） | 建军 |
| 目录绑定 | 用户手动指定目录，可绑多个 | 建军 |
| 绑定层级 | Channel 级绑定，但仅 owner 可见 | 建军 |
| 实时性 | v1 手动刷新 | 建军 |
| 存储 | 绑定关系存数据库（SQLite） | 飞马+野马 |

## 3. 架构

### 3.1 连接模型

- 远程机器上运行轻量 agent 进程（`@collab/remote-agent`）
- Agent 主动 WebSocket 连到 Collab server（outbound，穿 NAT）
- 每台机器生成唯一 ID + 携带机器名
- 连接时需认证（owner 在 UI 生成 connection token）

### 3.2 数据库

```sql
remote_nodes (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  machine_name TEXT NOT NULL,
  connection_token TEXT NOT NULL,
  last_seen_at TEXT,
  created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

remote_bindings (
  id TEXT PRIMARY KEY,
  node_id TEXT NOT NULL REFERENCES remote_nodes(id),
  channel_id TEXT NOT NULL REFERENCES channels(id),
  path TEXT NOT NULL,        -- 绑定的目录路径
  label TEXT,                -- 显示名（如 "collab 源码"）
  created_at TEXT DEFAULT CURRENT_TIMESTAMP
);
```

### 3.3 数据流

```
浏览器 → Collab Server → WebSocket → Remote Agent → 文件系统
                                                      ↓
                                              ls / read / stat
```

### 3.4 API

- `GET /api/v1/remote/nodes` — 当前用户的所有 node
- `POST /api/v1/remote/nodes` — 生成 connection token
- `DELETE /api/v1/remote/nodes/:id`
- `GET /api/v1/remote/nodes/:nodeId/ls?path=...` — 列目录
- `GET /api/v1/remote/nodes/:nodeId/read?path=...` — 读文件
- `POST /api/v1/remote/bindings` — 绑定目录到 channel
- `DELETE /api/v1/remote/bindings/:id`

所有 API 校验 `user_id === owner`。

### 3.5 前端

- 侧边栏 "Remote" tab
- 文件树组件（与 Workspace 共用）
- FileViewer 渲染：代码高亮 + Markdown + 图片 + 文本 fallback + 二进制提示

## 4. 未来扩展（不在 v1）

- 实时文件监听（fs.watch → push 到前端）
- 聊天联动：Agent 消息中的本地路径自动变可点击链接（需 Plugin 配合）
- 可写操作

## 5. 依赖

- 共享 FileViewer 组件（与 B20 Workspace 共用）

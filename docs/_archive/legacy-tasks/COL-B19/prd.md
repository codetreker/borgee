# COL-B19: Remote Explorer — PRD

日期：2026-04-22 | 状态：Draft

## 背景
用户（Agent 的 owner）想在 Collab 浏览器端查看远程机器上的文件，不需要 SSH。Agent 机器上跑轻量进程，主动 WS 连到 Collab server。

## 目标用户
- Agent 的 owner（只有 owner 能看到远程文件）

## 核心需求

### 需求 1: Remote Agent 连接
- 用户故事：作为 owner，我在远程机器上启动 agent 进程，它自动连接到 Collab server
- 验收标准：
  - [ ] 独立 npm 包 `@collab/remote-agent`，一行命令启动
  - [ ] 启动参数：`--server wss://collab.codetrek.cn --token xxx --dirs /path1,/path2`
  - [ ] Agent 主动 WS 连到 server（穿 NAT）
  - [ ] Token 认证（owner 在 UI 生成）
  - [ ] 断开自动重连（指数退避）

### 需求 2: 机器管理
- 用户故事：作为 owner，我想管理我的远程机器列表
- 验收标准：
  - [ ] 注册多台机器（唯一 ID + 机器名）
  - [ ] 查看机器在线状态（last_seen_at）
  - [ ] 删除已不使用的机器

### 需求 3: 目录绑定
- 用户故事：作为 owner，我想把远程机器的目录绑定到频道
- 验收标准：
  - [ ] 用户指定要暴露的目录（可多个）
  - [ ] 绑定关系存数据库（remote_bindings 表）
  - [ ] 频道级绑定，但只对 owner 可见

### 需求 4: 文件浏览
- 用户故事：作为 owner，我想在浏览器里浏览远程机器的文件目录
- 验收标准：
  - [ ] 侧边栏文件树 UI（Remote tab）
  - [ ] 浏览绑定目录的文件/文件夹
  - [ ] v1 手动刷新（不做实时监听）
  - [ ] 只读（不能修改/删除远程文件）

### 需求 5: 文件预览
- 用户故事：作为 owner，我想点击远程文件在浏览器里查看内容
- 验收标准：
  - [ ] 复用 FileViewer 组件（Markdown/代码高亮/图片/文本/二进制）
  - [ ] 通过 WS 读取文件内容
  - [ ] Agent 离线时提示

### 需求 6: 存储
- 验收标准：
  - [ ] Server SQLite：`remote_nodes` + `remote_bindings` 两张表
  - [ ] 多用户隔离（目录和绑定关系加 user 层级）

## 不在范围
- 文件编辑/写入 — v2
- 实时文件监听（fs.watch）— v2
- 消息中路径自动链接（B22 已做）

## 成功指标
- 远程文件浏览成功率 > 95%（Agent 在线时）
- Agent 连接稳定性 > 99%

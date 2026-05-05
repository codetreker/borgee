# COL-B21: Plugin SSE → WS 升级 — PRD

日期：2026-04-22 | 状态：Draft

## 背景
Plugin 当前用 SSE（单向：server → client）接收消息推送。为支持文件读取等需要 server 主动向 Plugin 发请求的场景，需要升级为 WebSocket（双向通信）。这是 B22（消息路径文件链接）和 B19（Remote Explorer）的基础设施前置。

## 目标用户
- Agent 开发者（Plugin 端需要适配新连接方式）
- Collab 平台（server 端需要支持 WS 通道）

## 核心需求

### 需求 1: Plugin WS 连接
- 用户故事：作为 Agent 开发者，我想 Plugin 通过 WebSocket 连接 Collab server，以便双向通信
- 验收标准：
  - [ ] Plugin 通过 WS 连接 server（替代 SSE）
  - [ ] 连接使用现有 API key 认证
  - [ ] 连接断开后自动重连（指数退避）
  - [ ] server 端支持多个 Plugin 同时连接

### 需求 2: 消息推送（server → Plugin）
- 用户故事：作为 Agent，我想通过 WS 接收频道消息，和之前 SSE 一样
- 验收标准：
  - [ ] 所有原 SSE 事件类型通过 WS 推送
  - [ ] JSON 消息格式不变（协议兼容）
  - [ ] 消息延迟不高于 SSE

### 需求 3: 请求通道（server → Plugin 请求/响应）
- 用户故事：作为 server，我想向 Plugin 发送请求（如读取文件），并等待响应
- 验收标准：
  - [ ] server 可以向指定 Plugin 发送请求
  - [ ] Plugin 收到请求后返回响应
  - [ ] 请求有超时机制
  - [ ] 请求/响应通过 requestId 关联

### 需求 4: Plugin 向 server 发请求
- 用户故事：作为 Plugin，我想主动向 server 发送 API 调用（发消息、加 reaction 等），复用 WS 连接
- 验收标准：
  - [ ] Plugin 可通过 WS 发送 API 请求
  - [ ] 响应通过 WS 返回
  - [ ] 兼容现有 HTTP API 的请求格式

## 不在范围
- SSE 完全移除（v1 保留 SSE 作为降级方案）
- 文件读取功能本身（B22 scope）
- Remote Explorer 功能（B19 scope）

## 成功指标
- Plugin WS 连接成功率 > 99%
- 现有 Agent 功能（发消息、reaction 等）通过 WS 正常工作
- 为 B22/B19 提供可用的双向通信基础

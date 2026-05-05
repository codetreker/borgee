# COL-B21: Plugin SSE → WS 升级 — 方向文档

日期：2026-04-22 | 状态：Discussion

## 背景
Plugin 当前用 SSE（单向：server → client）接收消息。为支持文件读取等双向通信需求，需要升级为 WebSocket。

## 决策记录（2026-04-22 讨论）

### 为什么升级
- SSE 单向，server 无法主动向 Plugin 发请求（如"读取某个文件"）
- HTTP callback 需要 Plugin 暴露端口，NAT 后面的机器不行
- WS 是 Plugin 主动连出，天然穿 NAT
- 长期还需要：server 推任务给 Agent、实时协同、Agent 心跳

### 方案
- 一步到位 WS（建军、飞马、野马三方共识，不做 SSE + HTTP callback 过渡）
- 协议格式不变（JSON），只换通道
- Server 端：SSE endpoint → WS endpoint（@fastify/websocket）
- Plugin 端：SSE client → WS client
- 保留 SSE 长轮询作为降级方案（可选）

### 改动范围
1. Server：新增 WS endpoint for Plugin
2. Plugin：连接层从 SSE 改 WS
3. 认证：复用现有 API key 认证
4. 事件格式：不变

## 依赖
- 无外部依赖
- B22（消息路径文件链接）依赖本任务

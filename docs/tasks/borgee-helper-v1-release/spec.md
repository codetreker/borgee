# 681 — Remote-Agent 网页配 OpenClaw

> Issue: https://github.com/codetreker/borgee/issues/681
> 优先级: P1 / backlog (跟 host-bridge 大重写一起做)

## 要做什么

扩展 remote-agent, 用户能在网页上一键配 OpenClaw:

1. 网页装 plugin
2. 创建 agent 并配 channel
3. 给已有 agent 配 channel

## 蓝图依据

- `blueprint/current/agent-lifecycle.md §2.2 默认路径` — remote-agent 升级为 runtime 安装管家
- `blueprint/current/host-bridge.md §3` — host-bridge 大重写 + install-butler 新建

## 不做什么

- 不含 Hermes / 自建 runtime 的网页配 (蓝图 v1 只 OpenClaw)
- 不含 Windows 网页配 (蓝图 v1 只 Mac/Linux)
- 不含 plugin marketplace UI (v2+)
- 不含 power user 直连路径 (蓝图明文保留)
- 不含远程主机配置 (只配本机)

## 关联

- heima Security pre-work 见 `heima-prework.md`
- 跟 host-bridge 大重写 milestone 一起做, 不单独切小 PR

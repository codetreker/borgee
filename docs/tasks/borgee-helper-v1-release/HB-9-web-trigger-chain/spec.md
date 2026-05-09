# HB-9 — 网页配 OpenClaw 触发链

> Wave: borgee-helper-v1-release
> Issue: gh#681
> Status: placeholder — 真起这 milestone 时由 4 角色完整 4-piece 替换

## 入口

HB-7 install-butler binary 已合; HB-3 host_grants endpoint 已落 (#507 / #520).

## 出口

网页"添加 agent" → server 下发一次性 token → install-butler 走 osascript / pkexec OS prompt → host-bridge 注册到 server, 真跑通; install 全机 1 次 rate limit.

## 蓝图引用

- `agent-lifecycle.md §2.2` 默认路径
- `heima-prework.md §1.1` (sudo 走 OS 原生 prompt 不 cache)

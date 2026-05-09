# HB-10 — 4 类授权弹窗 UI + per-agent SQLite

> Wave: borgee-helper-v1-release
> Issue: gh#681
> Status: placeholder — 真起这 milestone 时由 4 角色完整 4-piece 替换

## 入口

HB-3 schema + REST 已落; HB-7 host-bridge daemon SQLite 启动路径已开.

## 出口

装机 2 类 (install/exec) + 触发 2 类 (filesystem/network) UI 弹窗; per-agent 授权写 host-bridge 本地 SQLite (file 0600); JSX 转义 user-controlled 字段; 拒绝 60s 冷却; 默认 focus 取消按钮.

## 蓝图引用

- `host-bridge.md §1.3` 情境化授权 4 类
- `heima-prework.md §1.3` (XSS / SQLite 0600 / 冷却期 60s)

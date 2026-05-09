# HB-11 — 一键完全卸载完整链

> Wave: borgee-helper-v1-release
> Issue: gh#681
> Status: placeholder — 真起这 milestone 时由 4 角色完整 4-piece 替换

## 入口

HB-7 + HB-9 + HB-10 已落 (卸载需要清的全集才齐).

## 出口

二进制 / 配置 / runtime / server 注册 / OS user-group / launchd / systemd unit 全清; 反向 grep `~/.borgee\|/var/lib/borgee-*\|Application Support/Borgee` 残留 0 hit; server 注销走当前 user cookie 不接 client-supplied user_id.

## 蓝图引用

- `host-bridge.md §1.2 D` 一键完全卸载
- `heima-prework.md §1.2 D` 三风险 (路径注入 / 残留 token / IDOR)

# HB-12 — 安全补丁 banner UX

> Wave: borgee-helper-v1-release
> Issue: gh#681
> Status: placeholder — 真起这 milestone 时由 4 角色完整 4-piece 替换

## 入口

HB-7 binary version 检查路径已开.

## 出口

启动时 banner 显眼提示安全补丁 + 一键确认 (默认 focus 取消, 反误升级); 功能更新藏在设置面板; 反向 grep `auto.{0,3}update\|silentUpdate` 0 hit.

## 蓝图引用

- `host-bridge.md §1.2 C` 更新策略 — 分类不自动
- `heima-prework.md §1.2 C` ("一键确认"默认 focus 取消)

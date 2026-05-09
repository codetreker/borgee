# HB-7 — install-butler binary 真实施

> Wave: borgee-helper-v1-release
> Issue: gh#681
> Status: placeholder — 真起这 milestone 时由 4 角色完整 4-piece 替换

## 入口

HB-1 server endpoint 已落 (#491); HB-2 host-bridge daemon 已落 (#617); HB-8 manifest signing 设计已 freeze.

## 出口

短命 binary 跑 install / verify / uninstall 三动作; manifest 双签 (SHA256 + GPG) 真验通过; 反向 grep `child_process.exec\|shell:\s*true` 0 hit.

## 蓝图引用

- `host-bridge.md §1.1` 双 daemon (install-butler 短命特权 + host-bridge 常驻无 sudo)
- `host-bridge.md §1.2 A` 白名单 + 双签
- `heima-prework.md §1.1` + `§1.2 A`

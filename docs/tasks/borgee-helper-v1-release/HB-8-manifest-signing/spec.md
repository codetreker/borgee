# HB-8 — manifest signing toolchain

> Wave: borgee-helper-v1-release
> Issue: gh#681
> Status: placeholder — 真起这 milestone 时由 4 角色完整 4-piece 替换

## 入口

HB-1 server endpoint (#491) 已落; HSM / 离线签流程 yema + heima 联签.

## 出口

server 端 GPG 私钥 HSM-only; cert pinning fingerprint 编译期 const; manifest URL hardcode 反 env 覆盖; key rotation runbook 落 `docs/current/server/api/host-grants.md` 同模式.

## 蓝图引用

- `host-bridge.md §1.2 A` 白名单 + 双签 manifest
- `heima-prework.md §1.2 A` 三风险 (key 泄露 / TOCTOU / DNS 劫持)

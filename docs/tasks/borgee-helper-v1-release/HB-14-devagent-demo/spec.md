# HB-14 — DevAgent v1 release demo 真跑通 (closure milestone)

> Wave: borgee-helper-v1-release
> Issue: gh#681
> Status: placeholder — 真起这 milestone 时由 4 角色完整 4-piece 替换 (wave closure milestone, 4 角色联签 = Dev + PM + QA + Security)

## 入口

HB-7 ~ HB-13 全合.

## 出口

用户 @DevAgent → DevAgent 通过 OpenClaw shell tool 执行 pytest → 结果回流到 channel + workspace artifact; 沙箱 (Linux landlock + macOS Seatbelt) 真生效, 反 "为了 demo 禁沙箱"; liema e2e 真 UI 走 Playwright 验, 反 page.evaluate 假绿.

## 蓝图引用

- `host-bridge.md §1.5` v1 release 硬指标
- HB-4.2 deferred (closure 兑现)

# 681 进度

## 2026-05-14 Blueprintflow repair

- 状态修正: 本目录是 legacy intake, 不是当前执行 task。下一版状态已迁到 `docs/blueprint/next/README.md` 的 `HB-RA-1` anchor。
- 当前状态: `HB-RA-1` 仍是 `OPEN / PENDING`; 未进入 Phase/Milestone planning, 也未进入 milestone breakdown 或 task execution。
- 开工前必须先解决或拆分 lock blockers: sandbox profile、helper credential model、manifest / artifact binding、revoke race rules、Helper vs Remote Agent boundary。
- 禁止直接按旧 `扩展 remote-agent` 口径开 PR; 新口径是 Helper bounded remote actuator, Remote Agent file-proxy rail 与 host-management credentials / grants / enforcement rails 保持分离。

- heima Security pre-work: 已做 (见 heima-prework.md, 来自 wip/681-remote-agent-openclaw 分支)
- yema 产品视角判: 已做 (issue body 已含)
- 实施: 未开工, 等 host-bridge 大重写 milestone 起

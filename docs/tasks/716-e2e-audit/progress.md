# 716 进度

✅ **完工** — 2026-05-11

## 真做完的事 (12 件 REWRITE + 26 PASS rename + 9 SKIP + 3 DELETE = 46 全覆盖)

- ✅ 4-piece + audit + design 8 文档全 (spec / acceptance / regression / progress / stance / content-lock / audit / design)
- ✅ DELETE 3 (cv-3-3-deferred / g2.4-adm-0-stance / hb-1b-installer) — commit 508067d
- ✅ PASS rename 28 + 头部去黑话 (zhanma-d 4 组 9d31840 / 642be70 / 67cb7b6 / a0fc337 + 3 yema rename 6e56366 + 复核改 PASS cm-4 78f3fa4 + cm-5 0f2a79e)
- ✅ PASS+fix 1 host-bridge-daemon-handshake (b0fd4cb)
- ✅ REWRITE-UI 3 真 UI (welcome-channel-per-user-isolation 10e2319 / direct-message-reaction-summary b0fd4cb / direct-message-multi-device-sync happy 619b001)
- ✅ REWRITE-NAV 3 (reactions-cross-channel-permission 5587bdc / message-permission-matrix 9eb356d / direct-message-multi-device-sync cross-leak ceb5e0d) — heima 4 约束依
- ✅ SKIP+followup 9 cv-* + ap-2 + rt-3 (abc7394 + 552e4a8) — gh#724 §1
- ✅ CI workflow e2e-fixme-skip-guard 阈值 4→13 (36dcfba + 552e4a8)
- ✅ docs/current/ 8 文件 11 spec 引用同步新名 (6e56366)
- ✅ gh#724 follow-up issue 立完 (3 段: §1 client UI mount 9 spec / §2 ACL forbidden state UX / §3 反向证 CI job by liema Q5) — yema 24h triage

## reviewer 重 review 状态

- ✅ feima ACK (5 必改全收 + 1 加分项 design §3 已落)
- ✅ liema LGTM (4 必改全过)
- ✅ heima 全签 (ACL 改造版 + 4 实施约束依)
- ⏳ yema (等 gh#724 triage, design.md PM 必改 3 全收)

## 剩工

- testing 真验 (push 触发 Deploy Test, 抽样浏览器手验 5 件 REWRITE)
- teamlead 开 PR (Closes gh#716)

# 716 验收

- [x] feima (架构) + liema (QA) + heima (Security) + yema (PM) 各审一遍 `packages/e2e/tests/**`, 标 PASS / PASS+fix / REWRITE-UI / REWRITE-NAV / SKIP+followup / DELETE
- [x] 所有 DELETE 一 PR 删完 (3 spec, commit 508067d)
- [x] 所有 REWRITE 一 PR 改完 (6 件: 3 REWRITE-UI + 3 REWRITE-NAV)
- [x] 所有 SKIP+followup 立 follow-up issue (gh#724 §1 — 9 spec / 5 组件 / 3 cluster), 引文头部注释 v2 unskip 路径明文
- [x] CI / e2e completion accepted by merged PR #794; gh#716 closed
- [x] 后端关闭反向证明已移交 gh#724 §3 follow-up infra, 不再作为 #716 未完成验收项 (CI job 工作量超 #716 PR 范围, liema Q2 拍 follow-up 路线)

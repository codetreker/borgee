# 716 验收

- [ ] feima (架构师) + liema (QA) 各审一遍 `packages/e2e/tests/**`, 标 PASS / DELETE / REWRITE
- [ ] 所有 DELETE 一 PR 删完
- [ ] 所有 REWRITE 立 follow-up issue, 单独排进 milestone, 不掩盖
- [ ] CI e2e 全绿, 且每个绿 case 关掉 backend 后必 fail (反向证它真在测产品)

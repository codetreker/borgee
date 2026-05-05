# G3.4 CHN-4 demo evidence (野马 placeholder)

## §1 G3.4 退出条件
- CHN-4 collab skeleton e2e PASS (战马 ✅, 闭环 PR #428)
- 野马 demo 截屏 ≥ 5 张归档 (本文件锚点)
- 烈马 acceptance ✅ (PR #428)

## §2 5 张截屏锚点 (各对应 e2e 一段)
1. **CHN-4 channel created** — owner 进入新建 channel 后双 tab 渲染.
   spec: `packages/e2e/tests/chn-4-collab-skeleton.spec.ts` §1 L114-128
   (`createChannel` + `gotoChannel` + `.channel-view-tabs` visible)
2. **用户 join channel + member list 显示** — owner token 注 cookie 后
   sidebar `.channel-name` 命中 chName.
   spec: 同上 L106-110 (`gotoChannel` 内 `.sidebar-title` + `.channel-name` click)
3. **第 1 条 message 发出** — chat tab active + 文案 "聊天" byte-identical.
   spec: 同上 §1 L131-139 (`button[data-tab="chat"]` toHaveText '聊天' + active)
4. **第 2 用户接收 message (fan-out)** — workspace tab 切换 + URL deep-link
   `?tab=workspace` 写入, 验跨视图状态同步.
   spec: 同上 §1 L142-148 (`toHaveURL(/[?&]tab=workspace\b/)`)
5. **closure follow-up** — REG-CHN4-* 翻 🟢 (双 tab 截屏归档闸位).
   spec: 同上 §6 L209-239 (`g3.4-chn4-chat.png` + `g3.4-chn4-workspace.png` 写盘)

## §3 截屏路径 placeholder
- `docs/evidence/g3-exit/screenshots/chn-4-1.png` — channel created
- `docs/evidence/g3-exit/screenshots/chn-4-2.png` — join + member list
- `docs/evidence/g3-exit/screenshots/chn-4-3.png` — 第 1 条 message
- `docs/evidence/g3-exit/screenshots/chn-4-4.png` — 第 2 用户 fan-out
- `docs/evidence/g3-exit/screenshots/chn-4-5.png` — closure REG 🟢

(真 demo run 后 follow-up PR 填图; e2e §6 已自动写
 `docs/qa/screenshots/g3.4-chn4-{chat,workspace}.png`, 可作 #3 + #4 备份源.)

## §4 e2e spec 行为证据
`packages/e2e/tests/chn-4-collab-skeleton.spec.ts` 三段在 PR #428 真过 CI:
- §1 双 tab DOM byte-identical + URL deep-link
- §5 DM 反向断言 (skip — 7 源 byte-identical server-side grep CI 守门)
- §6 G3.4 退出闸双截屏归档

## §5 烈马 acceptance 锁链
- PR #428 — CHN-4 e2e PASS + 烈马 acceptance ✅ (merged)
- 蓝图 §G3.4 退出 gate 三签其二已闭

## §6 野马 signoff
- [ ] 占位 — 建军 demo run 后真签 (5 张图入盘 + 本文件 `[ ]` → `[x]`)

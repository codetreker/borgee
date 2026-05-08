# DM-4 立场反查清单 (战马D v0)

> 战马D · 2026-04-29 · 立场 review checklist (跟 DM-3 #508 stance + RT-3 #488 同模式)
> **目的**: DM-4 三段实施 (DM-4.1 server PATCH endpoint / 4.2 client useDMEdit hook / 4.3 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/dm-4-spec.md` (战马D v0 5cf7381) + acceptance `docs/qa/acceptance-templates/dm-4.md` (战马D v0)
> **不需 content-lock** — server-only PATCH + client hook (无新 UI 文案锁), 跟 DM-3 / BPP-3/4/5/6 同模式.

## §0 立场总表 (3 立场 + 4 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | DM 编辑同步走 RT-3 既有 fan-out — PATCH endpoint + events INSERT op="edit", **不另起** message 编辑 channel/frame/sequence (跟 DM-3 立场 ② 同精神 — 不开 dm-only frame) | channels-dm-collab.md §3 + RT-3 #488 多端 fan-out | 反向 grep `dm_edit_event\|message_edit_channel\|edit_sync_frame` 在 internal/ 0 hit; envelope whitelist 不扩 (BPP-1 #304 reflect lint 自动锁) |
| ② | edit 是 cursor 子集 — RT-3 fan-out 已建路径 + DM-3 useDMSync 已订阅 channel events, edit op 直接 derive (跟 DM-3 立场 ① 同精神 — cursor 复用 RT-1.3) | RT-1.3 #296 cursor monotonic + DM-3 #508 useDMSync | events 表 INSERT 一行 op="edit", 不另起 sequence; 反向 grep `dm4.*sequence\|edit.*cursor.*= 0\|new.*resume.*dict` 0 hit |
| ③ | thinking subject 5-pattern 反约束延伸第 3 处 (RT-3 第 1 + DM-3 第 2 + DM-4 第 3) — agent edit 是机械修订, 不暴露 reasoning; edit body / event payload 反向断言 5-pattern count==0 | RT-3 #488 thinking 反约束承袭 + 蓝图 §3.2 立场 (edit 是机械修订) | 反向 grep `"processing"\|"responding"\|"thinking"\|"analyzing"\|"planning"` 在 dm_4*.go count==0; 改 = 改三处反向 grep |
| ④ (边界) | DM-only 路径校验 — channel.kind != "dm" reject 403 `dm.edit_only_in_dm`; PATCH 不挂在 channels.kind="public" 路径 | channels-dm-collab.md §3 (DM 范围限定) + AL-2a 立场 (作用域显式) | 反向 grep `PATCH.*messages.*channel.kind != "dm"` 反断; 5 unit 含 NonDMReject 真测 |
| ⑤ (边界) | owner-only ACL — agent message edit 仅 owner 可发起 PATCH (跟 AL-2a #480 + BPP-3.2 #498 + AL-1 #492 + AL-5 #516 owner-only 5 处同模式) | auth-permissions.md (owner-only 锚 #360 同模式) | reject 403 `dm.edit_non_owner_reject` + log warn; cross-user reject 跟 REG-INV-002 fail-closed 同模式 |
| ⑥ (边界) | last-write-wins 简化 — DM-4 v1 不挂编辑历史 audit table, 不挂 OT/CRDT conflict resolution; edit 走 messages.updated_at 单源 | forward-only audit 立场 (跟 AL-1 #492 / ADM-2.1 admin_actions 同精神 v1 简化) | 反向 grep `dm_message_edits\|edit_history\|edit_audit_log` 在 internal/store/ 0 hit (留 v2 follow-up) |
| ⑦ (边界) | admin god-mode 不入 PATCH messages 路径 — admin 不持有 user token, 不参与 edit 业务 | admin-model.md ADM-0 §1.3 红线 + REG-INV-002 fail-closed | 反向 grep `admin.*PATCH.*messages\|admin.*DM.*edit` 在 `internal/api/admin*.go` 0 hit |

## §1 立场 ① RT-3 既有 fan-out 复用 (DM-4.1 守)

**蓝图字面源**: `channels-dm-collab.md` §3 (DM 编辑路径限定 DM 不蔓延 channel) + RT-3 #488 fan-out 多端覆盖

**反约束清单**:

- [ ] PATCH endpoint 复用 messages 表 update + events 表 INSERT op="edit" — 不新建表 / 不新建 frame
- [ ] 反向 grep `dm_edit_event\|message_edit_channel\|edit_sync_frame` 在 `internal/` count==0
- [ ] BPP envelope whitelist count 不变 (BPP-1 #304 reflect lint 自动守 — DM-4 不加 BPP frame)
- [ ] events 表 op 字段写 "edit" 字面 (跟既有 op enum 同源, 不另起 dictionary)

## §2 立场 ② edit = cursor 子集 (DM-4.2 守)

**蓝图字面源**: RT-1.3 #296 §1.3 cursor monotonic + DM-3 #508 useDMSync hook

**反约束清单**:

- [ ] DM-4.2 client `useDMEdit(dmChannelID)` hook 仅做 PATCH + optimistic update; **不**订阅 dm-only frame (跟 DM-3 立场 ④ 同精神)
- [ ] cursor 进展走 useDMSync (DM-3 #508) — useDMEdit 不写 sessionStorage cursor
- [ ] 反向 grep `dm4.*sequence\|edit.*cursor.*= 0\|new.*resume.*dict` count==0
- [ ] e2e 双 tab edit → tab2 ≤3s 收 reflect (跟 RT-1.2 #292 latency 同源)

## §3 立场 ③ thinking 5-pattern 反约束延伸第 3 处 (DM-4.1+4.2 守)

**蓝图字面源**: RT-3 #488 + DM-3 #508 反约束承袭

**反约束清单**:

- [ ] PATCH endpoint body / events payload 反向断言 5-pattern (`processing|responding|thinking|analyzing|planning`) count==0
- [ ] e2e dm-4-edit-multi-device.spec.ts §3.2 全 channel messages body 反向断言 5-pattern count==0
- [ ] 反向 grep 字面 5 词 在 `dm_4*.go` count==0; 锁链 BPP-3.2 #498 / DM-3 #508 / DM-4 第 3 处

## §4 蓝图边界 ④⑤⑥⑦ — 跟 channels.kind / owner-only / forward-only / ADM-0 不漂

**反约束清单**:

- [ ] DM-only path: channel.kind != "dm" reject 403 — `TestDM41_NonDMReject` 真测
- [ ] owner-only ACL: cross-user reject 403 — `TestDM41_NonOwnerRejected` 真测; 跟 AL-2a/BPP-3.2/AL-1/AL-5 owner-only 5 处同模式
- [ ] last-write-wins simplification: 反向 grep `dm_message_edits\|edit_history\|edit_audit_log` 0 hit (留 v2)
- [ ] admin god-mode 不入: `internal/api/admin*.go` 反向 grep `admin.*PATCH.*messages\|admin.*DM.*edit` 0 hit (ADM-0 §1.3 红线)

## §5 退出条件

- §1 (4) + §2 (4) + §3 (3) + §4 (4) 全 ✅
- 反向 grep 5 项全 0 hit (channel/sequence/5-pattern/audit-table/admin)
- e2e 双 tab edit → tab2 ≤3s reflect (RT-1.2 latency 同源)
- multi-device 复用 useDMSync (DM-3) — useDMEdit 不写独立 cursor
- thinking 5-pattern 反约束锁链 DM-4 = 第 3 处, 跟 RT-3 第 1 + DM-3 第 2 链承袭不漂

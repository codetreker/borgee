# CHN-15 立场反查清单 (战马C v0)

> 战马C · 2026-04-30 · CHN-15 channel readonly toggle 立场反查 (跟 CHN-7 #550 / CHN-9 #561 / CHN-13 #583 同模式)

## §0 立场总表 (3 + 3 边界)

| # | 立场 | 蓝图字面 | 反约束 |
|---|---|---|---|
| ① | 0 schema 改 — bit 4 channel-wide 走 creator 单行 SSOT | CHN-3.1 #410 collapsed bitmap 复用 (bit 0=collapse / bit 1=mute / bits 2-3=notif / **bit 4=readonly**) | 反向 grep `ALTER TABLE channels.*readonly|channel_readonly_states|read_only_channels` 0 hit; ReadonlyBit=16 const 双向锁 跟 client `READONLY_BIT` byte-identical |
| ② | owner-only toggle (channel.created_by gate) | 跟 CV-1.2/CV-2 v2/CV-3 v2/CV-4/AL-5/AP-3/CV-6/AL-9/DM-7/DM-8 owner-only ACL 锁链同模式 (第 21 处) | PUT/DELETE 仅 channel.CreatedBy == user.ID; admin god-mode 不挂 (反向 grep `/admin-api/.*readonly` 0 hit, ADM-0 §1.3) |
| ③ | readonly 时 non-creator POST messages → 403 反断 | sender_id != channel.CreatedBy && IsReadonly → 403 `channel.readonly_no_send` 字面 byte-identical 跟 content-lock §3 | admin god-mode 不挂 send (admin 不入业务路径) |
| ④ (边界) | 3 文案 byte-identical | content-lock §1 (`已设为只读 / 已恢复编辑 / 只读频道, 仅创建者可发言`) | 同义词反向 reject (`frozen/locked/lockdown/禁言/封禁/冻结`) 在 user-visible 0 hit |
| ⑤ (边界) | IsReadonly 谓词单源 + SetChannelReadonly 走 creator 单行 | server `IsReadonly(collapsed)` + `SetChannelReadonly(channelID, ro)` Store wrap; 调用方禁止 inline `collapsed & 16` | 反向 grep `collapsed\s*&\s*16` 仅命中 chn_15_readonly.go 一处 + handler 强制 channel.CreatedBy 单行 |
| ⑥ (边界) | AST 锁链延伸 forbidden 3 token | best-effort 立场代码层守 | `pendingReadonly/readonlyQueue/deadLetterReadonly` 在 internal/ 0 hit |

## §1 立场 ① 0 schema 改

- [ ] 反向 grep `ALTER TABLE channels.*readonly|channel_readonly_states|read_only_channels` 在 internal/migrations/ count==0
- [ ] `ReadonlyBit = 16` const 字面单源 (跟 client `READONLY_BIT` byte-identical 双向锁, 跟 CHN-7 MuteBit=2 同模式)
- [ ] CHN-3.1 #410 既有 user_channel_layout 表不动, 仅 app 层扩 bit 4 互斥定位

## §2 立场 ② owner-only toggle

- [ ] PUT /api/v1/channels/{channelId}/readonly user-rail (channel.CreatedBy == user.ID 反断, else 403)
- [ ] DELETE /api/v1/channels/{channelId}/readonly 同 ACL (idempotent unset)
- [ ] admin-rail 0 endpoint 反向断言 (反向 grep `/admin-api/.*readonly|admin.*readonly.*PATCH` 0 hit)
- [ ] owner-only ACL 锁链第 21 处

## §3 立场 ③ readonly 时 non-creator send 403

- [ ] handleCreateMessage 加 readonly gate (sender_id != channel.CreatedBy && Store.GetChannelReadonly → 403 `channel.readonly_no_send`)
- [ ] readonly=true 时 creator 自己仍可 send (反向断言: creator POST messages 200)
- [ ] admin god-mode 不挂 send 路径 (admin 不入业务路径 ADM-0 §1.3)

## §4 立场 ④ 文案 byte-identical

- [ ] 3 文案 (已设为只读 / 已恢复编辑 / 只读频道, 仅创建者可发言) byte-identical 跟 content-lock §1+§3 同源
- [ ] 同义词反向 reject `frozen|locked|lockdown|禁言|封禁|冻结|locked_channel|lock-down` 0 hit

## §5 立场 ⑤ IsReadonly 谓词单源

- [ ] `IsReadonly(collapsed int64) bool` 单源 (跟 IsMuted CHN-7 同模式)
- [ ] `Store.GetChannelReadonly(channelID)` 走 channel.CreatedBy → user_channel_layout 单行查
- [ ] `Store.SetChannelReadonly(channelID, readonly bool)` wrap SetMuteBit (复用 #550 SSOT, bitmask=ReadonlyBit)

## §6 立场 ⑥ AST forbidden token

- [ ] `internal/api` AST scan `pendingReadonly/readonlyQueue/deadLetterReadonly` 0 hit

## §7 联签 (实施 PR 时填)

- [ ] 飞马 (spec ↔ 立场对齐): _(签)_
- [ ] 烈马 (反向 grep + cov ≥84% + 5 反约束 0 hit + 0 schema reverse + bitmap bit 4 互不影响): _(签)_
- [ ] 战马C (实施代码 ↔ 立场反查 6 项全过): _(签)_

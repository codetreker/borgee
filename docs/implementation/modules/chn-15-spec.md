# CHN-15 spec brief — channel readonly toggle (战马C v0)

> Phase 6+ wrapper milestone (跟 CHN-7 mute / CHN-9 visibility / CHN-12 sort / CHN-13 search 同模式) — owner 一键把 channel 切 "只读" 状态: 仅 channel.created_by 可发言, 其他成员发消息 403; owner-only toggle. **0 schema 改** 复用 user_channel_layout.collapsed bitmap (bit 4).
> **蓝图锚**: [`channel-model.md`](../../blueprint/current/channel-model.md) §3 layout per-user (extension) + §1.4 owner 主权 + [`auth-permissions.md`](../../blueprint/current/auth-permissions.md) §1.3 admin god-mode 红线 + ADM-0 §1.3.
> **关联**: CHN-3.1 #410 user_channel_layout schema (collapsed bitmap) + CHN-7 #550 mute (bit 1) + CHN-8 notification-pref (bits 2-3) + CHN-9 visibility (3 enum) + CHN-1.2 channel.created_by gate.

## §0 关键约束 (3 条立场, 蓝图字面承袭)

1. **0 schema 改 — bitmap bit 4 channel-wide 状态走 creator 单行 SSOT** (跟 CHN-7 #550 bit 1 + CHN-8 bits 2-3 同模式 — collapsed INTEGER bitmap 复用扩位): readonly 状态写入 **channel.created_by 的 user_channel_layout.collapsed** bit 4 (=16); 其他成员发消息时 server 查 creator 单行 ↔ bit 4. 反约束: 不裂 channels 列 (反向 grep `ALTER TABLE channels.*readonly|channel_readonly\|read_only_channels` count==0); 反约束: 不写非-creator 的 layout 行 (helper 强制 channel.CreatedBy 入参); ReadonlyBit=16 const 双向锁跟 client `lib/readonly.ts::READONLY_BIT` byte-identical.

2. **owner-only toggle + readonly POST messages 403 反断 ADM-0 §1.3** (跟 CHN-7 mute owner-only ACL 锁链第 15 处 + CV-1.2/CV-2 v2/CV-3 v2/CV-4/AL-5/AP-3 owner-only 锁链同模式): PUT /api/v1/channels/{channelId}/readonly + DELETE 互补二式 — 仅 channel.created_by 可调 (反断: admin god-mode + 跨 member 全 403 — 反向 grep `admin.*PATCH.*readonly\|/admin-api/.*readonly` 0 hit). readonly=true 时, **non-creator** POST `/api/v1/channels/{channelId}/messages` → 403 `channel.readonly_no_send` 字面 byte-identical (跟 content-lock §3 同源); creator 自己仍可发. 反约束: admin god-mode 不挂 send (admin 不入业务路径 ADM-0 §1.3 红线).

3. **3 文案 byte-identical + 同义词反向 grep 0 hit** (跟 CHN-7 mute / CHN-13 search content-lock 同模式): 文案锁三 (`已设为只读` 5 字 set toast / `已恢复编辑` 5 字 unset toast / `只读频道, 仅创建者可发言` 12 字 403 reject body). 反约束: 反向 grep 同义词 `frozen|locked|lock-down|禁言|封禁|冻结|locked_channel|channel_locked|lockdown` 在 packages/client/src/ + packages/server-go/internal/ user-visible 文案 0 hit.

## §1 拆段实施 (CHN-15.1 / 15.2 / 15.3, 一 milestone 一 PR)

| 段 | 范围 | 闭锁 |
|---|---|---|
| **CHN-15.1** server bit 4 + Store helper | `internal/api/chn_15_readonly.go` ReadonlyBit=16 const + IsReadonly(collapsed) 谓词单源 + `Store.GetChannelReadonly(channelID)` (走 channel.CreatedBy → collapsed bit 4 单行查) + `Store.SetChannelReadonly(channelID, readonly bool)` wrap SetMuteBit (复用 #550 单源, 走 creator userID + bitMask=16); 反约束 0 schema (反向 grep migrations/ chn_15_\d+ 0 hit) | 战马C |
| **CHN-15.2** server endpoints + send gate | PUT/DELETE /api/v1/channels/{channelId}/readonly owner-only ACL (channel.CreatedBy == user.ID 反断, else 403); messages.go::handleCreateMessage 既有 path 加 readonly gate (sender != channel.CreatedBy && IsReadonly → 403 `channel.readonly_no_send` 字面 byte-identical 跟 content-lock §3); 反约束 admin-rail 不挂; 9 unit (HappyPath set/unset/idempotent + Non-owner 403 + Admin not mounted + Send blocked non-creator + Send allowed creator + ReadonlyBit byte-identical + 0 schema reverse) | 战马C |
| **CHN-15.3** client + closure | `lib/readonly.ts::READONLY_BIT=16` 双向锁 + `setChannelReadonly/unsetChannelReadonly` API wrapper + `ReadonlyToggle.tsx` (data-testid + data-readonly enum + 文案 byte-identical 跟 content-lock §1 + click toggle); ReadonlyBadge.tsx (`只读` 标签, readonly=true 显示); REG-CHN15-001..006 6 🟢 + acceptance flip + PROGRESS [x] CHN-15 | 战马C |

## §2 留账边界

- v2 schedule readonly (临时只读 N 小时自动恢复) — 留 v2+, 复用 sweeper goroutine 同 AL-7 retention 模式
- v2 readonly 期间 message edit/delete (creator 自己改自己) — 留 v2+, 复用 DM-4 edit path
- v2 readonly 自动 archive (readonly + archive 两态联动) — 留 v2+, 跟 AL-7 retention 同期
- v2 readonly audit log row (admin_actions 写一行 readonly_set/readonly_unset) — 留 v2+
- agent send during readonly (agent 当 creator 时仍可发, 不当 creator 时同 user 403) — v0 自动同精神

## §3 反查 grep 锚 (5 反约束, count==0)

```bash
# 1) 0 schema 反向 — 不另起 channels.readonly 列 / 不裂 channel_readonly_states 表
git grep -nE 'ALTER TABLE channels.*readonly|channel_readonly_states|read_only_channels|channels\.readonly' \
  packages/server-go/internal/migrations/  # 0 hit
# 2) admin god-mode readonly 不挂 PATCH/PUT/DELETE (ADM-0 §1.3 红线)
git grep -nE '/admin-api/[^"\s]*readonly|admin\b[^/\n]*\breadonly.*\bPATCH' \
  packages/server-go/internal/  # 0 hit
# 3) 同义词 user-visible 反向 (frozen / locked / 禁言 / 冻结 等 0 hit)
grep -rE 'frozen|lock-down|禁言|封禁|冻结|locked_channel|channel_locked|lockdown' \
  packages/client/src/ packages/server-go/internal/  # 0 hit user-visible
# 4) IsReadonly 谓词单源 (反向 inline `collapsed & 16` 仅命中 chn_15_readonly.go)
git grep -nE 'collapsed\s*&\s*16|collapsed\s*&\s*int64\(ReadonlyBit\)' \
  packages/server-go/internal/api/  # 仅命中 chn_15_readonly.go::IsReadonly
# 5) AST 锁链延伸 forbidden 3 token
git grep -nE 'pendingReadonly|readonlyQueue|deadLetterReadonly' \
  packages/server-go/internal/  # 0 hit
```

## §4 不在范围

- v2 schedule readonly / message edit / archive 联动 / audit log row / agent runtime override (留 v2+)
- channels schema migration (永久不挂 — bitmap bit 4 单源 SSOT)
- admin god-mode readonly view (永久不挂 ADM-0 §1.3 红线)
- user-rail unread badge readonly indicator (留 client v2+)

## §5 跨 milestone byte-identical 锁

- 跟 CHN-3.1 #410 user_channel_layout 既有列 + collapsed INTEGER bitmap 同源 (CHN-15 复用 bit 4)
- 跟 CHN-7 #550 SetMuteBit Store helper 同源 (改 = 改 SetMuteBit 一处, bitmask 参数化)
- 跟 CHN-8 notification-pref bits 2-3 + CHN-7 mute bit 1 + CHN-3 collapse bit 0 共 collapsed bitmap 4 位互不影响
- 跟 CHN-1.2 #267 channel.created_by gate + CV-1.2/CV-2 v2/CV-3 v2/CV-4/AL-5/AP-3/CV-6/AL-9/DM-7/DM-8 owner-only ACL 锁链 (CHN-15 接第 21 处)
- 跟 CHN-9 visibility 三态 + ADM-0 §1.3 admin god-mode 红线 (admin 不挂)
- 跟 CHN-7/CHN-13 4 件套 + 错码字面单源 三向锁 (server const + client TOAST + content-lock) 同模式

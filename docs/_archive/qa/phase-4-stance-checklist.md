# Phase 4 立场反查表 — PM 用户感知角度跨链 audit (野马 v0)

> **状态**: v0 (野马, 2026-04-30)
> **范围**: Phase 4 大批 merged milestone (~30 个 PR: CV-2..13 / CHN-3..7 / DM-4..7 / AL-5..8 / AP-2..5 / BPP-2..8 / HB-3..5 / DL-4 / ADM-1..2 / CM-5) 跨链立场漂移反查
> **方法**: PM 用户视角 — byte-identical 文案锁链 + owner-only ACL 用户感知 + admin god-mode 私密信息泄漏 + agent/human 同源 四主题. 不复造内置 stance/content-lock (各 milestone 已锁), 仅查跨链一致性.
> **关联**: 蓝图 `concept-model.md` §0 立场 (agent=同事 / 用户主权 / admin 强权但不窥视 / 沉默胜于假活物感) + 14 立场原文锚.

---

## §1 byte-identical 文案锁链跨多 PR audit (用户可见标签)

| 标签字面 | 出处 PR / 文件 | 反向同义词 reject | 链状态 |
|---|---|---|---|
| `已归档` (3 字) | CHN-1 #288 `SortableChannelItem.tsx:83/109` + `ChannelMembersModal.tsx:141` + CHN-2 反向 grep `chn-2 §4.c` (DM 不触发) | `archived/已存档/封存/归档了` 0 hit | ✅ 守住 |
| `已置顶` (3 字, title 属性) | CHN-* `SortableChannelItem.tsx:82` (📌 emoji + title) | `pinned/已 pin/置顶了` 0 hit | ✅ 守住 |
| `已静音` (3 字) | CHN-7 #550 `MutedChannelIndicator.tsx:21` 🔕 + chn-7-content-lock.md §2 + REG-CHN7-006 | `mute/silence/dnd/disturb/quiet/屏蔽/关闭通知/勿扰` 全反向 reject | ✅ 守住 |
| `静音` / `取消静音` (2/4 字 button) | CHN-7 `MuteButton.tsx` + chn-7-stance ⑤ | 同上 | ✅ 守住 |
| `编辑历史` (4 字, modal title) | DM-7 #558 `EditHistoryModal.tsx:48` + dm-7-content-lock §1 + REG-DM7-006 | `history/changes/revisions/版本/修订/变更/回退` 0 hit | ✅ 守住 |
| `(已编辑)` + `已编辑于 {ts}` | `MessageItem.tsx:155-156` (DM-4 / DM-7 共享) | `edited/已修改/已更新` 0 hit | ✅ 守住 |
| `此消息已删除` / `(原消息已删除)` | `MessageItem.tsx:176` + CV-13 #557 `QuotedCommentBlock.tsx:28` MISSING_FALLBACK | `deleted/已移除` 0 hit | ✅ 守住, 跨 DM/CV 文案对齐 (`此消息`独立行 vs `原消息`引用 fallback 拆死) |

**疑似 drift A** (中度): CV-13 quote fallback `(原消息已删除)` 跟 DM message body delete `此消息已删除` 是两个不同字面 (一个带括号一个不带). 这是**有意拆死** — quote-rendering 上下文 vs 主消息 body 上下文 — 但建议 cv-13-content-lock §1 显式标注 "字面跟 MessageItem.tsx:176 `此消息已删除` 故意不同 (语境拆死), 不算 drift". **建议**: CV-13 战马E v0.1 patch 加一行注释 (≤2 行 comment, 不阻 PR).

---

## §2 owner-only ACL 用户感知红线 (用户改不到别人的设置)

| 资源 | owner 定义 | 守门点 | 跨 milestone 同源链 |
|---|---|---|---|
| channel 改名 / 归档 | `channel.created_by` | `channels.go::handleArchive/handleRename` PATCH | CHN-1 #288 + CHN-2 #413 + CHN-3 + CHN-4 同源 |
| message edit / delete (DM) | `messages.sender_id` | `dm_4_message_edit.go:144` 显式 sender match | DM-4 + DM-7 (history GET) + AP-5 #555 post-removal fail-closed |
| artifact rollback / preview / thumbnail | `artifact.channel.created_by` | `artifacts.go:201` + `preview.go:12` + `thumbnail.go:13` 三 endpoint 同 ACL | CV-1.2 + CV-2-v2 #517 + CV-3-v2 #528 跨 3 PR 同源 |
| artifact iteration | 同上 | `iterations.go:213` | CV-1 + CV-4 + CV-4-v2 spec |
| agent_configs PATCH | `agents.owner_id` | `agent_config.go::PATCH` owner-only + admin god-mode 不写 | AL-2a #454 + BPP-3.2 + AL-2b 同源 |
| user_channel_layout (个人偏好) | self only | `layout.go:87` admin /admin-api 不挂 | CHN-3 ⑤ + CHN-7 mute_pref (bitmap) |
| edit_history GET | `messages.sender_id` (user-rail) / admin readonly (admin-rail) | DM-7 #558 spec §0 双路径 | DM-7 + AL-7/AL-8 audit log readonly 同精神 |
| recover (deleted) | non-revival, owner-only window | `al_5_recover.go:60` admin 反向 grep `admin-api.*recover` 0 hit | AL-5 + AP-5 post-removal 同源 |

**owner-only ACL 锁链 PR# 计数 (野马核对)**: ≥19 处 (DM-7 spec §0 自标第 19 处), 跨 CV / CHN / DM / AL / AP / BPP 六系列对齐. ✅ 守住.

**疑似 drift B** (低): AP-5 #555 messages PUT/DELETE/PATCH post-removal fail-closed 是新加 ACL gate, 但跟 DM-4 `dm_4_message_edit.go:23` 既有 owner-only ACL 链怎么互动 (post-removal 时 DM-4 edit 路径是否同样 fail-closed)? **建议**: 让烈马 (acceptance) 在 AP-5 acceptance 加一行 `REG-AP5-00X DM-4 edit + AP-5 post-removal 双 gate 互动 e2e test plan` 闭合.

---

## §3 admin god-mode 不窥视私密信息红线 (蓝图 §0 强权但不窥视)

admin-rail (`/admin-api/*`) 跟 user-rail (`/api/v1/*`) 双 mux 隔离, admin 默认不挂用户私密路径. 反向 grep 0 hit 守门:

| 私密资源 | admin god-mode 应**不挂** | 反向 grep 锚 | 守门状态 |
|---|---|---|---|
| 用户草稿 (CV-10 artifact comment 草稿持久化) | admin 不读 | CV-10 #541 spec / 0 server diff (草稿仅 client localStorage) | ✅ 天然守 (无 server 路径) |
| 用户书签 / bookmark | admin 不读 (蓝图未提, 但若加需 admin 反向 grep) | 暂无 milestone | N/A 未实施 |
| mute_pref (bitmap bit 1) | admin 不读 | CHN-7 #550 stance + REG-CHN7-006 admin god-mode `bitmap.bit_1` 0 hit | ✅ 守住 |
| user_channel_layout (个人偏好) | admin 不读 | `layout.go:87` admin 401 by mw + CHN-3 ⑤ god-mode 字段白名单不含 | ✅ 守住 |
| edit_history (DM-7) | admin readonly OK (audit 用途, 非用户私密草稿) | DM-7 spec §0 admin-rail GET 显式合法 | ✅ 故意挂 (非漂移) |
| message body | admin 走 god-mode endpoint 仅元数据 | ADM-0 §1.3 红线 + AL-1 #492 audit metadata only | ✅ 守住 |
| agent_configs blob | admin 不写 (PATCH 反向); 读仅元数据 | AL-2a ④ + AL-4 ② + ADM-2 同源 (admin god-mode 元数据 only 6 源) | ✅ 守住 |
| recover (admin override) | admin 不挂 user-rail recover | `al_5_recover.go:60` 反向 grep `admin-api.*recover` 0 hit | ✅ 守住 |
| reactions / mentions | admin 不写 | AP-4 #551 + DM-5 #549 + CV-9 #539 0 server diff (复用既有) | ✅ 守住 |

**疑似 drift C** (低): 用户**搜索历史** + **未读计数** (CHN-* 范围) 这两类私密信息 admin 是否守得住, Phase 4 没专门 stance 锁. **建议**: Phase 5 起一份 `admin-godmode-private-fields.md` 跨链白名单总表 (≤40 行) 跟 ADM-2 audit_log schema 同源收口.

---

## §4 agent ↔ human 同源 (CV-5..13 / DM-* / 14 立场 §1.2 agent=同事)

蓝图核心立场: **agent 跟 human 在 channel 内同源, 不被特殊待遇** (sender_id 复用 users 表 + comment endpoint 不分 agent/human).

| 端点 / 行为 | agent 等同 human? | 同源锚 |
|---|---|---|
| artifact_comments (CV-5/7/8/9/10/11/12/13) | ✅ 同源 — sender_id 复用既有 users.id, 无 `is_agent` 分支 | `messages.go:381` CV-5 #530 thinking 5-pattern AST 锁 (agent thinking 不污染人类视角) — 是反约束**不是**特殊待遇 |
| DM messages (DM-4..7) | ✅ 同源 — sender_id 同表, edit/delete/history ACL 跟 human 同 | DM-4 dm_4_message_edit.go owner-only 不分类型 |
| reactions (DM-5 / AP-4 / CV-7) | ✅ 同源 — agent 也可 react, ACL 走 channel-member 不走 is_agent | AP-4 #551 + DM-5 #549 + CV-7 #535 三 PR 同模式 |
| @mention (CV-9 / DM-2) | ✅ 同源 — agent 也可被 @, fallback DM 同模式 | CV-9 #539 + DM-2 系列 |
| quote / reference (CV-13 / DM-6) | ✅ 同源 — agent 消息也可被引用, MISSING_FALLBACK 不分类型 | CV-13 #557 + DM-6 #556 |
| 编辑历史 (DM-7) | ✅ 同源 — agent edit message 也写 edit_history | DM-7 #558 (`reasons.Unknown='unknown'` AL-1a 锁链第 18 处, 跟 human 同源) |
| busy/idle 状态 (AL-1b) | ⚠️ 拆死 — agent 有 busy/idle dot, human 无 | 故意拆 — 14 立场 §11 sub: agent 是工作单元, human 是观察者 (**非漂移**) |
| 隐私页 (ADM-1) | ✅ 跟 human 同源 — admin 看不到 agent 也看不到, 三色锁同精神 | ADM-1 §4.1 R3 + g4.1-adm1-yema-signoff.md |

**疑似 drift D** (无): 4 主题 audit 链, agent ↔ human 同源链最干净 — Phase 4 30 个 milestone 没破立场 §1.2 红线 ("agent=同事"). ✅ 守住.

---

## §5 总结 + follow-up 建议

| 链 | 状态 | drift | 建议 |
|---|---|---|---|
| §1 byte-identical 文案锁 | ✅ 守住 | drift A (低): CV-13 `(原消息已删除)` 跟 DM `此消息已删除` 字面拆死建议加注释 | CV-13 v0.1 patch ≤2 行注释 |
| §2 owner-only ACL 用户感知 | ✅ 守住 19 处 | drift B (低): AP-5 post-removal × DM-4 edit 双 gate 互动 e2e | 烈马 AP-5 acceptance 加 1 行 REG plan |
| §3 admin god-mode 不窥视 | ✅ 守住 | drift C (低): 搜索历史 / 未读计数私密路径未跨链锁 | Phase 5 起 `admin-godmode-private-fields.md` 总表 |
| §4 agent ↔ human 同源 | ✅ 守住 | 无 | — |

**整体**: 4/4 链 **守住**, 仅 3 处低优先建议 follow-up (CV-13 注释 / AP-5 acceptance 1 行 / Phase 5 admin god-mode 私密总表). 跟 PM 立场 PM 反查"用户主权 + 强权但不窥视 + agent=同事"三大红线对齐, Phase 4 落地干净.

---

## §6 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-30 | 野马 | v0 — Phase 4 跨链立场反查 4 主题 audit (byte-identical 文案锁 7 标签 / owner-only ACL 8 资源 ≥19 处链 / admin god-mode 9 私密资源反向 grep / agent ↔ human 同源 8 端点); 跟 #467 cross-milestone count audit + ADM-1 G4.1 SIGNED + Phase 4 主线 ~30 merged milestone 同模式 PM 反查; 抓出 3 处低优先 follow-up 建议 (CV-13 注释 / AP-5 e2e / Phase 5 admin god-mode 私密总表), 0 阻塞 drift. |

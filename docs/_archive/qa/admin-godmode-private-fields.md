# admin god-mode 不窥视 9 私密资源总表 (野马 v0)

> **状态**: v0 (野马, 2026-04-30) — Phase 4 跨链总收口, 跟 PR #568 §3 同源 (Phase 4 立场反查表 9 私密资源 audit 升级为独立总表)
> **范围**: Phase 4 落地的 9 资源 admin god-mode "强权但不窥视" 边界字面汇总
> **立场锚**: 蓝图 `concept-model.md` §0 (强权但不窥视) + `admin-model.md` §0 + §1.3 (硬隔离 + god-mode 不返回内容) + §1.4 (分层透明) + ADM-0 #211 红线 + ADM-1 G4.1 SIGNED (#483) `g4.1-adm1-yema-signoff.md` 三色锁 ① "Admin 看不到消息 / 文件 / artifact 内容".
> **关联 PR**: ADM-0 #211 + ADM-1 #483 + ADM-2 系列 + AL-5 #492 + CHN-3.2 + CHN-7 #550 + CV-10 #541 + DM-7 #558 + AP-4 #551 + AL-2a #454.

---

## §1 用户主权红线 (蓝图 §0 字面)

> "强权但不窥视" — admin 是平台运维, 不是协作者. admin 永不出现在 channel/DM/团队列表里; admin 看不到消息/文件/artifact 内容; admin 能看的是元数据 (用户名 / channel 名 / 条数 / 登录时间).

承袭 ADM-1 G4.1 §4.1 R3 三条隐私承诺 byte-identical (跟蓝图 `admin-model.md` §4.1 + `PrivacyPromise.tsx:26-28` 同源).

---

## §2 admin readonly 边界 — 9 私密资源汇总

| # | 资源 | 来源 milestone / PR | admin 读? | admin 写? | 守门点 (反向 grep 锚) |
|---|------|--------------------|----------|----------|---------------------|
| ① | **草稿** (artifact comment 持久化) | CV-10 #541 | ❌ 不读 | ❌ 不写 | 0 server diff (草稿仅 client localStorage; 反向 grep `admin.*draft` 在 internal/api/ count==0) |
| ② | **书签** (bookmark) | DM-8 (即将落, v3+ 占号) | ❌ 不读 | ❌ 不写 | 待 DM-8 落地后加 `admin.*bookmark` 反向 grep 锚 |
| ③ | **mute_pref** (channel mute bitmap bit 1) | CHN-7 #550 | ❌ 不读 | ❌ 不写 | REG-CHN7-006 admin god-mode `bitmap.bit_1` 反向 grep 0 hit + admin /admin-api/* rail 隔离 |
| ④ | **layout** (user_channel_layout 个人偏好) | CHN-3.2 | ❌ 不读 | ❌ 不写 | `layout.go:87` admin /admin-api 不挂 + admin cookie → 401 by mw + CHN-3 ⑤ god-mode 字段白名单不含 |
| ⑤ | **edit_history** (DM message edit audit) | DM-7 #558 | ✅ readonly (audit 用途 — 非用户私密草稿) | ❌ 不写 | `/admin-api/v1/messages/{messageId}/edit-history` admin readonly **故意挂** + user-rail GET sender-only 双路径; admin 不挂 PUT/DELETE/PATCH 反向 grep 0 hit |
| ⑥ | **message body** (channel + DM 主消息) | ADM-0 #211 + AL-1 #492 audit | ❌ 不读 raw body | ❌ 不写 | god-mode endpoint sanitizer 白名单 = `{id, name, member_count, message_count, created_at, members}` 不含 `body/content`; `TestAdminGodModeOmitsContent` 锁; `internal/api/admin_*.go` 反向 grep `message.body / artifact.content` 0 hit |
| ⑦ | **agent_configs blob** (api_key / system_prompt / temperature 全字段) | AL-2a #454 + AL-4 #387/#461 + ADM-2 | ✅ 元数据 only (id / schema_version / updated_at / agent_id) | ❌ 不写 | AL-2a ④ "PATCH owner-only + admin god-mode 字段白名单不返 blob" 反向 grep `admin.*config.*update / admin.*agent_configs.*PATCH` count==0 (跨 milestone admin 元数据 only **6 源链** 守) |
| ⑧ | **recover** (al_5_recover 删除恢复) | AL-5 | ❌ 不挂 admin-rail (admin override 走另一条 rail) | ❌ 不写 | `al_5_recover.go:60` 反向 grep `admin-api.*recover` count==0 (admin 不入 user 的 recover 路径; admin override 走 ADM-2 audit_log 单源) |
| ⑨ | **reactions** (DM-5 / AP-4 / CV-7 emoji 反应) | AP-4 #551 + DM-5 #549 + CV-9 #539 + CV-7 #535 | ❌ 不读 (元数据 OK, raw emoji body 不读) | ❌ 不写 | AP-4 #551 ACL gap 闭合 (3 handler 加 channel-member 检查 — admin 非 channel-member 自动 fail-closed); 0 server production diff 在 admin-rail |

**边界总结**: 9 资源中 8 个 admin **完全不挂** + 1 个 admin readonly 故意挂 (edit_history 是 admin audit 合法用途, 跟 ADM-2 audit_log 同精神). admin 元数据 only **6 源链** (ADM-0 + AL-3 + AL-4 + AL-2a + ADM-1 + AL-1b) 守住跨 milestone byte-identical.

---

## §3 反向 grep 锚 (CI-grade) — 9 资源 cross-cutting 检查

```bash
# § 1 总闸: admin-api 不入 user-rail 私密路径
grep -rnE "admin-api.*\b(draft|bookmark|mute|layout|recover)\b" packages/server-go/internal/api/ | grep -v _test.go
# 预期: 0 hit (除 edit_history readonly)

# § 2 admin god-mode endpoint sanitizer 白名单
grep -rnE '"body"\s*:|"content"\s*:|"api_key"\s*:' packages/server-go/internal/api/admin_*.go | grep -v _test.go | grep -v key_present
# 预期: 0 hit (admin 仅 key_present:bool 不返 raw)

# § 3 admin 写动作反向 — agent_configs / messages PATCH/DELETE 不挂 admin-rail
grep -rnE "admin.*\b(config.*update|message.*update|message.*delete|reaction.*write)\b" packages/server-go/internal/api/ | grep -v _test.go
# 预期: 0 hit

# § 4 admin override 仅走 ADM-2 audit_log 单源
grep -rnE "admin.*override|godmode.*write" packages/server-go/internal/api/ | grep -v _test.go | grep -v adm_2
# 预期: 0 hit (除 ADM-2 自身路径)
```

跟 ADM-0 stance §2 反向 grep 模式 + DM-7 spec §0 admin readonly 模式 + CHN-7 REG-CHN7-006 同精神 byte-identical.

---

## §4 v3+ 留账 (Phase 4 没专门锁, Phase 5+ 跟进)

| 资源 | 当前状态 | Phase 5+ 跟进建议 |
|------|---------|------------------|
| **搜索历史** (用户搜索 query log) | Phase 4 没专门 stance 锁 (无 milestone 实施) | v3+ 起搜索功能 milestone 时, 加 `admin.*search.*history` 反向 grep + readonly 路径明确 (audit 合法 vs 内容 raw 拆死) |
| **未读计数** (channel unread) | Phase 4 没专门 stance 锁 (read_marker 路径未跨链 audit) | v3+ 加 `admin.*unread / admin.*read_marker` 反向 grep + admin readonly 元数据 only (不返 unread message body) |
| **typing indicator** | Phase 4 无实施 | v3+ 跟 RT-* 实时通信链落地时同步 stance |
| **last_seen_at** (在线时间) | AL-3 stance ⑦ admin readonly 元数据 OK 已锁 | ✅ 已守, 留作 Phase 5 cross-cutting 收口锚 |

**Phase 5 收口闸**: 启动 Phase 5 第一个含 admin god-mode 触点 milestone 时, 此总表 v0.x patch 加新行 + 反向 grep 锚 (跟 #467 cross-milestone count audit 同模式 follow-up).

---

## §5 改 = 改几处

改此总表 9 资源任一行 **= 改至少 3 处** byte-identical:
1. 此总表 (`admin-godmode-private-fields.md`) — 总入口
2. 对应 milestone stance/content-lock (ADM-0 / ADM-1 / ADM-2 / AL-2a / AL-5 / CHN-3 / CHN-7 / CV-10 / DM-7 / AP-4 等)
3. 实施代码反向 grep 锚 (`internal/api/admin_*.go` + 各 handler 注释)

跟 ADM-1 §4.1 R3 三条隐私承诺 byte-identical (改 = 改 PrivacyPromise.tsx + admin-model.md + 此总表) 同模式.

---

## §6 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-30 | 野马 | v0 — Phase 4 跨链总收口 (跟 PR #568 §3 升级为独立总表): 9 私密资源 admin readonly 边界 (草稿 / 书签 / mute_pref / layout / edit_history / message body / agent_configs blob / recover / reactions) + §3 4 行 cross-cutting 反向 grep CI-grade 锚 + §4 v3+ 留账 4 项 (搜索历史 / 未读计数 / typing / last_seen). 跟 ADM-0 #211 + ADM-1 G4.1 SIGNED (#483) + ADM-2 + AL-5 + AL-2a + CHN-3/7 + CV-10 + DM-7 + AP-4 同源 byte-identical. Phase 5 收口闸: 启动 Phase 5 第一个含 admin 触点 milestone 时此总表 v0.x patch 加新行 (跟 #467 cross-milestone audit 同模式). |

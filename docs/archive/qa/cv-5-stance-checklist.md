# CV-5 立场反查清单 (战马E v0)

> 战马E · 2026-04-29 · 立场 review checklist (跟 DM-2.2 / RT-3 / BPP-2.2 / AL-1b 同模式 5-pattern thinking 链)
> **目的**: CV-5 三段实施 (CV-5.1 server / 5.2 client / 5.3 e2e + closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/cv-5-spec.md` (战马E v0 857170d) + acceptance `docs/qa/acceptance-templates/cv-5.md` (战马E v0)
> **不需 content-lock** — comment 是用户文本 + 视觉栈承袭 CV-2.3 anchor comments 不引入新 DOM 锁.

## §0 立场总表 (3 立场 + 4 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | comment 走 messages 表单源 — channel_id 复用 `artifact:<id>` namespace (跟 DM-2 #312 `dm:` namespace 同精神); **不另起 `artifact_comments` 表** / 不裂 comment_id 生命周期 / 不开新 schema migration | canvas-vision.md L24 字面 "Linear issue + comment" + DM-2.2 #372 namespace 承袭 | 反向 grep `CREATE TABLE.*artifact_comments\|artifact_comments.*PRIMARY` 在 internal/migrations/ count==0 |
| ② | comment 复用 RT-3 既有 fan-out + cursor 共序 — frame `artifact_comment_added` 8 字段跟 MentionPushedFrame 同模式; **不另起 cursor sequence** / 不挂 admin god-mode 抄送 | RT-3 #488 hub.cursors 单源 + ADM-0 §1.3 红线 | 反向 grep `comment.*hub\.cursors\.NextCursor.*[^"]plugin\|admin.*comment.*push` count==0 |
| ③ | agent comment 必带 thinking subject (5-pattern 第 4 处链) — server reject 400 `comment.thinking_subject_required` 当 sender_role==agent 且 body 字面缺 subject | 蓝图 realtime §1.1 ⭐ + RT-3/BPP-2.2/AL-1b 同链承袭 | 反向 grep `body.*"thinking"$\|comment.*defaultSubject\|comment.*fallbackSubject\|"AI is thinking"\|comment.*subject\s*=\s*""` 在 internal/api/ 排除 _test.go count==0 |

## §1 立场 ① comment 单源 messages 表 (CV-5.1 守)

**蓝图字面源**: `canvas-vision.md` L24 字面 "Linear issue + comment, 不是 Miro 白板" + DM-2 #312 `dm:` namespace 承袭

**反约束清单**:

- [ ] POST `/api/v1/artifacts/:id/comments` 内部走 `Store.CreateMessage` + channel_id = `"artifact:" + artifact_id` 字面
- [ ] 不开新表 — git diff packages/server-go/internal/migrations/ 0 改 (反向 grep `CREATE TABLE.*artifact_comments` 0 hit)
- [ ] 不裂 comment_id — 复用 messages.id (反向断 comment 数据流 ID 即 message ID)
- [ ] artifact channel 自动 ACL — channel.created_by + agent_id 自动加 channel_members
- [ ] 错误码 `comment.target_artifact_not_found` 当 artifact_id 不存在 (跟 DM-2.2 mention.target_not_in_channel 同模式)

## §2 立场 ② RT-3 fan-out + cursor 共序 (CV-5.1+5.2 守)

**蓝图字面源**: RT-3 #488 hub.cursors 单源 + AL-2b #481 + BPP-2 #485 + BPP-3.1 #494 共一根 sequence

**反约束清单**:

- [ ] frame `artifact_comment_added` 8 字段 (type/channel_id/sender_id/sender_role/comment_id/body_preview/created_at/cursor) 跟 MentionPushedFrame 同模式
- [ ] body_preview cap 80 rune (跟 DM-2.2 #372 隐私 §13 同 cap)
- [ ] cursor 走 hub.cursors.NextCursor (RT-3 #488 既有, 反向断不另开 sequence)
- [ ] 不挂 admin god-mode 抄送 — 反向 grep `admin.*comment.*push\|comment.*hub\.cursors\.NextCursor.*plugin` 0 hit
- [ ] client `wsClient.ts` switch case 增量 append 不刷全列表

## §3 立场 ③ agent thinking subject 反约束 (5-pattern 第 4 处链)

**蓝图字面源**: 蓝图 realtime §1.1 ⭐ "thinking 必须带 subject" + RT-3 #488 + BPP-2.2 #485 + AL-1b #482 同链承袭 (CV-5 是第 4 处)

**反约束清单**:

- [ ] server validator: sender_role==agent 时, body 字面 grep 5-pattern 任一 → reject 400 `comment.thinking_subject_required`
  - pattern 1: body 末 "thinking$"
  - pattern 2: defaultSubject literal
  - pattern 3: fallbackSubject literal
  - pattern 4: "AI is thinking" 字面
  - pattern 5: subject="" 空字符串
- [ ] human comment (sender_role==human) 不走此 validator — body 文本自由
- [ ] 反向 grep `body.*"thinking"$\|comment.*defaultSubject\|comment.*fallbackSubject\|"AI is thinking"\|comment.*subject\s*=\s*""` 在 internal/api/ 排除 _test.go count==0
- [ ] 5-pattern 改 = 改 4 处 (RT-3 + BPP-2.2 + AL-1b + CV-5) byte-identical 同步

## §4 蓝图边界 ④⑤⑥⑦ — ACL fail-closed / 不裂 sequence / 不裂表 / forward-only

**反约束清单**:

- [ ] cross-channel reject — 非 artifact channel 成员调 endpoint 403 `comment.cross_channel_reject` (REG-INV-002 fail-closed)
- [ ] 不裂 sequence — cursor 跟 RT-3 / DM-2.2 / BPP-2 / BPP-3.1 共一根
- [ ] 不裂表 — 0 schema 改 (git diff internal/migrations/ 0 改)
- [ ] forward-only — CV-5 不挂 edit / delete (deferred CV-5.4+ Phase 6)

## §5 退出条件

- §1 (5) + §2 (5) + §3 (4) + §4 (4) 全 ✅
- 反向 grep 4 锚 0 hit / 1 锚 ≥1 hit (artifact_comments 表 / cursor 抄送 / thinking 5-pattern / admin god-mode / `channel_id\s*=\s*"artifact:` ≥1)
- e2e 5 case 全 PASS (human round-trip / agent thinking subject 必带 / cross-channel reject / cursor 共序 / admin god-mode 不消费)
- 0 schema 改 (git diff packages/server-go/internal/migrations/ 仅 _test.go 或空)

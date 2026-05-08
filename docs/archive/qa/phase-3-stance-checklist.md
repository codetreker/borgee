# Phase 3 第一波 milestone 立场反查表

> **状态**: v0 (野马, 2026-04-28)
> **目的**: Phase 3 entry 前 PM 立场锚点 — 战马 CHN-1 / CV-1 / RT-1 PR 直接吃此表为 acceptance; 飞马 / 野马 review 拿此表反查立场漂移。一句话立场 + §X.Y 锚 + 反约束 (X 是, Y 不是) + v0/v1 阶段策略。
> **关联**: `blueprint/README.md` 14 立场 §4-§6 / §11, `channel-model.md` §1-§2, `canvas-vision.md` §1.1-§1.6, `realtime.md` §1.1-§1.4 + §2.3 (Phase 2 /ws → Phase 4 BPP schema 等同性)。
> **依赖**: Phase 2 闸 4 签字 (CM-4 / ADM-0 / G2.4 demo) 全收口后 Phase 3 才解封。

---

## 1. CHN-1 立场反查表 (channel 协作场基础)

| # | 立场锚 | 一句话立场 | 反约束 (X 是, Y 不是) | v0 / v1 |
|---|--------|----------|----------------------|---------|
| ① | channel-model §1.1 + 14 立场 §6 | **channel = 协作场, agent 是原生成员**, 创建即建 workspace 占位 (即使 v1 仅 Markdown) | **是** "一群人 + 一群 agent 围绕一件事工作的地方" + 双支柱 (chat + workspace); **不是** Slack 聊天容器, **不是** Discord 社区频道, agent **不是** webhook/bot | v0: 创建 channel auto-init 空 workspace; v1: 双 tab UI 平级渲染 |
| ② | channel-model §2 不变量 + concept-model §4.2 | **默认成员 = 创建者本人 only** (非 org 全员自动加, 非"公共"语义); 其他人 / agent 走主动加入 / owner 邀请 | **是** 创建者 + 显式邀请的人 / agent; **不是** org 广播默认成员 (避免退化为团队群); agent 加入**必须** owner 触发 (跨 org 走 invitation 状态机, CM-4 已锚) | v0: creator-only 起点; v1 同 (multi-org channel 由邀请累积, 不靠"org 全员默认") |
| ③ | channel-model §1.1 + realtime §1.1 + 14 立场 §11 | **agent 进 channel 默认 silent** (不发"hello 我是 X agent" 入场白); 沉默胜于假活物感 | **是** join event 仅 system message 一行 `"{agent_name} joined"` (kind=system, sender=system); **不是** agent 自发问候消息 (会被当 noise 触发 mute), **不是** 静默 (要有 join 痕迹便于 audit) | v0/v1 同 — silent default 永久, 不开 agent 自发开场白入口 |
| ④ | channel-model §1.4 + 14 立场 §6 | **channel rename = 创建者 (owner) only** (作者定义大局, 个人偏好不污染他人) | **是** owner 改名全员同步看到; **不是** 任意成员可改 (那是个人化层); **不是** admin 可改 (admin 不入 channel, ADM-0 红线 §1.1); 个人侧只能折叠 / 排序 (channel-model §1.4) | v0: owner-only rename; v1 同, 加 audit row |
| ⑤ | channel-model §2 + ADM-0 admin-model §1.4 | **channel delete = 软删 (soft delete)**, owner 触发, 触发受影响者 system DM 通知 (复用 ADM-0 §1.4 红线 ③ 模式) | **是** `channels.deleted_at` 标位, 数据保留 ≥ 90d, 受影响成员收 system DM `"channel #{name} 已被 {owner_name} 关闭于 {ts}"`; **不是** 硬删立刻 cascade; **不是** 静默 (owner 删除也要广播给成员) | v0: soft delete + system DM; v1: 加 owner 端 "已关闭" 视图 + 90d 后 GC job |

---

## 2. CV-1 立场反查表 (canvas / artifact v1 形态)

| # | 立场锚 | 一句话立场 | 反约束 (X 是, Y 不是) | v0 / v1 |
|---|--------|----------|----------------------|---------|
| ① | canvas-vision §1.3 + §1.4 + 14 立场 §5 | **artifact 归属 = channel** (权限继承 channel 成员), 不是归属 author | **是** workspace per channel, artifact 跟 channel 走 (channel 删 → artifact 软删随; channel member 离开 → 失访问); **不是** 归属 author (author 离开 channel 不带走 artifact); **不是** 跨 channel 共享 (v1 不做多 artifact 视图) | v0: artifact 表 `channel_id NOT NULL`, 无 `owner_id` 主权语义; v1 同 |
| ② | canvas-vision §1.5 + §2 不做 + 14 立场 §2 | **多人编辑 = 一人编辑一锁 (last-writer-wins + 锁标位)**, 不上 CRDT | **是** v1 串行编辑, agent 写入触发新版本 (canvas-vision §1.4 自带版本); **不是** realtime CRDT (§2 显式不做); **不是** 无锁覆盖 (要有 conflict 提示 → 提示用户 reload) | v0: 单文档锁 (`artifacts.lock_holder_user_id` + `lock_acquired_at` + 30s TTL); v1 同, 加 conflict UI hint |
| ③ | canvas-vision §1.4 + §1.5 表 | **版本历史线性保留, agent 默认无权删历史**; 删除版本 = owner grant | **是** 每次 commit 一新版本行, 可回滚到前一版; agent 写内容 ✅ 默认, **删历史 ❌ 默认** (§1.5 表锁); **不是** 版本图状 (no fork v1); **不是** 无限保留 (v1 不限期, v2 加 GC 策略) | v0: 线性版本 + agent 默认无删权; v1 同, 加 owner grant UI |

---

## 3. RT-1 立场反查表 (WS push 顺序 / 失序 / 重连)

| # | 立场锚 | 一句话立场 | 反约束 (X 是, Y 不是) | v0 / v1 |
|---|--------|----------|----------------------|---------|
| ① | realtime §1.4 + §2.3 schema 等同性 | **WS push 顺序 = server 端 cursor 单调递增** (event.cursor 全局唯一 + 单调), client 端去重靠 cursor | **是** server 单调发号, client 收到乱序 → 按 cursor 排序; **不是** client 端时间戳排序 (跨 client 不可信); **不是** 无去重 (多端全推必 dup, §1.4 锁端上去重) | v0: /ws hub 全推 + cursor 单调; v1 切 BPP frame schema 等同 (§2.3 飞马 R3 锁) |
| ② | realtime §1.3 + §1.4 | **失序处理 = client 端缓冲 + 缺洞拉补** (不丢, 不假装到达); 人类端 full replay, agent 端 BPP `session.resume` 走 hint | **是** client 收到 cursor=N+2 但缺 N+1 → 触发 backfill 拉 (`/api/events?since=N`); **不是** 直接渲染跳过 (协作场每条都可能是决策, §1.3 锁 full); **不是** server 端硬保证有序 (UDP-like 假设, client 容错) | v0: 人类端 full replay + 缺洞 backfill; v1 末加 active client 智能推 (§1.4 B) |
| ③ | realtime §1.3 + agent-lifecycle (三态) | **重连 backfill = 拆人 / agent 两路**: 人走 full replay (端上虚拟列表), agent 走 `replay_mode` hint (full / summary / latest_n, runtime 自决) | **是** 人/agent 截然分; **不是** 一套 replay 走天下 (§1.3 显式打掉这隐性假设); agent **不**默认 full (烧 token, §1.3 设计直觉) | v0: 人 full + agent BPP `session.resume` 三 hint; v1 同 |

---

## 4. 黑名单 grep — Phase 3 入口反查 (PR merge 后跑, 全部预期 0 命中)

```bash
# CHN-1 ②: 创建 channel 不该自动 INSERT org 全员入 channel_members
grep -rnE "INSERT INTO channel_members.*SELECT.*FROM users" packages/server-go/internal/store/ | grep -v _test.go
# CHN-1 ③: agent join 不该发非 system 自发问候
grep -rnE "agent.*joined.*hello|hi.*I am" packages/server-go/ | grep -v _test.go
# CV-1 ①: artifacts 不应有 owner_id 主权语义 (channel_id 是唯一归属)
grep -rnE "artifacts.*owner_id" packages/server-go/internal/ | grep -v _test.go
# RT-1 ①: client 端不该用本地 timestamp 排序 events
grep -rnE "sort.*events.*timestamp|events\.sort\(.*createdAt" packages/client/src/ | grep -v _test.
```

---

## 5. 不在 Phase 3 第一波范围 (避免 PR 膨胀)

- ❌ 段落锚点对话 (canvas-vision §2 v2); ❌ 多 artifact 关联视图 (canvas §2 v2+)
- ❌ realtime CRDT (canvas §2 / realtime §1.1); ❌ 多 agent 编排可视化 (realtime §1.1 v2)
- ❌ 端 per-device 推送配置 (realtime §1.4 C v2); ❌ channel rename 完整审计 UI (v2)

---

## 6. 验收挂钩

- CHN-1.x PR: §1 5 项立场实施落点 + §4 黑名单 grep 命中 0 + 反向断言测试 (creator-only / agent silent / owner-only rename / soft delete + system DM)
- CV-1.x PR: §2 3 项立场 + artifact 表 `channel_id NOT NULL` + 单文档锁 + 版本线性 + agent 默认无删历史权
- RT-1.x PR: §3 3 项立场 + cursor 单调 + 缺洞 backfill + 人/agent 两路 replay + §2.3 schema 等同性 CI lint 绿
- Phase 3 entry 闸 (野马): §1-§3 共 11 项立场全锚 + §4 grep 0 → ✅ Phase 3 解封

---

## 7. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v0, CHN-1 5 项 + CV-1 3 项 + RT-1 3 项立场 + 黑名单 grep + 不在范围 6 条 |
| 2026-04-29 | 野马 | v0.1 patch — 二轮反查 (post-#334) 抓出 CV-1 ② drift: `workspace_files.locked_by_user_id` → `artifacts.lock_holder_user_id` (表名 + 列名 双 align), 加 `lock_acquired_at`; §4 黑名单 grep 同步去 `workspace_files`. 跟 #334 实施 + #337 acceptance + #338 cross-grep 反模式 三源对齐 |

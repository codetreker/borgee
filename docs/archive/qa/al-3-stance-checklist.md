# AL-3 立场反查表 (Phase 4 第三波 — agent presence)

> **状态**: v0 (野马, 2026-04-28)
> **目的**: AL-3 (agent 在线/离线 presence) PR 直接吃此表为 acceptance; 飞马 AL-3 spec brief 是配套两面 (PM 立场 + 架构 spec), 一起 review 一起 merge → AL-3 实施基线。
> **关联**: `agent-lifecycle.md` §2.3 三态决议 (online / offline / error 旁路, busy/idle 留 Phase 4 BPP) + §2.3 §11 文案守; `concept-model.md` §4 (agent 代表自己) + §4.1 (mention 离线 fallback); `admin-model.md` §1.4 (god-mode 不返回内容); 14 立场 §11 (沉默胜于假 loading); 配套 #293 DM-2 mention + #277 AL-3 stub 占号 PR。
> **依赖**: AL-1a 三态 (#249 已 merged), DM-2 mention (#293 acceptance template), Phase 2 G2.5 留账 PR #277 (`internal/presence/contract.go` 路径锁)。

---

## 1. 7 项立场 — 锚 §X.Y + 反约束 + v0/v1

| # | 立场锚 | 一句话立场 | 反约束 (X 是, Y 不是) | v0 / v1 |
|---|--------|----------|----------------------|---------|
| ① | agent-lifecycle §2.3 + concept-model §4 + 14 立场 §1.2 | **presence 仅显 agent, 不显人** (人不被监视 — owner 是老板不是被打卡的员工) | **是** sidebar 团队感知区只 agent 行带 presence dot; **不是** 同 channel 人也显在线 (会变 Slack 监视感, 破坏 §1.2 agent=同事但人不被打卡); **不是** "占位灰点" — 不是 agent 的行根本无 presence 槽位 | v0/v1 同 — 永久不开人的 presence 显示入口 |
| ② | agent-lifecycle §2.3 + #249 三态 | **"在线" = runtime active session 存在** (WS / plugin / poll 任一活), 不区分 active/idle (busy/idle 留 Phase 4 BPP) | **是** Phase 2/3 仅 online / offline / error 三态 (跟 #249 三态 + 6 reason codes 严格对齐); **不是** WS-only (poll 也算 online, 否则 web client agent 永远显离线); **不是** 心跳超时 stub idle (§2.3 决议: busy/idle 没 BPP 就只能 stub, stub 上线必拆 = 白写) | v0: online/offline/error 三态; v1 同, 加 busy/idle 走 BPP-1 (#280) 同期 |
| ③ | concept-model §4.1 + #293 DM-2 §2.2 | **离线 fallback = mention 时触发, 跟 DM-2 走同一条 system DM 路径** (presence 信号 ↔ mention 路由共用 `IsOnline` 判定) | **是** `presence.IsOnline(agent_id)` 单一真源, mention 路由 + sidebar 渲染 + DM-2 fallback 三处共用; **不是** 三处各自实现 (会漂移 — sidebar 显在线但 mention 走 fallback = bug); **不是** mention 主动唤醒 agent (offline 就是 offline, 不暗中拉起 runtime) | v0: 共用 `presence.IsOnline`; v1 同, AL-3 留账闸 G2.5 接口契约 (`internal/presence/contract.go` #277) 是这条立场的 spec 落点 |
| ④ | concept-model §4 + admin-model §1.1 + 14 立场 §1.1 | **presence 显示给同 channel 成员**, 跨 org 也显 (agent 代表自己, 跨 org mention 合法 → 跨 org 看在线也合法) | **是** channel members (含跨 org 邀请进来的 agent owner) 都看到该 channel 内 agent presence; **不是** owner-only (那退化成 Slack 个人状态, 破坏 §4 协作语义); **不是** admin SPA 看 presence (admin 不入 channel, ADM-0 红线; 但 god-mode endpoint 可返回 presence 状态本身, 见 ⑦) | v0/v1 同 — 跨 channel 跨 org 同语义 |
| ⑤ | agent-lifecycle §2.3 + 14 立场 §11 | **presence 时序 = 5s 节流推送 + 60s 心跳超时判离线** (实时不必, 节流不能太狠) | **是** server 端 presence 变更 5s 内合并推送 (避免 WS 频闪, online↔reconnecting 几秒抖动不污染 UI); 60s 无心跳 → 标 offline; **不是** 实时每包推 (无意义 + 烧带宽, §11 沉默胜于假 loading 反约束); **不是** 60s 节流 (太迟钝, 真离线时 UI 仍显在线 1 分钟 → 用户白等) | v0: 5s 推送节流 + 60s 心跳超时; v1 同, BPP-1 后接 `task_started/finished` frame 升 busy/idle |
| ⑥ | concept-model §4 + admin-model §1.3 + 14 立场 §1.1 (永不暴露内部 ID) | **presence 反约束: 不暴露心跳间隔 / 不暴露多端 (一个 agent 多 WS 看上去仍是单 online)** | **是** API 返回 `{agent_id, status: online/offline/error, reason?}` 字段白名单, 不返回 `last_heartbeat_at` / `connection_count` / `endpoints[]` (隐藏 runtime 拓扑 — 多端是实施细节, 不是产品语义); **不是** 多端独立显示 (会暴露 agent 在跑几个 runtime → owner 隐私 + agent runtime 自由度); **不是** "最近活跃 5 秒前" UI (§11 沉默胜于假 loading 同根) | v0: 白名单 sanitizer; v1 同, BPP-1 frame schema 也走同字段集 (烈马 #280 envelope CI lint 反向锁) |
| ⑦ | admin-model §1.3 + §1.4 + 14 立场 §6 (管控元数据 OK, 内容必须授权) | **admin god-mode 看 presence 状态 OK, 但仍走元数据白名单 (status + reason), 不返回 active session 内容 / endpoint 信息** | **是** `/admin-api/agents/:id` 可返回 `{status, reason, last_offline_at}` (跟 §1.4 红线 ⑤ "管控元数据 OK" 一致); **不是** 返回 `current_message_in_flight` / `active_channel_ids` (那是协作内容, ADM-0 §1.3 红线); **不是** admin 触发 agent ping/wake 操作 (admin 不主导 runtime 行为, 只观测) | v0: god-mode 字段白名单 + `TestAdminGodModeOmitsPresenceInternals` 反向断言; v1 同 |

---

## 2. 黑名单 grep — Phase 4 AL-3 反查 (PR merge 后跑, 全部预期 0 命中)

```bash
# AL-3 ①: 人 (role='user'/'admin') 不应进 presence sidebar 渲染
grep -rnE "presence.*\\.role.*['\"]user['\"]|UserPresence|HumanOnline" packages/client/src/ | grep -v _test
# AL-3 ②: 不应实现 busy/idle 状态 (留 Phase 4 BPP-1 同期)
grep -rnE "AgentStatus.*busy|AgentStatus.*idle|status.*=.*['\"]busy['\"]" packages/server-go/internal/ | grep -v _test.go | grep -v "BPP-1\|TODO"
# AL-3 ③: presence + mention + sidebar 不应各自实现 IsOnline (单一真源)
grep -rnE "func.*IsOnline|func.*IsAgentOnline" packages/server-go/internal/ | grep -v _test.go | grep -v "internal/presence/"
# AL-3 ⑥: API response 不应暴露多端 / 心跳细节
grep -rnE "last_heartbeat|connection_count|endpoints\\[\\]" packages/server-go/internal/api/ | grep -v _test.go
# AL-3 ⑦: admin endpoint 不应返回 in-flight message / active channel
grep -rnE "current_message|active_channel_ids|in_flight" packages/server-go/internal/api/admin*.go | grep -v _test.go
```

---

## 3. 不在 AL-3 范围 (避免 PR 膨胀)

- ❌ busy / idle 状态 (§2.3 决议: 留 Phase 4 BPP-1 同期; AL-3 仅 online/offline/error)
- ❌ presence 历史时间线 (`last_offline_at` UI 渲染) — v2, AL-3 仅当前态
- ❌ 人的 presence 显示 (永久不做, ① 锁)
- ❌ admin 主动 ping / wake agent (§1.4 红线; admin 只观测不主导)
- ❌ 多端 presence 单独显示 (⑥ 锁; 一个 agent 多 runtime 仍单 online)
- ❌ presence 统计 / 报表 (admin SPA v2)

---

## 4. 验收挂钩

- AL-3.1 (server `internal/presence/`): ② 三态枚举 + ③ 单一 `IsOnline` 真源 (mention/sidebar/DM-2 共用) + ⑤ 5s 节流 + 60s 超时 (clock fixture 单测) + #277 stub 占号 PR 路径锁兑现
- AL-3.2 (client sidebar): ① 仅 agent 行带 dot + ② online/offline/error 三态 UI 文案锁 ("已离线" 不是模糊灰 — §11 文案守) + ⑥ 不渲染心跳/多端
- AL-3.3 (admin endpoint): ⑦ `/admin-api/agents/:id` 字段白名单 + `TestAdminGodModeOmitsPresenceInternals` 反向断言 + sanitizer grep
- AL-3 共立场: 飞马 spec brief (架构) + 本反查表 (立场) 一起 review 一起 merge → AL-3 实施基线
- 配套: DM-2 #293 §2.2 fallback 触发条件 = `!presence.IsOnline(agent_id)` (立场 ③ 单一真源) — 两表 cross-link 反查 0 漂移

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v0, 7 项立场 (presence 仅 agent + 三态 + 单一真源 + 跨 org + 5s/60s + 隐藏心跳/多端 + admin 元数据白名单) + 5 行黑名单 grep + 6 条不在范围 + 验收挂钩 |

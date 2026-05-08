# RT-3 ⭐ stance checklist — 多端全推 + presence 活物感 + thinking subject 反约束 (升级)

> 7 立场 byte-identical 跟 rt-3-spec.md §0+§2 (飞马 v0 待 commit, ⭐ 升级 scope). **真有 prod code (多端 fanout + presence 活物感 UI + EventBus 接) + client UI 字面 (content-lock 真锁) 但 0 schema 改 / 复用既有 presence_states / cursor 表**. 跟 DL-1 #609 EventBus + DL-2 cold stream 解耦 + REFACTOR-1 #611 / REFACTOR-2 #613 字面锁 + thinking 5-pattern 锁链 (BPP-3 + CV-7 + CV-8/9/11/12/13/14 + DM-3/4/9/12) 同模式承袭.

## 1. cursor 复用 RT-1.3 #296 mechanism (不另起 sequence)
- [ ] cursor opaque token 走 RT-1.3 既有路径, 反"另起 cursor_v2 / multi_device_cursor 表"漂
- [ ] 反向 grep `cursor_v2|cursorV2|multiDeviceCursor` 在 packages/server-go/ 0 hit
- [ ] 跟 DM-3 useDMSync + RT-1 cursor opaque + RT-3 #588 既有 sequence 字面承袭一致

## 2. EventBus byte-identical 跟 DL-1 #609 (Publish/Subscribe 0 改)
- [ ] DL-1 EventBus 签名 byte-identical 不破 (4 interface count==4 锚守)
- [ ] 0 新增 interface, 反 `EventBusV2` / `MultiDeviceEventBus` / `PresenceEventBus` 同义词漂
- [ ] factory `NewDataLayer(cfg)` 单源不破 + handler baseline N=108 (DL-1 #609 CI 守门链第 6 处)
- [ ] 跟 DL-2 events 双流 (hot live + cold archive) 协同 — RT-3 走 hot live 不污染 cold archive

## 3. 多设备 fanout 单源 (不重复持久化, 跟 DL-2 cold stream 解耦)
- [ ] fanout 路径单源 — 跟 RT-3 #588 既有 server派生 hook 承袭 + 多端走同 EventBus.Publish 一次
- [ ] 不重复持久化 — fanout 仅 hot live, 反 per-device 持久化重复行 (反 cold archive 双写)
- [ ] 跟 DL-2 events_archive 表解耦, 反向 grep `fanoutPersist|deviceCursorWrite|per_device_event_log` 0 hit

## 4. presence "活物感" 显示而非 typing-indicator (反语义漂)
- [ ] presence 字面锁 — `在线` / `离线` / `刚刚活跃` (待 content-lock §1 真定字面) byte-identical
- [ ] last-seen 字面 — `最近活跃 N 分钟前` (相对时间) byte-identical 跟蓝图 §3 realtime 立场承袭
- [ ] 反 typing-indicator 漂入 — 反 "正在输入..." / "用户正在打字" / "compose..." 漂 (跟 thinking 5-pattern 立场承袭)
- [ ] presence 字面跨 spec / content-lock / DOM data-attr / 单测四处对锁 byte-identical

## 5. thinking subject 反约束 (承袭 5-pattern thinking 锁链)
- [ ] **5 字面 0 hit 反向 grep** — `processing` / `responding` / `thinking` / `analyzing` / `planning` 在 RT-3 client/server 路径 0 hit (跟 BPP-3 #489 + CV-7 #535 + CV-8/9/11/12/13/14 + DM-3/4/9/12 既有锁链承袭, RT-3 ⭐ 升级 = 锁链第 N+1 处)
- [ ] typing 同义词反向 grep — `typing|composing|isTyping|user_typing|composing_indicator|正在输入|正在打字` 0 hit
- [ ] thinking 字面在 messages 路径已锁, RT-3 不漂入 reactions / presence / cursor 路径

## 6. 0 schema 改 (复用既有 presence_states / cursor 表)
- [ ] 复用既有 presence_states 表 (AL-3 #324 PresenceTracker 同源)
- [ ] 复用既有 cursor 表 (RT-1.3 #296)
- [ ] 反向 grep `migrations/rt_3_` 0 hit + `currentSchemaVersion` 不动
- [ ] 反 ALTER 既有 schema (反"加 last_active_at 字段"等漂入)

## 7. admin god-mode 不挂 (ADM-0 §1.3 红线)
- [ ] 反向 grep `admin.*presence|admin.*cursor|admin.*fanout` 在 packages/server-go/ 0 hit
- [ ] 反向 grep `/admin-api.*presence|/admin-api.*cursor` 0 hit
- [ ] presence + cursor + fanout 走 user-rail (anchor #360 owner-only ACL 锁链承袭)

## 反约束 — 真不在范围
- ❌ typing-indicator 真启 (永久不挂, thinking 5-pattern 锁链立场承袭)
- ❌ 0 schema / 0 migration / 0 endpoint shape / 0 既有 ACL 改
- ❌ DL-2 cold archive 写路径 (RT-3 仅 hot live)
- ❌ NATS / Redis Streams EventBus 真切 (留 v3+)
- ❌ admin god-mode 加挂 presence / cursor / fanout (永久不挂)
- ❌ 加新 CI step (跟 DL-1/2 + REFACTOR-1/2 + INFRA-3 + TEST-FIX-* 同精神)

## 跨 milestone byte-identical 锁链 (5 链)
- **DL-1 #609** EventBus interface byte-identical + factory 单源 + handler baseline N=108
- **DL-2 events 双流** RT-3 hot live 不污染 cold archive (双流解耦立场承袭)
- **AL-3 #324 PresenceTracker** + **RT-1.3 #296 cursor opaque** RT-3 复用既有 mechanism
- **thinking 5-pattern 锁链** BPP-3 + CV-7 + CV-8/9/11/12/13/14 + DM-3/4/9/12 第 N+1 处延伸
- **anchor #360 owner-only ACL 锁链 22+ PRs** + REG-INV-002 fail-closed + ADM-0 §1.3 红线

## PM 拆死决策 (3 段)
- **presence 活物感 vs typing-indicator 拆死** — 活物感 (在线/离线/最近活跃) 选, typing-indicator 永久反
- **多端 fanout 单源 vs per-device 重复持久化拆死** — 单 Publish 多 Subscribe, 反 per-device 写 cold archive 漂
- **EventBus byte-identical vs V2/MultiDevice 漂拆死** — DL-1 4 interface 不动, 反 V2/MultiDevice 同义词漂

## 用户主权红线 (5 项)
- ✅ 0 行为改既有 endpoint / 0 schema 改 (复用既有 presence + cursor)
- ✅ 既有 ACL gate 字面 + 行为 byte-identical (anchor #360 + REG-INV-002 守)
- ✅ 0 user-facing 字面漂 (presence byte-identical + thinking 5 字面反向 grep 0 hit)
- ✅ 多端用户主权 (一用户多设备 cursor 同步, 反 per-device 状态 drift)
- ✅ admin god-mode 不挂 (ADM-0 §1.3 红线)

## PR 出来 5 核对疑点
1. 黑名单 grep `cursor_v2|multiDeviceCursor|EventBusV2|fanoutPersist` count==0
2. thinking 5 字面 + typing 同义词反向 grep 0 hit
3. 0 schema 改 + EventBus byte-identical (DL-1 #609 锚 + factory 单源)
4. presence 字面跨 spec/content-lock/DOM/单测四处对锁 byte-identical
5. cov ≥85% (#613 gate) + 0 race-flake + admin grep 0 hit

# CHN-4 立场反查表 (channel 协作场骨架 demo, G3.4 退出闸)

> **状态**: v0 (野马, 2026-04-29)
> **目的**: CHN-4 实施 PR 直接吃此表为 acceptance; 飞马 spec #375 (锚) / 烈马 acceptance template (待派) / 战马拆段实施 review 拿此表反查立场漂移。一句话立场 + §X.Y 锚 + 反约束 (X 是, Y 不是) + v0/v1。
> **关联**: `channel-model.md` §1.1 (channel = 协作场: 聊天流 + workspace 双支柱) + §3.1 (workspace 升级为协作场另一支柱); `canvas-vision.md` §1.3 (workspace 内置 channel 自带, 权限继承); 飞马 spec brief #375 §0 ①②③ (集成 e2e 反再造 / 双 tab byte-identical / e2e 走真不 mock); execution-plan.md line 168 字面 G3.4 demo 退出公告硬约束 ("新建 channel → 默认 workspace → 邀 agent → 放 artifact; 双 tab 各 1 张截屏")。
> **依赖**: 已闭/进行中 9 milestone — CV-1 ✅ / CV-2 spec ✅ #356 v3 + server #359/#360 / CV-3 spec ✅ #363 + 文案锁 #370 / CV-4 spec ✅ #365 / CHN-1 ✅ / CHN-2 spec ✅ #357 + 文案锁 #354/#364 / CHN-3 spec ✅ #371 + stance #366 / RT-1 ✅ / DM-2.2 ✅ #361/#372 / CM-4 ✅ / AL-3 ✅。
> **#338 cross-grep 反模式遵守**: CHN-4 是集成 demo 不引入新组件字面池, 既有 ChannelView/ArtifactPanel/Sidebar 字面已稳定 (#288/#346/#347 等), 本 stance 字面跟既有 byte-identical 不臆想新词。

---

## 1. CHN-4 立场反查表 (channel 协作场骨架 demo)

| # | 立场锚 | 一句话立场 | 反约束 (X 是, Y 不是) | v0 / v1 |
|---|--------|----------|----------------------|---------|
| ① | spec #375 §0 ① + 蓝图 §1.1 双支柱 | **CHN-4 = 集成 e2e + 演示, 反"再造轮子"** — 不开新表 / 新 endpoint / 新 frame, 仅 wiring + e2e + 截屏归档 | **是** 复用 CHN-1/2/3 + CV-1/2/3/4 + RT-1/DM-2 + AL-3 既有 9 milestone 路径串端到端; **不是** 新 schema bump (CHN-4.1 **不抢 v=20**, 留账给后续真 schema 改的 milestone, 跟 CV-3 立场 ① + CV-4 立场 ② 同精神); **不是** 新 GET/scene 拼装端点 (走 client 4 调 Promise.all ≤200ms 够, 不开 server 拼装层) | v0/v1 同 — sequencing 锁字面延续到真有 schema 的 milestone |
| ② | spec #375 §0 ② + 蓝图 §1.1 双支柱 | **双 tab (chat + workspace) 视觉 byte-identical 跟既有 SPA + 文案锁同源** | **是** chat tab 走 CHN-1 既有 messages 路径 + agent 🤖 角标跟 CV-1 #347 立场 ⑥ 二元 byte-identical; workspace tab 走 CV-1.3 #346 ArtifactPanel 既有, kind 三态跟 #370 文案锁 ① `data-artifact-kind` byte-identical; **不是** 双 tab 交叉 (chat 不渲染 artifact body / workspace 不渲染 message); **不是** mention `@artifact` preview 改写 — 走 CV-3 #370 ⑥ 三模式独立路径 (md 80字 / code 5行+徽标 / image 192px) | v0: 双 tab DOM `data-tab="chat\|workspace"` byte-identical; v1 同 |
| ③ | spec #375 §0 ③ + execution-plan G3.1/G3.3 | **e2e 走真 server-go(4901) + vite(5174), 反 mock 走真路径** (跟 G3.1 #348 / G3.3 同模式) | **是** e2e 跑真 4901 server + 真 5174 SPA; **不是** server endpoint mock (`mock.*4901|jest.mock.*server-go|fakeServer.*4901` count==0); **要明确区分**: agent runtime stub via direct owner commit (CV-4 接管前 walkaround) 是 **runtime 行为 stub** 非 server mock — 实施 e2e 显式注释标 "// CV-4 runtime stub: direct owner commit (not server mock)" 区分两层 (跟 #375 §1 CHN-4.3 字面对应) | v0: runtime stub via direct commit; v1: AL-4 落地后切真 runtime |
| ④ | spec #375 §2 + CHN-2 7 源 byte-identical | **DM 视图永不含 workspace tab** (Phase 3 全 milestone 最稳反约束 7 源同根) | **是** DM 视图 DOM `[data-tab="workspace"]` count==0 跟 **7 源 byte-identical** (CHN-2 #354 ④ + #353 §3.1 + #357 立场 ② + #364 patch + #371 + #374 + 本 stance); **不是** DM 视图加 anchor / iterate / artifact 入口 (双 tab 不交叉立场 ② 延伸到 DM 视图全无 workspace 维度) | v0/v1 永久锁 — DM 是 1v1 私聊不是协作场 (蓝图 §1.2 字面) |
| ⑤ | spec #375 §0 + 蓝图 §1.1 + 跨 milestone byte-identical | **mention × artifact × anchor × iterate 四路径互不污染** (跨 milestone 立场承袭) | **是** mention 走 messages + message_mentions (DM-2 #314+#372); artifact 走 artifacts + artifact_versions (CV-1); anchor 走 artifact_anchors + anchor_comments (CV-2 #356 v3 + #359); iterate 走 artifact_iterations (CV-4 #365); **不是** messages 表加 artifact_id / iteration_id / anchor_id 反指 (DM-2 / CV-4 / CV-2 立场字面禁); **不是** workspace tab 渲染 message / chat tab 渲染 artifact body (双 tab 不交叉) | v0/v1 同 — 4 路径数据契约永久拆死 |
| ⑥ | spec #375 §1 CHN-4.2 + CHN-3 立场 ① 同精神 | **default_tab 是 server 端常量 "chat", 不是作者级偏好不是个人偏好** | **是** server `GET /channels/:id` body 加 `default_tab="chat"` 字段 byte-identical; **不是** `PUT /channels/:id/default_tab` 作者级偏好 endpoint (留 v3+); **不是** user_channel_layout 加 default_tab 列 (CHN-3 立场 ② "偏好仅 collapsed + position 两维" 锁); 个人偏好覆盖 default_tab 走 CHN-3 偏好层 (但 v1 不开 — server 端常量足够 demo 价值) | v0: server 常量 "chat"; v1: 加作者级 PUT (留 v3+) |
| ⑦ | spec #375 §1 CHN-4.3 + G2.4#5/G2.5/G2.6 demo 联签 | **G3.4 退出闸三签机制**: 战马 (e2e 真过 ≤3s) + 烈马 (acceptance template 对齐) + **野马 (双 tab 截屏文案锁验)** | **是** G3.4 demo 双 tab 截屏归档 `docs/qa/screenshots/g3.4-chn4-{chat,workspace}.png` (Playwright `page.screenshot()`) — 撑 Phase 3 退出公告; 野马签 = chat tab 文案 (#347 二元 🤖↔👤) + workspace tab kind 三态 (#370 ①) byte-identical 验; **不是** 单一角色签 (跟 G2.x demo 联签同模式, 缺一签则退出闸不通过); **不是** 截屏后期 PS 修改 (CI Playwright 主动截屏入 git, 防伪造) | v0: 三签 + 双截屏入 git; v1 同, 加 CI 自动比对 |

---

## 2. 黑名单 grep — CHN-4 实施 PR merge 后跑, 全部预期 0 命中 (除标 ≥1)

```bash
# 立场 ① CHN-4 不抢 v=20 (留账给后续真 schema 改的 milestone)
grep -rnE "CREATE TABLE.*chn_4|ALTER TABLE channels.*ADD.*tab|migrations.*v=20.*chn_4" packages/server-go/internal/migrations/ | grep -v _test.go
# 立场 ① 不开新拼装端点 (反 #374 GET /scene)
grep -rnE "GET /api/v1/channels/.*\\/scene|POST.*\\/scenes\\/|PUT.*\\/scenes\\/" packages/server-go/internal/api/ | grep -v _test.go
# 立场 ② 双 tab DOM 锚 (预期 ≥2 — chat + workspace 各 1)
grep -rnE 'data-tab=["'"'"'](chat|workspace)["'"'"']' packages/client/src/components/ChannelView.tsx | grep -v _test  # 预期 ≥2
# 立场 ② 双 tab 不交叉 (chat 不渲染 artifact body, workspace 不渲染 message)
grep -rnE 'data-tab=["'"'"']chat["'"'"'].*ArtifactPanel|data-tab=["'"'"']workspace["'"'"'].*MessageList' packages/client/src/ | grep -v _test
# 立场 ③ e2e 反 mock server (走真 4901)
grep -rnE 'mock.*4901|jest\\.mock.*server-go|fakeServer.*4901|nock.*4901' packages/e2e/tests/chn-4*.spec.ts 2>/dev/null | grep -v _test
# 立场 ④ DM 视图永不含 workspace tab (6+源 byte-identical 锁)
grep -rnE 'data-kind=["'"'"']dm["'"'"'].*data-tab=["'"'"']workspace["'"'"']|dm.*workspace.*tab' packages/client/src/components/ | grep -v _test
# 立场 ⑤ messages 不反指 artifact/iteration/anchor (4 路径数据契约拆死)
grep -rnE 'ALTER TABLE messages.*ADD.*artifact_id|ALTER TABLE messages.*ADD.*iteration_id|ALTER TABLE messages.*ADD.*anchor_id' packages/server-go/internal/migrations/ | grep -v _test.go
# 立场 ⑥ 不开作者级 default_tab PUT endpoint
grep -rnE 'PUT /api/v1/channels/.*/default_tab|POST.*channel.*tab.*config' packages/server-go/internal/api/ | grep -v _test.go
# 立场 ⑦ 双截屏归档 (预期 ≥2)
grep -rnE 'g3\\.4-chn4-(chat|workspace)\\.png|page\\.screenshot.*g3\\.4-chn4' packages/e2e/tests/chn-4*.spec.ts 2>/dev/null | grep -v _test  # 预期 ≥2
# 反约束: 不新起 WS frame (RT-1 4 frame 已锁: ArtifactUpdated 7 / AnchorCommentAdded 10 / MentionPushed 8 / IterationStateChanged 9, CHN-4 不引入第 5 个)
grep -rnE 'NewChannelTabFrame|ChannelTabChanged|TabSwitchedFrame' packages/server-go/internal/ws/ | grep -v _test.go
```

---

## 3. 不在 CHN-4 范围 (避免 PR 膨胀)

- ❌ 新数据表 / schema 改动 (立场 ① — 集成 demo 不再造轮子, 不抢 v=20 留账)
- ❌ 新 WS frame (tab 切换是 client URL state, 不上 server)
- ❌ 作者级 default_tab 偏好 endpoint (server 端常量 "chat" 足够 v1, PUT 留 v3+)
- ❌ chat tab 渲染 artifact body 内联 (mention preview 走 CV-3 #370 ⑥ 独立路径)
- ❌ workspace tab 渲染 message (双 tab 不交叉立场 ②)
- ❌ DM 视图加 workspace / anchor / iterate (CHN-2 立场 ② 6+源永久锁)
- ❌ admin SPA 看 channel chat + workspace (admin god-mode 不入业务路径, ADM-0 §1.3 红线)
- ❌ multi-channel 视图 / channel 切换器 (蓝图 §3.1 v1 不做)

---

## 4. 验收挂钩

- CHN-4.1 client wiring PR: 立场 ②④⑥ — `<ChannelView>` tab switcher (URL `?tab=chat|workspace` deep-link) + DM 反向断言无 workspace tab + 双 tab DOM byte-identical
- CHN-4.2 server PR: 立场 ⑥ — `GET /channels/:id` 返 `default_tab="chat"` 字面 + 反向断言无 schema 改 (channels 表 PRAGMA 不变) + §2 反约束 grep 立场 ① + ⑥ 0 命中
- CHN-4.3 e2e + 截屏 PR: 立场 ③⑤⑦ — e2e 走真 4901+5174 (注释区分 server mock vs runtime stub) + 4 路径互不污染 e2e 反断 + G3.4 双 tab 截屏归档 + **三签** (战马 e2e 真过 / 烈马 acceptance / 野马 双 tab 文案锁验)
- CHN-4 章程退出闸 (Phase 3 收口): 立场 ①-⑦ 全锚 + §2 反向 grep 全 0 (除 ≥1 标记) + 跨 milestone byte-identical (DM 永不含 workspace 7 源) + G3.4 双截屏 + 三签

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 野马 | v0, 7 立场 (集成 e2e 反再造 / 双 tab 视觉 byte-identical / e2e 走真不 mock + runtime stub 区分 / DM 永不含 workspace 6+ 源 byte-identical / 4 路径互不污染 / default_tab server 常量不裂偏好 / G3.4 三签 + 双截屏) 承袭 #375 spec brief 3 立场拆细 + 跨 milestone byte-identical 锁; 10 行反向 grep (含 8 反约束 + 2 预期 ≥1) + 8 项不在范围 + 验收挂钩三段对齐 + 章程退出闸字面. 跟既有 cross-grep #338 反模式: CHN-4 集成 demo, 既有 ChannelView/ArtifactPanel/Sidebar 字面 (#288/#346/#347 等) 已稳定不动 |
| 2026-04-29 | 野马 | v0.x patch — cross-milestone reason count audit (跟 #461 同模式 follow-up): "五处单测锁"/"四处单测锁"/"三处单测锁" → "六处单测锁" (AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461); CHN-4 stance ④ "6 源" → "7 源" 跟 body 列举 7 源对齐. 跟 #339/#393/#387/#461 follow-up patch 同模式, 历史干净 |

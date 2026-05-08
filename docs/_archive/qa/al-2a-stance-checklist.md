# AL-2a 立场反查表 (agent 配置 SSOT 表 + update API)

> **状态**: v0 (野马, 2026-04-29)
> **目的**: AL-2a 实施 PR 直接吃此表为 acceptance; 飞马 spec brief / 烈马 acceptance template `al-2a.md` 7 项 / 战马A 实施 review 拿此表反查立场漂移. 一句话立场 + §X.Y 锚 + 反约束 (X 是, Y 不是) + v0/v1.
> **关联**: 蓝图 `plugin-protocol.md` §1.4 (Borgee SSOT 字段划界 — 用户做选择 vs 系统调优) + §1.5 (热更新分级); 蓝图 `agent-lifecycle.md` §2.1 (用户完全自主决定 agent 的 name/prompt/能力/model); R3 决议 AL-2 拆 a/b (AL-2a 落 SSOT 表 + REST update, AL-2b 切 BPP `agent_config_update` frame); 烈马 acceptance #al-2a.md 7 项 (REG-AL2A-001..007 占号); 野马 #al-2a-content-lock.md 7 处字面锁; AL-1a #249 6 reason codes byte-identical (跨 milestone reason 八处单测锁源).
> **依赖**: 无 (AL-2a 是独立 milestone, 蓝图 §1.4 字面对应); 跟 BPP-1 #304 envelope 无 frame 冲突 (AL-2a 走 REST 不裂 frame); 跟 AL-1a #249 / AL-3 #310 / AL-4 #379 reason byte-identical 同源.
> **#338 cross-grep 反模式遵守**: AL-2a 是新表新功能 (agent_configs), 既有 lib/agent-state.ts REASON_LABELS (#249) 字面已稳定, 立场跟既有 byte-identical 引用不臆想新词。

---

## 1. AL-2a 立场反查表 (SSOT 表 + REST update API)

| # | 立场锚 | 一句话立场 | 反约束 (X 是, Y 不是) | v0 / v1 |
|---|--------|----------|----------------------|---------|
| ① | plugin-protocol §1.4 + agent-lifecycle §2.1 | **Borgee 是 agent 配置 SSOT** (用户做选择项), runtime 是执行面 (系统调优项) — 字段划界永久锁 | **是** Borgee 管 7 字段: `name` / `avatar` / `prompt` / `model` / `capabilities` / `enabled` / `memory_ref` (蓝图 §1.4 表字面 byte-identical); **不是** Runtime-only 字段进 Borgee schema (`temperature` / `token_limit` / `api_key` / `retry_policy` / `rate_limit` / `memory_content` 蓝图 §1.4 字面禁); **不是** 双源 (Borgee 不再 mirror runtime 字段, runtime 不再 mirror Borgee 字段) | v0/v1 永久锁 — SSOT 拆死是产品定位红线 |
| ② | spec AL-2a + R3 决议 + 烈马 acceptance §1.4 | **agent_configs 整 blob 替换, 不是 multi-row config_key** — schema 锁 `(agent_id PK, schema_version int, blob JSON, updated_at)` | **是** 单行 per agent 整 blob JSON (整体替换语义 + JSON schema 校验 by application layer); **不是** `agent_config(agent_id, key, value)` multi-row 模式 (反 SSOT 漂 — multi-row 易丢/易错算 last-writer-wins / 难原子替换); **不是** 第 4 列拆字段 (反 schema 爆炸); **不是** blob 二级 cache (跟 AL-3 #310 / CV-1 #347 锁 ② 同精神 — 不裂 cache) | v0: agent_configs 单表整 blob; v1 同 |
| ③ | spec AL-2a + 烈马 acceptance §4.1.a | **schema_version 单调递增, last-writer-wins** — server 端单调发号, 不依赖 client 端版本号 | **是** server PATCH `/config` → 原子事务 INSERT new row (UNIQUE(agent_id) → REPLACE 或 UPDATE) + schema_version = MAX(prev) + 1 + updated_at = NOW(); 并发 2 写 → 末次胜出 + schema_version 严格递增 + 无丢失 (idempotent: 同 payload 重发不增 version, 跟 acceptance §4.1.a 同源); **不是** first-write-wins (那是 CV-1 #347 锁 ② 30s TTL 路径, AL-2a 不裂); **不是** client 端算 schema_version 回传 server (server 单调发号锁 — 反 race condition); **不是** 多版本历史 (跟 CV-1 立场 ③ 线性版本历史不同 — AL-2a 是配置覆盖, 历史走 audit_log 留 ADM-2 #266) | v0: last-writer-wins server 单调; v1 同 |
| ④ | spec AL-2a + 烈马 acceptance §4.1.b + ADM-0 §1.3 红线 | **PATCH owner-only — 跨 agent owner 调用 / admin god-mode 写 → 403** | **是** `RequirePermission('agent.config.update')` 默认仅 grant agent.owner_id (跟 AL-4 立场 ② "启停 owner-only" 同模式 + AL-3 #303 ⑦ god-mode 字段白名单同精神); admin god-mode `GET /admin/agents/:id/config` **仅返元数据**, 不返 blob 内容 + admin 不入 PATCH 路径 (跟 ADM-0 §1.3 红线 + AL-4 #379 v2 §3 grep `last_error_reason.*admin` 0 hit 同模式); **不是** admin 可写 (反向 grep `admin.*config.*update|admin.*agent_configs.*PATCH` count==0); **不是** 任意 channel member 可写 (跟 channel-model §1.4 owner-only 同模式) | v0: owner-only 闸 + admin 元数据 only; v1 同, 加 audit_log 行 (跟 ADM-2 #266 同 schema) |
| ⑤ | plugin-protocol §1.5 + R3 决议 + 烈马 acceptance §蓝图行为对照 | **AL-2a 不含 BPP frame `agent_config_update`** — agent 端 reload 走轮询临时, BPP frame 留 AL-2b 跟 BPP-3 同合 | **是** AL-2a 走 REST `PATCH /config` + agent runtime 端 GET `/api/v1/agents/:id/config` 轮询 reload (临时); **不是** AL-2a emit BPP frame (跟 acceptance §蓝图行为对照 grep `agent_config_update` count==0 + RT-1 4 frame + BPP-1 9 frame 已锁 + AL-4 立场 ⑥ "不裂 runtime-only frame" 同模式); **不是** AL-2a 等 BPP-3 落地 (AL-2a/AL-2b 拆段, AL-2a 不阻塞); **不是** agent 端 push 触发 reload (走 client poll 临时, 跟 RT-1 cursor 单调 frame 拆死 — AL-2a 不进 frame 路径) | v0: REST + 轮询; v1: AL-2b 切 BPP frame, AL-2a 不破 |
| ⑥ | plugin-protocol §1.4 + 蓝图字段 marshal 红线 | **runtime 上报 model schema, Borgee UI 通用渲染, 不写死 OpenClaw/Hermes 模型列表** | **是** runtime 通过 `runtime_schema_advertise` 上报 model list + metadata; Borgee UI 走 `<select>` 通用渲染读 schema (跟蓝图 §1.4 "关键设计: runtime 上报 model schema" 字面同源); model 字段 `string` 类型存任意值 (server 端不校验枚举, runtime 自负责); **不是** Borgee server 写死 `gpt-4`/`claude-3` 枚举 CHECK (跟反向 grep `'gpt-4'|'claude-3'` 0 hit 同源); **不是** UI 写死下拉模型列表 (反 AL-2a UI 写死, 跟 #al-2a-content-lock ② "不准 model 字段写死下拉" 同精神); **不是** Borgee 校验 model 跟 runtime 兼容 (那是 runtime 错时返 error, AL-2a 不前置校验) | v0: model 通用渲染; v1 同 |
| ⑦ | plugin-protocol §1.4 memory 边界 + 蓝图 #11 沉默胜于假活物感 | **memory_ref 是 Borgee, memory 内容是 runtime — v1 不让 Borgee 变向量库** | **是** `memory_ref` 字段存指针 string (路径 / ID / URL); memory 内容由 runtime 维护 (向量库/RAG 索引); **不是** Borgee 存 memory 内容 (蓝图 §1.4 "v1 不让 Borgee 变向量库基础设施" 字面禁); **不是** Borgee 解析 memory_ref (透明传递 — 跟蓝图 §1.4 "runtime 自己的私有 opaque blob 字段, Borgee 不解读, 只透明传递" 同精神); **不是** form 显示 memory 内容编辑器 (跟 #al-2a-content-lock ② 反约束同源); 配置变化 — agent silent default (跟 #11 + AL-3 #305 ③ "agent join silent" 同精神, 不发 system message / 不 fanout) | v0/v1 永久锁 — Borgee 不入 memory 内容侧 |

---

## 2. 黑名单 grep — AL-2a 实施 PR merge 后跑, 全部预期 0 命中 (除标 ≥1)

```bash
# 立场 ① — agent_configs 表不含 runtime-only 字段 (蓝图 §1.4 字面禁)
grep -rnE 'agent_configs.*ADD.*(temperature|token_limit|api_key|retry_policy|rate_limit|memory_content)' packages/server-go/internal/migrations/ | grep -v _test.go
# 立场 ① — blob JSON schema 反向断言 (CI grep 守 SSOT 字段池)
grep -rnE "['\"](temperature|token_limit|api_key|retry_policy|rate_limit|memory_content)['\"]" packages/server-go/internal/api/agent_configs*.go 2>/dev/null | grep -v _test.go
# 立场 ② — agent_configs 表 schema 字段固化 (预期 ≥1 — 4 列锁)
grep -rnE "CREATE TABLE.*agent_configs.*\\(.*agent_id.*schema_version.*blob.*updated_at" packages/server-go/internal/migrations/ 2>/dev/null | grep -v _test.go  # 预期 ≥1
# 立场 ② — 反 multi-row config_key 模式
grep -rnE "agent_config.*\\(agent_id.*key.*value\\)|CREATE TABLE.*agent_config_entries|config_key" packages/server-go/internal/migrations/ | grep -v _test.go
# 立场 ③ — schema_version 单调 (server 端单调发号, 反 client 端算回传)
grep -rnE 'r\\.URL\\.Query\\(\\)\\.Get\\("schema_version"|body.*schema_version.*=' packages/server-go/internal/api/agent_configs*.go 2>/dev/null | grep -v _test.go
# 立场 ④ — owner-only RequirePermission (预期 ≥1 — PATCH endpoint)
grep -rn 'RequirePermission..agent\\.config\\.update' packages/server-go/internal/api/agent_configs*.go 2>/dev/null | grep -v _test.go  # 预期 ≥1
# 立场 ④ — admin 不入 PATCH (god-mode 不返 blob)
grep -rnE 'admin.*config.*update|admin.*agent_configs.*PATCH|GodModeFields.*blob' packages/server-go/internal/api/admin*.go | grep -v _test.go
# 立场 ⑤ — AL-2a 不裂 BPP frame (留 AL-2b)
grep -rnE 'agent_config_update' packages/server-go/internal/ws/ packages/server-go/internal/bpp/ | grep -v _test.go
# 立场 ⑤ — agent runtime 端轮询 reload (临时路径, 反 push)
grep -rnE 'agent.*push.*config|hub.*broadcast.*config|fanout.*agent_configs' packages/server-go/internal/ws/ | grep -v _test.go
# 立场 ⑥ — Borgee 不写死模型枚举
grep -rnE "model.*CHECK.*\\('gpt-4'|'claude-3'|'gemini'\\)|model.*IN \\('gpt" packages/server-go/internal/migrations/ | grep -v _test.go
# 立场 ⑦ — memory_ref 是指针不存内容 + agent silent default (无 system message broadcast)
grep -rnE 'memory_content.*BLOB|agent_configs.*memory_content' packages/server-go/internal/migrations/ | grep -v _test.go
grep -rnE "['\"]\\{agent_name\\} 已更新设置['\"]|agent.*config.*system.*message.*broadcast" packages/server-go/internal/api/ | grep -v _test.go
```

---

## 3. 不在 AL-2a 范围 (避免 PR 膨胀, 跟 spec + acceptance 同源)

- ❌ runtime-only 字段配置 (蓝图 §1.4 字面禁, AL-2a 永久锁)
- ❌ BPP frame `agent_config_update` (R3 决议留 AL-2b, 跟 spec + acceptance §1.5 字面)
- ❌ multi-row config_key 模式 (反 SSOT 漂, 立场 ② 锁)
- ❌ admin SPA 改 agent_configs (admin 不入业务路径, ADM-0 §1.3 红线 + 立场 ④)
- ❌ memory 内容编辑 / 向量库存储 (蓝图 §1.4 "v1 不让 Borgee 变向量库" 字面禁)
- ❌ schema_version client 端锁 / 多版本历史 (AL-2a 是配置覆盖非线性版本, 立场 ③ + CV-1 立场 ③ 拆死)
- ❌ Borgee 写死 OpenClaw/Hermes 模型枚举 (走 runtime_schema_advertise, 立场 ⑥)
- ❌ agent self-update (跟 #11 silent default + agent 不能自 grant 同源, runtime 配置由 owner 决定)
- ❌ agent 端 push reload (走 client poll 临时, 等 AL-2b 切 BPP frame)
- ❌ system message broadcast 设置变化 (跟 #11 silent default 永久锁, 立场 ⑦)

---

## 4. 验收挂钩

- AL-2a schema PR (新 v=N): 立场 ①②③ — `agent_configs` 表 4 列 (`agent_id PK / schema_version int / blob JSON / updated_at`) + 反向断言无 runtime-only 字段 + 反 multi-row config_key 模式
- AL-2a server PR: 立场 ①③④⑤⑥⑦ — `PATCH /config` owner-only `RequirePermission('agent.config.update')` ≥1 hit + 整 blob 替换 atomic + schema_version server 单调发号 + admin god-mode 字段白名单不返 blob + AL-2b BPP frame 不裂 + 反向断言无 system message broadcast
- AL-2a entry 闸: 立场 ①-⑦ 全锚 + §2 黑名单 grep 全 0 (除标 ≥1) + 跨 milestone byte-identical (reason 八处单测锁 AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-4 #387/#461 + AL-1b #458 + 野马 #al-2a-content-lock + AL-2b #481 frame + ack handler 第 7+8 处) + RT-1 4 frame + BPP-1 9 frame 锁守 + AL-2b 留账锁

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 野马 | v0, 7 立场 (Borgee SSOT 字段划界永久锁 / 整 blob 替换反 multi-row / schema_version server 单调反 client 算 / PATCH owner-only admin 元数据 only / AL-2a 不裂 BPP frame 留 AL-2b / model 通用渲染不写死枚举 / memory_ref 指针不存内容) 承袭蓝图 §1.4 字段划界 + R3 决议 AL-2 拆 a/b + 跨 milestone byte-identical (reason 五处单测锁 + admin 元数据 only 跟 AL-3 ⑦ + AL-4 ② 同模式); 12 行反向 grep (含 2 预期 ≥1 + 10 反约束) + 10 项不在范围 + 验收挂钩二段对齐. #338 cross-grep 反模式遵守: 既有 REASON_LABELS (#249) + AL-3/CV-4/AL-4 立场字面已稳定, 本 stance 跟既有 byte-identical 引用 |
| 2026-04-29 | 野马 | v0.x patch — cross-milestone reason count audit (跟 #461 同模式 follow-up): "五处单测锁"/"四处单测锁"/"三处单测锁" → "六处单测锁" (AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461); CHN-4 stance ④ "6 源" → "7 源" 跟 body 列举 7 源对齐. 跟 #339/#393/#387/#461 follow-up patch 同模式, 历史干净 |

# REFACTOR-1 stance checklist — 4 helper SSOT + 行为不变量 byte-identical (refactor only)

> 5 立场 byte-identical 跟 refactor-1-spec.md (飞马 v0 89 行) §0 + §2. **Refactor milestone — 真有 prod code (4 helper SSOT 拆出) 但 0 schema + 0 endpoint + 0 migrations + 0 用户体验改**. 跟 INFRA-3 #594 + RT-3 #588 + AP-4-enum #591 + TEST-FIX-1 #596 + DL-1 #609 真有 prod code wrapper/refactor 类别同模式承袭. content-lock 不需 (refactor 0 文案改, 跟 TEST-FIX-1/2/3 同精神).

## 1. 行为不变量 byte-identical (content-lock 字面不漂)

- [ ] **DM-gate 4 字面 byte-identical** (refactor before/after grep count 等同):
  - `DM 不参与个人分组` (字面在 channels.go DM-gate 等位置, count 不变)
  - `layout.dm_not_grouped` (错码字面 byte-identical)
  - `dm.edit_only_in_dm` (错码字面 byte-identical, 跟 dm_4_message_edit.go 同精神承袭)
  - `metadata.target` (字面在 message metadata 路径 byte-identical)
- [ ] **既有 user-facing 字面 0 改** — 反向 grep `git diff` 应 0 hit `+.*"DM"` / `+.*"频道"` / `+.*"消息"` 等 user-facing string 新增/改 (refactor 仅迁字面到 helper SSOT, 字面值不动)
- [ ] **既有错码字面 0 改** — `dm.*` / `pin.*` / `chn.*` / `dm_search.*` / `auth.*` 错码 before/after byte-identical (反 refactor 顺手改错码漂)
- [ ] **既有 DOM data-attr 字面 0 改** — refactor 仅 server 侧 helper SSOT, client DOM 不动 (反 refactor 漂入 client)

## 2. 用户主权不破 (refactor 0 用户体验改)

- [ ] **0 endpoint shape 改** — `git diff --name-only | grep -E 'internal/api/.*\.go' | xargs grep -lE 'http\.Method|r\.Method'` 应 0 行新增/改 method/path
- [ ] **0 user-facing behavior 改** — 反向断言 e2e + 既有 unit 全 PASS byte-identical (反 refactor 改语义)
- [ ] **0 ACL gate 改** — channel-member ACL / owner-only ACL / DM-only path 等既有 gate 字面 + 行为 byte-identical (跟 anchor #360 owner-only 锁链 22+ PRs 立场承袭)
- [ ] **admin god-mode 不挂** — 反向 grep `admin.*<helper-name>` 0 hit (ADM-0 §1.3 红线 + PR #571 §2 ⑥ 精神延伸)
- [ ] **agent ↔ human 同源** — refactor 不分 sender 类型 (PR #568 §4 端点延伸承袭)

## 3. 4 helper SSOT 拆死 (反"顺手清理")

- [ ] **4 helper byte-identical 跟 spec §1 表** (待 spec brief commit 后真填名):
  - helper-1 (例: `IsDMChannel(ch *Channel) bool`) — DM-gate 单源, 反 N+ 处分散 `ch.Type == "dm"`
  - helper-2 (例: `RequireChannelMember(...)` ) — channel-member ACL 单源 (复用 AP-4 #551 + AP-5 #555 既有 helper byte-identical)
  - helper-3 (例: `RequireOwner(...)` ) — owner-only 单源 (跟 anchor #360 锁链承袭)
  - helper-4 (例: `MaskDeleted(...)` ) — soft-delete leak 守 (跟 dm_4_message_edit.go + DM-11 #600 maskDeletedMessages 同精神)
- [ ] **反"顺手清理" 5 模式 reject** (refactor scope 严守):
  - ❌ 顺手改函数命名 (反 PR scope 红线 + 反 git 历史可追溯)
  - ❌ 顺手改错码字面 (反字面 byte-identical)
  - ❌ 顺手改 ACL 行为 (反用户主权)
  - ❌ 顺手加新 helper 不在 spec 4 个内 (反 SSOT 拆死)
  - ❌ 顺手删既有 helper (反"行为不变量 byte-identical")
- [ ] **helper 单源闸反向 grep**:
  - `^func IsDMChannel|^func RequireChannelMember|^func RequireOwner|^func MaskDeleted` count==4 (待 spec 真填) 单源 SSOT
  - 反向 grep 旧调用点 (例: `ch.Type == "dm"` 在 internal/api/ 除 helper) 应 0 hit (refactor 真兑现 SSOT 单源)
- [ ] **Go idiom 命名锁** — `Is*` / `Require*` / `Mask*` 风格 byte-identical (反 `Check*` / `Verify*` / `Validate*` 同义词漂)

## 4. 0 schema / 0 endpoint / 0 migrations

- [ ] 反向 grep `migrations/refactor_1_` 0 hit (Refactor 0 schema 改)
- [ ] 反向 grep 新 endpoint path 在 server.go register 0 hit (0 endpoint 加)
- [ ] 反向 grep `internal/migrations/*.go` 新文件 0 (0 migration)
- [ ] schema version `currentSchemaVersion` 不动 (反向断 git diff 0 行)
- [ ] 既有 unit tests 全 PASS (CHN-* / DM-* / AL-* / HB-* 等既有不动)

## 5. 跨 milestone 立场承袭不破 (CHN-* / DM-* / AL-* / HB-*)

- [ ] **CHN-* 立场承袭** — CHN-1 archived_at + CHN-5 archive UI + CHN-6 PinThreshold + CHN-7 mute + CHN-13 search + CHN-14 description audit + CHN-15 readonly 字面 + 行为 byte-identical
- [ ] **DM-* 立场承袭** — DM-3/4/5/7/9/10/11/12 + DM-only path 锁链 + dm_4_message_edit.go DM-gate 字面 byte-identical
- [ ] **AL-* 立场承袭** — AL-1a 6-dict + AL-3 PresenceTracker + AL-7 archived_at sweeper + AL-8 字面 + 行为 byte-identical
- [ ] **HB-* 立场承袭** — HB-1 7-dict + HB-2 8-dict + HB-2.0 + HB-2 v0(C) + HB-3 v2 字面 + 行为 byte-identical
- [ ] **anchor 锁链承袭** — anchor #360 owner-only + REG-INV-002 fail-closed + ADM-0 §1.3 admin god-mode 不挂 红线 byte-identical 不破

## 反约束 — REFACTOR-1 真不在范围

- ❌ 改 production behavior (refactor scope 严守, 反语义改)
- ❌ 0 schema / 0 endpoint / 0 migrations / 0 client / 0 acceptance template / 0 content-lock 改
- ❌ 加 retry / sleep / timeout 调整 (反 mask, 跟 TEST-FIX-1/2/3 精神承袭)
- ❌ admin god-mode 加挂任何 helper (永久不挂, ADM-0 §1.3 红线)
- ❌ 加新 CI step (跟 INFRA-3 + TEST-FIX-* 同精神, 既有 step byte-identical)
- ❌ 改函数命名 (反 git 历史可追溯; 仅迁字面到 helper SSOT, 名 byte-identical)
- ❌ 加 helper 不在 spec 4 个内 (反 SSOT 拆死)

## 反约束 — DM-gate 字面 before/after byte-identical grep 真测

```
# refactor 前 grep (基线 baseline):
git grep -nE '"DM 不参与个人分组"|"layout\.dm_not_grouped"|"dm\.edit_only_in_dm"|"metadata\.target"' packages/server-go/internal/

# refactor 后 grep (期 count 等同 baseline, 仅出现位置可能从分散迁到 helper SSOT):
git grep -nE '"DM 不参与个人分组"|"layout\.dm_not_grouped"|"dm\.edit_only_in_dm"|"metadata\.target"' packages/server-go/internal/

# 真测期: 字面 count == baseline (4 字面各自 count byte-identical)
```

## 跨 milestone byte-identical 锁链 (5 链)

- **DL-1 #609 wrapper milestone 同精神** — DL-1 是 4 interface 抽象 (Storage/Presence/EventBus/Repository), REFACTOR-1 是 4 helper SSOT (DM-gate/ACL/owner/mask) 同模式承袭 — 真有 prod code refactor 类别第 N 处
- **dm_4_message_edit.go #549 DM-only path 锁链** — REFACTOR-1 把 DM-gate 字面集中到 helper SSOT 同精神承袭 (反分散字面 N+ 处)
- **AP-4 #551 + AP-5 #555 channel-member ACL helper 复用** — REFACTOR-1 不另起 ACL helper, 复用既有 (跟 DM-9 + DM-10 + DM-11 + DM-12 同模式)
- **anchor #360 owner-only ACL 锁链** — REFACTOR-1 owner-only helper SSOT 跟 22+ PRs 立场延伸承袭, 字面 + 行为 byte-identical 不破
- **maskDeletedMessages helper 锁链** — REFACTOR-1 把 mask 字面集中到 helper SSOT, 跟 dm_4_message_edit.go + DM-11 #600 maskDeletedMessages 同精神承袭

## PM 立场拆死决策

**Refactor 真有 prod code vs 改 production 行为拆死**:
- ✅ REFACTOR-1 = 4 helper SSOT 拆出 (本 PR 选, 0 schema + 0 endpoint + 0 行为改)
- ❌ 反"refactor 顺手改语义" (反用户主权 + 反字面锁)
- ✅ 跟 DL-1 #609 wrapper milestone (4 interface) + INFRA-3 #594 拆分 + TEST-FIX-* 真有 prod code refactor 类别同模式承袭

**4 helper SSOT vs 顺手清理 5 模式拆死**:
- ✅ 4 helper byte-identical 跟 spec (反第 5 helper 漂入 / 反 3 偷工减料)
- ❌ 反"refactor 顺手改命名" (反 git 历史可追溯)
- ❌ 反"refactor 顺手改错码" (反字面 byte-identical)
- ❌ 反"refactor 顺手改 ACL 行为" (反用户主权)
- ❌ 反"refactor 顺手加 helper" (反 SSOT 拆死)

**字面 byte-identical vs SSOT 单源拆死立场**:
- ✅ 字面值 byte-identical (4 DM-gate 字面 grep count 等同 baseline)
- ✅ 字面位置从分散 N+ 处迁到 helper SSOT (refactor 真兑现 SSOT)
- ❌ 反"refactor 改字面" (反 user-facing 不变量)

## 用户主权红线锚 (5 项)

- ✅ **行为不变量 byte-identical** — refactor 0 用户体验改 (e2e + unit 全 PASS byte-identical)
- ✅ **既有 ACL gate 字面 + 行为 byte-identical** (anchor #360 owner-only + REG-INV-002 fail-closed 守)
- ✅ **0 user-facing change** — refactor 不动 client UI / 文案 / 翻译键 / DOM data-attr
- ✅ **0 production behavior 改** — refactor scope 严守 (反向断言 unit + e2e)
- ✅ **admin god-mode 不挂 helper** — 反向 grep 0 hit (ADM-0 §1.3 红线 + PR #571 §2 ⑥ 精神延伸)

## PR 出来 4 核对疑点 (PM 真测)

1. **DM-gate 4 字面 grep count baseline byte-identical** — refactor 前后 `git grep -cE "..."` count 等同 (字面值 0 漂)
2. **0 schema / 0 endpoint / 0 migration** — `git diff --name-only origin/main..HEAD | grep -E 'migrations/|server\.go.*Register'` 应 0 hit
3. **既有 unit tests 全 PASS byte-identical** — server-go ./... 全 25+ packages 全绿 (+sqlite_fts5 tag) 含 CHN-* / DM-* / AL-* / HB-* 既有验证
4. **4 helper count==4 真守门** — `^func (IsDMChannel|RequireChannelMember|RequireOwner|MaskDeleted)` count==4 (待 spec 真填) + 旧分散调用点反向 grep 0 hit (SSOT 单源真兑现)

# Acceptance Template — ADMIN-SPA-SHAPE-FIX (6 client/server shape + admin gate drift 真修)

> Spec brief `admin-spa-shape-fix-spec.md` (飞马 v0.2). Owner: 战马E 实施 / 飞马 review / 烈马 验收.
>
> **ADMIN-SPA-SHAPE-FIX 范围**: 6 真 drift (D1 login P1 + D2 AdminSession + D3 member_count + D4 archived_at + D5 note + D6 admin gate) 真修 byte-identical 跟 server SSOT (ADM-0.1/0.2 + AL-8 + CAPABILITY-DOT #628). server diff ≤13 行 (D4 sanitizer +5 行 + D6 gate +5 行 + struct field +3 行); client 改最小补丁 ~20 行.

## 验收清单

### §1 行为不变量 (6 drift 真修)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 D1 login body+sig 真改 — `adminLogin(login, password)` 发 `{login, password}` byte-identical 跟 server `loginRequest{Login, Password}` | unit | `admin-api-shape.test.ts::REG-ASF-D1.body` + `_D1.sig` PASS |
| 1.2 D2 AdminSession 重写 `{id: string, login: string}` byte-identical 跟 server handleMe `{id, login}` (auth.go:281,314) | unit | `_D2` PASS — 反向 0 hit role/username/admin_id/expires_at |
| 1.3 D3 AdminChannel.member_count 死字段真删 + ChannelsPage.tsx 不再渲染 | unit + grep | `_D3` PASS |
| 1.4 D4 走 A — server 加 ArchivedAt struct + sanitizeAdminAction surface; client 加 archived_at 字段 + UI 三态 row class | Go unit + vitest | server build PASS + `_D4` vitest + `data-archived-state` row attr 真挂 |
| 1.5 D5 InviteCode.note 收紧 `string` non-null | unit | `_D5` PASS |
| 1.6 D6 admin-rail capability gate — handleGrantPermission 加 `auth.IsValidCapability` 守门, invalid → 400 `invalid_capability` | Go unit | `admin_grant_permission_gate_test.go` 4 case PASS (valid dot 200 / legacy snake 400 / typo 400 / empty 400) |

### §2 数据契约 (server diff ≤13 行 + 0 endpoint URL/schema/routes)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 server diff ≤13 行 production code (D4 +5 + D6 +5 + struct +3 = 13) | git diff | `git diff origin/main -- packages/server-go/ \| grep -cE "^\+"` ≤13 |
| 2.2 0 endpoint URL 改 / 0 schema migration / 0 routes.go 改 | grep | reverse grep ALTER/migration v=N 0 hit |
| 2.3 admin-rail SSOT (CookieName / loginRequest / handleMe writeJSON) 字面 byte-identical 不动 | inspect | `git diff origin/main -- packages/server-go/internal/admin/auth.go` 0 行 |

### §3 closure (REG + cov gate + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 既有全包 unit + vitest 全绿不破 + post-#629 haystack gate 三轨过 | full test + CI | go-test-cov SUCCESS + 106/106 vitest PASS |
| 3.2 立场承袭 — ADM-0.1/0.2 server SSOT byte-identical + AL-8 archived 三态 + ADM-2 #484 admin god-mode + CAPABILITY-DOT #628 14 const SSOT + ADM-0 §1.3 admin/user 路径分叉 | inspect | spec §0+§4 byte-identical |

## REG-ASF-* (initial ⚪ → 🟢)

- REG-ASF-001 🟢 D1 login body+sig 真改 byte-identical 跟 server loginRequest
- REG-ASF-002 🟢 D2 AdminSession `{id, login}` byte-identical 跟 server handleMe writeJSON
- REG-ASF-003 🟢 D3 AdminChannel.member_count 死字段真删 + UI 跟随
- REG-ASF-004 🟢 D4 走 A — server ArchivedAt struct + sanitizer surface + client UI 三态 row class
- REG-ASF-005 🟢 D5 InviteCode.note `string` non-null (类型 narrowing 收紧)
- REG-ASF-006 🟢 D6 admin-rail handleGrantPermission IsValidCapability gate (400 invalid_capability)
- REG-ASF-007 🟢 server diff ≤13 行 + 0 endpoint URL/schema/routes 改 + 6 vitest + 4 Go unit 全绿

## 退出条件

- §1 (6) + §2 (3) + §3 (2) 全绿 — 一票否决
- 6 drift 真修 byte-identical 跟 server SSOT
- server diff ≤13 行 production
- 6+ vitest 真测 D1-D5 shape + 4+ Go unit 真测 D6 gate
- 登记 REG-ASF-001..007

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | 烈马 | v0 — acceptance template. 立场承袭 ADM-0.1/0.2 server SSOT + ADM-2 #484 admin god-mode + AL-8 archived 三态 + CAPABILITY-DOT #628 14 const + ADM-0 §1.3 红线 + post-#629 wave 合后启动. zhanma-c/e 三轮 audit 抓 6 drift. |
| 2026-05-01 | 战马E | v1 实施 — D1 client adminLogin sig+body / D2 AdminSession {id,login} interface / D3 member_count 删 / D4 server ArchivedAt+sanitizer +client UI archived state row class / D5 InviteCode.note string non-null / D6 admin handleGrantPermission IsValidCapability gate +400 invalid_capability + 4 unit + 8 vitest. server diff ~13 行. REG-ASF-001..007 ⚪→🟢. |

# Acceptance Template — AP-4-enum: capability 清单 enum 化 + reflect-lint 单源

> Spec: `docs/implementation/modules/ap-4-enum-spec.md` (飞马 v0, 71 行)
> 蓝图: `docs/blueprint/current/auth-permissions.md` §1.1 (ABAC + capability list = SSOT) + §3 反 hardcode drift
> Stance: `docs/qa/ap-4-enum-stance-checklist.md` (野马 / 飞马 v0)
> 前置: AP-1 #493 capabilities.go 14 const ✅ + reasons SSOT 包 #496 ✅ + BPP-4 / HB-3 release-gate ✅
> 命名澄清: 跟 AP-4 #551 reactions ACL 共用 AP-4 标号 — 本 acceptance 用 `ap-4-enum.md` 避双义.
> Owner: 战马 (主战) / 飞马 (spec) / 烈马 (acceptance)

## 验收清单

### 立场 ① — `var ALL` ordered slice + init() 自动 rebuild Capabilities map

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 `var ALL = []string{...}` ordered slice 落 `internal/auth/capabilities.go`, byte-identical 跟 14 const 声明顺序 | unit | 战马 | `capabilities.go::ALL` + `TestAP4E1_ALL_OrderedByteIdentical` |
| 1.2 `func init()` 自动 rebuild `Capabilities = make(map[string]bool); for _, c := range ALL { Capabilities[c] = true }` (反约束: 不在 var 直接 init map literal) | unit | 战马 | `capabilities.go::init` + `TestAP4E1_Capabilities_AutoBuildFromAll` |
| 1.3 `len(ALL) == 14` 锁 (跟 AP-1 #493 14 const byte-identical) | unit | 战马 | `TestAP4E1_ALL_Length14` |
| 1.4 reflect-lint 验 ALL 内的字符串 == 14 const 字面值 (无 orphan const, 无 ALL 多余) | unit | 战马 | `capabilities_lint_test.go::TestAP4E1_reflect_lint_NoOrphanConst` + `TestAP4E1_reflect_lint_NoExtraInMap` |
| 1.5 admin god-mode 字面不入 ALL (ADM-0 §1.3 红线 反向 grep) | unit | 战马 | `TestAP4E1_NoAdminGodModeInALL` (反向 grep `admin_|godmode_|impersonat`) |

### 立场 ② — 反向 grep CI 守门 (handler hardcode capability 字面 0 hit)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 `.github/workflows/release-gate.yml` 加 step `ap4enum-no-hardcode-capability` (跟 BPP-4 AST scan / HB-3 dict-isolation 同模式) | CI step | 战马 | `release-gate.yml::ap4enum-no-hardcode-capability` step 字面 |
| 2.2 反向 grep `git grep -nE 'HasCapability\("[a-z_]+"' packages/server-go/internal/api/` count==0 in production code (允许 `_test.go` 白名单) | CI script + unit | 战马 | step bash + `TestAP4E2_ReverseGrep_HardcodeCapability` |
| 2.3 一行 hardcode → CI fail-block (workflow exit 1) | CI script | 战马 | step bash 行尾 `&& exit 1` 字面 |
| 2.4 step 名 `ap4enum-no-hardcode-capability` 真存在 (反向 grep workflow yaml) | unit | 战马 | `TestAP4E2_CIWorkflowStepExists` (扫 .github/workflows/release-gate.yml) |

### 立场 ③ — `IsValidCapability(name)` helper 单源 (handler 不直查 map)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 `func IsValidCapability(name string) bool` 落 `internal/auth/capabilities.go` (内部 `return Capabilities[name]`) | unit | 战马 | `capabilities.go::IsValidCapability` + `TestAP4E1_IsValidCapability_TruthTable` (14 true + 1 false) |
| 3.2 既有 handler 路径 `auth.Capabilities[name]` → `auth.IsValidCapability(name)` (查 grep 找出 ≥2 hit: capability_grant.go + me_grants.go) | refactor | 战马 | `capability_grant.go` + `me_grants.go` diff |
| 3.3 反向 grep `auth\.Capabilities\[` packages/server-go/internal/api/ count==0 (handler 走 helper 单源) | unit | 战马 | `TestAP4E2_HandlerHelperOnly` (filepath.Walk + regex 自动扫) |
| 3.4 反向 grep `Capabilities\[".*"\]\s*=` packages/server-go/internal/auth/ 仅 init() 1 hit (反 mutate map) | unit | 战马 | `TestAP4E2_ReverseGrep_DirectMapAccess` |

### 既有 AP-1 / AP-3 / AP-4 reactions / AP-5 unit tests 全 PASS (ALL slice 不破)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 AP-1 #493 capabilities.go 14 const 字面 byte-identical 不动 (跟 spec §1 立场 ③ 同源) | unit | 战马 | git diff `internal/auth/capabilities.go` const 区块 — 14 行字面 unchanged |
| 4.2 全套 server test PASS (`go test -timeout=180s -tags sqlite_fts5 ./...`) | full test | 战马 | CI + 本地 PASS log |

## 不在本轮范围 (spec §3 字面承袭)

- ❌ enum schema migration (capability 是 string SSOT, 不入 DB enum)
- ❌ capability bundle UI (蓝图 §1 client UI 糖, 留 AP-N)
- ❌ generic auth analyzer (vet/staticcheck plugin, v3+)
- ❌ admin god-mode capability (永久不挂, ADM-0 §1.3)
- ❌ ABAC condition (time/ip) (v2+)
- ❌ 测试代码 hardcode 字面禁 (允许 `_test.go` 白名单)

## 退出条件

- 立场 ① 1.1-1.5 (ALL slice + init + 长度锁 + reflect-lint + admin 红线) ✅
- 立场 ② 2.1-2.4 (CI step 真触发 + 反向 grep 0 hit + workflow 字面锚) ✅
- 立场 ③ 3.1-3.4 (IsValidCapability + handler 路径替换 + 反向 grep 0 hit + mutate 仅 1 hit) ✅
- 既有 4.1-4.2 (14 const unchanged + 全套 server test PASS) ✅
- REG-AP4ENUM-001..006 = **6 行 🟢**

## 更新日志

- 2026-04-30 — 战马 / 飞马 / 烈马 v0: AP-4-enum 4 件套 acceptance template, 跟 spec 3 立场 + stance §2 黑名单 grep + 跨 milestone byte-identical (reasons.IsValid #496 / BPP-4 AST scan / HB-3 dict-isolation 同精神) 三段对齐. 0 schema 改 + 0 endpoint 加 — wrapper milestone 复用 AP-1 既有 14 const.

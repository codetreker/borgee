# AP-4-enum spec brief — capability 清单 enum 化 + reflect-lint 单源 (≤80 行)

> 飞马 · 2026-04-30 · Phase 5+ wrapper milestone (跟 AL-5 / CV-2 v2 / AP-2 #525 / AP-3 #521 同模式)
> **蓝图锚**: [`auth-permissions.md`](../../blueprint/current/auth-permissions.md) §1.1 (ABAC 存储, capability list = SSOT) + §3 反 hardcode drift
> **关联**: AP-1 #493 capabilities.go 14 const 已落 + Capabilities map 反向 lookup; AP-4 #551 reactions ACL 闭 (双义占号修正 — 本 spec 是 capability **enum 化** 真路径, 跟 AP-4 reactions ACL 不冲突)
> **命名澄清**: PROGRESS.md line 272 `[ ] AP-4 capability 清单 enum 化` 占号无 spec, 本文真补 — 跟 AP-4 #551 reactions ACL 是**两个 milestone 共用 AP-4 标号**, 本 spec 用文件名 `ap-4-enum-spec.md` 避双义

> ⚠️ Wrapper milestone — 复用 AP-1 既有 capabilities.go 14 const, 仅补:
> (1) reflect-lint 自动 audit (非 hardcode), (2) ALL slice 顺序锁, (3) 反向 grep CI 守门. **0 schema 改, 0 endpoint 加**.

## 0. 关键约束 (3 条立场)

1. **ALL slice 顺序锁 + reflect-lint 自动 audit** (跟 AL-1a #492 reasons SSOT 包 #496 同精神 — 一处改, 所有引用自动跟进): 把 `Capabilities` map 改成 `var ALL = []string{...}` ordered slice + 重建 map 自动派生 (`func init() { Capabilities = make(map[string]bool); for _, c := range ALL { Capabilities[c] = true } }`); ALL 顺序 byte-identical 跟 spec §1 立场 ③ + 蓝图 auth-permissions.md §1; **改 capability = 改 ALL 一处** (rebuild map / Capabilities lookup / IsValid helper / acceptance template / spec brief 全自动一致). 反约束: 不允许直接 mutate `Capabilities` map.

2. **反向 grep CI 守门 — hardcode capability 字面 0 hit** (跟 AP-1 #493 反约束 #1 强化, 加入 CI 真守): `git grep -nE 'HasCapability\("[a-z_]+"' packages/server-go/internal/api/` count==0 (production code 必走 const, 测试代码白名单允许); 加入 `.github/workflows/release-gate.yml` 跟 BPP-4/HB-3 AST scan 同模式 — 反向 grep 失败 → CI fail-block. **改 = 改 ci.yml 一处**.

3. **`auth.IsValidCapability(name string) bool` helper 单源** (跟 reasons.IsValid #496 同模式): handler 不准直查 `Capabilities[name]` map (允许 dispatcher 内部走 ALL slice ranged check); 反约束: `git grep -nE 'auth\.Capabilities\[' packages/server-go/internal/api/` count==0 (走 helper 单源).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **AP-4-enum.1** ALL slice + reflect-lint | `internal/auth/capabilities.go` (改): 14 const 不动 + `var ALL = []string{...}` ordered slice + `func init()` auto-rebuild Capabilities map + `IsValidCapability(name)` helper; `internal/auth/capabilities_lint_test.go` (新): reflect-lint 验 ALL 长度 == const 数 (使用 `reflect.TypeOf(struct{...}{})` 内部 slice + go/ast scan); 6 unit (TestAP4E1_ALL_OrderedByteIdentical + Capabilities_AutoBuildFromAll + IsValidCapability_TruthTable + ALL_Length14 + reflect_lint_NoOrphanConst + reflect_lint_NoExtraInMap) | 战马 (主) / 飞马 (spec) / 烈马 (acceptance) |
| **AP-4-enum.2** handler 路径替换 + grep 守门 | 既有调 `Capabilities[name]` 路径 (查 grep) 改 `IsValidCapability(name)`; `.github/workflows/release-gate.yml` 加 step (跟 BPP-4 AST scan / HB-3 dict-isolation 同模式) `git grep -nE '[模式]' && exit 1` 反向 grep CI block; 4 unit (TestAP4E2_HandlerHelperOnly + ReverseGrep_HardcodeCapability + ReverseGrep_DirectMapAccess + CIWorkflowStepExists) | 战马 (主) / 烈马 |
| **AP-4-enum.3** closure | REG-AP4E-001..005 + acceptance + PROGRESS [x] **AP-4-enum** (区分 AP-4 reactions #551) + content-lock 不需 (server-only) + docs/current sync (server/auth.md §capabilities-enum + ALL slice 字面引用) | 战马C / 烈马 |

## 2. 反向 grep 锚 (5 反约束, count==0)

```bash
# 1) hardcode capability 字面 in handler (走 const 单源)
git grep -nE 'HasCapability\("[a-z_]+"' packages/server-go/internal/api/  # 0 hit

# 2) 直查 map (走 helper 单源)
git grep -nE 'auth\.Capabilities\[' packages/server-go/internal/api/  # 0 hit

# 3) 直 mutate map
git grep -nE 'Capabilities\[".*"\]\s*=' packages/server-go/internal/auth/  # 仅 init() 1 hit

# 4) ALL slice 顺序漂 (acceptance 反向)
diff <(go run -tags=auth_lint ./tools/auth-list-all) <(grep -E '^\t[A-Z][a-zA-Z]+ +=' internal/auth/capabilities.go | awk '{print $1}')  # exit 0

# 5) admin god-mode capability 入白名单 (ADM-0 §1.3 红线)
git grep -nE 'admin_|godmode_|impersonat' packages/server-go/internal/auth/capabilities.go  # 0 hit
```

## 3. 不在范围 (留账)

- ❌ enum schema migration — capability 是 string SSOT, 不入 DB enum (反 schema lock-in)
- ❌ capability bundle UI — PROGRESS 留账 (蓝图 §1 "bundle 是 client UI 糖, 不入数据"); 跟本 spec 拆死, 留 AP-N spec
- ❌ generic auth analyzer — vet/staticcheck plugin 抽象, v3+
- ❌ admin god-mode capability — 永久不挂 ADM-0 §1.3 红线
- ❌ ABAC condition (time/ip) — v2+

## 4. 跨 milestone byte-identical 锁

- 复用 reasons.IsValid #496 SSOT 包模式 (ALL slice + init build map + IsValid helper) — 改 1 处, lint 自动跟进
- 跟 BPP-4 AST scan / HB-3 dict-isolation reverse-grep CI 守门 同模式 (release-gate.yml 加 step)
- 跟 AP-1 #493 capabilities.go 14 const 字面 byte-identical 不动 (本 spec 仅加 ALL slice / helper / lint)
- 跟 ADM-3 #586 audit-forward-only / CHN-15 ReadonlyBit SSOT 同精神 (single-source const + lint guard)

## 5. 验收挂钩

- REG-AP4E-001..005 (5 反向 grep + reflect-lint 全 PASS)
- 既有 AP-1 / AP-3 / AP-4 reactions / AP-5 unit tests 全 PASS (ALL slice 不破)
- CI release-gate.yml 加 step 真触发 (一行 hardcode → CI block)

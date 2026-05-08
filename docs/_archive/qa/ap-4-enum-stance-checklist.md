# AP-4-enum 立场反查表 (capability 清单 enum 化 + reflect-lint 单源)

> **状态**: v0 (野马 / 飞马, 2026-04-30)
> **目的**: AP-4-enum 实施 PR 直接吃此表为 acceptance; 飞马 spec brief / 烈马 acceptance template `ap-4-enum.md` 反查立场漂移 — 一句话立场 + §X.Y 锚 + 反约束 + v0/v1.
> **关联**: 蓝图 `auth-permissions.md` §1.1 (ABAC + capability list = SSOT) + §3 反 hardcode drift; AP-1 #493 capabilities.go 14 const 已落; AL-1a #492 reasons SSOT 包 #496 同精神 (ALL slice + init build map + IsValid helper); BPP-4 / HB-3 dict-isolation reverse-grep CI 守门 同模式; ADM-0 §1.3 admin god-mode 永久不挂.
> **命名澄清**: 跟 AP-4 #551 reactions ACL 共用 AP-4 标号 — 本 spec 用文件名 `ap-4-enum-spec.md` 避双义; PROGRESS 写 `[ ] AP-4-enum`.
> **依赖**: 复用 AP-1 #493 capabilities.go 14 const byte-identical 不动; 0 schema 改 + 0 endpoint 加.

---

## 1. AP-4-enum 立场反查表 (3 立场)

| # | 立场锚 | 一句话立场 | 反约束 (X 是, Y 不是) | v0 / v1 |
|---|--------|----------|----------------------|---------|
| ① | spec §0 ① + auth-permissions §1.1 + reasons SSOT #496 | **`var ALL = []string{...}` ordered slice 单源, init() 自动 rebuild Capabilities map** — 改 capability = 改 ALL 一处, 跟踪自动一致 | **是** ALL 顺序 byte-identical 跟 14 const 声明顺序 (channel scope → artifact scope → messaging → channel admin); init() 派生 `Capabilities = make(map[string]bool); for _, c := range ALL { Capabilities[c] = true }`; **不是** 直接 mutate Capabilities map (反向 grep `Capabilities\[".*"\]\s*=` 仅 init() 1 hit); **不是** 改 const 不改 ALL (reflect-lint 单测立刻 fail); **不是** ALL 顺序漂 (跨 milestone byte-identical 锁) | v0/v1 永久锁 — SSOT 单源是反 drift 红线 |
| ② | spec §0 ② + BPP-4 / HB-3 release-gate CI 同模式 | **反向 grep CI 守门 — handler hardcode capability 字面 0 hit** | **是** `git grep -nE 'HasCapability\("[a-z_]+"' packages/server-go/internal/api/` count==0 (production code 必走 const); 加入 `.github/workflows/release-gate.yml` 真守 — 一行 hardcode → CI fail-block; **不是** 测试代码也禁 (allow `_test.go` 白名单 — 测试可写字面验); **不是** docstring / comment 也扫 (仅扫真函数调用); **不是** 可人工 sign-off skip (跟 §3 立场 ① no-bypass 同精神) | v0: CI step 真触发; v1 同 |
| ③ | spec §0 ③ + reasons.IsValid #496 同模式 + AP-1 §1 立场 ③ | **`auth.IsValidCapability(name string) bool` helper 单源 — handler 不准直查 Capabilities map** | **是** handler 走 `auth.IsValidCapability(name)` (反向 grep `auth\.Capabilities\[` packages/server-go/internal/api/ count==0); helper 内部 `return Capabilities[name]` (复用 init 派生 map, O(1) lookup); **不是** handler 直查 `Capabilities[req.Capability]` (走 helper 单源 — 后续若改 lookup 策略仅改 helper 一处); **不是** dispatcher 也禁 (允许 dispatcher / auth 包内部走 ALL slice 或 Capabilities map); **不是** 加新 helper 重复 (`IsValidCapability` 是唯一对外 API) | v0/v1 永久锁 |

---

## 2. 黑名单 grep — AP-4-enum 实施 PR merge 后跑, 全部预期 0 命中 (除标 ≥1)

```bash
# 立场 ② — handler hardcode capability 字面 (走 const 单源)
git grep -nE 'HasCapability\("[a-z_]+"' packages/server-go/internal/api/  # 0 hit
# 立场 ③ — handler 直查 Capabilities map (走 helper 单源)
git grep -nE 'auth\.Capabilities\[' packages/server-go/internal/api/  # 0 hit
# 立场 ① — 反 mutate Capabilities map (仅 init() 唯一写)
git grep -nE 'Capabilities\[".*"\]\s*=' packages/server-go/internal/auth/  # 1 hit (仅 init)
# 立场 ① — admin god-mode capability 入白名单 (ADM-0 §1.3 红线)
git grep -nE 'admin_|godmode_|impersonat' packages/server-go/internal/auth/capabilities.go  # 0 hit
# 立场 ① — ALL slice 长度跟 const 数对齐 (reflect-lint 真测守, 此处文档锚)
grep -cE '^\t[A-Z][a-zA-Z]+ +=' packages/server-go/internal/auth/capabilities.go  # 14
# 立场 ② — release-gate.yml 加 step 真触发
grep -E 'name: ap4enum-no-hardcode-capability' .github/workflows/release-gate.yml  # 1 hit
```

---

## 3. 不在 AP-4-enum 范围 (避免 PR 膨胀, 跟 spec §3 同源)

- ❌ enum schema migration (capability 是 string SSOT, 不入 DB enum, 反 schema lock-in)
- ❌ capability bundle UI (蓝图 §1 "bundle 是 client UI 糖, 不入数据"; 留 AP-N spec)
- ❌ generic auth analyzer (vet/staticcheck plugin 抽象, v3+)
- ❌ admin god-mode capability (永久不挂, ADM-0 §1.3 红线)
- ❌ ABAC condition (time/ip) (v2+)
- ❌ 测试代码 hardcode 字面禁 (允许 `_test.go` 白名单, 测试自由验)

---

## 4. 验收挂钩

- AP-4-enum.1 PR: 立场 ①③ — `var ALL` ordered slice + `func init()` 自动 rebuild + `IsValidCapability` helper + reflect-lint 6 unit (TestAP4E1_*) + 14 长度锁
- AP-4-enum.2 PR: 立场 ②③ — handler 路径替换 (`auth.Capabilities[x]` → `auth.IsValidCapability(x)`) + `.github/workflows/release-gate.yml` 加反向 grep step + 4 unit (TestAP4E2_*)
- AP-4-enum.3 entry 闸: 立场 ①-③ 全锚 + §2 黑名单 grep 全 0 (除标 ≥1) + 跨 milestone byte-identical (跟 reasons.IsValid #496 SSOT 包 + BPP-4 AST scan + HB-3 dict-isolation 同精神) + REG-AP4ENUM-001..006 全 🟢

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-30 | 野马 / 飞马 | v0, 3 立场 (ALL slice + init 自动 rebuild / 反向 grep CI 守门 / IsValidCapability helper 单源) 承袭蓝图 §1.1 + AP-1 #493 14 const + reasons SSOT #496 同精神 + BPP-4 / HB-3 release-gate 同模式. 6 行反向 grep (含 2 预期 ≥1 + 4 反约束) + 6 项不在范围 + 验收挂钩三段对齐. 命名澄清: 跟 AP-4 reactions ACL 共用 AP-4 标号双义解锁 (本 stance 用 `ap-4-enum-stance-checklist.md` 避漂). |

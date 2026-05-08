# AP-4-enum 文案锁 — capability ALL slice 字面 + reflect-lint 锚

> **状态**: v0 (野马, 2026-04-30)
> **目的**: AP-4-enum 实施 PR 落 capabilities.go ALL slice + IsValidCapability helper 前锁字面 — 跟 reasons.IsValid #496 / AP-1 #493 14 const 同模式, 防 capability 字面漂移.
> **关联**: spec `ap-4-enum-spec.md` §0 立场 ① + AP-1 #493 capabilities.go 14 const + reasons SSOT 包 #496.

---

## 1. capability 字面白名单 — byte-identical 锁 (跟 AP-1 #493 同源, 14 项)

| 顺序 | 字面 | 分组 | 反约束 |
|------|------|------|--------|
| 1 | `read_channel` | channel scope | 不准漂 `view_channel` / `channel_read` |
| 2 | `write_channel` | channel scope | 不准漂 `edit_channel` |
| 3 | `delete_channel` | channel scope | 不准漂 `remove_channel` |
| 4 | `read_artifact` | artifact scope | 不准漂 `view_artifact` |
| 5 | `write_artifact` | artifact scope | — |
| 6 | `commit_artifact` | artifact scope | 不准漂 `submit_artifact` |
| 7 | `iterate_artifact` | artifact scope | — |
| 8 | `rollback_artifact` | artifact scope | 不准漂 `revert_artifact` |
| 9 | `mention_user` | messaging | — |
| 10 | `read_dm` | messaging | — |
| 11 | `send_dm` | messaging | — |
| 12 | `manage_members` | channel admin | — |
| 13 | `invite_user` | channel admin | — |
| 14 | `change_role` | channel admin | — |

**通用反约束**:
- ❌ admin god-mode 字面入清单 (ADM-0 §1.3 红线 — `admin_*` / `godmode_*` / `impersonat*` 0 hit)
- ❌ ALL slice 顺序漂 (跟 const 声明顺序 byte-identical, reflect-lint 锁)
- ❌ Capabilities map literal init (走 init() 派生, 反向 grep `Capabilities\[".*"\]\s*=` 仅 init 1 hit)

---

## 2. 反向 grep 锚 (跟 stance §2 + spec §2 同源)

```bash
# ① handler hardcode capability 字面 (走 const, 反 hardcode)
git grep -nE 'HasCapability\("[a-z_]+"' packages/server-go/internal/api/  # 0 hit
# ② handler 直查 map (走 helper 单源)
git grep -nE 'auth\.Capabilities\[' packages/server-go/internal/api/  # 0 hit
# ③ Capabilities map mutate (仅 init() 1 hit)
git grep -nE 'Capabilities\[".*"\]\s*=' packages/server-go/internal/auth/  # 1 hit
# ④ admin god-mode 字面入白名单 (ADM-0 §1.3 红线)
git grep -nE 'admin_|godmode_|impersonat' packages/server-go/internal/auth/capabilities.go  # 0 hit
# ⑤ const 14 项字面 byte-identical (AP-1 #493 锁)
grep -cE '^\t[A-Z][a-zA-Z]+ +=' packages/server-go/internal/auth/capabilities.go  # 14
```

---

## 3. CI step 字面锚 (release-gate.yml `ap4enum-no-hardcode-capability`)

| 项 | 字面锁 |
|---|------|
| **step name** | `ap4enum-no-hardcode-capability` (反向 grep workflow yaml ≥1 hit) |
| **bash 主体** | `git grep -nE 'HasCapability\("[a-z_]+"' packages/server-go/internal/api/` (跟 §2 ① byte-identical) |
| **fail-block** | `&& exit 1` (一行 hardcode → CI red) |

**反约束**:
- ❌ step 名漂 (单测扫 yaml 字面)
- ❌ 人工 sign-off skip (跟 §3 立场 ① no-bypass 同精神)
- ❌ 包含 `_test.go` (允许测试白名单)

---

## 4. 验收挂钩

- AP-4-enum.1 PR: §1 14 字面 byte-identical + reflect-lint 单测 (TestAP4E1_ALL_OrderedByteIdentical 锁字面顺序) + admin 红线反向 grep 0 hit
- AP-4-enum.2 PR: §3 CI step 字面锚 (TestAP4E2_CIWorkflowStepExists) + §2 ①②③ 反向 grep 全 0 (除 ③ ≥1 init)
- AP-4-enum.3 entry 闸: §1 + §2 + §3 全锚 PASS

---

## 5. 不在范围

- ❌ capability 字面 i18n (英文 string 永久锁, 中文是 UI 层)
- ❌ ALL slice 自动从 const reflect 派生 (反 magic, 显式声明 + reflect-lint 守)
- ❌ ApplyMigration 风格 multi-source (单源 capabilities.go 一文件)

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-30 | 野马 | v0 — AP-4-enum 文案锁 (14 字面 byte-identical 跟 AP-1 #493 同源 + ALL 顺序锁 + Capabilities 反 mutate + admin 红线) + CI step 字面锚 (ap4enum-no-hardcode-capability) + 5 行反向 grep + 验收三段对齐. 跟 reasons.IsValid #496 / BPP-4 AST scan / HB-3 dict-isolation 同精神. |

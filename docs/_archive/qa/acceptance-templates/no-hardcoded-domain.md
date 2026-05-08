# Acceptance Template — no-hardcoded-domain (≤50 行)

> Spec: `no-hardcoded-domain-spec.md` (战马C v0). Owner: 战马C 实施 / 飞马 review / 烈马 验收
>
> **范围**: 2 production file + 1 .env.example + 1 Dockerfile + 2 deploy workflow. 0 hardcoded codetrek.cn in production code (excl. comments). CORS_ORIGIN env panic-fast.

## 验收清单

### §1 行为不变量 (env injection + panic-fast + reverse-grep)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 NodeManager.tsx 改字面 → `import.meta.env.VITE_AGENT_WS_SERVER \|\| 'wss://localhost:4900'` (build-time inject + dev sandbox fallback) | 数据契约 | `no-hardcoded-domain.test.tsx::REG-NHD-001` PASS — env import + fallback 字面 reverse-grep |
| 1.2 config.go `envStr("CORS_ORIGIN", "")` (默认空) + `Validate` production 路径 fail-loud `CORS_ORIGIN env required` | 行为不变量 | `_REG-NHD-002` PASS + `config_test.go::TestConfigValidate` 4 路径全 PASS (含 dev 允许空 + prod 必返 err) |
| 1.3 packages/client/.env.example 列 VITE_AGENT_WS_SERVER + 4 env 注 (prod / staging / testing / dev) | doc | `_REG-NHD-003` PASS |
| 1.4 Dockerfile 加 `ARG VITE_AGENT_WS_SERVER` + `ENV VITE_AGENT_WS_SERVER=${VITE_AGENT_WS_SERVER}` 透传到 pnpm build | 数据契约 | `_REG-NHD-004` PASS |
| 1.5 deploy-test.yml + deploy.yml 加 `--build-arg VITE_AGENT_WS_SERVER=wss://...` per env + deploy-test.yml inline compose 加 `CORS_ORIGIN` | 数据契约 | `_REG-NHD-005` PASS |

### §2 反向 grep 锚 (production code 0 hit)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 production code (excl. comments) 0 hit codetrek.cn — vitest stripComments helper + 2 file 反向锁 | reverse-grep | `_REG-NHD-006` PASS |

### §3 closure (REG + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 既有 client vitest 全绿不破 + 1 新 file 6 case PASS + go config tests 不破 | full vitest + go test | vitest 110 file 722 case PASS + `go test ./internal/config/` PASS |
| 3.2 立场承袭 #635 admin-password panic-on-missing + #634 cookie-name SSOT + 用户 2026-05-04 fork-friendly | inspect | spec §4 立场承袭 byte-identical |

## REG-NHD-* (initial ⚪ → 🟢 post-impl)

- REG-NHD-001 🟢 NodeManager.tsx VITE_AGENT_WS_SERVER 真用 + localhost:4900 fallback
- REG-NHD-002 🟢 config.go CORS_ORIGIN env panic-fast in non-dev (Validate 真返 err)
- REG-NHD-003 🟢 .env.example 列 4 env 注
- REG-NHD-004 🟢 Dockerfile ARG/ENV 透传
- REG-NHD-005 🟢 2 deploy workflow per-env --build-arg + testing inline compose CORS_ORIGIN
- REG-NHD-006 🟢 production code 0 hit codetrek.cn (cross-package reverse-grep, code only)

## 退出条件

- §1 (5) + §2 (1) + §3 (2) 全绿 — 一票否决
- vitest 6 case PASS + 既有 109 file 716 case 全绿不破
- go config_test PASS (4 路径含 prod CORS_ORIGIN required)
- 0 hardcoded codetrek.cn in production code (excl. comments)
- 登记 REG-NHD-001..006

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-04 | 战马C | v0 实施 — no-hardcoded-domain 4 件套 + 2 production file 改 + 1 .env.example + 1 Dockerfile + 2 deploy workflow + 6 vitest + 4 go config_test 真挂. REG-NHD-001..006 ⚪→🟢 全翻. 立场承袭 #635 / #634 / 用户 fork-friendly 铁律. |

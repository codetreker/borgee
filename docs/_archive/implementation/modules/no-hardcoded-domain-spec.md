# no-hardcoded-domain spec brief — NodeManager + CORS 默认值 (≤80 行)

> 战马C · 2026-05-04 · post #642 changelog-slim wave; 真 fork-friendly milestone
> 关联: 用户拍铁律 0 hardcoded domain in production source / #635 admin-password panic-on-missing 同模式

> ⚠️ 真 production 改 — **2 file 真 production code change** + 1 client env example + 1 Dockerfile + 2 deploy workflow inject build-arg + REG-NHD-001..006. 反 silent prod default 烧 fork / staging / testing / on-prem 部署.

## 0. 关键约束 (3 立场)

1. **0 hardcoded codetrek.cn 字面 in production source** — `grep -rnE 'codetrek\.cn' packages/ --include='*.go' --include='*.ts' --include='*.tsx' | grep -v _test` 0 hit (production code, excl. comments).
2. **CORS_ORIGIN env panic-on-missing 跟 #635 admin-password 同模式** — Validate 真返 err in non-dev (cfg.IsDevelopment() == false 且 cfg.CORSOrigin == "" 时 main.go 真 exit 1, 反 silent prod default).
3. **VITE_AGENT_WS_SERVER build-time inject** — Vite import.meta.env 在 `pnpm build` 阶段读 env, baked into client bundle. Dockerfile ARG 透传 → CI deploy.yml/deploy-test.yml inject per-env value (testing/staging/prod 各自字面).

## 1. 拆段实施 (3 段一 PR)

| 段 | 范围 |
|---|---|
| **NHD.1 client env injection** | NodeManager.tsx 改 `wss://collab.codetrek.cn` → `import.meta.env.VITE_AGENT_WS_SERVER \|\| 'wss://localhost:4900'`; `packages/client/.env.example` 新文件列 4 env 注 (prod / staging / testing / dev); Dockerfile 加 `ARG VITE_AGENT_WS_SERVER` + `ENV` 透传 |
| **NHD.2 server CORS panic-fast** | `config.go::Load` 改 `envStr("CORS_ORIGIN", "")` (默认空); `Validate` 加 production 路径 fail-loud; config_test 加 REG-NHD-002 + REG-NHD-002b 真测 |
| **NHD.3 deploy workflows + closure** | deploy-test.yml + deploy.yml 加 `--build-arg VITE_AGENT_WS_SERVER=wss://...` + deploy-test.yml inline compose 加 `CORS_ORIGIN`; vitest reverse-grep test 6 case (REG-NHD-001..006); REG / acceptance / PROGRESS / docs/current sync |

## 2. 反向 grep 锚 (4 反约束)

```bash
# 1) production source 0 hardcoded codetrek.cn (excl. comments)
# (vitest stripComments helper 真测; CI 守 REG-NHD-001/002/006)

# 2) config.go production 路径 fail-loud
grep -E "CORS_ORIGIN env required" packages/server-go/internal/config/config.go  # ≥1 hit

# 3) build-time inject 真挂
grep -E "ARG VITE_AGENT_WS_SERVER" packages/server-go/Dockerfile  # ≥1 hit
grep -E "import\.meta\.env\.VITE_AGENT_WS_SERVER" packages/client/src/components/NodeManager.tsx  # ≥1 hit

# 4) deploy workflow 注 per-env value
grep -E "build-arg VITE_AGENT_WS_SERVER" .github/workflows/deploy*.yml  # ≥2 hit
```

## 3. 不在范围 (留账)

- ❌ staging/prod docker-compose.yml on aliyun host CORS_ORIGIN env 真改 (host-side compose 文件 GitHub 看不到, runbook 一行 `add CORS_ORIGIN` to /opt/dockers/borgee/.env, 留 deploy P0 wave 1 真上线时 ops 改)
- ❌ Playwright e2e 真验证 (playwright skip — VITE env build-time, prod-only, 真验证留 deploy P0 wave 1 + smoke 真过 testing → staging → prod)
- ❌ 其它 hardcoded domain audit (本 milestone 仅 2 file, 真 fork-friendly 真凿实)
- ❌ generic `BORGEE_PUBLIC_URL` SSOT env (留 v2+ — 现 CORS_ORIGIN + VITE_AGENT_WS_SERVER 已足, 反 over-engineering)

## 4. 跨 milestone byte-identical 锁

- #635 admin-password-plain-env panic-on-missing 模式承袭 (Validate 真返 err, main.go 真 exit 1, 反 silent prod default)
- #634 cookie-name-cleanup SSOT 立场承袭 (1 处 SSOT + 反 hardcode 散落)
- 用户铁律 2026-05-04 fork-friendly: 0 hardcoded codetrek.cn

| 2026-05-04 | 战马C | v0 spec brief — no-hardcoded-domain milestone (2 production file 改 + 1 .env.example + 1 Dockerfile + 2 deploy workflow). 0 hardcoded codetrek.cn 字面 in production code (excl. comments). CORS_ORIGIN env panic-fast 跟 #635 admin-password 同模式. |

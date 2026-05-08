# no-hardcoded-domain stance checklist (≤80 行)

> 战马C · 2026-05-04 · 跟 spec brief 1:1 byte-identical, 用户铁律 fork-friendly

## 1. 立场单源 (5 立场)

- **立场 ①**: 0 hardcoded codetrek.cn 字面 in production source (excl. comments) — `grep -rnE 'codetrek\.cn' packages/ --include='*.go' --include='*.ts' --include='*.tsx' | grep -v _test` 真凿实 0 production code hit
- **立场 ②**: CORS_ORIGIN env panic-fast in non-dev (跟 #635 admin-password 同模式) — Validate 真返 err 反 silent prod default
- **立场 ③**: VITE_AGENT_WS_SERVER build-time inject — Vite import.meta.env baked into bundle, Dockerfile ARG 透传, CI per-env build-arg 真挂
- **立场 ④**: localhost fallback for dev sandbox — `wss://localhost:4900` 真挂 fallback 反 dev 路径破
- **立场 ⑤**: REG-NHD reverse-grep CI 守门 — vitest 6 case + go config_test 真挂, drift 抓得到

## 2. 反约束 (4 项)

- ❌ production code 含 codetrek.cn 字面 (反 fork-friendly 红线)
- ❌ silent prod default `https://borgee.codetrek.cn` (反 #635 panic-on-missing 模式)
- ❌ 客户端 runtime fetch /config endpoint 真路 (反 over-engineering, build-time inject 已足)
- ❌ 拆 client + server 多 PR (反 one_milestone_one_pr 铁律)

## 3. 跨 milestone 锁链 (3 处)

- #635 admin-password-plain-env panic-on-missing — Validate 真返 err 模式承袭
- #634 cookie-name-cleanup SSOT 立场 — 1 处 SSOT + 反 hardcode 散落
- 用户 2026-05-04 fork-friendly 铁律 — 0 hardcoded domain in production source

## 4. PM 拆死决策 (3 段)

- **build-time vs runtime inject 拆死** — 走 build-time (Vite import.meta.env baked into bundle); 反 runtime fetch /config endpoint 增延迟 + 增 surface, build-time 已足 (cookbook 同其它 Vite 项目)
- **CORS env panic vs warn 拆死** — 走 panic (跟 #635 admin-password 同模式); 反 warn-only silent prod 烧, fail-loud 真凿实运维真改
- **staging vs prod build-arg 拆死** — staging+prod 共用 `wss://borgee.codetrek.cn` (staging 是 smoke-test prod artifact 的环境, 不发 real traffic 给 staging URL); 反单独 staging-borgee.codetrek.cn build 增 deploy 复杂度. testing 独立 build 走 deploy-test.yml

## 5. 用户主权红线 (3 项)

- ✅ fork 真用 — 0 hardcoded codetrek.cn, fork 改自己 .env 即生效
- ✅ on-prem 真用 — staging/prod docker compose 改 CORS_ORIGIN env + Dockerfile build-arg 真生效
- ✅ deploy 真验 — testing → staging → prod 三环境 build-arg 字面 byte-identical 跟 host 域名

## 6. PR 出来 5 核对疑点

1. production code (excl. comments) 0 hit codetrek.cn (vitest REG-NHD-001/002/006 守)
2. config.go Validate 真 panic in non-dev (go config_test 真测 4 路径)
3. NodeManager.tsx 真用 import.meta.env.VITE_AGENT_WS_SERVER + 'wss://localhost:4900' fallback
4. Dockerfile ARG VITE_AGENT_WS_SERVER + ENV 透传
5. 2 deploy workflow 注 per-env --build-arg + deploy-test.yml inline compose 加 CORS_ORIGIN

| 2026-05-04 | 战马C | v0 stance — 5 立场 byte-identical 跟 spec brief, 4 反约束 + 3 跨链 + 3 拆死 + 3 红线 + 5 PR 核对. 立场承袭 #635 / #634 / 用户 fork-friendly 铁律. |

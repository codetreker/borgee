# Client build env — VITE_AGENT_WS_SERVER (no-hardcoded-domain milestone)

> 2026-05-04 · client build-time env SSOT for fork-friendly deploy.
> 0 hardcoded codetrek.cn in `packages/client/src/` production source.

## VITE_AGENT_WS_SERVER

NodeManager.tsx 真用 `import.meta.env.VITE_AGENT_WS_SERVER` 字面 (Vite 在 `pnpm build` 阶段读 env, baked into bundle). 反 runtime fetch /config endpoint (反 over-engineering, build-time inject 已足).

### Per-env value

| 环境 | URL |
|---|---|
| prod | `wss://borgee.codetrek.cn` |
| staging | `wss://borgee.codetrek.cn` (共用, staging 是 smoke-test prod artifact 的环境) |
| testing | `wss://testing-borgee.codetrek.cn` |
| dev (fallback) | `wss://localhost:4900` (NodeManager.tsx 真挂 fallback when env unset) |

### Build-time inject (Dockerfile)

```dockerfile
# packages/server-go/Dockerfile (client-builder stage)
ARG VITE_AGENT_WS_SERVER
ENV VITE_AGENT_WS_SERVER=${VITE_AGENT_WS_SERVER}
RUN pnpm --filter @borgee/client build
```

### CI deploy workflows

```yaml
# .github/workflows/deploy.yml (staging+prod)
docker build \
  --build-arg VITE_AGENT_WS_SERVER=wss://borgee.codetrek.cn \
  ...

# .github/workflows/deploy-test.yml (testing)
docker build \
  --build-arg VITE_AGENT_WS_SERVER=wss://testing-borgee.codetrek.cn \
  ...
```

### fork / on-prem custom build

```bash
# 本地 build 真改自己域名
VITE_AGENT_WS_SERVER=wss://my-fork.example.com pnpm --filter @borgee/client build
```

或 `packages/client/.env.local` (Vite 默认读 .env.local 优先 .env.example):

```
VITE_AGENT_WS_SERVER=wss://my-fork.example.com
```

## 反向 grep 锚 (REG-NHD-001..006)

```bash
# production code 0 hit codetrek.cn (excl. comments) — vitest stripComments helper 真守
# (no-hardcoded-domain.test.tsx::REG-NHD-001/002/006)
```

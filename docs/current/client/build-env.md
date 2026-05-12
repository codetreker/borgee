# Client build env — VITE_AGENT_WS_SERVER (no-hardcoded-domain milestone)

> 2026-05-04 · client build-time environment variable source of truth for forked and on-prem deployments.
> 0 hardcoded codetrek.cn in `packages/client/src/` production source.

## VITE_AGENT_WS_SERVER

NodeManager.tsx reads the literal `import.meta.env.VITE_AGENT_WS_SERVER` value. Vite reads that environment variable during `pnpm build` and writes it into the bundle. There is no runtime `/config` fetch endpoint because build-time injection is sufficient.

### Per-env value

| 环境 | URL |
|---|---|
| production | `wss://borgee.codetrek.cn` |
| staging | `wss://borgee.codetrek.cn` (shared with production; staging runs smoke tests against the production artifact) |
| testing | `wss://testing-borgee.codetrek.cn` |
| development (fallback) | `wss://localhost:4900` (NodeManager.tsx uses this fallback when the env var is unset) |

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

### Fork / on-prem custom build

```bash
# 本地 build 真改自己域名
VITE_AGENT_WS_SERVER=wss://my-fork.example.com pnpm --filter @borgee/client build
```

Or use `packages/client/.env.local` (Vite reads `.env.local` before `.env.example`):

```
VITE_AGENT_WS_SERVER=wss://my-fork.example.com
```

## Grep Checks (REG-NHD-001..006)

```bash
# production code 0 hit codetrek.cn (excl. comments) — vitest stripComments helper 真守
# (no-hardcoded-domain.test.tsx::REG-NHD-001/002/006)
```

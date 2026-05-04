# Server config — required env vars (no-hardcoded-domain milestone)

> 2026-05-04 · server config SSOT for env vars. fork-friendly: 0 hardcoded
> codetrek.cn in production source. Pattern承袭 #635 admin-password
> panic-on-missing review checklist 1.A bootstrap fail-loud.

## Required env vars (production / NODE_ENV != "development")

| Env var | Required | Purpose | Fail mode |
|---|---|---|---|
| `JWT_SECRET` | ✅ prod | JWT signing key | `config error: JWT_SECRET is required in production` → exit 1 |
| `CORS_ORIGIN` | ✅ prod | CORS allow-origin (e.g. `https://your-deploy-host.example.com`) | `config error: CORS_ORIGIN env required (e.g. https://your-deploy-host.example.com)` → exit 1 |
| `BORGEE_ADMIN_LOGIN` | ✅ all | admin bootstrap login | panic 提示 |
| `BORGEE_ADMIN_PASSWORD` *or* `BORGEE_ADMIN_PASSWORD_HASH` | ✅ all (二选一) | admin bootstrap password | panic 提示 mutually exclusive |

## Optional env vars

| Env var | Default | Purpose |
|---|---|---|
| `PORT` | `4900` | listen port |
| `HOST` | `0.0.0.0` | listen host |
| `LOG_LEVEL` | `info` | slog level (debug/info/warn/error) |
| `NODE_ENV` | `""` | `"development"` 解锁 dev paths (CORS_ORIGIN/JWT_SECRET 可空) |
| `DATABASE_PATH` | `data/collab.db` | sqlite path |
| `UPLOAD_DIR` | `data/uploads` | upload storage |
| `WORKSPACE_DIR` | `data/workspaces` | workspace storage |
| `CLIENT_DIST` | `packages/client/dist` | client static dist |

## fork / on-prem deploy

```bash
# /opt/dockers/borgee/.env on host
JWT_SECRET=<random secret, ≥32 chars>
CORS_ORIGIN=https://your-domain.example.com
BORGEE_ADMIN_LOGIN=admin
BORGEE_ADMIN_PASSWORD=<plain text, hashed at startup>
NODE_ENV=production
```

docker-compose.yml `env_file: .env` 真挂. 反 silent default 烧.

## Validation flow (`config.go::Validate`)

1. NodeEnv != "development" + JWTSecret == "" → return err "JWT_SECRET is required in production"
2. NodeEnv != "development" + CORSOrigin == "" → return err "CORS_ORIGIN env required (...)"
3. main.go 真 `os.Exit(1)` on config.Load err

Pattern 跟 #635 admin-password 同精神 (review checklist 1.A bootstrap fail-loud + 1.B idempotent).

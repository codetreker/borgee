# syntax=docker/dockerfile:1.7
# Build context = monorepo root (e.g. docker build -f packages/server-go/Dockerfile .)

FROM node:22-bookworm AS client-builder
WORKDIR /src
# pnpm 版本锁到 @10, 跟 .github/workflows/ci.yml + deploy.yml 用的
# pnpm/action-setup@v4 version: 10 一致. 不用 @latest 浮动 — pnpm 升级到
# 新版本后 onlyBuiltDependencies 处理变了, 跟 CI 不一致会撞 build script
# ERR_PNPM_IGNORED_BUILDS (gh#683 PR #694 testing 部署事故).
RUN corepack enable && corepack prepare pnpm@10 --activate
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY packages/client/package.json packages/client/
RUN pnpm install --frozen-lockfile --filter @borgee/client
COPY packages/client packages/client
# no-hardcoded-domain: VITE_AGENT_WS_SERVER injected at build time per env
# (deploy.yml / deploy-test.yml). Default fallback `wss://localhost:4900`
# (set in NodeManager.tsx) when build arg absent — local dev sandbox.
ARG VITE_AGENT_WS_SERVER
ENV VITE_AGENT_WS_SERVER=${VITE_AGENT_WS_SERVER}
RUN pnpm --filter @borgee/client build

FROM golang:1.25.11-bookworm AS go-builder
WORKDIR /src/packages/server-go
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev sqlite3 libsqlite3-dev \
  && rm -rf /var/lib/apt/lists/*
COPY packages/server-go/go.mod packages/server-go/go.sum ./
RUN go mod download
COPY packages/server-go ./
RUN CGO_ENABLED=1 GOOS=linux go build -tags sqlite_fts5 -trimpath -ldflags="-s -w" -o /out/collab ./cmd/collab

FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates sqlite3 libsqlite3-0 \
  && rm -rf /var/lib/apt/lists/* \
  && useradd -r -u 10001 -g root borgee
WORKDIR /app
COPY --from=go-builder /out/collab /app/collab
COPY --from=client-builder /src/packages/client/dist /app/client/dist
RUN mkdir -p /app/data/uploads /app/data/workspaces && chown -R 10001:0 /app/data
USER 10001
ENV HOST=0.0.0.0 PORT=4900 DATABASE_PATH=/app/data/collab.db UPLOAD_DIR=/app/data/uploads WORKSPACE_DIR=/app/data/workspaces CLIENT_DIST=/app/client/dist NODE_ENV=production
EXPOSE 4900
ENTRYPOINT ["/app/collab"]

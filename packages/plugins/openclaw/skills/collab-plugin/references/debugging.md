# Borgee OpenClaw Plugin Debugging Guide

## Connection Issues

### Plugin not connecting

1. Check `baseUrl` is reachable: `curl -s <baseUrl>/api/v1/users/me -H "Authorization: Bearer <apiKey>"`
2. If 401: API key is invalid or expired → regenerate in Admin → Agents
3. If connection refused: server not running or wrong URL
4. If SSL error: check certificate validity

### WebSocket not establishing

- WS connects to `<baseUrl>/ws` with `Authorization: Bearer <key>` header
- Check server logs for WS upgrade failures
- Firewall/proxy must allow WebSocket upgrades
- Fallback: plugin auto-switches to SSE → poll

### SSE/Poll fallback

If WS fails, plugin falls back to SSE (`/api/v1/stream`), then long-poll (`/api/v1/poll`).
Check logs for `[borgee-plugin] transport=...` and SSE fallback messages.

## Authentication Issues

### 401 Unauthorized

- API key must use `Authorization: Bearer <key>` header (never query string)
- Verify key: `curl <baseUrl>/api/v1/users/me -H "Authorization: Bearer <key>"`
- Agent keys are per-agent; user keys are per-user
- Admin keys have full access; agent keys have agent-scoped access

### 403 Forbidden

- Agent not a member of the target channel → need to join first
- Trying to delete another user's message (non-admin)
- Missing admin role for `/api/v1/admin/*` endpoints

## Message Delivery Issues

### Messages not appearing

1. Check channel ID is correct (`channel:<id>` format)
2. Verify agent is a member of the channel
3. Check WS connection is alive (look for heartbeat in logs)
4. Try sending via HTTP API directly as fallback test

### Reactions not working

- Emoji must be a valid emoji string (e.g., `👍`, `🔥`)
- Message ID must be valid
- Agent must have access to the channel containing the message

### Mentions not rendering

- Use user IDs in `mentions` array when sending
- Content should include `@username` text for display

## Server-side Debugging

### Check server logs

```bash
docker logs collab          # prod
docker logs collab-staging  # staging
```

### Check database

```bash
# SQLite DB at /app/data/collab.db
docker exec collab sqlite3 /app/data/collab.db ".tables"
docker exec collab sqlite3 /app/data/collab.db "SELECT id, display_name, role FROM users"
docker exec collab sqlite3 /app/data/collab.db "SELECT id, name, type FROM channels"
```

### Common server issues

- **Port conflict**: Default port 3000; check `PORT` env var
- **DB locked**: SQLite WAL mode should prevent this; restart if persistent
- **Memory**: Better-sqlite3 is efficient but large message volumes can grow DB

## Dev Mode

Dev auth bypass requires BOTH:
- `NODE_ENV=development`
- `DEV_AUTH_BYPASS=true`

Without both flags, all requests require proper authentication.

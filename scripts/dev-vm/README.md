# Borgee helper VM-simulator

Single-command bring-up for local helper testing. The container runs
Ubuntu 24.04 with systemd as PID 1, close enough to a real Linux VM
for end-to-end exercise of the borgee install / claim / daemon
lifecycle.

## Quick start

```bash
cd scripts/dev-vm
docker compose up -d
docker compose exec borgee-vm systemctl is-system-running
# expected: "running" within ~10s of `up`
```

## Then install borgee

```bash
docker compose exec borgee-vm bash -c '
  sudo npx @codetreker/borgee-remote-agent install \
    --server wss://testing-borgee.codetrek.cn \
    --token <enrollment_id>.<enrollment_secret>
'
```

## Teardown

```bash
cd scripts/dev-vm
docker compose down --volumes
```

## Full reference

See `docs/runbooks/local-e2e-helper-container.md` for:
- Reboot survival test
- Crash recovery test
- 8-JobType verification matrix
- Known limitations (macOS daemon flows not covered,
  `--privileged` requirement, ephemeral state, etc.)

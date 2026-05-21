# Local end-to-end via Docker VM simulator

Borgee helper installs as a systemd service. This runbook documents how to
spin up a Docker container that behaves close enough to a real Linux VM for
end-to-end testing of the install / claim / daemon lifecycle / reboot / crash
flows. macOS daemon flows (launchctl / sandbox-exec) cannot be tested this
way — they need a real macOS host.

## When to use

- Verifying a PR's helper changes before merge.
- Reproducing user-reported install failures locally.
- Smoke testing `borgee install / claim / daemon` against a staging or
  testing server (e.g. `testing-borgee.codetrek.cn`).

## Prerequisites

- Docker 24+ with cgroup v2:
  ```bash
  docker info | grep -iE 'Cgroup Version|cgroupns'
  # Expect: Cgroup Version: 2  and  cgroupns
  ```
- Kernel + host configured to allow `--privileged` containers. Not safe for
  hardened CI; intended for dev machines.
- Network access to the Borgee server you plan to point the helper at.

## Bringing the VM-sim container up (canonical path)

The canonical entry point is the compose stack under `scripts/dev-vm/`:

```bash
cd scripts/dev-vm
docker compose up -d
```

That builds (first run only) an Ubuntu 24.04 image with systemd as PID 1
plus node 20 preinstalled, then starts a container named `borgee-vm`. See
[scripts/dev-vm/README.md](../../scripts/dev-vm/README.md) for the short
operator pointer; the Dockerfile and compose file there are the source of
truth for image contents (apt list, masked units, stop signal, cgroup
flags).

Wait ~5-10s for systemd to settle before exec'ing in. On the validation
host used while writing this runbook, `systemctl is-system-running`
returned `running` ~8s after `docker compose up -d`.

### Container name

Once `docker compose up` finishes, the container is named `borgee-vm`
(see `container_name:` in `scripts/dev-vm/docker-compose.yml`). The
exec / restart commands later in this runbook use that name. If you also
shell in via `docker compose exec borgee-vm ...`, both work — the latter
is preferred from `scripts/dev-vm/` because it stays inside the compose
project.

### Manual `docker run` fallback

If you prefer not to use compose (e.g. on a host without the compose
plugin, or to spin up a second container with a different name for A/B
testing), the equivalent raw command is:

```bash
docker build -t borgee-vm-base:latest scripts/dev-vm
docker run -d \
  --privileged \
  --cgroupns=host \
  -v /sys/fs/cgroup:/sys/fs/cgroup:rw \
  --tmpfs /run --tmpfs /run/lock \
  --stop-signal SIGRTMIN+3 \
  --name borgee-vm-test \
  --hostname borgee-vm-test \
  borgee-vm-base:latest
```

Flag notes (same constraints either way, compose just hides them):
- `--privileged`: required to access cgroup controllers + mount API. There
  are non-privileged alternatives (`--cap-add SYS_ADMIN` + specific bind
  mounts) but they are fragile across kernel versions; for dev use, just
  go privileged.
- `--cgroupns=host`: makes container cgroups share the host cgroup
  namespace, which systemd inside the container needs to read. Compose
  expresses this as `cgroup: host`.
- `/sys/fs/cgroup` bind mount: required even with cgroup v2 hosts so that
  systemd can write its slice/scope hierarchy.
- `--tmpfs /run --tmpfs /run/lock`: systemd expects these as tmpfs; image
  ships them as empty dirs.
- `--stop-signal SIGRTMIN+3`: matches the Dockerfile `STOPSIGNAL` so
  `docker stop` triggers a clean systemd shutdown instead of waiting out
  the 10s grace period.

## Validating systemd is alive

The compose path names the container `borgee-vm`. If you used the manual
`docker run` form above, substitute `borgee-vm-test` (or whatever
`--name` you passed) for `borgee-vm` in every `docker exec` below.

```bash
docker exec borgee-vm ps -p 1 -o comm=
# Expect: systemd

docker exec borgee-vm systemctl is-system-running
# Expect: running (or degraded — degraded is acceptable as long as the
# units you care about are active)

docker exec borgee-vm systemctl list-units --type=service --state=active
```

## Restoring full tooling (unminimize, optional)

Ubuntu minimal images strip man pages and several standard utilities. The
compose image ships `systemd`, `dbus`, `ca-certificates`, `curl`, `sudo`,
and `nodejs` already, so this step is only needed if you want man pages
or other dev conveniences inside the container.

```bash
docker exec -e DEBIAN_FRONTEND=noninteractive borgee-vm bash -c 'yes | unminimize'
```

Takes 2-5 min depending on network. After it finishes you may see
`A reboot is required to replace the running dbus-daemon.` — that's expected
and benign; nothing borgee touches needs the new dbus right away.

If your image base lacks `unminimize` itself, install it first:
`apt-get install -y unminimize`.

## Installing the borgee prerequisites

The `scripts/dev-vm` image already ships node 20 (via NodeSource), `curl`,
`ca-certificates`, and `sudo` — everything `npx @codetreker/borgee-remote-agent`
needs. Verify inside the container:

```bash
docker exec borgee-vm node --version   # v20.x
docker exec borgee-vm npm --version    # 10.x
```

If you built the image yourself with a stripped Dockerfile (or rolled your
own base), install the prereqs manually:

```bash
docker exec borgee-vm bash -c \
  'apt-get update && apt-get install -y --no-install-recommends sudo curl ca-certificates'

docker exec borgee-vm bash -c \
  'curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && apt-get install -y nodejs'
```

## Installing the helper (Stage 2)

Stage 2 of the local e2e plan covers the actual `borgee install` invocation
and per-JobType exercises. The command shape is:

```bash
docker exec borgee-vm bash -c '
  sudo npx @codetreker/borgee-remote-agent install \
    --server wss://testing-borgee.codetrek.cn \
    --token <enrollment_id>.<enrollment_secret>
'
```

Expected on-disk after a successful install (from
[docs/current/host-bridge/installer.md](../current/host-bridge/installer.md)
and [helper-daemon.md](../current/host-bridge/helper-daemon.md)):

- `/usr/local/lib/borgee/bin/borgee` (0755, root-owned)
- `/etc/systemd/system/borgee.service` (0644)
- `/etc/systemd/system/borgee-rootd.service` (0644)
- `/var/lib/borgee/{queue,status,audit-handoff,credential,openclaw,plugins,state}`
  (0750, borgee-owned)
- `/run/borgee/borgee.sock` (UDS)
- `/run/borgee/borgee-rootd.sock` (UDS 0660 root:borgee)

## Reboot survival check

Borgee's units use `WantedBy=multi-user.target`, so a container restart
should bring them back. Quick smoke test using a probe unit (substitute
`borgee.service` once Stage 2 lands):

```bash
docker exec borgee-vm bash -c '
  cat > /usr/local/bin/probe.sh <<"S"
#!/bin/sh
echo "probe-ok-$(date +%s%N)" > /tmp/probe-output
S
  chmod +x /usr/local/bin/probe.sh
  cat > /etc/systemd/system/probe.service <<"U"
[Unit]
Description=Reboot survival probe
[Service]
Type=oneshot
ExecStart=/usr/local/bin/probe.sh
RemainAfterExit=yes
[Install]
WantedBy=multi-user.target
U
  systemctl daemon-reload && systemctl enable --now probe.service
  cat /tmp/probe-output
'

docker restart borgee-vm && sleep 10
docker exec borgee-vm cat /tmp/probe-output
# The timestamp should be newer than before docker restart.
```

## Crash recovery check

Borgee daemon units carry `Restart=on-failure`. To confirm the container
honours that, kill the main PID and watch systemd respawn it:

```bash
docker exec borgee-vm bash -c '
  PID=$(systemctl show borgee.service -p MainPID --value)
  kill -9 "$PID"
  sleep 5
  echo "Before: $PID, After: $(systemctl show borgee.service -p MainPID --value)"
  systemctl is-active borgee.service           # active
  systemctl show borgee.service -p NRestarts --value   # >= 1
'
```

If `borgee.service` is not yet installed, use a stand-in unit that runs
`sleep 3600` to verify the mechanism — see this PR's validation transcript.

## Network reachability

Confirm the container can reach the Borgee server you plan to point it at:

```bash
docker exec borgee-vm bash -c \
  'curl -fsS -o /dev/null -w "%{http_code}\n" https://testing-borgee.codetrek.cn/'
# Expect: 200
```

If you get TLS failures, check `ca-certificates` installed cleanly during
prereqs.

## Trigger jobs from the web UI

To be filled in Stage 2. Outline of what Stage 2 will document:
- Mapping of each JobType to its trigger surface (user UI vs admin UI vs
  server-internal scheduler).
- The minimum claim payload needed before a job can be dispatched.
- Where to read job status: helper logs (`journalctl -u borgee.service`),
  helper state dir (`/var/lib/borgee/status`), and server-side run records.

## Cleanup

Compose path (canonical):

```bash
cd scripts/dev-vm
docker compose down --volumes
```

That stops the container, removes it, and clears the compose-created
network. The base image stays cached so the next `docker compose up -d`
is fast.

Manual `docker run` fallback:

```bash
docker rm -f borgee-vm-test
```

To also drop the base image:

```bash
docker rmi borgee-vm-base:latest
```

## Known limitations

- `--privileged` is required for cgroup access; not suitable for hardened CI.
- macOS daemon flows (launchctl plists, sandbox-exec, keychain) cannot be
  tested here — Linux container can only validate the Linux helper.
- `docker restart` is not a true cold boot. Kernel state, page cache, and
  some `/run` content survive in ways a real VM reboot would not. For
  "fresh boot" tests recreate the container with `docker rm -f` + `docker run`.
- Container filesystem is ephemeral by default. State across recreations
  needs explicit `-v` volume mounts (e.g. mount a host dir at
  `/var/lib/borgee` to persist queue/status across teardown).
- The Ubuntu base image already strips `systemd-resolved` and several
  udev rules; do not rely on resolver-side or device-event behaviour you
  haven't reproduced on a real VM at least once.

## Reference

- `packages/borgee/cmd/borgee/main.go` — entry
- `packages/borgee/internal/cli/install/install.go` — install flow
- `packages/borgee/internal/cli/setup/setup.go` — unit/plist + state dirs
- [docs/current/host-bridge/lifecycle.md](../current/host-bridge/lifecycle.md) — reboot/crash narrative
- [docs/current/host-bridge/helper-daemon.md](../current/host-bridge/helper-daemon.md) — daemon architecture
- [docs/current/host-bridge/installer.md](../current/host-bridge/installer.md) — install layout + state dirs

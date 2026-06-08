# Borgee helper VM-simulator base image.
# Ubuntu 24.04 + systemd PID 1 + Node.js 20 + openclaw runtime.
# Used by docker-compose.yml.
# Full workflow: docs/runbooks/local-e2e-helper-container.md

FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# No version pins: Ubuntu rolls point releases weekly and pinning breaks rebuilds.
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
        systemd \
        systemd-sysv \
        dbus \
        libpam-systemd \
        ca-certificates \
        curl \
        sudo \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

# Mask units that always fail in a container (udev/logind/getty/hugepages/fuse) so is-system-running != degraded.
RUN systemctl mask -- \
        systemd-udevd.service \
        systemd-udevd-control.socket \
        systemd-udevd-kernel.socket \
        systemd-logind.service \
        'getty@.service' \
        getty.target \
        console-getty.service \
        dev-hugepages.mount \
        sys-fs-fuse-connections.mount \
        systemd-firstboot.service \
    || true

# Node 22 for both the borgee remote-agent daemon (installed globally from the
# locally-packed @codetreker/borgee-remote-agent tarball, COPY'd in below)
# AND the openclaw runtime below (openclaw@2026.5.5 requires Node.js >= 22.12).
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
 && apt-get install -y --no-install-recommends nodejs \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

# Borgee remote-agent daemon, installed from the locally-packed npm tarball
# (staged into this build context as borgee-remote-agent.tgz by
# scripts/dev-vm/build-image.sh — see that script for the pack step). The
# tarball carries all 4 platform `borgee` binaries under bin/platforms/; the
# dispatcher resolves linux-x64 and chmod 0755's it before spawn (npm strips
# the exec bit at pack). After this, `borgee-remote-agent install --server …
# --token … --dirs …` brings a daemon online inside the VM — what T6's e2e drives.
COPY borgee-remote-agent.tgz /tmp/borgee-remote-agent.tgz
RUN npm install -g /tmp/borgee-remote-agent.tgz \
 && rm -f /tmp/borgee-remote-agent.tgz \
 && borgee-remote-agent --version

# OpenClaw runtime (#1078 / #1050 run_7 — replaces the sentinel "echo &&
# exit 0" shell script the dev-stack used to ship as the openclaw
# binary). Pinned to 2026.5.5 to match the peer-dep range declared by
# packages/plugins/openclaw/package.json (>=2026.4.15). The CLI binary
# `openclaw` lands on PATH; `openclaw plugins list` works out of the
# box, and `openclaw plugins install --force <tarball.tgz>` is what the
# path-watcher unit below calls when install-butler delivers the real
# plugin tarball into /usr/local/lib/borgee/openclaw/.
RUN npm install -g openclaw@2026.5.5 \
 && /usr/bin/openclaw --version

# systemd path + service unit that watches the openclaw install dir for
# the tarball install-butler writes (file name has no extension because
# the manifest declares an artifact_id `openclaw-plugin`, so the final
# on-disk path is `/usr/local/lib/borgee/openclaw/openclaw-plugin`).
# The service copies it to a `.tgz` filename openclaw's path-extension
# check accepts and runs `openclaw plugins install --force` so the real
# borgee channel plugin actually loads into the openclaw runtime — the
# verification step run_7 was created to make honest.
RUN printf '%s\n' \
    '[Unit]' \
    'Description=Watch install-butler drop for openclaw plugin tarball' \
    'After=multi-user.target' \
    '' \
    '[Path]' \
    'PathChanged=/usr/local/lib/borgee/openclaw/openclaw-plugin' \
    'PathExists=/usr/local/lib/borgee/openclaw/openclaw-plugin' \
    'Unit=openclaw-plugin-install.service' \
    '' \
    '[Install]' \
    'WantedBy=multi-user.target' \
    > /etc/systemd/system/openclaw-plugin-install.path \
 && printf '%s\n' \
    '[Unit]' \
    'Description=Install the dropped borgee plugin tarball into openclaw' \
    'After=network.target' \
    '' \
    '[Service]' \
    'Type=oneshot' \
    'ExecStart=/usr/bin/install -m 0644 /usr/local/lib/borgee/openclaw/openclaw-plugin /tmp/borgee-plugin.tgz' \
    'ExecStart=/usr/bin/openclaw plugins install --force /tmp/borgee-plugin.tgz' \
    'StandardOutput=journal' \
    'StandardError=journal' \
    > /etc/systemd/system/openclaw-plugin-install.service \
 && mkdir -p /etc/systemd/system/multi-user.target.wants \
 && ln -sf /etc/systemd/system/openclaw-plugin-install.path \
      /etc/systemd/system/multi-user.target.wants/openclaw-plugin-install.path

# Clean shutdown signal for systemd; without it `docker stop` waits the 10s grace before SIGKILL.
STOPSIGNAL SIGRTMIN+3

ENTRYPOINT ["/sbin/init"]

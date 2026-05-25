# Borgee helper VM-simulator base image.
# Ubuntu 24.04 + systemd PID 1 + Node.js 20. Used by docker-compose.yml.
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

# Node 20 for the helper installer (`npx @codetreker/borgee-remote-agent`).
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
 && apt-get install -y --no-install-recommends nodejs \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

# Clean shutdown signal for systemd; without it `docker stop` waits the 10s grace before SIGKILL.
STOPSIGNAL SIGRTMIN+3

ENTRYPOINT ["/sbin/init"]

# Borgee helper VM-simulator base image.
#
# Builds an Ubuntu 24.04 container that runs systemd as PID 1, close enough
# to a real Linux VM for end-to-end testing of the borgee helper install /
# claim / daemon-lifecycle flows. See
# ../../docs/runbooks/local-e2e-helper-container.md for the full workflow,
# limitations, and known caveats.
#
# Apt versions are intentionally not hard-pinned: Ubuntu's apt repos roll
# point releases (e.g. systemd 255.4-1ubuntu8.X) and pinning to a single
# build makes the image fail to rebuild within weeks. Pairing
# --no-install-recommends with an explicit package list keeps the surface
# minimal without the fragility of exact version pins.

FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Core systemd + supporting libraries. ca-certificates / curl / sudo are
# required by the borgee installer (NodeSource setup + sudo npx invocation
# documented in the runbook).
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

# Mask units that always fail inside an unprivileged-ish container. Without
# these masks `systemctl is-system-running` reports `degraded` because
# udev / logind / getty / hugepages / fuse cannot initialise against the
# host kernel from a container namespace.
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

# Node.js 20 via NodeSource. The helper installer is published on npm
# (`@codetreker/borgee-remote-agent`), so node + npm must be present before
# running `npx @codetreker/borgee-remote-agent install` inside the container.
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
 && apt-get install -y --no-install-recommends nodejs \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

# Clean shutdown signal for systemd. Without this, `docker stop` sends
# SIGTERM, which systemd ignores; the container then takes the full 10s
# grace period before docker SIGKILLs PID 1.
STOPSIGNAL SIGRTMIN+3

ENTRYPOINT ["/sbin/init"]

#!/bin/sh
# preinstall — runs before files are unpacked (dpkg / rpm).
#
# Responsible for: creating the `borgee-helper` system user/group + state
# directories that the systemd unit's StateDirectory/ReadWritePaths reference.
# Idempotent: re-running on an already-installed host is a no-op.

set -eu

HELPER_USER="borgee-helper"
HELPER_GROUP="borgee-helper"
STATE_ROOT="/var/lib/borgee-helper"

# Create system group if missing.
if ! getent group "$HELPER_GROUP" >/dev/null 2>&1; then
    groupadd --system "$HELPER_GROUP"
fi

# Create system user if missing.
if ! getent passwd "$HELPER_USER" >/dev/null 2>&1; then
    useradd \
        --system \
        --gid "$HELPER_GROUP" \
        --home-dir "$STATE_ROOT" \
        --no-create-home \
        --shell /usr/sbin/nologin \
        --comment "Borgee Helper daemon" \
        "$HELPER_USER"
fi

# Pre-create the StateDirectory subdirs so the daemon starts cleanly even
# before systemd's StateDirectory= kicks in (systemd will keep them in sync).
for sub in queue status audit-handoff; do
    install -d -o "$HELPER_USER" -g "$HELPER_GROUP" -m 0750 "$STATE_ROOT/$sub"
done

# Audit log directory (the daemon opens audit.log.jsonl directly).
install -d -o "$HELPER_USER" -g "$HELPER_GROUP" -m 0750 /var/log/borgee-helper

exit 0

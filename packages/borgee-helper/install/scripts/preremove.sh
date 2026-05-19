#!/bin/sh
# preremove — runs before files are removed (dpkg upgrade or purge).
#
# Stops + disables the unit. We intentionally do NOT touch the state
# directories under /var/lib/borgee-helper: the one-key uninstall path
# (issue #998) is responsible for purging credentials and audit handoff
# state cleanly, and an `apt upgrade` should not silently throw away the
# operator's enrollment.

set -eu

if command -v systemctl >/dev/null 2>&1; then
    systemctl stop borgee-helper.service || true
    systemctl disable borgee-helper.service || true
fi

exit 0

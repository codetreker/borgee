#!/bin/sh
# postinstall — runs after files are unpacked.
#
# Responsibilities:
#   1. Reload systemd so the unit file we just dropped is visible.
#   2. Enable (NOT start) borgee-helper.service so the daemon comes up on
#      next boot. We do NOT auto-start because the daemon needs claim files
#      under /var/lib/borgee-helper to produce heartbeats; a freshly
#      installed host with no claim would just log "no enrollment
#      configured" until the operator runs borgee-helper-claim.
#   3. Print the operator's next step.

set -eu

if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true
    systemctl enable borgee-helper.service || true
fi

cat <<'EOF'

Borgee Helper installed.

Next steps:
  1. Generate an enrollment in the Borgee web UI (Hosts -> Add).
  2. On this machine, run as root:

       sudo borgee-helper-claim \
           --enrollment-id   <ID from web UI> \
           --enrollment-secret <one-time secret>

  3. Start the daemon:

       sudo systemctl start borgee-helper.service

  4. Verify status:

       sudo systemctl status borgee-helper.service
       sudo journalctl -u borgee-helper.service -f

The service is enabled for boot but is not started automatically because the
heartbeat producer requires claim files. Re-run the claim CLI if you ever
need to re-enroll this host.
EOF

exit 0

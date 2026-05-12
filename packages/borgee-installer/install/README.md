# HB-1B-INSTALLER install assets

> hb-1b-installer-spec §0.2: per-platform install assets are sourced from
> `packages/borgee-helper/install/` (HB-2 v0(D) #617). The installer (`cmd/borgee-installer-{linux,darwin}`)
> deploys these unit files; this README enumerates the contract so repository checks can find
> them on the canonical path.
>
> The installer **invokes** `sudo apt install` / `sudo /usr/sbin/installer` which install
> the existing `.service` / `.plist` from the borgee-helper module. We do **not** duplicate
> those unit files here.

## Linux: systemd unit

Path inside .deb: `/lib/systemd/system/borgee-helper.service`.
Canonical source: `packages/borgee-helper/install/borgee-helper.service` (HB-2 v0(D) #617).
Installer step: `sudo systemctl enable borgee-helper.service` (see `internal/deploy/deploy.go::LinuxPlan`).

## macOS: launchd plist

Path inside .pkg: `/Library/LaunchDaemons/cloud.borgee.host-bridge.plist`.
Canonical source: `packages/borgee-helper/install/cloud.borgee.host-bridge.plist` (HB-2 v0(D) #617).
Installer step: `sudo launchctl load /Library/LaunchDaemons/cloud.borgee.host-bridge.plist`
(see `internal/deploy/deploy.go::DarwinPlan`).

## Windows: planned for v2

Linux .deb and macOS .pkg are in the v1 installer scope. Windows MSI and Windows
Service registration are deferred to v2. The v1 installer does not include a
`borgee-installer-windows` command.

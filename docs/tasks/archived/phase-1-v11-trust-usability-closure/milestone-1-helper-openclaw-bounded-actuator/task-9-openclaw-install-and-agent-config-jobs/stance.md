# Stance: OpenClaw Install And Agent Config Jobs

Task9 completes the queued job authority needed before Configure OpenClaw can hand off to local action work. It enables only two OpenClaw job records: `openclaw.install_from_manifest` and `openclaw.configure_agent`.

The server, not the browser, derives effective payloads and manifest binding. Browser input can identify the target agent or express OpenClaw install intent, but it cannot provide command text, scripts, executable paths, service units, arbitrary paths, arbitrary domains, URLs, credentials, manifest/artifact IDs, config hashes, install plan IDs, TTLs, or expiry windows.

Helper local policy remains mandatory before action. This task does not execute OpenClaw, does not bind Borgee plugin channels, does not manage services, does not upload raw logs, does not cache sudo, and does not reuse Remote Agent rails.

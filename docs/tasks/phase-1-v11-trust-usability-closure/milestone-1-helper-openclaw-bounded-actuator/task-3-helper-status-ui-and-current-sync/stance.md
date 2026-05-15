# PM Stance: Helper Status UI And Current Sync

## Scope Position

This task is the user-visible status slice for Helper enrollment. It should make the existing Helper enrollment state understandable without expanding into Configure OpenClaw completion, job progress, privacy/compliance product surfaces, or Remote Agent management.

## Stances

1. Helper status is enrollment/device status, not OpenClaw success.
   - Anchors: `remote-actuator-design.md` section 1.2 and section 11; milestone acceptance boundary.
   - Constraint: UI may say the Helper enrollment is connected/offline/revoked/uninstalled and show last seen, but must not say Configure OpenClaw succeeded, OpenClaw is connected, services are installed, or jobs completed.
   - Blacklist grep: `Configure OpenClaw succeeded`, `OpenClaw connected`, `configured successfully`, `job succeeded`, `service running`, `install succeeded`.

2. The browser consumes user-rail status only.
   - Anchors: `remote-actuator-design.md` section 1.2; task 1 API boundary.
   - Constraint: user UI reads list/detail Helper enrollment responses through user-authenticated API helpers. It must not call claim/status/uninstall Helper credential endpoints or expose Helper credentials/secrets.
   - Blacklist grep: `helper_credential`, `enrollment_secret`, `/claim`, `/status`, `/uninstall` in client UI code unless test fixtures explicitly prove they are not user UI calls.

3. Helper and Remote Agent stay visibly separate.
   - Anchors: `remote-actuator-design.md` section 1.2; current Remote Agent and Host Bridge docs.
   - Constraint: Helper status should live in a user-owned Host Bridge/Helper surface or equivalent shell surface, not inside Remote Node detail as though Remote Agent and Helper share one status rail.
   - Blacklist grep: `Remote Node Helper`, `remote.*helper credential`, `helper.*connection_token`, `remote node.*allowed categories`, `shared status rail`.

4. Allowed categories are bounded delegation categories, not runnable actions.
   - Anchors: `remote-actuator-design.md` section 1.2 and section 11.
   - Constraint: present `openclaw_lifecycle`, `openclaw_config`, `helper_lifecycle`, and `status_collect` as category-level visibility. Do not render run buttons, command arguments, service names, or job payload details.
   - Blacklist grep: `Run`, `Execute`, `shell`, `argv`, `service unit`, `job payload`, `lease` near Helper category UI.

5. Terminal status must be explicit.
   - Anchors: `remote-actuator-design.md` section 1.2 and section 11.
   - Constraint: revoked and uninstalled are distinct from offline and pending. Terminal state UI should not keep polling into an indefinite spinner or present terminal state as successful configuration.
   - Blacklist grep: `pending forever`, `revoked.*online`, `uninstalled.*online`, `success.*revoked`, `success.*uninstalled`.

6. Current-doc sync is part of the task PR.
   - Anchors: task contract; Blueprintflow one task/one PR rule.
   - Constraint: accepted UI/API behavior, placement, status semantics, and rail separation must be reflected in `docs/current` in the same task PR, or no-op rationale must be recorded.
   - Blacklist grep: `Remote Agent status` as the only current-doc explanation for Helper status, or current docs claiming job/log/OpenClaw success from Helper enrollment status.

7. Privacy/security stays an internal boundary.
   - Anchors: `migration-analysis.md` section 6.1.
   - Constraint: do not add user-facing privacy/compliance promise surfaces. The status surface may avoid leaking private content and credentials, but it should not become a privacy dashboard, compliance center, audit viewer, or legal promise.
   - Blacklist grep: `GDPR`, `DPA`, `compliance center`, `privacy dashboard`, `audit UI`, `admin impact`, `legal agreement`, `privacy promise`.

## Out-Of-Scope Locks

- No production implementation before design review and TDD dispatch.
- No job progress, job logs, failure reason UI, lease/result state, queue state, or Configure OpenClaw closure.
- No Helper credential rotation/replacement mechanics or local uninstall execution.
- No merged Helper/Remote Agent credential, grant, route, status, or UI rail.
- No new user-facing privacy/compliance product surface.

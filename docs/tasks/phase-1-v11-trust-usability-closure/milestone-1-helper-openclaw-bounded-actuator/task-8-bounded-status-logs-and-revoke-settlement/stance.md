# Stance: Bounded Status Logs And Revoke Settlement

1. Terminal truth beats optimistic UX. A failed, revoked, stale, expired, cancelled, denied, or lease-lost Helper job must never look successful or remain active indefinitely. Anchors: `remote-actuator-design.md` sections 1.2, 10, and 11.

2. Revoke, uninstall, and stale credential authority changes are state transitions, not display hints. They win over queued, leased, and running work at the next server/helper boundary. Anchors: `remote-actuator-design.md` section 10.

3. Task8 reports bounded terminal metadata; it does not execute OpenClaw or close Configure OpenClaw. Success here means the substrate settles truthfully, not that OpenClaw was installed or connected. Anchors: task contract and `remote-actuator-design.md` section 11.

4. Result metadata is references and redacted summaries only. Raw tokens, credentials, private files/messages, full env dumps, scripts, commands, arbitrary paths, arbitrary URLs, and unbounded logs are forbidden. Anchors: `remote-actuator-design.md` sections 1.2 and 11; `migration-analysis.md` section 6.1.

5. Preserve Task6 and Task7 semantics. Task8 may harden result validation and visibility, but it must not change the outbound-only helper rail or local policy denial authority into a broader execution surface.

6. Privacy remains backend/security boundary work, not a new product surface. This task may document redaction and enforcement but must not add user-facing privacy/compliance promises. Anchor: `migration-analysis.md` section 6.1.

Blacklist grep before PR:

```text
Remote Agent|reverse-ws|sudo|install-butler|shell|argv|script|service_unit|private file|private message|env dump|privacy dashboard|compliance center
```

Matches are acceptable only when they appear in explicit negative constraints, docs boundary text, or tests proving rejection.

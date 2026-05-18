# Stance: Configure OpenClaw Terminal UI

Task12 is a truthfulness and closure slice, not a new authority slice.

1. Configure OpenClaw success must be earned by the accepted typed job chain.
   - Constraint: partial install/config/plugin/service progress cannot render as final success.
   - Guard: success requires latest successful rows for the required OpenClaw install, agent config, Borgee plugin binding, and service lifecycle job types.

2. Failure and denial visibility must stay bounded.
   - Constraint: the UI may show reason codes, safe messages, and bounded audit/log refs.
   - Guard: no raw payload, raw result summary, raw log body, manifest digest, payload hash, credential, owner/org, path, domain, service unit, command, shell, or argv is rendered.

3. Revoked and uninstalled Helper states are terminal from the Configure OpenClaw perspective.
   - Constraint: revoked/uninstalled enrollments cannot continue to look queued, running, or successful.
   - Guard: Helper terminal states override job-derived Configure OpenClaw state.

4. The client remains display-only.
   - Constraint: no Configure OpenClaw action button, Helper credential rail call, Remote Node fallback, service restart command, or raw log download is introduced.

5. Phase closure stays scoped.
   - Constraint: this PR may carry minimal M1/M2/M3 planning state sync because status-only follow-up PRs are forbidden, but it must not reopen milestone breakdown or expand beyond Task12 closure.

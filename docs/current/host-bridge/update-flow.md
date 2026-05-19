# host-bridge update detection (#999)

Blueprint锚: `docs/blueprint/current/host-bridge.md` §1.3 — "更新策略: 分类,
不自动. 自动更新仍是反模式. 安全补丁启动时显眼提示 + 用户一键确认; 功能
更新只在设置面板提示."

## Detection (this PR)

1. `install-butler` writes `/var/lib/borgee-helper/installed-versions.json`
   after each successful install (atomic tempfile+rename, `0644`). Schema:

   ```json
   {"plugins": {"<plugin-id>": {"version": "...", "installed_at": <ms>, "sha256": "..."}}}
   ```

2. `borgee-helper` daemon runs an `updatecheck.Checker` goroutine (interval
   default 15 minutes; see `internal/updatecheck/updatecheck.go`). Each
   tick: reads the snapshot file, POSTs `{helper_device_id, installed:
   [{id, version}]}` to
   `POST /api/v1/helper/enrollments/{id}/installed-versions` with the
   helper Bearer credential.

3. The server loads the current signed manifest (env `BORGEE_MANIFEST_
   ENTRIES_JSON` / `_FILE` / built-in fallback) and computes drift: each
   manifest entry whose `Version` differs from the helper-reported
   `version` (or that is absent in the helper's snapshot) becomes one
   drift entry `{plugin_id, current_version, manifest_version, class}`
   where `class` is `NormalizeUpdateClass(entry.Class)` (empty defaults
   to `feature`).

4. Server persists the latest snapshot in `helper_enrollments.updates_
   available_json` + `last_update_check_at`, and returns the drift list
   in the response. The serializer exposes `updates_available` +
   `last_update_check_at` on the enrollment GET projection.

5. Helper logs one line per drift entry tagged by class —
   `update-available.security` (operator must surface prominently per
   blueprint) vs `update-available.feature` (settings panel only).

## Classification

`PluginManifestEntry.Class` is the source of truth. `"security"` triggers
the prominent path; any other / empty value normalizes to `"feature"`.
Ops change classification by updating the manifest env JSON — no schema
migration, no code change.

## Apply (deferred)

Per blueprint §1.3, application is always user-confirmed. The apply
executor is intentionally NOT part of this PR. The expected wiring once
the UI surface exists:

- User UI → `POST /api/v1/helper/enrollments/{id}/jobs/enqueue` with
  `{type: "plugin.update", payload: {plugin_id, target_version}}`
- Dispatcher (#1001+#1002) picks up the leased job, runs a new
  `internal/executors/update` executor that invokes `install-butler`
  with the manifest's target version and atomically replaces the binary.

Auto-apply is a banned anti-pattern. There is no scheduled / silent path
that flips versions without an explicit user confirmation event.

## Out of scope

- Web UI for `updates_available` (frontend follow-up)
- `plugin.update` executor (helper-side, follow-up)
- Rollback UX (separate item per issue #999 "Out of scope")

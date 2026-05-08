# docs/implementation/modules/ — milestone spec brief 索引

> 实施 milestone 的 spec brief SOT. 每个 milestone 一个 `<id>-spec.md`. 实施 PR 必引此 § 锚 (反 cross-PR drift, 见 `blueprintflow:milestone-fourpiece` skill).
>
> Phase 4+ 已 ✅ closure. 历史 milestone 长期保留作 frozen historical spec brief (registry REG-* row Source field 真引).

## 按 prefix 分组

### ADM (admin god-mode 系列, 蓝图 admin-model + ADM-0 §1.3)

| Spec | 范围 |
|---|---|
| [`adm-0-review-checklist.md`](adm-0-review-checklist.md) | admin bootstrap fail-loud + idempotent review checklist |
| [`adm-2-spec.md`](adm-2-spec.md) | ADM-2 admin god-mode (#484) |
| [`adm-2-followup-spec.md`](adm-2-followup-spec.md) | ADM-2 follow-up: AdminAuditLogPage data-* (#626) |
| [`adm-3-spec.md`](adm-3-spec.md) | ADM-3 multi-source audit (#619) |
| [`adm-3-v1-e2e-spec.md`](adm-3-v1-e2e-spec.md) | ADM-3 v1 e2e |

### ADMIN-* (admin SPA 系列)

| Spec | 范围 |
|---|---|
| [`admin-spa-shape-fix-spec.md`](admin-spa-shape-fix-spec.md) | 6 client/server shape drift fix (#633) |
| [`admin-spa-archived-ui-followup-spec.md`](admin-spa-archived-ui-followup-spec.md) | #633 D4-A client filter UI 闭环 (#638) |
| [`admin-spa-ui-coverage-spec.md`](admin-spa-ui-coverage-spec.md) | 第一波 users 详情页 capability grant UI (#639) |
| [`admin-spa-ui-coverage-wave2-spec.md`](admin-spa-ui-coverage-wave2-spec.md) | 第二波 4 endpoint UI (#641) |
| [`admin-password-plain-env-spec.md`](admin-password-plain-env-spec.md) | BORGEE_ADMIN_PASSWORD 明文 env 自哈希 (#635) |

### AL (Agent Lifecycle, 蓝图 agent-lifecycle.md)

| Spec | 范围 |
|---|---|
| [`al-1b-spec.md`](al-1b-spec.md) | AL-1b agent state busy/idle |
| [`al-2-wrapper-spec.md`](al-2-wrapper-spec.md) | AL-2 wrapper |
| [`al-2b-spec.md`](al-2b-spec.md) / [`al-2b.2-server-hook-spec.md`](al-2b.2-server-hook-spec.md) | AL-2b BPP frame fanout |
| [`al-3-spec.md`](al-3-spec.md) | AL-3 presence sessions |
| [`al-4-spec.md`](al-4-spec.md) | AL-4 agent_runtimes (#398) |
| [`al-5-spec.md`](al-5-spec.md) | AL-5 |
| [`al-7-spec.md`](al-7-spec.md) | AL-7 SweeperReason |
| [`al-8-spec.md`](al-8-spec.md) | AL-8 archived 三态 server 真兑现 |

### AP (Auth Permissions, 蓝图 auth-permissions.md)

| Spec | 范围 |
|---|---|
| [`ap-1-spec.md`](ap-1-spec.md) | AP-1 14 const whitelist (admin-rail 入口守) |
| [`ap-2-spec.md`](ap-2-spec.md) | AP-2 user-rail BundleSelector (#620) |
| [`ap-3-spec.md`](ap-3-spec.md) | AP-3 |
| [`ap-4-spec.md`](ap-4-spec.md) / [`ap-4-enum-spec.md`](ap-4-enum-spec.md) | AP-4 enum |
| [`ap-5-spec.md`](ap-5-spec.md) | AP-5 |
| [`capability-dot-spec.md`](capability-dot-spec.md) | CAPABILITY-DOT 14 const dot-notation SSOT (#628) |

### BPP (Borgee Plugin Protocol, 蓝图 plugin-protocol.md)

| Spec | 范围 |
|---|---|
| [`bpp-1.md`](bpp-1.md) / [`bpp-1-envelope-lint.md`](bpp-1-envelope-lint.md) / [`bpp-1-envelope-ci-lint-spec.md`](bpp-1-envelope-ci-lint-spec.md) | BPP-1 frame envelope + CI lint |
| [`bpp-2-spec.md`](bpp-2-spec.md) | BPP-2 |
| [`bpp-3.1-spec.md`](bpp-3.1-spec.md) / [`bpp-3.2-spec.md`](bpp-3.2-spec.md) | BPP-3 |
| [`bpp-4-spec.md`](bpp-4-spec.md) ~ [`bpp-8-spec.md`](bpp-8-spec.md) | BPP-4..8 (heartbeat / reconnect / etc) |

### CHN (Channel, 蓝图 channel-model.md)

| Spec | 范围 |
|---|---|
| [`chn-1-spec.md`](chn-1-spec.md) / [`chn-1-tasks.md`](chn-1-tasks.md) | CHN-1 创建频道 (#194) |
| [`chn-2-spec.md`](chn-2-spec.md) ~ [`chn-15-spec.md`](chn-15-spec.md) | CHN-2..15 (description / archive / member / sidebar / cross-org / DM-only ACL / readonly / edit-history etc) |
| [`chn-4-spec-v1-wrapper.md`](chn-4-spec-v1-wrapper.md) | CHN-4 v1 wrapper |

### CV (Canvas / Vision, 蓝图 canvas-vision.md)

| Spec | 范围 |
|---|---|
| [`cv-1-spec.md`](cv-1-spec.md) ~ [`cv-15-spec.md`](cv-15-spec.md) | CV-1..15 (artifact create / commit / kind enum / iterate / thumbnail / preview / etc) |
| [`cv-2-v2-media-preview-spec.md`](cv-2-v2-media-preview-spec.md) | CV-2 v2 media preview |
| [`cv-3-v2-spec.md`](cv-3-v2-spec.md) | CV-3 v2 thumbnail |
| [`cv-4-v2-spec.md`](cv-4-v2-spec.md) | CV-4 v2 iterate task |

### DM (Direct Message, 蓝图 channel-model.md DM 段)

| Spec | 范围 |
|---|---|
| [`dm-2-spec.md`](dm-2-spec.md) / [`dm-2.2-spec.md`](dm-2.2-spec.md) / [`dm-2.3-spec.md`](dm-2.3-spec.md) | DM-2 发消息 + WS fanout |
| [`dm-3-spec.md`](dm-3-spec.md) ~ [`dm-12-spec.md`](dm-12-spec.md) | DM-3..12 (mention / reply / search / pin / edit-history / etc) |

### HB (Host Bridge, 蓝图 host-bridge.md)

| Spec | 范围 |
|---|---|
| [`hb-1-spec.md`](hb-1-spec.md) / [`hb-1b-installer-spec.md`](hb-1b-installer-spec.md) | HB-1 plugin manifest + installer |
| [`hb-2-spec.md`](hb-2-spec.md) / [`hb-2-0-spec.md`](hb-2-0-spec.md) / [`hb-2-v0c-spec.md`](hb-2-v0c-spec.md) / [`hb-2-v0d-spec.md`](hb-2-v0d-spec.md) / [`hb-2-v0d-e2e-spec.md`](hb-2-v0d-e2e-spec.md) | HB-2 helper IPC |
| [`hb-3-spec.md`](hb-3-spec.md) / [`hb-3-v2-spec.md`](hb-3-v2-spec.md) | HB-3 |
| [`hb-4-spec.md`](hb-4-spec.md) | HB-4 release gate |
| [`hb-6-spec.md`](hb-6-spec.md) | HB-6 |

### RT (Realtime, 蓝图 realtime.md)

| Spec | 范围 |
|---|---|
| [`rt-1-spec.md`](rt-1-spec.md) | RT-1 cursor allocator + WS push |
| [`rt-3-spec.md`](rt-3-spec.md) | RT-3 multi-device fanout |
| [`rt-4-spec.md`](rt-4-spec.md) | RT-4 |

### CM (Communication, 蓝图 channel-model.md 协作场段)

| Spec | 范围 |
|---|---|
| [`cm-5-spec.md`](cm-5-spec.md) | CM-5 |

### CS (Client / Settings)

| Spec | 范围 |
|---|---|
| [`cs-1-spec.md`](cs-1-spec.md) ~ [`cs-4-spec.md`](cs-4-spec.md) | CS-1..4 (settings / Web Push 三态 / etc) |

### DL (Data Layer / Push, 蓝图 data-layer.md + realtime.md)

| Spec | 范围 |
|---|---|
| [`dl-1-spec.md`](dl-1-spec.md) ~ [`dl-4-spec.md`](dl-4-spec.md) | DL-1..4 (events_store / push notifier / etc) |
| [`dl-4-hb-1-drift-anchor.md`](dl-4-hb-1-drift-anchor.md) | DL-4 HB-1 drift anchor |

### INFRA / 工程治理

| Spec | 范围 |
|---|---|
| [`infra-3-spec.md`](infra-3-spec.md) | INFRA-3 PROGRESS line budget + 5 phase 子文件 |
| [`ci-split-race-cov-spec.md`](ci-split-race-cov-spec.md) | CI race / cov 拆 job |
| [`ulid-migration-spec.md`](ulid-migration-spec.md) | ULID-MIGRATION 26-char ID SSOT (#625) |
| [`wire-1-spec.md`](wire-1-spec.md) | WIRE-1 |
| [`naming-1-spec.md`](naming-1-spec.md) | NAMING-1 |
| [`refactor-1-spec.md`](refactor-1-spec.md) / [`refactor-2-spec.md`](refactor-2-spec.md) / [`refactor-reasons-spec.md`](refactor-reasons-spec.md) | REFACTOR 系列 |
| [`perf-ast-lint-spec.md`](perf-ast-lint-spec.md) | PERF AST lint |
| [`test-fix-1-spec.md`](test-fix-1-spec.md) / [`test-fix-2-spec.md`](test-fix-2-spec.md) / [`test-fix-3-spec.md`](test-fix-3-spec.md) | TEST-FIX (race / cov / etc) |
| [`deferred-unwind-spec.md`](deferred-unwind-spec.md) | DEFERRED-UNWIND test.fixme/skip audit真删 (#629) |
| [`cookie-name-cleanup-spec.md`](cookie-name-cleanup-spec.md) | COOKIE-NAME-CLEANUP user-rail SSOT (#634) |
| [`no-hardcoded-domain-spec.md`](no-hardcoded-domain-spec.md) | NO-HARDCODED-DOMAIN VITE_AGENT_WS_SERVER + CORS_ORIGIN panic-fast (#644) |
| [`e2e-scenarios-establishment-spec.md`](e2e-scenarios-establishment-spec.md) | E2E 17 smoke + 86 regression 场景 SSOT (#637) |

## 配套位置

- 接受验收: `docs/qa/acceptance-templates/<id>.md`
- 立场清单: `docs/qa/<id>-stance-checklist.md`
- 文案锁 (client UI only): `docs/qa/<id>-content-lock.md`
- 实施设计草稿 (optional): `docs/implementation/design/`
- REG-* 寄存器: `docs/qa/regression-registry.md`
- 实施进度: `docs/implementation/PROGRESS.md` + `progress/phase-{0,1,2,3,4}.md`

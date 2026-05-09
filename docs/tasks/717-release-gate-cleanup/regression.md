# 717 — Regression Registry

| 规格条 | 验证 | 负责 | PR | 状态 |
|---|---|---|---|---|
| HB-3 §1.5 撤销 <100ms | `TestHB_DELETE_RevokeStampsRevokedAt` | dev | #717 | ✅ behavior test |
| HB-3 §2 第 ⑩ 条: host_grants 不 JOIN user_permissions | `TestHB_NoUserPermissionsJoin` | dev | #717 | ✅ behavior test |
| HB-3 §2 第 ⑥ 条: AST scan, internal/api 内禁出现 grant queue | `TestHB_NoGrantQueueInAPIPackage` | dev | #717 | ✅ behavior test |
| BPP-4 §2 第 ⑥ 条: BPP 包内禁出现 retry queue | `TestBPP_NoRetryQueueInBPPPackage` | dev | #717 | ✅ behavior test |
| BPP-5 §2 第 ⑥ 条: BPP 包内禁出现 reconnect queue | `TestBPP_NoReconnectQueueInBPPPackage` | dev | #717 | ✅ behavior test |
| AL-1.1 5 状态机 valid edges | `TestValidateTransition_ValidEdges` 等 | dev | #717 | ✅ behavior test |
| AL-1.4 state log append | `TestAppendAgentStateTransition_HappyPath` 等 | dev | #717 | ✅ behavior test |
| AP-4-enum §2 handler 不 hardcode capability literal | `TestAP_ReverseGrep_HardcodeCapability` | dev | #717 | ✅ behavior test |
| AP-4-enum §3 handler 不直查 Capabilities map | `TestAP_HandlerHelperOnly` | dev | #717 | ✅ behavior test |
| AP-4-enum §1 Capabilities map 仅 init() 写 | `TestAP_ReverseGrep_DirectMapAccess` | dev | #717 | ✅ behavior test |
| DL-1.2 internal/api production .go 直 import store ≤ baseline 50 (hard ratchet) | `TestDL12_DirectStoreImportBaseline` | dev | #717 | ✅ 新加 (替 yml grep) |
| BPP-4 §3 第 ⑤ 条: heartbeat 30s 单源 + 禁 drift 涨到 >30s | `TestLint_BPPHeartbeat30sSingleSource` | dev | #717 | ✅ 新加 (替 inline grep) |
| AL-1.1 §1.3 6 reason 字典禁出现第 7 个 | `TestLint_ReasonChainNo7th` | dev | #717 | ✅ 新加 (替 inline grep) |
| AL-1a #496 reasons SSOT 包存在 | `TestLint_ReasonsSSOTExists` | dev | #717 | ✅ 新加 (替 inline grep) |
| AL-1a #496 reasons 跨 milestone ≥6 hit | `TestLint_ReasonsCrossMilestoneCoverage` | dev | #717 | ✅ 新加 (替 inline grep) |
| BPP-5 §1.4 connecting 不入持久态 | `TestLint_AgentStateLogNoConnecting` | dev | #717 | ✅ 新加 (替 inline grep) |
| AL-1b §2 第 ② 条: presence_sessions 不写 busy 列 | `TestLint_PresenceSessionsNoBusyWrite` | dev | #717 | ✅ 新加 (替 inline grep) |
| AL stack vs HB stack 字典分立, 表不互相 JOIN | `TestLint_ALHBStackDictIsolation` | dev | #717 | ✅ 新加 (替 inline grep) |
| HB-3 §1.4 audit 5 字段 byte-identical (actor/action/target/when/scope) | `TestLint_AuditSchema5FieldsByteIdentical` | dev | #717 | ✅ 新加 (替 inline grep) |

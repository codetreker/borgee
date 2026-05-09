# 717 — Regression Registry

| 规格条 | 验证 | 负责 | PR | 状态 |
|---|---|---|---|---|
| HB-3 §1.5 撤销 <100ms | `TestHB_DELETE_RevokeStampsRevokedAt` | dev | #717 | ✅ behavior test |
| HB-3 §2 立场 ⑩ host_grants 不 JOIN user_permissions | `TestHB_NoUserPermissionsJoin` | dev | #717 | ✅ behavior test |
| HB-3 §2 立场 ⑥ AST scan no Grant queue | `TestHB_NoGrantQueueInAPIPackage` | dev | #717 | ✅ behavior test |
| BPP-4 §2 立场 ⑥ no retry queue | `TestBPP_NoRetryQueueInBPPPackage` | dev | #717 | ✅ behavior test |
| BPP-5 §2 立场 ⑥ no reconnect queue | `TestBPP_NoReconnectQueueInBPPPackage` | dev | #717 | ✅ behavior test |
| AL-1.1 5 状态机 valid edges | `TestValidateTransition_ValidEdges` 等 | dev | #717 | ✅ behavior test |
| AL-1.4 state log append | `TestAppendAgentStateTransition_HappyPath` 等 | dev | #717 | ✅ behavior test |
| AP-4-enum 立场 ② handler 不 hardcode capability | `TestAP_ReverseGrep_HardcodeCapability` | dev | #717 | ✅ behavior test |
| AP-4-enum 立场 ③ handler 不直查 Capabilities map | `TestAP_HandlerHelperOnly` | dev | #717 | ✅ behavior test |
| AP-4-enum 立场 ① Capabilities map 仅 init() 写 | `TestAP_ReverseGrep_DirectMapAccess` | dev | #717 | ✅ behavior test |
| DL-1.2 internal/api 直 import store ≤ baseline 115 | `TestDL12_DirectStoreImportBaseline` | dev | #717 | ✅ 新加 (替 yml grep) |

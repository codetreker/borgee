# COL-B24 Task Breakdown Review — CC2 Round 2

Reviewer: CC2 | Date: 2026-04-23

---

## Verdict: 1 CRITICAL remaining (design.md code not updated)

The task-breakdown.md correctly documents how all 3 CRITICALs should be resolved (lines 305-311), but design.md v4 — the authoritative implementation reference — still contains the original broken code. Implementers following design.md will reproduce the bugs.

---

## CRITICAL

### C1 (REMAINING). design.md code contradicts task-breakdown resolutions

**Scope**: C2-C1, C2-C2, C2-C3 are all "resolved in prose, unresolved in code."

| Original CRITICAL | task-breakdown says | design.md v4 still shows |
|---|---|---|
| C2-C1: waitForMessage no timeout | "所有等待函数均带 timeoutMs 参数，默认 5000ms" | §1.4 (line 75-87): `waitForMessage` and `waitForClose` have NO timeout parameter |
| C2-C2: WS connection leak | "ws-helpers 加 try/finally cleanup；TestContext.close() 统一关闭所有连接" | §2.5/2.8/2.9: all test code uses bare `ws.close()` at end of test — no try/finally, no cleanup list, no registration in TestContext |
| C2-C3: Plugin channelId/msgId undefined | "T6 使用 TestContext 提供的 channel/message" | §2.8 (lines 704, 735): `channelId` and `msgId` used but never declared in `beforeAll`; `beforeAll` has comment "// setup admin + agent with api_key" with no channel/message setup |

Additionally, §2.9 Remote Explorer (lines 829-834) still creates `server = await buildFullApp()` and `ctx = await TestContext.create()` as **separate instances with separate DBs**. The task-breakdown C2-H2 resolution says "两者共享同一 DB" but design.md code does not implement this.

**Fix**: Update design.md §1.4 code to add `timeoutMs` parameter + cleanup list. Update §2.8 `beforeAll` to seed channel/message via TestContext or HTTP. Update §2.9 to use a single DB. Either update design.md v4 → v5, or add a prominent note that task-breakdown.md supersedes design.md code examples.

---

## Previously raised CRITICALs — status

| ID | Status | Notes |
|---|---|---|
| C2-C1 (timeout) | Resolved in task-breakdown, NOT in design.md code | See C1 above |
| C2-C2 (WS leak) | Resolved in task-breakdown, NOT in design.md code | See C1 above |
| C2-C3 (channelId/msgId) | Resolved in task-breakdown, NOT in design.md code | See C1 above |

## Other checks — no new CRITICALs

- **TestContext + buildFullApp same DB**: task-breakdown documents the intent (line 323) but design.md contradicts it (covered in C1 above).
- **Test case count**: 76 total (15 stub/todo for T8) — reasonable after de-duplication from ~100+ to 76.
- **No new CRITICAL issues** introduced beyond the design.md/task-breakdown inconsistency.

# 717 — release-gate / al-release-gate workflow 整治

> Issue: https://github.com/codetreker/borgee/issues/717
> 优先级: P2 / tech-debt
> PR #719 已经把 `startup-benchmark` / `progress-line-budget` 这类纯 placeholder 删了, 这一锅清剩下的.

## 要做什么

`.github/workflows/release-gate.yml` + `.github/workflows/al-release-gate.yml` 两个 workflow 现在大半 step 是 Phase 收尾期间为防一种写法跑偏临时加的字符串 grep — Phase 闭后没增量价值, 改一行字符串就能绕过. 真正长期保系统的是行为 invariant 的 Go test, 不是文本 grep.

行动:

1. **保留行为 test**: 把以下 5 类 Go test 节点接到 `ci.yml::go-test-cov` (已经 `go test ./...`, 默认包含):
   - `TestHB_DELETE_RevokeStampsRevokedAt` (revoke 100ms latency 行为)
   - `TestHB_NoUserPermissionsJoin` (host_grants vs user_permissions 字典分立)
   - `TestHB_NoGrantQueueInAPIPackage` (HB-3 AST scan)
   - `TestBPP_NoRetryQueueInBPPPackage` + `TestBPP_NoReconnectQueueInBPPPackage` (BPP package 边界)
   - `TestValidateTransition*` + `TestAppendAgentStateTransition*` (5 状态机 valid edges)

   这些已经在 `go-test-race` / `go-test-cov` job 里被 `./...` 默认跑到了, 不需要单独 step. 实测: 上面 release-gate.yml 里的 `-run TestAL14` / `-run TestAL2A2` / `-run TestAL2B2` 三个 step 全部 `[no tests to run]` (函数名早就改了, fake-green 守门).

2. **删两个 workflow**:
   - `.github/workflows/release-gate.yml` 整文件删
   - `.github/workflows/al-release-gate.yml` 整文件删

3. **branch protection ruleset**: 不需要改. 当前 ruleset (id 15323733) required checks 是 `check / e2e / PR lint (current 同步) / client-vitest / go-test-race / go-test-cov`, 本来就没 release-gate / al-release-gate. 删 yml 就行.

4. **代码内引用清理**:
   - `packages/server-go/internal/api/permission_reverse_grep_test.go` 里 `TestAP_CIWorkflowStepExists` 在代码里 grep `release-gate.yml` 有 `ap4enum-no-hardcode-capability` step — 这本身就是字符串 grep 锁文本, workflow 删了之后这个 test 也得删
   - `packages/server-go/internal/api/messages_self_unread_test.go` L203 注释提 release-gate DL-1.2 sentinel — 改注释
   - `packages/borgee-helper/internal/grants/sqlite_consumer.go` + `sqlite_consumer_test.go` 注释提 "release-gate 第 5 行 byte-identical" — 改注释为指向行为 test
   - `packages/e2e/tests/hb-2-v0d.spec.ts` L219 注释 "<100ms 是 release-gate 阈值" — 改注释 (e2e case 本身保留 100ms 断言, 改文字描述)
   - `docs/current/server/abac.md` + `docs/current/server/expires-sweeper.md` + `docs/current/server/data-layer.md` 提 release-gate 的段落改写 (指向行为 test 而非 yml step)

5. **DL-1.2 dl1-no-direct-store**: release-gate.yml 里有一段 `internal/api 直 import internal/store 文件数 ≤ baseline`. 这是真有用的代码搜索约束 (防 handler 绕 datalayer interface 直查 store), 把它转成 `dl12_direct_store_baseline_test.go` 真 Go test (走 `./...` 默认覆盖).

   **重要 — baseline=50 hard ratchet 透明度**: 此 baseline 锁的是 production code 文件数 (跟 DL-1.2 §4 B 一致, 不锁 _test.go fixture). 当前真量 = 50 production .go. 任何新加 1 个 production 直 import store 的 .go 文件就立即 fail, 没缓冲. 期望模式是 ratchet 单调下降 — DL-1.3+ 渐进迁移, 想加新 handler 必须先 PR 把别处一个 handler 迁到 datalayer 把数字降 1, 才能加新的. 这是好的强制迁移, 但意味着每个新 handler PR 都要附带一次 datalayer 迁移工作量.

   liema #722 Q1 review 修: 上一版 baseline=115 把 65 个 _test.go 一起算了, 撞 DL-1.2 §4 B 只锁 production 路径. 改 baseline=50 + 跳 _test.go.

6. **inline grep 反约束转 Go test (feima review #722 双 review 必答)**: 删 yml 里有 7 条 inline bash grep 反约束守"未来 commit drift" — 这是 constraint inequality 不是 behavior equality, `./...` 不会兜. 转 `lint_constraints_test.go` 真 Go test (跟 dl12 baseline 同模式) 守门:

   - `TestLint_BPPHeartbeat30sSingleSource` — BPP-4 §3 第 ⑤ 条: 30s 单源 + 禁 drift 涨到 >30s
   - `TestLint_ReasonChainNo7th` — AL-1.1 §1.3 6 reason 字典禁出现第 7 个
   - `TestLint_ReasonsSSOTExists` + `TestLint_ReasonsCrossMilestoneCoverage` — AL-1a #496 reasons SSOT 跨 milestone ≥6 hit
   - `TestLint_AgentStateLogNoConnecting` — BPP-5 §1.4 connecting 不入持久态
   - `TestLint_PresenceSessionsNoBusyWrite` — AL-1b §2 第 ② 条: presence_sessions 不写 busy 列
   - `TestLint_ALHBStackDictIsolation` — AL stack vs HB stack audit 字典分立, 表不互相 JOIN
   - `TestLint_AuditSchema5FieldsByteIdentical` — HB-3 §1.4 audit 5 字段 byte-identical

   当前 7 条都 0 drift, 转 test 后走 `./...` 默认覆盖, 长期守门.

## 不做什么

- 不动 `ci.yml::go-test-race` / `go-test-cov` 现有跑法 (覆盖 ./..., 行为 test 已经在跑)
- 不动 `lint.yml` / `bpp-envelope-lint` / `installer.yml` 等其它 workflow
- 不删任何 Go 行为 test (test 函数本身保留, 只删 release-gate.yml 里 `-run` 单挑那行)
- 不立 stance.md / content-lock.md (本 PR 是 tech-debt 整治, 跟 milestone 4-piece 标准 deliberate 不同)

## 边界

- 一 PR 一锅清: yml 删 + dl1-no-direct-store 搬 + 7 条 inline grep 转 lint test + 代码内引用清 + docs 改, 同一 PR
- 必须 `pnpm install --frozen-lockfile` 不动, go module 不动, 真 dev infra 不动
- 改完用代码搜索自核: `grep -rn 'release-gate' --include='*.md' --include='*.go' --include='*.ts' --include='*.tsx' --include='*.yml'` 应该 0 hit (除了 docs/_archive/ 历史归档 + 本 PR 4-piece self-doc)

## 已知反例 (issue body §真量 已列)

- 3 placeholder step (startup-benchmark / dogfood-crash-rate / signature-pass-rate) 当时没东西测 echo 占位 — 删
- audit-schema-cross-milestone / numeric-singletons / no-bypass / ap4enum-no-hardcode-capability / dict-isolation-al-vs-hb / busy-idle-bpp-source — 全部字符串 grep, 改写法即绕; yml 删, 反约束转 lint Go test
- ast-scan-bpp4 / bpp5 / hb3 / state-graph-reflect / al-1-4-state-log-coverage / al-2a-config-blob-validation / al-2b-bpp-fanout — 真 Go test, 但 `-run` 单挑 (`TestAL14` 等) 的函数名早就改了, 走 `./...` 默认跑

## 验证 (代码搜索 + Go test 跑通)

PR 做完后必须验:

```bash
# 1. release-gate / al-release-gate 仅剩 #717 4-piece self-doc 引用, 无活引用
grep -rn 'release-gate' --include='*.md' --include='*.go' --include='*.ts' --include='*.tsx' --include='*.yml' \
  | grep -v 'docs/_archive/' \
  | grep -v '\.claude/worktrees/'

# 2. dl1-no-direct-store 转 Go test 后仍守 production-only baseline=50
grep -lE '"borgee-server/internal/store"' packages/server-go/internal/api/*.go | grep -v _test.go | wc -l
# 应 ≤ 50

# 3. lint_constraints_test.go 8 case 跑通
cd packages/server-go && go test -tags sqlite_fts5 ./internal/api/ -run 'TestLint_|TestDL12_' -count=1 -timeout=60s
# 应 PASS
```

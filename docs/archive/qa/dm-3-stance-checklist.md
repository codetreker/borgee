# DM-3 立场反查清单 (战马D v0)

> 战马D · 2026-04-29 · ≤80 行 · 跟 spec brief `dm-3-spec.md` §0 立场 3 项配套
> 反查: 每项立场反向断言一句话锚 + 反约束 grep / 字面 / 行为级守门

---

## 1. 立场反查表 (5 项)

| # | 立场 | 反向断言 (反约束) | 守门 |
|---|---|---|---|
| ① | DM cursor 复用 RT-1.3 既有 mechanism, 不另起序列 | 反 grep `'/api/v1/dm/sync'\|'/dm/cursor'\|'dm-only.*backfill'` 在 `packages/server-go/internal/api/` count==0 — DM cursor 走 channel events 同 path, 不开旁路 endpoint | server unit `dm_3_1_no_sync_endpoint_test.go` 真跑 grep + 真 GET /api/v1/channels/{dmID}/messages?since=N 200 |
| ② | 多端同步走 RT-3 多端推, 不另起 dm-only frame | 反 envelope whitelist 加 `'dm_session_changed'\|'dm_multi_device_sync'\|'dm_synced'` count==0 (跟 BPP-1 #304 reflect lint 同源) | server unit + reflect 包扫白名单 13 frame 不变 |
| ③ | thinking subject 反约束延伸 (RT-3 #488 5-pattern byte-identical) | DM-3.1 server push frame body + DM-3.2 client useDMSync hook + DM-3.3 e2e UI 反向 grep `'thinking'\|'processing'\|'analyzing'\|'planning'\|'responding'` 在 system DM body / DOM 文案 count==0 | client vitest + e2e dom 双锁 |
| ④ | useDMSync hook 复用 useArtifactUpdated 模式, 不裂 dm-only WS subscription | 反 grep `'borgee:dm-sync'\|'dmSubscribe'\|'subscribeDM'` 在 packages/client/src/ count==0 — sessionStorage `dm:<id>:cursor` round-trip 跟 CV-1.3 last-seen-cursor 同 key 模式 | client vitest 5 case (cold-start / monotonic / page-reload / corrupt-clamp / multi-device) |
| ⑤ | server 0 行新增 (复用 RT-1.3 events backfill) | git diff `packages/server-go/internal/api/` 仅含新 _test.go (反约束 grep test), 0 行 production code 新增 | git diff line count + 反向 grep "dm-3.1 server new endpoint" 0 hit |

---

## 2. 跨 milestone byte-identical 锁

- ① cursor 跟 RT-1 #290 + AL-2b #481 + CV-* + BPP-3.1 #494 共一根 sequence
- ③ thinking 5-pattern 跟 RT-3 #488 byte-identical (改 = 改 5+ 处)
- ④ useDMSync hook 跟 CV-1.3 #346 useArtifactUpdated + LastSeenCursor 同模式 sessionStorage round-trip
- ⑤ server 0 行新增 跟 CM-5.2 立场 ① 同精神 (复用人协作 path)

---

## 3. 不在范围 (反约束)

- 不开 e2ee (跨 milestone 留 future Phase)
- 不开 dm 跨 org (依赖 AP-3, out-of-scope)
- 不开 dm-channel layout 排序同步 (CHN-3 已盖)
- 不开 offline DM 队列 (DL-4 web push 已盖)

---

## 4. 反约束 grep 清单 (CI lint hooks)

```bash
# A. 不开 dm-only endpoint (立场 ①)
grep -rn '"/api/v1/dm/sync"\|"/dm/cursor"' packages/server-go/internal/api/ --include='*.go'   # 0 hit

# B. 不开 dm-only frame (立场 ②)
grep -rn '"dm_session_changed"\|"dm_synced"\|"dm_multi_device_sync"' packages/server-go/internal/   # 0 hit

# C. thinking 5-pattern 文案不出现 system DM (立场 ③)
grep -rnE 'thinking|processing|analyzing|planning|responding' packages/server-go/internal/store/welcome.go packages/client/src/components/   # 仅 RT-3 既有锁文件命中, 不新增

# D. 不开 dm-only WS subscription (立场 ④)
grep -rn '"borgee:dm-sync"\|dmSubscribe\|subscribeDM' packages/client/src/   # 0 hit

# E. server 0 行新增 (立场 ⑤)
git diff origin/main -- packages/server-go/internal/api/ | grep -E '^\+' | grep -v '_test.go' | grep -v '^\+\+\+'   # 0 行 (除 test)
```

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 战马D | v0 — DM-3 立场反查 5 项 (cursor 复用 / 0 旁路 endpoint / 0 旁路 frame / thinking 5-pattern 延伸 / hook 复用 + server 0 行新增). 跨 milestone byte-identical 锁 5 源同根. CI lint hooks 5 grep 全 0 hit 守门. 不在范围 4 项 (e2ee / 跨 org / CHN-3 layout / offline DM). |

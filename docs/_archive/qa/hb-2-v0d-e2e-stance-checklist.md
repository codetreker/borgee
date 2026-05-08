# HB-2 v0(D) e2e stance checklist — landlock + 真 IO + sqlite consumer e2e 真验

> 7 立场 byte-identical 跟 hb-2-v0d-e2e-spec.md (飞马待 commit). **真有 prod code (HB-2 v0(D) e2e Playwright + Linux landlock 真启验 + 5 截屏 G4.x #4)** 但 0 server schema / 0 既有 endpoint shape 改. 跟 #599 HB stack Go + #605 + #606 v0(C) + HB-2 v0(D) stance (commit 366a4537) 4 步路径承袭. content-lock 不需 (复用 HB-2 stance content-lock 5 支柱字面 byte-identical).

## 1. e2e Playwright 真验 (Linux landlock + 真 IO + sqlite consumer)
- [ ] daemon 真启 + Linux landlock 真 sandbox 验 (`Apply` 真 syscall, 反 #606 v0(C) no-op stub)
- [ ] read_file / list_files 真 IO 验 (反 ACL 决策仅 stub)
- [ ] HB-3 host_grants SQLite consumer 真接 (替 MemoryConsumer, 撤销 < 100ms 真验 HB-4 第 5 行)

## 2. 5 截屏 G4.x #4 真归档 (PM 必修 #2 真兑现)
- [ ] HB-4 5 支柱状态页截屏 → `docs/qa/signoffs/g4-screenshots/g4-4-hb4-5-pillars.png`
- [ ] 情境授权弹窗截屏 → `g4-4-hb4-grants-prompt.png`
- [ ] 撤销后行为截屏 → `g4-4-hb4-revoke-after.png`
- [ ] 5 支柱字面 byte-identical 跟 HB-2 content-lock §1 (启动 < 800ms / 崩溃率 < 0.1% / 签名 / audit / 撤销 < 100ms)

## 3. HB-2 stance 7 立场 e2e 真兑现 (跟 commit 366a4537 锁)
- [ ] Go 重写 (反 Cargo/.rs/crate 真验 0 hit) + landlock Linux 真启 + 真 IO + sqlite consumer + 复用 HB-3 grants ACL + 0 server schema 改 + admin god-mode 不挂

## 4. cross-agent ACL 闸真验 ((agent_id, scope) 双键)
- [ ] e2e 反 cross-agent leak (agent_a 持 grant 不能给 agent_b 用)
- [ ] cross_agent_reject reason 字面 byte-identical 跟 HB-2 8-dict
- [ ] anchor #360 owner-only + REG-INV-002 fail-closed 立场承袭

## 5. 0 schema 改 + 0 既有 endpoint shape 改
- [ ] daemon = client side, server-go 0 行 production 改
- [ ] 反向 grep `migrations/hb_2_v0d_` 在 packages/server-go/ 0 hit
- [ ] 既有 endpoint + ACL byte-identical 不破

## 6. 8-dict reason byte-identical 跨 spec/code/REG/e2e 四处对锁
- [ ] HB-2 8-dict (`ok / path_outside_grants / grant_expired / grant_not_found / host_exceeds_max_bytes / egress_domain_not_whitelisted / cross_agent_reject / io_failed`) byte-identical
- [ ] 跟字典分立锁链第 8 处承袭 + 5-field audit schema 跨六源不破

## 7. admin god-mode 不挂 daemon (ADM-0 §1.3 + HB-2 spec §1.3 红线)
- [ ] 反向 grep `admin.*helper|admin.*host-bridge|admin.*daemon` 在 packages/borgee-helper/ 0 hit
- [ ] daemon 走用户机本地 OS user/group, 反 admin override

## 反约束 — 真不在范围
- ❌ mac sandbox-exec / Windows AppContainer 真启 (留 v1)
- ❌ 网络出站 outbound proxy / 文件 write 真启 (host-bridge 永久 read-only)
- ❌ 0 server schema / 0 endpoint shape / 0 既有 ACL 改
- ❌ admin god-mode 加挂 (永久不挂)

## 跨 milestone byte-identical 锁链 (5 链)
- HB stack 4 步路径 (#599+#605+#606+HB-2 v0(D) stance 366a4537) 真兑现 e2e
- HB-1 #589 manifest endpoint ed25519 对接 e2e 验
- HB-3 host_grants SQLite consumer (interface byte-id) 真接
- HB-4 release gate 5 支柱真验 (PM 必修 #2 G4.x #4 真兑现)
- anchor #360 owner-only ACL 22+ PRs

## PM 拆死决策 (3 段)
- **e2e 真验 vs unit only 拆死** — Playwright + Linux runner 真 syscall 验 (本 PR), 反 unit mock-only
- **landlock 真启 vs no-op stub 拆死** — Linux landlock 真 syscall (本 PR), 反 #606 v0(C) stub
- **嵌入 SQLite consumer vs 6 缓存模式漂拆死** — A 嵌入 read-only (本 PR 真兑现), 反缓存

## 用户主权红线 (5 项)
- ✅ host-bridge 永久 read-only / landlock 真启 / 撤销 < 100ms 真验
- ✅ cross-agent ACL fail-closed / admin 不挂 daemon
- ✅ 0 server schema / 0 endpoint shape / 0 既有 ACL 改

## PR 出来 5 核对疑点
1. e2e Playwright Linux runner 真启 + landlock syscall PASS
2. 5 截屏 G4.x #4 归档 docs/qa/signoffs/g4-screenshots/g4-4-hb4-*.png 3 张
3. cross-agent ACL e2e 真验 (agent_a 不能用 agent_b grant)
4. 8-dict + 5-field audit + 撤销 < 100ms HB-4 release-gate 5 支柱真验
5. cov ≥85% (#613 gate) + admin grep 0 hit

# admin-spa-ui-coverage stance checklist (≤80 行)

> 战马C · 2026-05-02 · 跟 spec brief 1:1 byte-identical

## 1. 立场单源 (5 立场)

- **立场 ①**: 0 server / 0 endpoint / 0 schema / 0 routes 改 — server 已挂 endpoint, 仅 client 接 UI
- **立场 ②**: CAPABILITY-DOT #628 14 const SSOT byte-identical — UserDetailPage 走 `lib/capabilities::CAPABILITY_TOKENS` 单源, 反 hardcode 字面散落
- **立场 ③**: ADMIN-SPA-SHAPE-FIX #633 D6 server 入口 IsValidCapability gate + 本 milestone client UI dropdown 走 SSOT — 客户端只能选 14 dot-notation, 反 admin cURL 蔓延
- **立场 ④**: admin god-mode 路径独立 (ADM-0 §1.3 红线) — UserDetailPage 仅访问 /admin-api/*, 不串 user-rail (/api/v1/) + 不 import user-rail lib/api
- **立场 ⑤**: data-asuc-* DOM 锚 byte-identical (跟 ADM-2-FOLLOWUP data-adm2-* + ADMIN-SPA-SHAPE-FIX 模式承袭) + 中文 UI 文案 byte-identical (改 = 改 content-lock + 此组件)

## 2. 反约束 (4 项)

- ❌ server endpoint 加 / shape 改 (本 milestone 仅 client UI)
- ❌ 14 capability hardcode 字面 in UserDetailPage (反 SSOT 漂)
- ❌ admin SPA 串 user-rail api (反 ADM-0 §1.3 红线)
- ❌ bundle-grant UI 重复实现 (AP-2 #620 user-rail BundleSelector 已有, admin-rail 不串)

## 3. 跨 milestone 锁链 (4 处)

- ADMIN-SPA-SHAPE-FIX #633 D6 server gate + 本 milestone client UI byte-identical 走 SSOT
- ADM-2-FOLLOWUP #626 AdminAuditLogPage data-adm2-* DOM 锚模式承袭
- AP-2 #620 user-rail PermissionsView 不破 (admin-rail 独立 UserDetailPage)
- ADM-0 §1.3 admin/user 路径分叉红线

## 4. PM 拆死决策 (3 段)

- **第一波 vs 第二波拆死** — 第一波核心 (users 详情页 capability + PATCH 3 字段); 第二波 B 类 (runtimes / heartbeat-lag / channels archived / description history) 留下个 milestone
- **server 加字段 vs 复用既有拆死** — 复用 PATCH /users/{id} 既有 5 字段 body, 反 reset_password 旁路 endpoint (admin 设 password 走 PATCH body)
- **admin SPA bundle UI vs 不重复拆死** — 不重复 AP-2 #620 user-rail BundleSelector (admin god-mode 不挂 bundle 入口)

## 5. 用户主权红线 (4 项)

- ✅ admin god-mode 不串 user session 数据
- ✅ admin SPA UI 改 0 server 行为
- ✅ capability grant 走 server post-#633 D6 IsValidCapability gate (admin cURL 也守, UI 也守, 双侧 SSOT)
- ✅ admin god-mode 永不挂 user-rail (ADM-0 §1.3 红线)

## 6. PR 出来 5 核对疑点

1. 0 server diff (`git diff origin/main -- packages/server-go/` 0 行)
2. 14 capability hardcode 0 hit in UserDetailPage (反 SSOT 漂)
3. admin god-mode 路径独立 (`fetch.*'/api/v1/'` + `from '../../lib/api'` 在 UserDetailPage 0 hit)
4. 9 DOM data-asuc-* 锚 ≥9 hit (vitest reverse-grep 守)
5. 既有 client vitest 全绿 + 1 新 file 7 case PASS

| 2026-05-02 | 战马C | v0 stance checklist — 5 立场 byte-identical 跟 spec brief, 4 反约束 + 4 跨链 + 3 拆死决策 + 4 红线 + 5 PR 核对疑点. 立场承袭 ADMIN-SPA-SHAPE-FIX #633 D6 + ADM-2-FOLLOWUP data-* + ADM-0 §1.3. |

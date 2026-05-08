# ADMIN-SPA-SHAPE-FIX content-lock v0.draft (≤40 行)

> 飞马/野马 · 2026-05-01 · v0.draft · 锚 spec brief §0..§4 + stance §1.1-§1.7 byte-identical
> 范围: 5 drift D1+D2 UI 段 (D3-D5 server-side data-only 不入). UI 真 bug 修, 截屏非强制.

## §1 LoginPage 文案 byte-identical (D1 跟随)

- `<label htmlFor="login">登录账号</label>` (中文字面 byte-identical, 不写 "Username" 英文 label)
- `<input id="login" name="login" placeholder="请输入登录账号" />` (`name="login"` 真接 server `loginRequest.Login`)
- `<button type="submit">登 录</button>` (反同义词 "Sign in / Log in / 進入")

## §2 AdminApp DOM data-attr SSOT (D2 跟随, zhanma-e 真值修订)

- `<div data-admin-session-login={session.login}>` byte-identical (替既有 `data-admin-session-username`) + `<div data-admin-session-id={session.id}>` (server handleMe `{id, login}` 真值)
- `<main data-page="admin-login">` (LoginPage) / `<main data-page="admin-app">` (AdminApp) 跟 ADM-2-FOLLOWUP #626 同模式
- **反假加** `data-admin-session-expires` / `data-admin-session-admin-id` (server 真值无, 反 audit 概念漂)

## §3 反约束 (黑名单 grep, 6 锚)

```bash
# 1) 反同义词禁词 (admin SPA UI DOM 字面 0 hit)
grep -rnE '"[Uu]sername"|>\s*Username\s*<|user_?[Nn]ame|userName' packages/client/src/admin/  # 0 hit
# 2) 反 data-admin-session-username 死字面
grep -rnE 'data-admin-session-username|data-admin-username' packages/client/src/admin/  # 0 hit
# 3) data-admin-session-login/id ≥2 hit (D2 真值修订: 反假加 expires/admin-id)
grep -rnE 'data-admin-session-(login|id)\b' packages/client/src/admin/  # ≥2 hit
grep -rnE 'data-admin-session-(expires|admin-id)' packages/client/src/admin/  # 0 hit (真值无)
# 4) LoginPage 中文文案锁 byte-identical (≥3 hit)
grep -nE '登录账号|请输入登录账号|登 录' packages/client/src/admin/pages/LoginPage.tsx  # ≥3 hit
# 5) input name="login" 真接 server SSOT (反 name="username" 漂)
grep -cE 'name="login"' packages/client/src/admin/pages/LoginPage.tsx  # ≥1; grep -cE 'name="username"' = 0
# 6) data-page anchor 跟 ADM-2-FOLLOWUP #626 同模式
grep -rnE 'data-page="admin-(login|app)"' packages/client/src/admin/  # ≥2 hit
```

## §4 demo 截屏 (非强制 — 真 bug 修, UI 跑通即可) + 跨 milestone 锁链

`docs/evidence/admin-spa-shape-fix/{login-page,admin-app-loaded}.png` 预备. ADM-2-FOLLOWUP #626 `data-page` 同模式 + ADM-0 §1.3 红线 + spec/stance byte-identical.

| 2026-05-01 | 飞马/野马 | v0.draft content-lock — LoginPage 3 中文字面 + AdminApp 4 data-attr SSOT + 6 黑名单 grep + 截屏非强制. 5 drift D1+D2 UI 段全锁. |

I have the full picture. Here's the task breakdown:

---

## COL-B04 Task Breakdown

Based on the design doc and current codebase, **the registration API and page already exist** (`auth.ts:register` route, `RegisterPage.tsx`, `api.register()`). The work is about **verifying completeness, fixing gaps, and adding missing pieces** per the design spec.

### T1: 后端注册 API 补全 & 加固

| 项目 | 详情 |
|------|------|
| **文件** | `packages/server/src/auth.ts` (~251行), `packages/server/src/queries.ts` (~858行) |
| **改动** | ~30-50 行 |
| **内容** | 1) 补全输入校验：email 格式校验、密码 8-72 字节（bcrypt 限制，多字节字符需按 byte 计算）、display_name 1-50 字符 2) 确认 email lowercase normalize 3) 确认 IMMEDIATE 事务包裹全部步骤（claim→create→perms→channels→JWT），JWT 失败需完整回滚（claim invite → check email → hash → create user → permissions → join channels → JWT） 4) Rate limiting: 注册端点 IP 级 10次/分 |
| **验证** | curl 测试各错误场景（400/404/409），并发邀请码 claim 测试，事务回滚测试（模拟下游失败确认无残留数据） |
| **依赖** | 无 |

### T2: 前端注册页面补全

| 项目 | 详情 |
|------|------|
| **文件** | `packages/client/src/components/RegisterPage.tsx` (~84行), `packages/client/src/App.tsx` (~194行) |
| **改动** | ~40-60 行 |
| **内容** | 1) 前端表单校验（email 格式、密码长度、显示名长度）2) Inline 错误提示在字段下方（目前可能只有全局 error）3) 确认已登录用户访问 /register 跳转到 / 4) 确认 "已有账号？登录" 链接正常 5) Loading 状态 UI |
| **验证** | 浏览器手动测试：各校验场景、成功注册后跳转、已登录访问 /register |
| **依赖** | T1（后端错误码需对齐） |

### T3: 注册后自动登录流程修复

| 项目 | 详情 |
|------|------|
| **文件** | `packages/client/src/components/RegisterPage.tsx`, `packages/client/src/lib/api.ts` (~460行) |
| **改动** | ~10-15 行 |
| **内容** | 当前 RegisterPage 注册后调 `register()` 再调 `login()`——但后端 register 已经 set cookie，不需要二次 login。去掉冗余 login 调用，直接 `onLogin()` |
| **验证** | 注册后确认只有一次 /register 请求，无额外 /login 请求，自动跳转主页 |
| **依赖** | T1 |

### T4: 路由保护逻辑确认

| 项目 | 详情 |
|------|------|
| **文件** | `packages/client/src/App.tsx` |
| **改动** | ~5-10 行（如有缺失） |
| **内容** | 确认：1) 未登录 → 显示 login/register 2) 已登录访问 register → 跳转主页 3) 已登录访问 login → 跳转主页（当前是 state-driven 无 URL 路由，需确认行为正确） |
| **验证** | 手动测试三种场景 |
| **依赖** | T2 |

### T5: E2E 集成验证

| 项目 | 详情 |
|------|------|
| **文件** | 无新文件（手动或脚本测试） |
| **改动** | 0 行 |
| **内容** | 完整流程：admin 生成邀请码 → 新用户注册 → 自动登录 → 在 #general 发消息 → 创建新频道 |
| **验证** | 1) 新用户出现在 #general 成员列表 2) 新用户有 channel.create + message.send 权限 3) 新用户能发消息和创建频道 |
| **依赖** | T1, T2, T3, T4 |

---

**执行顺序**: T1 → T2/T3 (可并行) → T4 → T5

**总预估改动**: ~85-135 行新增/修改。大部分是补全校验逻辑和修复冗余登录，核心注册流程已基本实现。

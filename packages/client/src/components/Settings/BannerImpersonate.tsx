// BannerImpersonate — ADM-2.2 顶部红横幅 (业主端).
//
// Blueprint: docs/blueprint/current/admin-model.md §1.4 红线 2 ("Impersonate 必须显眼,
// 红色横幅 + 倒计时") + ADM-1 §4.1 R3 第 2 条 ("24h 时窗顶部红色横幅常驻可
// 随时撤销") 兑现.
// Content lock: docs/qa/adm-2-content-lock.md §2 (字面 byte-identical).
// Spec: docs/implementation/modules/adm-2-spec.md §2.5 + §3 e2e ④.
//
// DOM 锚 (e2e/反查): `[data-banner="impersonate-active"]` 跟 ADM-1
// `data-row-kind` / CHN-3 `data-collapsed` 同模式 — e2e visibility lock.
//
// 立场反查:
//   - 立场 ⑦ impersonate 显眼: 横幅常驻 (无 dismiss 按钮, 蓝图 R3 字面);
//     倒计时 setInterval(1000) 重算 client 端 (反约束: server 不 push 第 5
//     RT-1 frame, 跟 CHN-4 立场 ⑥ 同精神 — 客户端轻量 polling 即可).
//   - admin_username 走 server 派生 (sanitizeImpersonateGrant 不返 raw
//     actor_id; 此组件接收 string admin_login 走 GET 响应; 反向 ADM2-NEG-001
//     不渲染 raw UUID).
import { useEffect, useState } from 'react';

interface ImpersonateGrant {
  id: string;
  user_id: string;
  granted_at: number;
  expires_at: number;
  revoked_at: number | null;
  // Server may attach admin_username when impersonate is currently in use
  // (admins.Login lookup); v1 we just show "support" prefix per content-lock §2
  // 字面 (admin 端 impersonate 真使用 grant 时, server 会 stamp 字段; 此 v1
  // 横幅不依赖该字段渲染 admin_username, 走通用 "support admin" 字面承袭蓝图
  // §1.4 row 2 "support 张三正在协助你, 剩 23h").
  admin_username?: string;
}

interface Props {
  // 注入的 fetch 钩子, 测试可 mock; 实际 client 走 lib/api.ts apiFetch.
  fetchGrant: () => Promise<ImpersonateGrant | null>;
  revokeGrant: () => Promise<void>;
}

function formatRemaining(ms: number): string {
  if (ms <= 0) return '已过期';
  const totalSec = Math.floor(ms / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  return `${h}h${m}m`;
}

export default function BannerImpersonate({ fetchGrant, revokeGrant }: Props) {
  const [grant, setGrant] = useState<ImpersonateGrant | null>(null);
  const [now, setNow] = useState(Date.now());

  // Initial fetch + 30s 轮询 (反约束: 不依赖 ws frame, 立场 ⑥).
  useEffect(() => {
    let cancelled = false;
    const reload = () => {
      void fetchGrant().then((g) => {
        if (!cancelled) setGrant(g);
      });
    };
    reload();
    const t = setInterval(reload, 30_000);
    return () => {
      cancelled = true;
      clearInterval(t);
    };
  }, [fetchGrant]);

  // 1s tick for countdown (倒计时刷新).
  useEffect(() => {
    if (!grant) return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [grant]);

  if (!grant || grant.revoked_at !== null) return null;
  if (grant.expires_at <= now) return null;

  const adminLabel = grant.admin_username ?? 'support';
  const remaining = formatRemaining(grant.expires_at - now);

  return (
    <div
      className="banner-impersonate"
      data-banner="impersonate-active"
      role="alert"
      aria-live="polite"
    >
      <span className="banner-impersonate-text">
        {/* content-lock §2 字面 byte-identical */}
        support {adminLabel} 正在协助你, 剩 {remaining}。
      </span>
      <button
        type="button"
        className="banner-impersonate-revoke"
        data-action="revoke-impersonate"
        onClick={() => {
          void revokeGrant().then(() => setGrant(null));
        }}
      >
        立即撤销
      </button>
    </div>
  );
}

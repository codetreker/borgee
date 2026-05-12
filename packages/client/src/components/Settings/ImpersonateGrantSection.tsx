// ImpersonateGrantSection — ADM-2.2 业主授权页子段 (acceptance §4.2.a).
//
// Blueprint: docs/blueprint/current/admin-model.md §1.4 constraint 2 second level (业主主动
// 授权 24h impersonate) + ADM-1 §4.1 R3 第 2 条 (24h 时窗 + 顶部红色
// 横幅常驻 + 可随时撤销).
// Content lock: docs/qa/adm-2-content-lock.md §3.
// Spec: docs/implementation/modules/adm-2-spec.md §2.5.
//
// DOM 锚: `[data-section="impersonate-grant"]` + `[data-action="grant-impersonate"]`.
// 设计 ⑦ 蓝图 §3 字面 "由 user 创建": 业主自己 grant; 24h 期限 server 固定.
import { useEffect, useState } from 'react';

interface ImpersonateGrant {
  id: string;
  user_id: string;
  granted_at: number;
  expires_at: number;
  revoked_at: number | null;
  admin_username?: string;
}

interface Props {
  fetchGrant: () => Promise<ImpersonateGrant | null>;
  createGrant: () => Promise<ImpersonateGrant>;
  revokeGrant: () => Promise<void>;
}

function formatRemaining(ms: number): string {
  if (ms <= 0) return '已过期';
  const totalSec = Math.floor(ms / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  return `${h}h${m}m`;
}

function formatGrantedAt(ms: number): string {
  const d = new Date(ms);
  const pad = (n: number) => n.toString().padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export default function ImpersonateGrantSection({ fetchGrant, createGrant, revokeGrant }: Props) {
  const [grant, setGrant] = useState<ImpersonateGrant | null>(null);
  const [now, setNow] = useState(Date.now());
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    fetchGrant().then((g) => {
      if (!cancelled) setGrant(g);
    });
    return () => {
      cancelled = true;
    };
  }, [fetchGrant]);

  // 1s tick 倒计时.
  useEffect(() => {
    if (!grant) return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [grant]);

  const isActive = grant !== null && grant.revoked_at === null && grant.expires_at > now;

  const handleGrant = () => {
    setBusy(true);
    setError(null);
    createGrant()
      .then((g) => setGrant(g))
      .catch((e) => setError(String(e)))
      .finally(() => setBusy(false));
  };

  const handleRevoke = () => {
    setBusy(true);
    setError(null);
    revokeGrant()
      .then(() => setGrant(null))
      .catch((e) => setError(String(e)))
      .finally(() => setBusy(false));
  };

  return (
    <section className="impersonate-grant" data-section="impersonate-grant">
      <h3>临时授权 admin 影响</h3>
      <p className="impersonate-grant-desc">
        授权后 24h 内, admin 可对你的账号执行 password 重置 / suspend / role
        调整等写动作; 24h 后自动失效。
      </p>

      <div className="impersonate-grant-status">
        {isActive ? (
          <>
            <span data-grant-state="active">
              当前状态: 已授权剩 {formatRemaining(grant!.expires_at - now)}
              {grant!.admin_username !== undefined ? ` (由 admin ${grant!.admin_username} 于 ${formatGrantedAt(grant!.granted_at)} 起算)` : ` (于 ${formatGrantedAt(grant!.granted_at)} 起算)`}
            </span>
            <button
              type="button"
              className="impersonate-revoke-btn"
              data-action="revoke-impersonate"
              onClick={handleRevoke}
              disabled={busy}
            >
              立即撤销
            </button>
          </>
        ) : (
          <>
            <span data-grant-state="inactive">当前状态: 未授权</span>
            <button
              type="button"
              className="impersonate-grant-btn"
              data-action="grant-impersonate"
              onClick={handleGrant}
              disabled={busy}
            >
              授权 (24h, 顶部会显示红色横幅常驻)
            </button>
          </>
        )}
      </div>

      {error !== null && (
        <p className="impersonate-grant-error" role="alert">
          {error.includes('grant_already_active')
            ? '已有未过期授权, 请先撤销当前授权或等待自动过期。'
            : `操作失败: ${error}`}
        </p>
      )}
    </section>
  );
}

// AdminActionsList — ADM-2.2 用户设置页 → "影响记录" 子段 (acceptance §4.1.c).
//
// Blueprint: docs/blueprint/current/admin-model.md §1.4 第四档 (普通 user 只见
// 与自己相关的 audit 条目).
// Content lock: docs/qa/adm-2-content-lock.md §4 (audit 列表字面锁 + 空态).
// Spec: docs/implementation/modules/adm-2-spec.md §2.2.
//
// DOM 锚: `[data-section="admin-actions-history"]` + 每行 `[data-action-row]`.
// 设计 ④ user 只见自己 — 走 GET /api/v1/me/admin-actions, 反向 ?target_user_id
// 参数 server 端被忽略.
import { useEffect, useState } from 'react';

interface AdminActionRow {
  id: string;
  target_user_id: string;
  action: string;
  metadata: string;
  created_at: number;
}

interface Props {
  fetchActions: () => Promise<AdminActionRow[]>;
}

// 5 action verb 中文字面跟 content-lock §4 同源 (跨端拆死: client 中文,
// admin SPA 走英文 enum, 字面承袭 stance §2 ADM2-NEG-010).
const ACTION_VERBS: Record<string, string> = {
  delete_channel: '删除了你的 channel',
  suspend_user: '暂停了你的账号',
  change_role: '调整了你的账号角色',
  reset_password: '重置了你的登录密码',
  start_impersonation: '开启了对你账号的 24h impersonate',
};

function formatTs(ms: number): string {
  const d = new Date(ms);
  // YYYY-MM-DD HH:MM 跟 server side time.Format("2006-01-02 15:04") 字面同源.
  const pad = (n: number) => n.toString().padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export default function AdminActionsList({ fetchActions }: Props) {
  const [rows, setRows] = useState<AdminActionRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetchActions()
      .then((data) => {
        if (!cancelled) setRows(data);
      })
      .catch((e) => {
        if (!cancelled) setError(String(e));
      });
    return () => {
      cancelled = true;
    };
  }, [fetchActions]);

  if (error !== null) {
    return (
      <section className="admin-actions-history" data-section="admin-actions-history">
        <h3>admin 对你的影响记录</h3>
        <p className="admin-actions-error">加载失败: {error}</p>
      </section>
    );
  }

  if (rows === null) {
    return (
      <section className="admin-actions-history" data-section="admin-actions-history">
        <h3>admin 对你的影响记录</h3>
        <p>正在加载...</p>
      </section>
    );
  }

  if (rows.length === 0) {
    // 空态字面 byte-identical 跟 content-lock §4 同源.
    return (
      <section className="admin-actions-history" data-section="admin-actions-history">
        <h3>admin 对你的影响记录</h3>
        <p className="admin-actions-empty">从未被 admin 影响过 — 你的隐私边界完整。</p>
      </section>
    );
  }

  return (
    <section className="admin-actions-history" data-section="admin-actions-history">
      <h3>admin 对你的影响记录 (最近 50 条)</h3>
      <table className="admin-actions-table">
        <thead>
          <tr>
            <th scope="col">时间</th>
            <th scope="col">做了什么</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.id} data-action-row data-action={row.action}>
              <td>{formatTs(row.created_at)}</td>
              <td>{ACTION_VERBS[row.action] ?? row.action}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

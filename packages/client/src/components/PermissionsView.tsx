// AP-2 client — PermissionsView component (capability 透明 UI 无角色名).
//
// 设计沿用 (ap-2-spec.md §0 + content-lock §1+§2):
//   - 走 capability token 字面渲染 (反 RBAC 角色名漂入 UI)
//   - capabilityLabel SSOT 单源 (反 inline 字面散落)
//   - DOM data-attr SSOT: data-ap2-capability-row + data-ap2-capability-token
//     + data-ap2-scope (按 content-lock §2)
//   - 反 typing-indicator / thought-process 5-pattern 漂入 (跟 RT-3 #616 承袭)
import { useEffect, useState } from 'react';
import { capabilityLabel, isKnownCapability } from '../lib/capabilities';
import type { PermissionEntry } from '../hooks/usePermissions';

export interface PermissionsViewProps {
  /** Optional injection — caller may pre-fetch and pass; else hook fetches. */
  entries?: PermissionEntry[];
  /** Override fetch path for tests. */
  fetchPath?: string;
}

interface MePermissionsResponse {
  user_id: string;
  // role: kept for legacy callers; AP-2 设计 ② UI 不显此字段 (反 role bleed).
  role?: string;
  permissions: string[];
  details: PermissionEntry[];
  // AP-2 设计 ② capability 数组 (server 新加, 跟 14 const SSOT byte-identical).
  capabilities?: string[];
}

async function fetchPermissions(path: string): Promise<MePermissionsResponse> {
  const res = await fetch(path, { credentials: 'include' });
  if (!res.ok) {
    throw new Error(`fetch ${path}: ${res.status}`);
  }
  return (await res.json()) as MePermissionsResponse;
}

/**
 * PermissionsView — capability 透明 UI for any user (member / agent).
 * 不显角色名 (反 RBAC bleed); 列出已授权 capability token + scope (字面).
 * 未知 token forward-compat 渲染原 token (反 silent drop).
 */
export function PermissionsView({ entries, fetchPath = '/api/v1/me/permissions' }: PermissionsViewProps) {
  const [resolved, setResolved] = useState<PermissionEntry[] | null>(entries ?? null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (entries) {
      setResolved(entries);
      return;
    }
    let cancelled = false;
    fetchPermissions(fetchPath)
      .then((data) => {
        if (cancelled) return;
        setResolved(data.details ?? []);
      })
      .catch((e) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : String(e));
      });
    return () => {
      cancelled = true;
    };
  }, [entries, fetchPath]);

  if (error) {
    return (
      <div data-ap2-error="true" role="alert">
        加载失败
      </div>
    );
  }
  if (!resolved) {
    return <div data-ap2-loading="true">加载中</div>;
  }
  if (resolved.length === 0) {
    return <div data-ap2-empty="true">暂无授权</div>;
  }

  return (
    <ul data-ap2-permissions-view="true">
      {resolved.map((entry) => {
        const token = entry.permission;
        const label = token === '*' ? '完整能力' : capabilityLabel(token);
        const known = token === '*' || isKnownCapability(token);
        return (
          <li
            key={`${entry.id}-${entry.permission}-${entry.scope}`}
            data-ap2-capability-row="true"
            data-ap2-capability-token={token}
            data-ap2-scope={entry.scope}
            data-ap2-known={known ? 'true' : 'false'}
          >
            <span data-ap2-capability-label>{label}</span>
            <span data-ap2-capability-scope>{entry.scope}</span>
          </li>
        );
      })}
    </ul>
  );
}

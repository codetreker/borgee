// AP-2 client — PermissionsView component (capability-based UI without role names).
//
// 设计沿用 (ap-2-spec.md §0 + content-lock §1+§2):
//   - 走 capability token 字面渲染 (RBAC role labels must not drift into UI)
//   - capabilityLabel SSOT 单源 (avoid duplicated inline labels)
//   - DOM data-attr SSOT: data-ap2-capability-row + data-ap2-capability-token
//     + data-ap2-scope (按 content-lock §2)
//   - typing-indicator / thought-process 5-pattern wording must not drift in (跟 RT-3 #616 承袭)
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
  // role: kept for legacy callers; AP-2 设计 ② UI 不显此字段 (avoid role bleed).
  role?: string;
  permissions: string[];
  details: PermissionEntry[];
  // AP-2 设计 ② capability 数组 (server 新加, shared with the capability constants).
  capabilities?: string[];
}

class PermissionsFetchError extends Error {
  constructor(public status: number, path: string) {
    super(`fetch ${path}: ${status}`);
    this.name = 'PermissionsFetchError';
  }
}

async function fetchPermissions(path: string): Promise<MePermissionsResponse> {
  const res = await fetch(path, { credentials: 'include' });
  if (!res.ok) {
    throw new PermissionsFetchError(res.status, path);
  }
  return (await res.json()) as MePermissionsResponse;
}

/**
 * PermissionsView — capability-based UI for any user (member / agent).
 * 不显角色名 (avoid RBAC bleed); 列出已授权 capability token + scope (字面).
 * 未知 token forward-compat 渲染原 token (avoid silent drop).
 */
export function PermissionsView({ entries, fetchPath = '/api/v1/me/permissions' }: PermissionsViewProps) {
  const [resolved, setResolved] = useState<PermissionEntry[] | null>(entries ?? null);
  const [error, setError] = useState<string | null>(null);
  const [forbidden, setForbidden] = useState(false);

  useEffect(() => {
    if (entries) {
      setResolved(entries);
      setError(null);
      setForbidden(false);
      return;
    }
    let cancelled = false;
    setResolved(null);
    setError(null);
    setForbidden(false);
    fetchPermissions(fetchPath)
      .then((data) => {
        if (cancelled) return;
        setResolved(data.details ?? []);
      })
      .catch((e) => {
        if (cancelled) return;
        if (e instanceof PermissionsFetchError && (e.status === 401 || e.status === 403)) {
          setResolved(null);
          setForbidden(true);
          return;
        }
        setError(e instanceof Error ? e.message : String(e));
      });
    return () => {
      cancelled = true;
    };
  }, [entries, fetchPath]);

  if (forbidden) {
    return (
      <div data-ap2-forbidden="true" role="alert">
        无权查看授权
      </div>
    );
  }
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

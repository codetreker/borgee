// PrivacyPromise — ADM-1 用户侧隐私承诺组件 (Phase 4 启动 milestone).
//
// Blueprint: docs/blueprint/current/admin-model.md §4.1 (3 条承诺锁 + 8 行 ✅/❌ 表格)
// Spec: docs/qa/adm-1-implementation-spec.md §1-§5 (1 组件 + 1 页面 + 5 反向断言)
// Acceptance: docs/qa/acceptance-templates/adm-1.md §1/§2/§3 (11 验收项)
// Review checklist: docs/qa/adm-1-privacy-promise-checklist.md (#211/#228 spec)
//
// 设计反查 (admin-model.md §0):
//   - "强权但不窥视" — admin 是平台运维, 不是协作者
//   - admin 看元数据不看正文 (DM body / artifact / API key 全 ❌)
//   - impersonate 是临时 amber 态 (24h 红色横幅常驻, 可撤销)
//
// 反约束:
//   - 默认展开不可折叠 (R3 requirement: do not wrap in a details element; spec §4 第 2 项)
//   - 三色锁 byte-identical (gray / #d33 红 / #d97706 amber, 不开第 4 色)
//   - 文案 1:1 跟 admin-model §4.1 + spec §2 同源 (drift test 双声明锁)
//   - grep 检查 折叠 / 展开收起 同义词 0 hit (acceptance §2.3)
import { renderMarkdown } from '../../lib/markdown';

/**
 * PRIVACY_PROMISES — admin-model.md §4.1 R3 三条承诺字面 1:1.
 * drift test 反查 doc-as-truth: 任何字面修改必同步改 docs/blueprint/current/admin-model.md
 * (跟 CM-onboarding TestWelcomeConstantsMirrorMigrations 同模式).
 */
export const PRIVACY_PROMISES = [
  '**Admin 是平台运维, 不是协作者** — 永不出现在 channel / DM / 团队列表里。',
  '**Admin 看不到消息 / 文件 / artifact 内容** — 除非你主动授权 impersonate (24h 时窗, 顶部红色横幅常驻, 可随时撤销)。',
  '**Admin 能看的是元数据** (用户名 / channel 名 / 条数 / 登录时间), **看不到正文**。',
] as const;

/**
 * PRIVACY_TABLE_ROWS — spec §3 八行 ✅/❌/✅(临时) 表格 byte-identical.
 * 三色锁: allow=gray default / deny=#d33 加粗 / impersonate=#d97706 amber.
 * 顺序不变 (acceptance §1 "顺序不变").
 */
export const PRIVACY_TABLE_ROWS = [
  { category: '用户名 / 邮箱', mark: '✅', kind: 'allow' },
  { category: 'channel 名 / 列表', mark: '✅', kind: 'allow' },
  { category: '消息条数 / 登录时间', mark: '✅', kind: 'allow' },
  { category: '消息正文 (channel / DM)', mark: '❌', kind: 'deny' },
  { category: 'artifact / 文件内容', mark: '❌', kind: 'deny' },
  { category: '你和 owner-agent 内置 DM', mark: '❌', kind: 'deny' },
  { category: 'API key 原值', mark: '❌', kind: 'deny' },
  { category: '授权 impersonate 后 24h 实时入站', mark: '✅ (临时)', kind: 'impersonate' },
] as const;

export type PrivacyRowKind = 'allow' | 'deny' | 'impersonate';

export default function PrivacyPromise() {
  return (
    <section className="privacy-promise" data-section="privacy-promise">
      <h2 className="privacy-promise-title">隐私承诺</h2>

      {/* 设计 §4.1 — 三条承诺字面 1:1 (drift test 锁); 默认展开不可折叠
          (R3 spec §4 第 2 项: do not wrap in a details element). */}
      <ol className="privacy-promise-list">
        {PRIVACY_PROMISES.map((promise, i) => (
          <li
            key={i}
            className="privacy-promise-item"
            // marked + DOMPurify 渲染 **bold** (跟 system message bubble
            // 同 stack, 设计 ④ Markdown ONLY 同源).
            dangerouslySetInnerHTML={{ __html: renderMarkdown(promise) }}
          />
        ))}
      </ol>

      {/* spec §3 八行 ✅/❌ 表格 — 三色锁 byte-identical */}
      <table className="privacy-promise-table">
        <thead>
          <tr>
            <th scope="col">类别</th>
            <th scope="col">admin 可见?</th>
          </tr>
        </thead>
        <tbody>
          {PRIVACY_TABLE_ROWS.map((row, i) => (
            <tr
              key={i}
              className={`privacy-row-${row.kind}`}
              data-row-kind={row.kind}
            >
              <td>{row.category}</td>
              <td className="privacy-row-mark">{row.mark}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

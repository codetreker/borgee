// AgentConfigPanel.tsx — AL-2a.3 client SPA agent config SSOT editor.
//
// Acceptance: docs/qa/acceptance-templates/al-2a.md (#264 §4.1.a-d).
// Blueprint: agent-lifecycle.md §2.1 (用户完全自主决定 agent 的
// name/prompt/能力/model) + plugin-protocol.md §1.4 (Borgee=SSOT 字段
// 划界) + §1.5 (热更新分级 — AL-2a 走轮询 reload, BPP frame
// agent_config_update 留 AL-2b + BPP-3 同合).
//
// What this renders:
//   - Form 编辑 blob 白名单字段 (name / avatar / prompt / model /
//     capabilities / enabled / memory_ref) — 跟 server allowedConfigKeys
//     同源 byte-identical (蓝图 §1.4 SSOT 字段划界).
//   - schema_version 显示 (server-stamp monotonic, agent 端轮询 reload
//     drift 锚 — acceptance §4.1.d).
//   - 失败 toast 文案锁 byte-identical: "agent 配置保存失败, 请重试"
//     (跟 server-go agent_config.go agentConfigSaveErrorMsg + AL-2a
//     content-lock al-2a-content-lock.md ① 同源).
//   - SSOT blob 整体替换语义 — Save 按钮提交全 form 为 PATCH (model 字段
//     不写入则消失, 跟 server TestAL2A2_PatchAndGet 显式断言对齐).
//
// 反约束:
//   - 立场 ⑤ (蓝图 §1.4): runtime-only 字段 (api_key / temperature /
//     token_limit / retry_policy) 不入此 form (server fail-closed reject,
//     此 UI 也不渲染对应输入框 — UI 层 + server 层双层 fail-closed).
//   - 立场 ⑥ (蓝图 §1.5): 此组件不订阅 ws push (反向 grep 锚: 无 ws hub
//     subscription 调用,
//     reload 走轮询 — onMount + Save 后 re-fetch).

import { useEffect, useState } from 'react';
import {
  fetchAgentConfig,
  updateAgentConfig,
  type AgentConfig,
  type AgentConfigBlob,
} from '../lib/api';
import { useUnsavedChangesGuard } from '../hooks/useUnsavedChangesGuard';

// 失败 toast 文案锁 — 跟 server-go agent_config.go const
// agentConfigSaveErrorMsg byte-identical 同源 (al-2a-content-lock.md ①).
// 改此字面 = 改 server const + acceptance §4.1.d follow-up.
export const AGENT_CONFIG_SAVE_TOAST = 'agent 配置保存失败, 请重试';

// allowedConfigKeys whitelist — 跟 server-go internal/api/agent_config.go
// allowedConfigKeys map 同源 byte-identical (蓝图 §1.4 字段划界).
// 改此 list = 改 server map + acceptance §4.1.c reflect scan.
export const ALLOWED_CONFIG_KEYS: ReadonlyArray<keyof AgentConfigBlob> = [
  'name',
  'avatar',
  'prompt',
  'model',
  'capabilities',
  'enabled',
  'memory_ref',
];

interface AgentConfigPanelProps {
  agentId: string;
  onError?: (msg: string) => void;
}

export function AgentConfigPanel({ agentId, onError }: AgentConfigPanelProps) {
  const [config, setConfig] = useState<AgentConfig | null>(null);
  const [draft, setDraft] = useState<AgentConfigBlob>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  // 立场 ⑥ 轮询 reload — 加载时 GET, Save 后 re-GET (无 ws subscription).
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    fetchAgentConfig(agentId)
      .then(c => {
        if (cancelled) return;
        setConfig(c);
        setDraft(c.blob);
      })
      .catch(() => {
        if (cancelled) return;
        if (onError) onError(AGENT_CONFIG_SAVE_TOAST);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [agentId, onError]);

  // gh#703 — sidepane 切换守卫. agent 配置 form 是 SSOT blob 整体编辑,
  // isDirty 按 JSON.stringify draft vs config.blob 推算 (反 deep equal 引
  // 第三方 + 反字段逐个比). 加载中或没 config 时不算 dirty (避免初始 render
  // 误触发). saving 中也不算 dirty (用户已经点了保存等服务器, 不要再弹一次).
  useUnsavedChangesGuard(
    () =>
      !loading &&
      !saving &&
      config !== null &&
      JSON.stringify(draft) !== JSON.stringify(config.blob),
    'Agent 配置有未保存改动, 离开会丢失. 确认离开吗?',
  );

  async function handleSave() {
    setSaving(true);
    try {
      // SSOT blob 整体替换 — 提交完整 draft (即便字段为空也覆盖, 跟
      // server TestAL2A2_PatchAndGet "blob 整体替换 model 消失" 同语义).
      const updated = await updateAgentConfig(agentId, draft);
      setConfig(updated);
      setDraft(updated.blob);
    } catch {
      // 失败 toast byte-identical "agent 配置保存失败, 请重试" — 跟
      // server const agentConfigSaveErrorMsg + content-lock ① 同源.
      if (onError) onError(AGENT_CONFIG_SAVE_TOAST);
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return <div data-agent-config="loading">加载中...</div>;
  }

  return (
    <section data-agent-config="root" data-schema-version={config?.schema_version ?? 0}>
      <header>
        <h3>Agent 配置</h3>
        <span data-agent-config-version aria-label="schema version">
          v{config?.schema_version ?? 0}
        </span>
      </header>

      {/* gh#698: 6 个 label 加内联 style 防 800px 父容器下排版重叠.
          5 个 text/textarea label 用 display:block (跟 CreateAgentModal
          AgentManager.tsx L414 Agent ID label 同款), 1 个 checkbox label
          用 display:flex inline (跟 AgentManager.tsx L430 KNOWN_PERMISSIONS
          块 inline checkbox 同款, yema review 拍 b inline). 反约束: 不引入
          新 CSS class (反 pre-mature 抽象), 跟项目"按需重构"原则一致;
          后续 ≥3 form 同模式时再抽 .form-stack/.field-block 公共类
          (留 followup issue, p3-low). */}
      <label style={{ display: 'block', marginTop: 8 }}>
        名称
        <input
          type="text"
          data-agent-config-field="name"
          value={draft.name ?? ''}
          onChange={e => setDraft({ ...draft, name: e.target.value })}
        />
      </label>

      <label style={{ display: 'block', marginTop: 8 }}>
        头像 URL
        <input
          type="text"
          data-agent-config-field="avatar"
          value={draft.avatar ?? ''}
          onChange={e => setDraft({ ...draft, avatar: e.target.value })}
        />
      </label>

      <label style={{ display: 'block', marginTop: 8 }}>
        Prompt
        <textarea
          data-agent-config-field="prompt"
          value={draft.prompt ?? ''}
          onChange={e => setDraft({ ...draft, prompt: e.target.value })}
          rows={8}
          style={{ resize: 'vertical', width: '100%', boxSizing: 'border-box' }}
        />
      </label>

      <label style={{ display: 'block', marginTop: 8 }}>
        模型
        <input
          type="text"
          data-agent-config-field="model"
          value={draft.model ?? ''}
          onChange={e => setDraft({ ...draft, model: e.target.value })}
        />
      </label>

      <label style={{ display: 'block', marginTop: 8 }}>
        memory_ref
        <input
          type="text"
          data-agent-config-field="memory_ref"
          value={draft.memory_ref ?? ''}
          onChange={e => setDraft({ ...draft, memory_ref: e.target.value })}
        />
      </label>

      {/* checkbox 例外行 (yema review #1 拍 b inline): text 节点放在
          input 后面 (inline 视觉顺序: 先 ☐ 后 "启用"), 跟 CreateAgentModal
          Permissions 块同款. */}
      <label style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 8 }}>
        <input
          type="checkbox"
          data-agent-config-field="enabled"
          checked={draft.enabled ?? false}
          onChange={e => setDraft({ ...draft, enabled: e.target.checked })}
        />
        启用
      </label>

      <button
        type="button"
        data-agent-config-action="save"
        disabled={saving}
        onClick={handleSave}
      >
        {saving ? '保存中...' : '保存'}
      </button>
    </section>
  );
}

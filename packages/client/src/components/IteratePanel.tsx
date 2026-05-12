// IteratePanel — CV-4.3 client iterate UI (#409 server / #405 schema).
//
// Spec: docs/implementation/modules/cv-4-spec.md §0 设计 ②
//   (owner triggers iterate / agent commit goes through CV-1 single source)
//   + §1 CV-4.3.
// 文案锁: docs/qa/cv-4-content-lock.md §1 ①②③⑥⑦.
// Stance: docs/qa/cv-4-stance-checklist.md §1 ②④⑤⑥.
// Acceptance: docs/qa/acceptance-templates/cv-4.md §3 (client) + §4 (e2e).
//
// 设计反查:
//   - ② intent textarea 协作语境 placeholder 锁 (蓝图 §1.5 "agent 是
//     同事"); agent picker 候选仅 channel member.kind='agent' 行 (反 admin /
//     人 行).
//   - ③ state 4 态 inline DOM `data-iteration-state` byte-identical
//     ('pending'/'running'/'completed'/'failed') + 文案锁 byte-identical:
//     pending → "等待 agent 开始…" + spinner
//     running → "agent 正在迭代…" + 进度条
//     completed → "已生成 v{N}" + 自动跳新版本
//     failed → "失败: {REASON_LABELS[reason]}"
//   - ⑤ iterate 进度仅 inline 此面板, 不进 messages 流 (域隔离永久锁,
//     #374/#378/#380 反约束承袭).
//   - ⑥ iterate 触发按钮 owner-only DOM omit (防御深度跟 #347 line 254
//     showRollbackBtn 同模式 — 由父组件 ArtifactPanel 负责 omit).
//   - ⑦ failed UI 仅显示 "失败: {reason_label}" + 不显示重试按钮
//     (blueprint negative constraint + spec #365 §4 — 失败重试 = owner 重新触发新 iteration,
//     retry-button synonyms must not appear in this UI).
//
// 反约束 (本组件强制 grep 锚, content-lock §2 一致):
//   - Prevent synonym drift (按钮锁定为 🔄, 文案锁中文 byte-identical;
//     英文/全名/同义词全部grep 检查 0 hit)
//   - state 4 态文案锁中文 byte-identical (反英文 4 态 + 模糊同义词)
//   - failed UI 不渲染重试按钮 (失败状态机锁死, 设计 ⑦)
//   - 不接 autoRetry / setTimeout POST iterate failed 隐式重试
//   - failed reason MUST 走 REASON_LABELS 三处单测锁 (#249 + AL-3 #305 + 此组件)
import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  ApiError,
  type ArtifactIteration,
  type ChannelMember,
  type IterationState,
  type AgentRuntimeReason,
  createIteration,
  fetchChannelMembers,
  listIterations,
} from '../lib/api';
import { REASON_LABELS } from '../lib/agent-state';
import { useIterationStateChanged } from '../hooks/useWsHubFrames';

interface Props {
  artifactId: string;
  channelId: string;
  /** Owner-only — 父组件 (ArtifactPanel) already controls rendering; 本面板再保险一层 (defense-in-depth). */
  isOwner: boolean;
  /** 当 iteration completed 时父组件 reload artifact + 跳新版本 view. */
  onIterationCompleted?: (newVersionId: number | null | undefined) => void;
}

/**
 * stateLabel — 4 态文案 byte-identical (content-lock §1 ③, 改 = 改三处).
 *
 * pending → "等待 agent 开始…"  (中文 + ellipsis)
 * running → "agent 正在迭代…"  (中文 + ellipsis)
 * completed → "已生成 v{N}"  (N = artifact_versions.id; iteration row
 *   created_artifact_version_id 是 FK PK 非用户号 version, 但展示给
 *   owner 仍按 PK 展开占位 — server 同步 emit ArtifactUpdated frame 让
 *   ArtifactPanel 再 reload 拿用户号 version)
 * failed → "失败: {REASON_LABELS[reason] ?? '未知错误'}"
 */
export function stateLabel(
  state: IterationState,
  ctx: { newVersionId?: number | null; reason?: AgentRuntimeReason | null },
): string {
  switch (state) {
    case 'pending':
      return '等待 agent 开始…';
    case 'running':
      return 'agent 正在迭代…';
    case 'completed':
      return `已生成 v${ctx.newVersionId ?? '?'}`;
    case 'failed': {
      const reason = ctx.reason;
      const label = reason ? (REASON_LABELS[reason] ?? '未知错误') : '未知错误';
      return `失败: ${label}`;
    }
  }
}

const INTENT_PREVIEW_MAX = 40;

function truncateIntent(s: string): string {
  if (s.length <= INTENT_PREVIEW_MAX) return s;
  return s.slice(0, INTENT_PREVIEW_MAX) + '…';
}

export default function IteratePanel({
  artifactId,
  channelId,
  isOwner,
  onIterationCompleted,
}: Props) {
  const [members, setMembers] = useState<ChannelMember[]>([]);
  const [iterations, setIterations] = useState<ArtifactIteration[]>([]);
  const [intent, setIntent] = useState('');
  const [targetAgentId, setTargetAgentId] = useState<string>('');
  const [busy, setBusy] = useState(false);
  const [errMsg, setErrMsg] = useState<string | null>(null);
  const [activeIteration, setActiveIteration] = useState<ArtifactIteration | null>(null);

  // agent picker 候选仅 channel member.kind='agent' 行 (反 admin / 人).
  const agentCandidates = useMemo(
    () => members.filter((m) => m.role === 'agent'),
    [members],
  );

  const reload = useCallback(async () => {
    try {
      const list = await listIterations(artifactId);
      setIterations(list.iterations);
      // active = 当前 pending/running 的最新; 没有则清空.
      const head = list.iterations.find((i) => i.state === 'pending' || i.state === 'running');
      setActiveIteration(head ?? null);
    } catch (err) {
      // CV-4.2 server 未 merged / endpoint 不在 → 静默, 让 UI 仍可触发.
      if (err instanceof ApiError && err.status === 404) {
        setIterations([]);
        setActiveIteration(null);
      }
    }
  }, [artifactId]);

  useEffect(() => {
    let cancelled = false;
    fetchChannelMembers(channelId)
      .then((list) => {
        if (cancelled) return;
        setMembers(list);
      })
      .catch(() => {
        if (cancelled) return;
        setMembers([]);
      });
    void reload();
    return () => {
      cancelled = true;
    };
  }, [channelId, reload]);

  // WS push hook — IterationStateChangedFrame 9 字段 byte-identical
  // 跟 server #409 envelope. envelope 仅信号, 不带 intent_text (设计 ⑦
  // 字段白名单反断), client 收到后必须 GET /iterations/:id 拉.
  const onFrame = useCallback(
    (frame: { artifact_id: string; iteration_id: string; state: IterationState }) => {
      if (frame.artifact_id !== artifactId) return;
      void reload().then(() => {
        if (frame.state === 'completed' && onIterationCompleted) {
          // 设计 ② commit 走 CV-1 既有路径 — ArtifactUpdated frame 同步
          // 触发 ArtifactPanel reload, 父组件回调让其跳新版本 view.
          // listIterations 返回的 row 含 created_artifact_version_id.
          listIterations(artifactId).then((list) => {
            const completed = list.iterations.find((i) => i.id === frame.iteration_id);
            onIterationCompleted(completed?.created_artifact_version_id ?? null);
          });
        }
      });
    },
    [artifactId, reload, onIterationCompleted],
  );
  useIterationStateChanged(onFrame);

  const handleTrigger = useCallback(async () => {
    if (!isOwner) return;
    if (!intent.trim() || !targetAgentId) return;
    setBusy(true);
    setErrMsg(null);
    try {
      await createIteration(artifactId, {
        intent_text: intent.trim(),
        target_agent_id: targetAgentId,
      });
      setIntent('');
      await reload();
    } catch (err) {
      setErrMsg(err instanceof Error ? err.message : '请求失败');
    } finally {
      setBusy(false);
    }
  }, [artifactId, intent, targetAgentId, isOwner, reload]);

  // 设计 ⑥ — 父组件已 omit non-owner; 此处再保险一层不渲染表单.
  if (!isOwner) return null;

  const headForRender = activeIteration;
  const showActiveBadge = !!headForRender;

  return (
    <div className="iterate-panel" data-section="iterate">
      <h4 className="iterate-title">请求 agent 迭代</h4>

      <textarea
        className="iterate-intent"
        placeholder="告诉 agent 你希望它做什么…"
        value={intent}
        onChange={(e) => setIntent(e.target.value)}
        rows={3}
        disabled={busy}
        spellCheck={false}
      />

      <div className="iterate-controls">
        <label className="iterate-agent-label">
          选择 agent
          <select
            className="iterate-agent-picker"
            value={targetAgentId}
            onChange={(e) => setTargetAgentId(e.target.value)}
            disabled={busy || agentCandidates.length === 0}
          >
            <option value="">{agentCandidates.length === 0 ? '此频道无 agent 成员' : '请选择'}</option>
            {agentCandidates.map((m) => (
              <option key={m.user_id} value={m.user_id} data-kind="agent">
                🤖 {m.display_name}
              </option>
            ))}
          </select>
        </label>

        <button
          type="button"
          className="iterate-btn iterate-trigger-btn"
          data-iteration-target-agent-id={targetAgentId}
          title="请求 agent 迭代"
          aria-label="请求 agent 迭代"
          disabled={busy || !intent.trim() || !targetAgentId}
          onClick={handleTrigger}
        >
          🔄
        </button>
      </div>

      {errMsg && <p className="iterate-err">{errMsg}</p>}

      {showActiveBadge && headForRender && (
        <div
          className="iteration-state"
          data-iteration-state={headForRender.state}
          aria-live="polite"
        >
          {headForRender.state === 'pending' && (
            <span className="iteration-spinner" aria-hidden="true" />
          )}
          {headForRender.state === 'running' && (
            <span className="iteration-progress" aria-hidden="true" />
          )}
          <span className="iteration-state-label">
            {stateLabel(headForRender.state, {
              newVersionId: headForRender.created_artifact_version_id,
              reason: headForRender.error_reason,
            })}
          </span>
          {/* 设计 ⑦ — failed 路径下不渲染重试按钮 同义词全无.
              owner 重新触发走 ① iterate 触发路径 (新 iteration_id, 不复用). */}
        </div>
      )}

      {iterations.length > 0 && (
        <div className="iteration-history" data-section="iteration-history">
          <h5 className="iteration-history-title">迭代历史</h5>
          <ul className="iteration-history-list">
            {iterations.slice(0, 5).map((it) => (
              <li
                key={it.id}
                className="iteration-history-row"
                data-iteration-state={it.state}
              >
                <span className="iteration-history-state-label">
                  {stateLabel(it.state, {
                    newVersionId: it.created_artifact_version_id,
                    reason: it.error_reason,
                  })}
                </span>
                <span className="iteration-history-intent" title={it.intent_text}>
                  {truncateIntent(it.intent_text)}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

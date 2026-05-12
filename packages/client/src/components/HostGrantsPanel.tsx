// HostGrantsPanel — HB-3.3 client SPA 弹窗 (蓝图 host-bridge.md §1.3
// 情境化授权 4 类). DOM text and attributes match content-lock §1.①+§1.②:
//   data-action="deny"            [拒绝]      hb3-button="danger"
//   data-action="grant_one_shot"  [仅这一次]  hb3-button="primary"
//   data-action="grant_always"    [始终允许]  hb3-button="primary"
//
// **改 = 改三处**: hb-3-content-lock.md §1.① + 此 component + spec
// brief §1 HB-3.3.
//
// Constraint: data-action ∈ {"deny", "grant_one_shot", "grant_always"} 三值;
// data-hb3-button ∈ {"danger", "primary"} 二值; tests cover enum外值
// (HostGrantsPanel.test.tsx 守).
import React from 'react';

export type HostGrantType = 'install' | 'exec' | 'filesystem' | 'network';

// actionLabel — content-lock §1.② 弹窗 title actionLabel exact text
// 跟蓝图 §1.3 弹窗 UX 字面 (改 = 改两处, 此 map + content-lock §1.②).
const actionLabel: Record<HostGrantType, string> = {
  install: '安装',
  exec: '执行',
  filesystem: '读取',
  network: '访问',
};

export interface HostGrantsPanelProps {
  agentName: string;
  grantType: HostGrantType;
  scopeLabel: string;
  capabilityLabel: string;
  onDecide: (action: 'deny' | 'grant_one_shot' | 'grant_always') => void;
}

export default function HostGrantsPanel(props: HostGrantsPanelProps): React.ReactElement {
  const { agentName, grantType, scopeLabel, capabilityLabel, onDecide } = props;
  const verb = actionLabel[grantType];
  // Title + body required text matches blueprint §1.3 popup UX (content-lock §1.②).
  const title = `${agentName} 想${verb}你的${scopeLabel}`;
  const body = `原因: ${agentName} 配置中的「${capabilityLabel}」能力\n      需要${verb}${scopeLabel}`;

  return (
    <div data-section="host-grants-panel" role="dialog" aria-label={title}>
      <h3 data-hb3-title>{title}</h3>
      <p data-hb3-body style={{ whiteSpace: 'pre-line' }}>{body}</p>
      <div data-hb3-buttons>
        <button
          data-action="deny"
          data-hb3-button="danger"
          onClick={() => onDecide('deny')}
        >
          拒绝
        </button>
        <button
          data-action="grant_one_shot"
          data-hb3-button="primary"
          onClick={() => onDecide('grant_one_shot')}
        >
          仅这一次
        </button>
        <button
          data-action="grant_always"
          data-hb3-button="primary"
          onClick={() => onDecide('grant_always')}
        >
          始终允许
        </button>
      </div>
    </div>
  );
}
